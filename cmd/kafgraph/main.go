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
	"syscall"
	"time"

	"github.com/scalytics/kafgraph/internal/config"
	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/ingest"
	"github.com/scalytics/kafgraph/internal/server"
	"github.com/scalytics/kafgraph/internal/storage"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
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

	// 3. Create servers
	httpAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	boltAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.BoltPort)

	httpSrv := server.NewHTTPServer(httpAddr, g)

	boltSrv, err := server.NewBoltServer(boltAddr)
	if err != nil {
		return fmt.Errorf("start bolt server: %w", err)
	}

	// 4. Graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 5. Start servers in goroutines
	errCh := make(chan error, 3)
	go func() { errCh <- httpSrv.Serve() }()
	go func() { errCh <- boltSrv.Serve(ctx) }()

	log.Printf("listening on HTTP %s, Bolt %s", httpAddr, boltAddr)

	// 5b. Start ingest processor (if enabled)
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

	// 6. Wait for signal or error
	select {
	case <-ctx.Done():
		log.Println("shutting down...")
	case err := <-errCh:
		log.Printf("server error: %v", err)
	}

	// 7. Graceful shutdown (5s timeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	_ = boltSrv.Close()

	return nil
}
