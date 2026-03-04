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

package compliance

import (
	"context"
	"log"
	"time"
)

// Scheduler runs compliance scans on a configured interval.
type Scheduler struct {
	engine   *Engine
	interval time.Duration
	autoScan bool
}

// NewScheduler creates a compliance scan scheduler.
func NewScheduler(engine *Engine, interval time.Duration, autoScan bool) *Scheduler {
	return &Scheduler{
		engine:   engine,
		interval: interval,
		autoScan: autoScan,
	}
}

// Run starts the scheduled scan loop. Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	if !s.autoScan || s.interval <= 0 {
		<-ctx.Done()
		return ctx.Err()
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	log.Printf("compliance scheduler started (interval=%s)", s.interval)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.runAllFrameworks(ctx)
		}
	}
}

func (s *Scheduler) runAllFrameworks(ctx context.Context) {
	frameworks := map[Framework]bool{}
	for _, r := range s.engine.Rules() {
		frameworks[r.Framework()] = true
	}

	for fw := range frameworks {
		result, err := s.engine.RunScan(ctx, ScanRequest{Framework: fw})
		if err != nil {
			log.Printf("compliance scan error (%s): %v", fw, err)
			continue
		}
		log.Printf("compliance scan completed (%s): score=%.1f%% pass=%d fail=%d",
			fw, result.Score, result.PassCount, result.FailCount)
	}
}
