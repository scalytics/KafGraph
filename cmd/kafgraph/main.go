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
	"github.com/scalytics/kafgraph/internal/cluster"
	"github.com/scalytics/kafgraph/internal/compliance"
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

	// 5b. Create analyzer-enabled reflection runner and inject into brain service
	analyzer := reflectpkg.NewHeuristicAnalyzer(g)
	cycleRunner := reflectpkg.NewCycleRunnerWithAnalyzer(g, analyzer)
	adapter := reflectpkg.NewBrainAdapter(cycleRunner)
	bs.SetReflectionRunner(adapter)
	bs.SetEnricher(analyzer)

	// 6. Cluster distribution (Phase 7)
	var qe cluster.QueryExecutor = exec // default: local executor
	var mem *cluster.Membership
	var pm *cluster.PartitionMap

	if cfg.Cluster.Enabled {
		strategy := &cluster.AgentIDPartitioner{}
		pm = cluster.NewPartitionMap(cfg.Cluster.NumPartitions, strategy)

		var merr error
		mem, merr = cluster.NewMembership(cluster.MembershipConfig{
			NodeName: cfg.Cluster.NodeName,
			BindAddr: cfg.Cluster.BindAddr,
			BindPort: cfg.Cluster.GossipPort,
			Seeds:    cfg.Cluster.Seeds,
			RPCPort:  cfg.Cluster.RPCPort,
			BoltPort: cfg.BoltPort,
			HTTPPort: cfg.Port,
		}, pm)
		if merr != nil {
			return fmt.Errorf("create cluster membership: %w", merr)
		}
		defer func() { _ = mem.Leave() }()

		if len(cfg.Cluster.Seeds) > 0 {
			if err := mem.Join(cfg.Cluster.Seeds); err != nil {
				log.Printf("warning: cluster join failed: %v", err)
			}
		}

		rpcAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Cluster.RPCPort)
		rpcSrv, err := cluster.NewRPCServer(rpcAddr, exec)
		if err != nil {
			return fmt.Errorf("start rpc server: %w", err)
		}
		defer func() { _ = rpcSrv.Close() }()

		qe = cluster.NewQueryRouter(exec, pm, mem.Self().Name)

		log.Printf("cluster enabled: node=%s gossip=%s:%d rpc=%s partitions=%d",
			mem.Self().Name, cfg.Cluster.BindAddr, cfg.Cluster.GossipPort,
			rpcAddr, cfg.Cluster.NumPartitions)

		// Start RPC server immediately (needed before other servers use the router).
		go func() { _ = rpcSrv.Serve(ctx) }()
	}

	// 6b. Create compliance engine
	var compEngine *compliance.Engine
	if cfg.Compliance.Enabled {
		compEngine = compliance.NewEngine(g)
		compliance.RegisterGDPRRules(compEngine)
		compliance.RegisterDataFlowRules(compEngine)
		if err := compEngine.LoadYAMLRules(cfg.Compliance.RulesDir); err != nil {
			log.Printf("warning: failed to load YAML rules: %v", err)
		}
		if err := compEngine.EnsureFrameworkNodes(); err != nil {
			log.Printf("warning: failed to create framework nodes: %v", err)
		}
		log.Printf("compliance engine enabled: %d rules loaded", len(compEngine.Rules()))
	}

	// 7. Create servers
	httpAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	boltAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.BoltPort)

	httpSrv := server.NewHTTPServer(httpAddr, g, qe,
		server.WithBrain(bs),
		server.WithConfig(cfg),
		server.WithMembership(mem),
		server.WithPartitionMap(pm),
		server.WithCompliance(compEngine),
	)

	boltSrv, err := server.NewBoltServer(boltAddr, qe)
	if err != nil {
		return fmt.Errorf("start bolt server: %w", err)
	}

	// 8. Start servers in goroutines
	errCh := make(chan error, 5)
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
			CycleRunner:   cycleRunner,
		})
		go func() { errCh <- sched.Run(ctx) }()
		log.Printf("reflection scheduler started (check=%s, feedback_topic=%s)",
			checkInterval, cfg.Reflect.FeedbackRequestTopic)
	}

	// 7d. Start compliance scheduler (if enabled)
	if compEngine != nil && cfg.Compliance.AutoScan {
		scanInterval, err := time.ParseDuration(cfg.Compliance.ScanInterval)
		if err != nil {
			return fmt.Errorf("parse compliance scan_interval: %w", err)
		}
		compSched := compliance.NewScheduler(compEngine, scanInterval, true)
		go func() { errCh <- compSched.Run(ctx) }()
		log.Printf("compliance scheduler started (interval=%s)", scanInterval)
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
