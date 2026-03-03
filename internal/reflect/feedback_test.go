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
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
)

// testPublisher is a test Publisher for the reflect package.
type testPublisher struct {
	mu       sync.Mutex
	messages []testPublishedMsg
}

type testPublishedMsg struct {
	Topic string
	Key   string
	Data  []byte
}

func (p *testPublisher) Publish(_ context.Context, topic, key string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages = append(p.messages, testPublishedMsg{
		Topic: topic, Key: key, Data: append([]byte(nil), data...),
	})
	return nil
}

func (p *testPublisher) len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.messages)
}

func TestFeedbackCheckerNoCycles(t *testing.T) {
	g := newTestGraph(t)
	fc := NewFeedbackChecker(g, 24*time.Hour)
	err := fc.CheckPending(context.Background())
	require.NoError(t, err)
}

func TestFeedbackCheckerGracePeriodNotExpired(t *testing.T) {
	g := newTestGraph(t)
	fc := NewFeedbackChecker(g, 24*time.Hour)

	now := time.Now()
	fc.nowFunc = func() time.Time { return now }

	g.UpsertNode("n:ReflectionCycle:test", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "PENDING",
		"completedAt":         now.Add(-1 * time.Hour).Format(time.RFC3339), // 1h ago, grace is 24h
	})

	err := fc.CheckPending(context.Background())
	require.NoError(t, err)

	// Should still be PENDING
	cycle, _ := g.GetNode("n:ReflectionCycle:test")
	assert.Equal(t, "PENDING", cycle.Properties["humanFeedbackStatus"])
}

func TestFeedbackCheckerGracePeriodExpired(t *testing.T) {
	g := newTestGraph(t)
	fc := NewFeedbackChecker(g, 24*time.Hour)

	now := time.Now()
	fc.nowFunc = func() time.Time { return now }

	g.UpsertNode("n:ReflectionCycle:test", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "PENDING",
		"completedAt":         now.Add(-25 * time.Hour).Format(time.RFC3339), // 25h ago
	})

	err := fc.CheckPending(context.Background())
	require.NoError(t, err)

	// Should be updated to NEEDS_FEEDBACK
	cycle, _ := g.GetNode("n:ReflectionCycle:test")
	assert.Equal(t, "NEEDS_FEEDBACK", cycle.Properties["humanFeedbackStatus"])
}

func TestFeedbackCheckerSkipsNonCompleted(t *testing.T) {
	g := newTestGraph(t)
	fc := NewFeedbackChecker(g, 24*time.Hour)

	now := time.Now()
	fc.nowFunc = func() time.Time { return now }

	g.UpsertNode("n:ReflectionCycle:running", "ReflectionCycle", graph.Properties{
		"status":              "RUNNING",
		"humanFeedbackStatus": "PENDING",
		"completedAt":         now.Add(-48 * time.Hour).Format(time.RFC3339),
	})

	err := fc.CheckPending(context.Background())
	require.NoError(t, err)

	// Should still be PENDING since status is not COMPLETED
	cycle, _ := g.GetNode("n:ReflectionCycle:running")
	assert.Equal(t, "PENDING", cycle.Properties["humanFeedbackStatus"])
}

func TestFeedbackCheckerSkipsAlreadyFeedback(t *testing.T) {
	g := newTestGraph(t)
	fc := NewFeedbackChecker(g, 24*time.Hour)

	now := time.Now()
	fc.nowFunc = func() time.Time { return now }

	g.UpsertNode("n:ReflectionCycle:done", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "RECEIVED",
		"completedAt":         now.Add(-48 * time.Hour).Format(time.RFC3339),
	})

	err := fc.CheckPending(context.Background())
	require.NoError(t, err)

	// Should still be RECEIVED
	cycle, _ := g.GetNode("n:ReflectionCycle:done")
	assert.Equal(t, "RECEIVED", cycle.Properties["humanFeedbackStatus"])
}

