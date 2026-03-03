// Copyright 2026 Scalytics, Inc.
// Copyright 2026 Mirko Kämpf
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/scalytics/kafgraph/internal/brain"
	"github.com/scalytics/kafgraph/internal/config"
	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/ingest"
	"github.com/scalytics/kafgraph/internal/query"
	reflectpkg "github.com/scalytics/kafgraph/internal/reflect"
	"github.com/scalytics/kafgraph/internal/search"
	"github.com/scalytics/kafgraph/internal/server"
	"github.com/scalytics/kafgraph/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log.Printf("KafGraph %s (commit: %s, built: %s)",
		config.Version, config.GitCommit, config.BuildDate)

	// 1. Open BadgerDB storage
	store, err := storage.NewBadgerStorage(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer func() { _ = store.Close() }()

	// 2. Create graph engine
	g := graph.New(store)
	defer func() { _ = g.Close() }()

	// 3. Create search engines
	blevePath := filepath.Join(cfg.DataDir, "bleve.idx")
	ft, err := search.NewBleveSearcher(blevePath, search.DefaultIndexedFields())
	if err != nil {
		log.Printf("warning: full-text search disabled: %v", err)
		ft = nil
	}
	if ft != nil {
		defer func() { _ = ft.Close() }()
	}

	vs := search.NewBruteForceVectorSearcher(store.DB())

	// 4. Create query executor
	exec := query.NewExecutor(g, ft, vs)

	// 5. Create brain tool service
	bs := brain.NewService(g, ft, vs)

	// 5b. Create reflection runner and inject into brain service
	cycleRunner := reflectpkg.NewCycleRunner(g)
	adapter := reflectpkg.NewBrainAdapter(cycleRunner)
	bs.SetReflectionRunner(adapter)

	// 6. Create servers
	httpAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	boltAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.BoltPort)

	httpSrv := server.NewHTTPServer(httpAddr, g, exec, server.WithBrain(bs))

	boltSrv, err := server.NewBoltServer(boltAddr, exec)
	if err != nil {
		return fmt.Errorf("start bolt server: %w", err)
	}

	// 7. Start servers in goroutines
	errCh := make(chan error, 4)
	go func() { errCh <- httpSrv.Serve() }()
	go func() { errCh <- boltSrv.Serve(ctx) }()

	log.Printf("listening on HTTP %s, Bolt %s", httpAddr, boltAddr)

	// 7b. Start ingest processor (if enabled)
	if cfg.Ingest.Enabled {
		pollInterval, err := time.ParseDuration(cfg.Ingest.PollInterval)
		if err != nil {
			return fmt.Errorf("parse ingest poll_interval: %w", err)
		}
		reader := ingest.NewMemoryReader() // placeholder until S3 reader
		proc := ingest.NewProcessor(reader, g, ingest.ProcessorConfig{
			PollInterval: pollInterval,
			BatchSize:    cfg.Ingest.BatchSize,
			Namespace:    cfg.Ingest.Namespace,
		})
		go func() { errCh <- proc.Run(ctx) }()
		log.Printf("ingest processor started (poll=%s, batch=%d)", pollInterval, cfg.Ingest.BatchSize)
	}

	// 7c. Start reflection scheduler (if enabled)
	if cfg.Reflect.Enabled {
		checkInterval, err := time.ParseDuration(cfg.Reflect.CheckInterval)
		if err != nil {
			return fmt.Errorf("parse reflect check_interval: %w", err)
		}
		gracePeriod, err := time.ParseDuration(cfg.Reflect.FeedbackGracePeriod)
		if err != nil {
			return fmt.Errorf("parse reflect feedback_grace_period: %w", err)
		}
		daily, err := reflectpkg.ParseSchedule(cfg.Reflect.DailyTime, "", 0)
		if err != nil {
			return fmt.Errorf("parse reflect daily schedule: %w", err)
		}
		weekly, err := reflectpkg.ParseSchedule(cfg.Reflect.WeeklyTime, cfg.Reflect.WeeklyDay, 0)
		if err != nil {
			return fmt.Errorf("parse reflect weekly schedule: %w", err)
		}
		monthly, err := reflectpkg.ParseSchedule(cfg.Reflect.MonthlyTime, "", cfg.Reflect.MonthlyDay)
		if err != nil {
			return fmt.Errorf("parse reflect monthly schedule: %w", err)
		}

		// Phase 6: wire feedback publisher (MemoryPublisher for now, Kafka in Phase 7/8)
		pub := ingest.NewMemoryPublisher()
		sched := reflectpkg.NewScheduler(g, reflectpkg.SchedulerConfig{
			CheckInterval: checkInterval,
			Daily:         daily,
			Weekly:        weekly,
			Monthly:       monthly,
			GracePeriod:   gracePeriod,
			Publisher:     pub,
			RequestTopic:  cfg.Reflect.FeedbackRequestTopic,
			TopN:          cfg.Reflect.FeedbackTopN,
		})
		go func() { errCh <- sched.Run(ctx) }()
		log.Printf("reflection scheduler started (check=%s, feedback_topic=%s)",
			checkInterval, cfg.Reflect.FeedbackRequestTopic)
	}

	// 8. Wait for signal or error
	select {
	case <-ctx.Done():
		log.Println("shutting down...")
	case err := <-errCh:
		log.Printf("server error: %v", err)
	}

	// 9. Graceful shutdown (5s timeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	_ = boltSrv.Close()

	return nil
}
