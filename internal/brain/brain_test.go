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

package brain

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/search"
	"github.com/scalytics/kafgraph/internal/storage"
)

func newTestService(t *testing.T) (*Service, *graph.Graph) {
	t.Helper()
	s, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	g := graph.New(s)
	svc := NewService(g, nil, nil)
	return svc, g
}

// --- brain_search tests ---

func TestSearchEmpty(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Search(SearchInput{Query: "hello", Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, out.Results)
}

func TestSearchWithMockFullText(t *testing.T) {
	s, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	g := graph.New(s)

	node, _ := g.CreateNode("Message", graph.Properties{"text": "hello world"})

	ft := &mockFullText{
		results: []search.TextSearchResult{{NodeID: node.ID, Score: 0.9}},
	}
	svc := NewService(g, ft, nil)

	out, err := svc.Search(SearchInput{Query: "hello", Limit: 10})
	require.NoError(t, err)
	// Mock returns the same node for all 4 searched labels; node exists so all 4 match
	require.GreaterOrEqual(t, len(out.Results), 1)
	assert.Equal(t, string(node.ID), out.Results[0].NodeID)
	assert.Equal(t, 0.9, out.Results[0].Score)
}

func TestSearchDefaultLimit(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Search(SearchInput{Query: "test"})
	require.NoError(t, err)
	assert.NotNil(t, out)
}

func TestSearchWithTimeRange(t *testing.T) {
	s, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	g := graph.New(s)

	node, _ := g.CreateNode("Message", graph.Properties{"text": "recent"})
	ft := &mockFullText{
		results: []search.TextSearchResult{{NodeID: node.ID, Score: 0.8}},
	}
	svc := NewService(g, ft, nil)

	// Time range that excludes the node (future range)
	out, err := svc.Search(SearchInput{
		Query: "recent",
		TimeRange: &TimeRange{
			From: time.Now().Add(1 * time.Hour),
			To:   time.Now().Add(2 * time.Hour),
		},
	})
	require.NoError(t, err)
	assert.Empty(t, out.Results)
}

// --- brain_recall tests ---

func TestRecallEmpty(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Recall(RecallInput{AgentID: "nonexistent"})
	require.NoError(t, err)
	assert.Empty(t, out.Context.ActiveConversations)
}

func TestRecallWithConnections(t *testing.T) {
	svc, g := newTestService(t)

	agent, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	conv, _ := g.CreateNode("Conversation", graph.Properties{"description": "test chat"})
	msg, _ := g.CreateNode("Message", graph.Properties{"text": "hello"})
	signal, _ := g.CreateNode("LearningSignal", graph.Properties{"summary": "learned X"})
	fb, _ := g.CreateNode("HumanFeedback", graph.Properties{"comment": "good"})

	g.CreateEdge("BELONGS_TO", conv.ID, agent.ID, nil)
	g.CreateEdge("AUTHORED", agent.ID, msg.ID, nil)
	g.CreateEdge("LINKS_TO", signal.ID, agent.ID, nil)
	g.CreateEdge("HAS_FEEDBACK", agent.ID, fb.ID, nil)

	out, err := svc.Recall(RecallInput{AgentID: string(agent.ID), Depth: "deep"})
	require.NoError(t, err)
	assert.Len(t, out.Context.ActiveConversations, 1)
	assert.Len(t, out.Context.UnresolvedThreads, 1)
	assert.Len(t, out.Context.RecentDecisions, 1)
	assert.Len(t, out.Context.PendingFeedback, 1)
}

func TestRecallShallowDepth(t *testing.T) {
	svc, g := newTestService(t)

	agent, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	for i := range 10 {
		conv, _ := g.CreateNode("Conversation", graph.Properties{"description": "chat", "idx": i})
		g.CreateEdge("BELONGS_TO", conv.ID, agent.ID, nil)
	}

	out, err := svc.Recall(RecallInput{AgentID: string(agent.ID), Depth: "shallow"})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out.Context.ActiveConversations), 5)
}

// --- brain_capture tests ---