func TestFeedbackCheckerCancellation(t *testing.T) {
	g := newTestGraph(t)
	fc := NewFeedbackChecker(g, 24*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := fc.CheckPending(ctx)
	assert.Error(t, err)
}

// --- Phase 6: Publisher + NEEDS_FEEDBACK → REQUESTED tests ---

func TestFeedbackCheckerNeedsFeedbackToRequested(t *testing.T) {
	g := newTestGraph(t)
	pub := &testPublisher{}
	fc := NewFeedbackChecker(g, 24*time.Hour)
	fc.WithPublisher(pub, "kafgraph.feedback.requests", 5)

	now := time.Now()
	fc.nowFunc = func() time.Time { return now }

	g.UpsertNode("n:ReflectionCycle:test", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "NEEDS_FEEDBACK",
		"agentId":             "alice",
		"cycleType":           "daily",
	})

	err := fc.CheckPending(context.Background())
	require.NoError(t, err)

	// Should transition to REQUESTED
	cycle, _ := g.GetNode("n:ReflectionCycle:test")
	assert.Equal(t, "REQUESTED", cycle.Properties["humanFeedbackStatus"])

	// Publisher should have received the event
	assert.Equal(t, 1, pub.len())
	assert.Equal(t, "kafgraph.feedback.requests", pub.messages[0].Topic)
	assert.Equal(t, "n:ReflectionCycle:test", pub.messages[0].Key)

	// Verify event payload
	var event FeedbackRequestEvent
	require.NoError(t, json.Unmarshal(pub.messages[0].Data, &event))
	assert.Equal(t, "n:ReflectionCycle:test", event.CycleID)
	assert.Equal(t, "alice", event.AgentID)
	assert.Equal(t, "daily", event.CycleType)
}

func TestFeedbackCheckerPublisherCalledWithTopSignals(t *testing.T) {
	g := newTestGraph(t)
	pub := &testPublisher{}
	fc := NewFeedbackChecker(g, 24*time.Hour)
	fc.WithPublisher(pub, "test.topic", 2) // top 2 only

	now := time.Now()
	fc.nowFunc = func() time.Time { return now }

	// Create cycle
	g.UpsertNode("n:ReflectionCycle:c1", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "NEEDS_FEEDBACK",
		"agentId":             "bob",
	})

	// Create 3 signals with different impacts
	g.UpsertNode("n:LearningSignal:s1", "LearningSignal", graph.Properties{
		"summary": "low impact", "impact": 0.2,
	})
	g.UpsertNode("n:LearningSignal:s2", "LearningSignal", graph.Properties{
		"summary": "high impact", "impact": 0.9,
	})
	g.UpsertNode("n:LearningSignal:s3", "LearningSignal", graph.Properties{
		"summary": "mid impact", "impact": 0.5,
	})

	// Link signals to cycle (LINKS_TO signal→cycle)
	g.UpsertEdge("e:LINKS_TO:s1c1", "LINKS_TO", "n:LearningSignal:s1", "n:ReflectionCycle:c1", nil)
	g.UpsertEdge("e:LINKS_TO:s2c1", "LINKS_TO", "n:LearningSignal:s2", "n:ReflectionCycle:c1", nil)
	g.UpsertEdge("e:LINKS_TO:s3c1", "LINKS_TO", "n:LearningSignal:s3", "n:ReflectionCycle:c1", nil)

	err := fc.CheckPending(context.Background())
	require.NoError(t, err)

	// Verify top-N = 2, sorted by impact desc
	var event FeedbackRequestEvent
	require.NoError(t, json.Unmarshal(pub.messages[0].Data, &event))
	require.Len(t, event.TopSignals, 2)
	assert.Equal(t, 0.9, event.TopSignals[0].Impact)
	assert.Equal(t, 0.5, event.TopSignals[1].Impact)
}

