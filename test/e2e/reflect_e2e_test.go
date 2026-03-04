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

//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
	reflectpkg "github.com/scalytics/kafgraph/internal/reflect"
	"github.com/scalytics/kafgraph/internal/storage"
)

// TestE2EReflectionCycle exercises the full reflection cycle pipeline.
func TestE2EReflectionCycle(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	// 1. Create an agent with messages and conversations
	agent, err := g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{
		"name": "alice",
	})
	require.NoError(t, err)

	conv, err := g.CreateNode("Conversation", graph.Properties{
		"description": "deployment planning",
	})
	require.NoError(t, err)

	msg1, err := g.CreateNode("Message", graph.Properties{
		"text": "we need to deploy the new service",
	})
	require.NoError(t, err)

	msg2, err := g.CreateNode("Message", graph.Properties{
		"text":      "deployment completed successfully",
		"inReplyTo": string(msg1.ID),
	})
	require.NoError(t, err)

	// Link everything
	g.CreateEdge("AUTHORED", agent.ID, msg1.ID, nil)
	g.CreateEdge("AUTHORED", agent.ID, msg2.ID, nil)
	g.CreateEdge("BELONGS_TO", msg1.ID, conv.ID, nil)
	g.CreateEdge("BELONGS_TO", msg2.ID, conv.ID, nil)
	g.CreateEdge("BELONGS_TO", conv.ID, agent.ID, nil)

	// 2. Run a daily reflection cycle
	runner := reflectpkg.NewCycleRunner(g)
	now := time.Now()
	result, err := runner.Execute(context.Background(), reflectpkg.CycleRequest{
		Type:        reflectpkg.CycleDaily,
		AgentID:     "alice",
		WindowStart: now.Add(-1 * time.Hour),
		WindowEnd:   now.Add(1 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", result.Status)
	assert.NotEmpty(t, result.LearningSignals)
	assert.Contains(t, result.Summary, "signals")

	// 3. Verify the ReflectionCycle node was created
	cycles, err := g.NodesByLabel("ReflectionCycle")
	require.NoError(t, err)
	require.NotEmpty(t, cycles)

	var completedCycle *graph.Node
	for _, c := range cycles {
		if c.Properties["status"] == "COMPLETED" {
			completedCycle = c
			break
		}
	}
	require.NotNil(t, completedCycle)
	assert.Equal(t, "alice", completedCycle.Properties["agentId"])

	// 4. Verify LearningSignal nodes were created with scores
	signals, err := g.NodesByLabel("LearningSignal")
	require.NoError(t, err)
	assert.NotEmpty(t, signals)

	for _, sig := range signals {
		if sig.Properties["cycleType"] != nil {
			impact, _ := sig.Properties["impact"].(float64)
			assert.GreaterOrEqual(t, impact, 0.0)
			assert.LessOrEqual(t, impact, 1.0)
		}
	}

	// 5. Run idempotent — same cycle should not duplicate
	result2, err := runner.Execute(context.Background(), reflectpkg.CycleRequest{
		Type:        reflectpkg.CycleDaily,
		AgentID:     "alice",
		WindowStart: now.Add(-1 * time.Hour),
		WindowEnd:   now.Add(1 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, result.CycleID, result2.CycleID)

	// 6. Verify feedback grace period checker
	checker := reflectpkg.NewFeedbackChecker(g, 0) // 0 = immediate expiry
	err = checker.CheckPending(context.Background())
	require.NoError(t, err)

	// The completed cycle should now be marked NEEDS_FEEDBACK
	updatedCycle, err := g.GetNode(result.CycleID)
	require.NoError(t, err)
	assert.Equal(t, "NEEDS_FEEDBACK", updatedCycle.Properties["humanFeedbackStatus"])
}

// TestE2EReflectionBrainIntegration tests brain → reflect delegation.
func TestE2EReflectionBrainIntegration(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	// Create agent
	g.UpsertNode("n:Agent:bob", "Agent", graph.Properties{"name": "bob"})
	g.CreateNode("Message", graph.Properties{"text": "test message"})

	// Create runner and adapter
	runner := reflectpkg.NewCycleRunner(g)
	result, err := runner.ExecuteForBrain(context.Background(), "bob", 24)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", result.Status)
	assert.NotEmpty(t, string(result.CycleID))
}

// TestE2EReflectionSchedulerShort tests the scheduler runs and stops cleanly.
func TestE2EReflectionSchedulerShort(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	sched := reflectpkg.NewScheduler(g, reflectpkg.SchedulerConfig{
		CheckInterval: 10 * time.Millisecond,
		GracePeriod:   24 * time.Hour,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = sched.Run(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