func TestCaptureInsight(t *testing.T) {
	svc, g := newTestService(t)

	agent, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})

	out, err := svc.Capture(CaptureInput{
		AgentID: string(agent.ID),
		Type:    "insight",
		Content: "users prefer dark mode",
		Tags:    []string{"ux", "design"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, out.NodeID)
	assert.Contains(t, out.LinkedTo, string(agent.ID))

	// Verify the node was created
	signals, _ := g.NodesByLabel("LearningSignal")
	require.Len(t, signals, 1)
	assert.Equal(t, "users prefer dark mode", signals[0].Properties["content"])
	assert.Equal(t, "insight", signals[0].Properties["type"])
}

func TestCaptureWithLinkedNodes(t *testing.T) {
	svc, g := newTestService(t)

	agent, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	target, _ := g.CreateNode("Message", graph.Properties{"text": "related"})

	out, err := svc.Capture(CaptureInput{
		AgentID:     string(agent.ID),
		Content:     "links to message",
		LinkedNodes: []string{string(target.ID)},
	})
	require.NoError(t, err)
	assert.Len(t, out.LinkedTo, 2) // agent + target
}

func TestCaptureEmptyContent(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Capture(CaptureInput{Content: ""})
	assert.Error(t, err)
}

func TestCaptureDefaultType(t *testing.T) {
	svc, g := newTestService(t)
	out, err := svc.Capture(CaptureInput{Content: "some observation"})
	require.NoError(t, err)
	assert.NotEmpty(t, out.NodeID)

	signals, _ := g.NodesByLabel("LearningSignal")
	require.Len(t, signals, 1)
	assert.Equal(t, "observation", signals[0].Properties["type"])
}

// --- brain_recent tests ---

func TestRecentEmpty(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Recent(RecentInput{WindowHours: 24})
	require.NoError(t, err)
	assert.Empty(t, out.Activity)
}

func TestRecentWithNodes(t *testing.T) {
	svc, g := newTestService(t)

	g.CreateNode("Message", graph.Properties{"text": "hello"})
	g.CreateNode("Message", graph.Properties{"text": "world"})

	out, err := svc.Recent(RecentInput{WindowHours: 24, Types: []string{"Message"}})
	require.NoError(t, err)
	assert.Len(t, out.Activity, 2)
}

func TestRecentDefaults(t *testing.T) {
	svc, g := newTestService(t)
	g.CreateNode("Message", graph.Properties{"text": "test"})

	out, err := svc.Recent(RecentInput{})
	require.NoError(t, err)
	assert.Len(t, out.Activity, 1)
}

func TestRecentLimitApplied(t *testing.T) {
	svc, g := newTestService(t)
	for i := range 10 {
		g.CreateNode("Message", graph.Properties{"text": "msg", "idx": i})
	}

	out, err := svc.Recent(RecentInput{Limit: 3, Types: []string{"Message"}})
	require.NoError(t, err)
	assert.Len(t, out.Activity, 3)
}

// --- brain_patterns tests ---

func TestPatternsEmpty(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Patterns(PatternsInput{MinOccurrences: 2})
	require.NoError(t, err)
	assert.Empty(t, out.Patterns)
}

func TestPatternsWithTags(t *testing.T) {
	svc, g := newTestService(t)

	for range 4 {
		g.CreateNode("LearningSignal", graph.Properties{"tags": "ux,design", "summary": "ux insight"})
	}
	for range 2 {
		g.CreateNode("LearningSignal", graph.Properties{"tags": "perf", "summary": "perf note"})
	}

	out, err := svc.Patterns(PatternsInput{MinOccurrences: 3})
	require.NoError(t, err)
	require.Len(t, out.Patterns, 2) // ux and design each have 4 occurrences
	assert.Equal(t, 4, out.Patterns[0].Occurrences)
}

func TestPatternsDefaultMinOccurrences(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Patterns(PatternsInput{})
	require.NoError(t, err)
	assert.NotNil(t, out)
}

// --- brain_reflect tests ---

func TestReflectEmpty(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Reflect(ReflectInput{AgentID: "agent1", WindowHours: 24})
	require.NoError(t, err)
	assert.NotEmpty(t, out.CycleID)
	assert.Equal(t, "PENDING", out.HumanFeedbackStatus)
	assert.Equal(t, "No learning signals in window.", out.Summary)
}

func TestReflectWithSignals(t *testing.T) {
	svc, g := newTestService(t)

	agent, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("LearningSignal", graph.Properties{"summary": "learned about UX"})
	g.CreateNode("LearningSignal", graph.Properties{"summary": "improved testing"})

	out, err := svc.Reflect(ReflectInput{AgentID: string(agent.ID), WindowHours: 24})
	require.NoError(t, err)
	assert.NotEmpty(t, out.CycleID)
	assert.Len(t, out.LearningSignals, 2)
	assert.Contains(t, out.Summary, "2 learning signals")
	assert.Equal(t, "PENDING", out.HumanFeedbackStatus)

	// Verify ReflectionCycle node was created
	cycles, _ := g.NodesByLabel("ReflectionCycle")
	assert.Len(t, cycles, 1)
}

func TestReflectDefaultWindow(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Reflect(ReflectInput{})
	require.NoError(t, err)
	assert.NotEmpty(t, out.CycleID)
}

// --- brain_feedback tests ---

func TestFeedbackSuccess(t *testing.T) {
	svc, g := newTestService(t)

	cycle, _ := g.CreateNode("ReflectionCycle", graph.Properties{"status": "PENDING"})

	out, err := svc.Feedback(FeedbackInput{
		CycleID:      string(cycle.ID),
		FeedbackType: "confirm",
		Scores:       FeedbackScores{Impact: 0.8, Relevance: 0.9, ValueContribution: 0.7},
		Comment:      "good reflection",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, out.FeedbackID)
	assert.Equal(t, "RECEIVED", out.CycleStatus)

	// Verify feedback node
	fbs, _ := g.NodesByLabel("HumanFeedback")
	require.Len(t, fbs, 1)
	assert.Equal(t, "confirm", fbs[0].Properties["feedbackType"])
	assert.Equal(t, 0.8, fbs[0].Properties["impact"])
}

func TestFeedbackMissingCycleID(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Feedback(FeedbackInput{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycleId is required")
}

func TestFeedbackCycleNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Feedback(FeedbackInput{CycleID: "nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- Reflection delegation tests ---

type mockReflectionRunner struct {
	called bool
	result *ReflectCycleResult
	err    error
}

func (m *mockReflectionRunner) ExecuteForBrain(_ context.Context, _ string, _ int) (*ReflectCycleResult, error) {
	m.called = true
	return m.result, m.err
}

func TestReflectDelegatesToRunner(t *testing.T) {
	svc, _ := newTestService(t)
	runner := &mockReflectionRunner{
		result: &ReflectCycleResult{
			CycleID:         "n:ReflectionCycle:daily:alice:2026-03-03T00:00:00Z",
			LearningSignals: []NodeSummary{{NodeID: "sig1", Type: "LearningSignal", Summary: "test"}},
			Summary:         "1 signal found",
		},
	}
	svc.SetReflectionRunner(runner)

	out, err := svc.Reflect(ReflectInput{AgentID: "alice", WindowHours: 24})
	require.NoError(t, err)
	assert.True(t, runner.called)
	assert.Equal(t, "n:ReflectionCycle:daily:alice:2026-03-03T00:00:00Z", out.CycleID)
	assert.Len(t, out.LearningSignals, 1)
	assert.Equal(t, "PENDING", out.HumanFeedbackStatus)
}

func TestReflectFallbackWithoutRunner(t *testing.T) {
	svc, _ := newTestService(t)
	// No runner set — should use fallback behavior
	out, err := svc.Reflect(ReflectInput{AgentID: "alice", WindowHours: 24})
	require.NoError(t, err)
	assert.NotEmpty(t, out.CycleID)
	assert.Equal(t, "PENDING", out.HumanFeedbackStatus)
}

func TestReflectDelegationError(t *testing.T) {
	svc, _ := newTestService(t)
	runner := &mockReflectionRunner{
		err: assert.AnError,
	}
	svc.SetReflectionRunner(runner)

	_, err := svc.Reflect(ReflectInput{AgentID: "alice"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reflection runner")
}

// --- Helper tests ---

func TestExtractSummary(t *testing.T) {
	node := &graph.Node{Label: "Test", Properties: graph.Properties{"text": "hello"}}
	assert.Equal(t, "hello", extractSummary(node))

	node2 := &graph.Node{ID: "test-id", Label: "Test", Properties: graph.Properties{}}
	assert.Equal(t, "Test:test-id", extractSummary(node2))
}

func TestInTimeRange(t *testing.T) {
	now := time.Now()
	tr := &TimeRange{From: now.Add(-1 * time.Hour), To: now.Add(1 * time.Hour)}
	assert.True(t, inTimeRange(now, tr))
	assert.False(t, inTimeRange(now.Add(-2*time.Hour), tr))
	assert.False(t, inTimeRange(now.Add(2*time.Hour), tr))
}

// --- Phase 6: Feedback status update + score override tests ---

func TestFeedbackUpdatesCycleStatus(t *testing.T) {
	svc, g := newTestService(t)

	cycle, _ := g.CreateNode("ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "REQUESTED",
	})

	_, err := svc.Feedback(FeedbackInput{
		CycleID:      string(cycle.ID),
		FeedbackType: "confirm",
		Scores:       FeedbackScores{Impact: 0.8},
	})
	require.NoError(t, err)

	// Verify cycle status updated to RECEIVED
	updated, err := g.GetNode(cycle.ID)
	require.NoError(t, err)
	assert.Equal(t, "RECEIVED", updated.Properties["humanFeedbackStatus"])
}

func TestFeedbackAppliesScoreOverrides(t *testing.T) {
	svc, g := newTestService(t)

	cycle, _ := g.CreateNode("ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "REQUESTED",
	})

	sig, _ := g.CreateNode("LearningSignal", graph.Properties{"summary": "test"})
	edge, _ := g.CreateEdge("LINKS_TO", sig.ID, cycle.ID, graph.Properties{
		"impact": 0.5, "relevance": 0.5,
	})

	_, err := svc.Feedback(FeedbackInput{
		CycleID: string(cycle.ID),
		Scores:  FeedbackScores{Impact: 0.9, Relevance: 0.8, ValueContribution: 0.7},
	})
	require.NoError(t, err)

	// Verify edge scores overridden
	updatedEdge, err := g.GetEdge(edge.ID)
	require.NoError(t, err)
	assert.Equal(t, 0.9, updatedEdge.Properties["impact"])
	assert.Equal(t, 0.8, updatedEdge.Properties["relevance"])
	assert.Equal(t, 0.7, updatedEdge.Properties["valueContribution"])
}

func TestFeedbackPartialScoreOverrides(t *testing.T) {
	svc, g := newTestService(t)

	cycle, _ := g.CreateNode("ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "REQUESTED",
	})

	sig, _ := g.CreateNode("LearningSignal", graph.Properties{"summary": "test"})
	edge, _ := g.CreateEdge("LINKS_TO", sig.ID, cycle.ID, graph.Properties{
		"impact": 0.5, "relevance": 0.5,
	})

	// Only override impact (relevance and vc are 0 → skip)
	_, err := svc.Feedback(FeedbackInput{
		CycleID: string(cycle.ID),
		Scores:  FeedbackScores{Impact: 0.9},
	})
	require.NoError(t, err)

	updatedEdge, err := g.GetEdge(edge.ID)
	require.NoError(t, err)
	assert.Equal(t, 0.9, updatedEdge.Properties["impact"])
	assert.Equal(t, 0.5, updatedEdge.Properties["relevance"]) // unchanged
}

// --- Mocks ---

type mockFullText struct {
	results []search.TextSearchResult
}

func (m *mockFullText) Index(_ *graph.Node) error   { return nil }
func (m *mockFullText) Remove(_ graph.NodeID) error { return nil }
func (m *mockFullText) Close() error                { return nil }
func (m *mockFullText) Search(_, _, _ string, _ int) ([]search.TextSearchResult, error) {
	return m.results, nil
}
