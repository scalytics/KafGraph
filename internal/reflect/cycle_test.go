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

func TestCycleRunnerExecuteEmpty(t *testing.T) {
	g := newTestGraph(t)
	cr := NewCycleRunner(g)

	now := time.Now()
	result, err := cr.Execute(context.Background(), CycleRequest{
		Type:        CycleDaily,
		AgentID:     "alice",
		WindowStart: DailyWindowStart(now),
		WindowEnd:   now,
	})
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", result.Status)
	assert.Empty(t, result.LearningSignals)
	assert.Contains(t, result.Summary, "No activity")
}

func TestCycleRunnerExecuteWithMessages(t *testing.T) {
	g := newTestGraph(t)
	cr := NewCycleRunner(g)

	// Create agent and messages
	agent, _ := g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})
	msg1, _ := g.CreateNode("Message", graph.Properties{"text": "hello world"})
	msg2, _ := g.CreateNode("Message", graph.Properties{"text": "deploy service"})
	conv, _ := g.CreateNode("Conversation", graph.Properties{"description": "deployment chat"})

	g.CreateEdge("AUTHORED", agent.ID, msg1.ID, nil)
	g.CreateEdge("AUTHORED", agent.ID, msg2.ID, nil)
	g.CreateEdge("BELONGS_TO", msg1.ID, conv.ID, nil)
	g.CreateEdge("BELONGS_TO", msg2.ID, conv.ID, nil)
	g.CreateEdge("BELONGS_TO", conv.ID, agent.ID, nil)

	now := time.Now()
	result, err := cr.Execute(context.Background(), CycleRequest{
		Type:        CycleDaily,
		AgentID:     "alice",
		WindowStart: now.Add(-1 * time.Hour),
		WindowEnd:   now.Add(1 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", result.Status)
	assert.NotEmpty(t, result.LearningSignals)
	assert.Contains(t, result.Summary, "signals")

	// Verify cycle node was created
	cycles, _ := g.NodesByLabel("ReflectionCycle")
	found := false
	for _, c := range cycles {
		if c.Properties["status"] == "COMPLETED" {
			found = true
		}
	}
	assert.True(t, found, "should have a COMPLETED ReflectionCycle")
}

func TestCycleRunnerIdempotent(t *testing.T) {
	g := newTestGraph(t)
	cr := NewCycleRunner(g)

	agent, _ := g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})
	msg, _ := g.CreateNode("Message", graph.Properties{"text": "hello"})
	g.CreateEdge("AUTHORED", agent.ID, msg.ID, nil)

	now := time.Now()
	req := CycleRequest{
		Type:        CycleDaily,
		AgentID:     "alice",
		WindowStart: now.Add(-1 * time.Hour),
		WindowEnd:   now.Add(1 * time.Hour),
	}

	// Execute twice
	r1, err := cr.Execute(context.Background(), req)
	require.NoError(t, err)
	r2, err := cr.Execute(context.Background(), req)
	require.NoError(t, err)

	// Same deterministic cycle ID
	assert.Equal(t, r1.CycleID, r2.CycleID)

	// Only one cycle node (upserted, not duplicated)
	cycles, _ := g.NodesByLabel("ReflectionCycle")
	cycleCount := 0
	for _, c := range cycles {
		if string(c.ID) == string(r1.CycleID) {
			cycleCount++
		}
	}
	assert.Equal(t, 1, cycleCount)
}

func TestCycleRunnerCancellation(t *testing.T) {
	g := newTestGraph(t)
	cr := NewCycleRunner(g)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := cr.Execute(ctx, CycleRequest{
		Type:        CycleDaily,
		AgentID:     "alice",
		WindowStart: time.Now(),
		WindowEnd:   time.Now(),
	})
	assert.Error(t, err)
}

func TestCycleRunnerDeterministicSignalIDs(t *testing.T) {
	g := newTestGraph(t)
	cr := NewCycleRunner(g)

	agent, _ := g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})
	msg, _ := g.CreateNode("Message", graph.Properties{"text": "test"})
	g.CreateEdge("AUTHORED", agent.ID, msg.ID, nil)

	now := time.Now()
	req := CycleRequest{
		Type:        CycleDaily,
		AgentID:     "alice",
		WindowStart: now.Add(-1 * time.Hour),
		WindowEnd:   now.Add(1 * time.Hour),
	}

	cr.Execute(context.Background(), req) //nolint:errcheck

	// Check that LearningSignal nodes have deterministic IDs
	signals, _ := g.NodesByLabel("LearningSignal")
	for _, sig := range signals {
		id := string(sig.ID)
		assert.Contains(t, id, "n:LearningSignal:daily:alice:")
	}
}

func TestCycleRunnerWeeklyRollup(t *testing.T) {
	g := newTestGraph(t)
	cr := NewCycleRunner(g)

	now := time.Now()
	ws := WeeklyWindowStart(now)

	// Create a prior daily cycle within the weekly window
	dailyWS := DailyWindowStart(now)
	dailyCycleID := CycleNodeID(CycleDaily, "alice", dailyWS)
	g.UpsertNode(dailyCycleID, "ReflectionCycle", graph.Properties{
		"cycleType":   string(CycleDaily),
		"windowStart": dailyWS.Format(time.RFC3339),
		"status":      "COMPLETED",
	})

	// Create agent
	g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})

	result, err := cr.Execute(context.Background(), CycleRequest{
		Type:        CycleWeekly,
		AgentID:     "alice",
		WindowStart: ws,
		WindowEnd:   now.Add(1 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", result.Status)
}

func TestCycleRunnerExecuteForBrain(t *testing.T) {
	g := newTestGraph(t)
	cr := NewCycleRunner(g)

	g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})

	result, err := cr.ExecuteForBrain(context.Background(), "alice", 24)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", result.Status)
	assert.NotEmpty(t, result.CycleID)
}

func TestCycleRunnerExecuteForBrainDefaultWindow(t *testing.T) {
	g := newTestGraph(t)
	cr := NewCycleRunner(g)

	g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})

	result, err := cr.ExecuteForBrain(context.Background(), "alice", 0)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", result.Status)
}

func TestCycleRunnerInjectableNowFunc(t *testing.T) {
	g := newTestGraph(t)
	cr := NewCycleRunner(g)

	fixed := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	cr.nowFunc = func() time.Time { return fixed }

	g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})

	result, err := cr.Execute(context.Background(), CycleRequest{
		Type:        CycleDaily,
		AgentID:     "alice",
		WindowStart: DailyWindowStart(fixed),
		WindowEnd:   fixed,
	})
	require.NoError(t, err)

	// Verify the cycle node has the fixed time
	cycleNode, err := g.GetNode(result.CycleID)
	require.NoError(t, err)
	assert.Contains(t, cycleNode.Properties["completedAt"], "2026-03-03")
}