func TestFeedbackCheckerNoPublisherFallback(t *testing.T) {
	g := newTestGraph(t)
	fc := NewFeedbackChecker(g, 24*time.Hour) // no publisher

	now := time.Now()
	fc.nowFunc = func() time.Time { return now }

	g.UpsertNode("n:ReflectionCycle:test", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "NEEDS_FEEDBACK",
	})

	err := fc.CheckPending(context.Background())
	require.NoError(t, err)

	// Should remain NEEDS_FEEDBACK (no publisher to transition)
	cycle, _ := g.GetNode("n:ReflectionCycle:test")
	assert.Equal(t, "NEEDS_FEEDBACK", cycle.Properties["humanFeedbackStatus"])
}

func TestFeedbackCheckerSkipsAlreadyRequested(t *testing.T) {
	g := newTestGraph(t)
	pub := &testPublisher{}
	fc := NewFeedbackChecker(g, 24*time.Hour)
	fc.WithPublisher(pub, "test.topic", 5)

	now := time.Now()
	fc.nowFunc = func() time.Time { return now }

	g.UpsertNode("n:ReflectionCycle:test", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "REQUESTED",
	})

	err := fc.CheckPending(context.Background())
	require.NoError(t, err)

	// Should still be REQUESTED, no new message
	cycle, _ := g.GetNode("n:ReflectionCycle:test")
	assert.Equal(t, "REQUESTED", cycle.Properties["humanFeedbackStatus"])
	assert.Equal(t, 0, pub.len())
}

func TestFeedbackCheckerSkipsWaived(t *testing.T) {
	g := newTestGraph(t)
	pub := &testPublisher{}
	fc := NewFeedbackChecker(g, 24*time.Hour)
	fc.WithPublisher(pub, "test.topic", 5)

	now := time.Now()
	fc.nowFunc = func() time.Time { return now }

	g.UpsertNode("n:ReflectionCycle:test", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "WAIVED",
	})

	err := fc.CheckPending(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, pub.len())
}

func TestFeedbackCheckerGatherTopSignalsEmpty(t *testing.T) {
	g := newTestGraph(t)
	fc := NewFeedbackChecker(g, 24*time.Hour)

	g.UpsertNode("n:ReflectionCycle:test", "ReflectionCycle", graph.Properties{})
	signals := fc.gatherTopSignals("n:ReflectionCycle:test", 5)
	assert.Empty(t, signals)
}

func TestFeedbackCheckerFullTransitionPipeline(t *testing.T) {
	g := newTestGraph(t)
	pub := &testPublisher{}
	fc := NewFeedbackChecker(g, 1*time.Hour)
	fc.WithPublisher(pub, "test.topic", 5)

	now := time.Now()
	fc.nowFunc = func() time.Time { return now }

	// Start with PENDING, 2 hours old
	g.UpsertNode("n:ReflectionCycle:pipeline", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "PENDING",
		"completedAt":         now.Add(-2 * time.Hour).Format(time.RFC3339),
		"agentId":             "alice",
	})

	// First check: PENDING → NEEDS_FEEDBACK
	err := fc.CheckPending(context.Background())
	require.NoError(t, err)
	cycle, _ := g.GetNode("n:ReflectionCycle:pipeline")
	assert.Equal(t, "NEEDS_FEEDBACK", cycle.Properties["humanFeedbackStatus"])
	assert.Equal(t, 0, pub.len()) // no publish yet

	// Second check: NEEDS_FEEDBACK → REQUESTED
	err = fc.CheckPending(context.Background())
	require.NoError(t, err)
	cycle, _ = g.GetNode("n:ReflectionCycle:pipeline")
	assert.Equal(t, "REQUESTED", cycle.Properties["humanFeedbackStatus"])
	assert.Equal(t, 1, pub.len()) // published

	// Third check: REQUESTED stays REQUESTED (no action)
	err = fc.CheckPending(context.Background())
	require.NoError(t, err)
	cycle, _ = g.GetNode("n:ReflectionCycle:pipeline")
	assert.Equal(t, "REQUESTED", cycle.Properties["humanFeedbackStatus"])
	assert.Equal(t, 1, pub.len()) // no new publish
}
