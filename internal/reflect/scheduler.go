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

package reflect

import (
	"context"
	"log"
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
)

// SchedulerConfig holds scheduler settings.
type SchedulerConfig struct {
	CheckInterval time.Duration
	Daily         Schedule
	Weekly        Schedule
	Monthly       Schedule
	GracePeriod   time.Duration
	// Phase 6: feedback request event emission (all optional, zero-value safe).
	Publisher    Publisher // nil = no event emission
	RequestTopic string    // default "kafgraph.feedback.requests"
	TopN         int       // top signals per request, default 5
}

// Scheduler runs reflection cycles on a schedule.
type Scheduler struct {
	graph   *graph.Graph
	runner  *CycleRunner
	checker *FeedbackChecker
	config  SchedulerConfig
	lastRun map[CycleType]time.Time
	nowFunc func() time.Time
}

// NewScheduler creates a new reflection scheduler.
func NewScheduler(g *graph.Graph, cfg SchedulerConfig) *Scheduler {
	checker := NewFeedbackChecker(g, cfg.GracePeriod)
	if cfg.Publisher != nil {
		topic := cfg.RequestTopic
		if topic == "" {
			topic = "kafgraph.feedback.requests"
		}
		topN := cfg.TopN
		if topN <= 0 {
			topN = 5
		}
		checker.WithPublisher(cfg.Publisher, topic, topN)
	}
	return &Scheduler{
		graph:   g,
		runner:  NewCycleRunner(g),
		checker: checker,
		config:  cfg,
		lastRun: make(map[CycleType]time.Time),
		nowFunc: time.Now,
	}
}

// Run starts the scheduler loop. It returns ctx.Err() on cancellation.
func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.tick(ctx); err != nil {
				log.Printf("reflect scheduler tick error: %v", err)
			}
		}
	}
}

// RunCycle executes a single cycle (useful for testing and brain delegation).
func (s *Scheduler) RunCycle(ctx context.Context, req CycleRequest) (*CycleResult, error) {
	return s.runner.Execute(ctx, req)
}

func (s *Scheduler) tick(ctx context.Context) error {
	now := s.nowFunc()

	// Check each schedule
	schedules := []struct {
		ct       CycleType
		schedule Schedule
		windowFn func(time.Time) time.Time
	}{
		{CycleDaily, s.config.Daily, DailyWindowStart},
		{CycleWeekly, s.config.Weekly, WeeklyWindowStart},
		{CycleMonthly, s.config.Monthly, MonthlyWindowStart},
	}

	for _, sc := range schedules {
		if !sc.schedule.IsDue(now, s.lastRun[sc.ct]) {
			continue
		}

		// Discover all Agent nodes
		agents, err := s.graph.NodesByLabel("Agent")
		if err != nil {
			log.Printf("reflect: failed to list agents: %v", err)
			continue
		}

		ws := sc.windowFn(now)
		for _, agent := range agents {
			agentID, _ := agent.Properties["name"].(string)
			if agentID == "" {
				agentID = string(agent.ID)
			}

			_, err := s.runner.Execute(ctx, CycleRequest{
				Type:        sc.ct,
				AgentID:     agentID,
				WindowStart: ws,
				WindowEnd:   now,
			})
			if err != nil {
				log.Printf("reflect: cycle %s for %s failed: %v", sc.ct, agentID, err)
			}
		}

		s.lastRun[sc.ct] = now
	}

	// Check feedback grace periods
	if err := s.checker.CheckPending(ctx); err != nil {
		log.Printf("reflect: feedback check failed: %v", err)
	}

	return nil
}
