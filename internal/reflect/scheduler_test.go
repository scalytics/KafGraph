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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
)

func TestSchedulerRunCycle(t *testing.T) {
	g := newTestGraph(t)
	s := NewScheduler(g, SchedulerConfig{
		CheckInterval: 1 * time.Minute,
		GracePeriod:   24 * time.Hour,
	})

	g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})

	now := time.Now()
	result, err := s.RunCycle(context.Background(), CycleRequest{
		Type:        CycleDaily,
		AgentID:     "alice",
		WindowStart: DailyWindowStart(now),
		WindowEnd:   now,
	})
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", result.Status)
}

func TestSchedulerTickDailyDue(t *testing.T) {
	g := newTestGraph(t)
	cfg := SchedulerConfig{
		CheckInterval: 1 * time.Minute,
		Daily:         Schedule{Enabled: true, Hour: 2, Minute: 0},
		GracePeriod:   24 * time.Hour,
	}
	s := NewScheduler(g, cfg)

	// Set nowFunc to 02:00 UTC
	fixed := time.Date(2026, 3, 3, 2, 0, 0, 0, time.UTC)
	s.nowFunc = func() time.Time { return fixed }
	s.runner.nowFunc = func() time.Time { return fixed }

	// Create an agent
	g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})

	err := s.tick(context.Background())
	require.NoError(t, err)

	// Should have created a ReflectionCycle
	cycles, _ := g.NodesByLabel("ReflectionCycle")
	assert.NotEmpty(t, cycles)

	// lastRun should be updated
	assert.Equal(t, fixed, s.lastRun[CycleDaily])
}

func TestSchedulerTickNotDue(t *testing.T) {
	g := newTestGraph(t)
	cfg := SchedulerConfig{
		CheckInterval: 1 * time.Minute,
		Daily:         Schedule{Enabled: true, Hour: 2, Minute: 0},
		GracePeriod:   24 * time.Hour,
	}
	s := NewScheduler(g, cfg)

	// Set nowFunc to 15:00 UTC (not the scheduled time)
	fixed := time.Date(2026, 3, 3, 15, 0, 0, 0, time.UTC)
	s.nowFunc = func() time.Time { return fixed }

	g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})

	err := s.tick(context.Background())
	require.NoError(t, err)

	// No cycles should be created
	cycles, _ := g.NodesByLabel("ReflectionCycle")
	assert.Empty(t, cycles)
}

func TestSchedulerRunCancellation(t *testing.T) {
	g := newTestGraph(t)
	s := NewScheduler(g, SchedulerConfig{
		CheckInterval: 10 * time.Millisecond,
		GracePeriod:   24 * time.Hour,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSchedulerTickMultipleAgents(t *testing.T) {
	g := newTestGraph(t)
	cfg := SchedulerConfig{
		CheckInterval: 1 * time.Minute,
		Daily:         Schedule{Enabled: true, Hour: 2, Minute: 0},
		GracePeriod:   24 * time.Hour,
	}
	s := NewScheduler(g, cfg)

	fixed := time.Date(2026, 3, 3, 2, 0, 0, 0, time.UTC)
	s.nowFunc = func() time.Time { return fixed }
	s.runner.nowFunc = func() time.Time { return fixed }

	g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})
	g.UpsertNode("n:Agent:bob", "Agent", graph.Properties{"name": "bob"})

	err := s.tick(context.Background())
	require.NoError(t, err)

	// Should have created cycles for both agents
	cycles, _ := g.NodesByLabel("ReflectionCycle")
	assert.GreaterOrEqual(t, len(cycles), 2)
}

func TestSchedulerTickFeedbackCheck(t *testing.T) {
	g := newTestGraph(t)
	cfg := SchedulerConfig{
		CheckInterval: 1 * time.Minute,
		GracePeriod:   1 * time.Hour,
	}
	s := NewScheduler(g, cfg)

	now := time.Now()
	s.nowFunc = func() time.Time { return now }
	s.checker.nowFunc = func() time.Time { return now }

	// Create a completed cycle that needs feedback
	g.UpsertNode("n:ReflectionCycle:old", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "PENDING",
		"completedAt":         now.Add(-2 * time.Hour).Format(time.RFC3339),
	})

	err := s.tick(context.Background())
	require.NoError(t, err)

	// Should have been updated to NEEDS_FEEDBACK
	cycle, _ := g.GetNode("n:ReflectionCycle:old")
	assert.Equal(t, "NEEDS_FEEDBACK", cycle.Properties["humanFeedbackStatus"])
}

func TestSchedulerRunShortDuration(t *testing.T) {
	g := newTestGraph(t)
	s := NewScheduler(g, SchedulerConfig{
		CheckInterval: 10 * time.Millisecond,
		GracePeriod:   24 * time.Hour,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := s.Run(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestNewScheduler(t *testing.T) {
	g := newTestGraph(t)
	cfg := SchedulerConfig{
		CheckInterval: 1 * time.Minute,
		GracePeriod:   24 * time.Hour,
		Daily:         Schedule{Enabled: true, Hour: 2},
	}
	s := NewScheduler(g, cfg)
	assert.NotNil(t, s.runner)
	assert.NotNil(t, s.checker)
	assert.NotNil(t, s.lastRun)
	assert.Equal(t, cfg.Daily.Hour, s.config.Daily.Hour)
}
