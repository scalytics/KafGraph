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
	"strings"
	"testing"

	"github.com/scalytics/kafgraph/internal/graph"
)

func TestHeuristicAnalyzer_AnalyzeText_Empty(t *testing.T) {
	g := newTestGraph(t)
	ha := NewHeuristicAnalyzer(g)

	result := ha.AnalyzeText("")
	if len(result.Entities) != 0 || len(result.Keywords) != 0 || len(result.Tags) != 0 {
		t.Errorf("expected empty result for empty text, got %+v", result)
	}
}

func TestHeuristicAnalyzer_EntityRecognition(t *testing.T) {
	g := newTestGraph(t)
	g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})   //nolint:errcheck
	g.UpsertNode("n:Agent:bob", "Agent", graph.Properties{"name": "bob"})       //nolint:errcheck
	g.UpsertNode("n:Skill:search", "Skill", graph.Properties{"name": "search"}) //nolint:errcheck

	ha := NewHeuristicAnalyzer(g)

	// Build corpus for keyword extraction.
	corpus := NewTFIDFCorpus()
	corpus.AddDocument("alice discussed search results with bob about distributed systems")
	corpus.AddDocument("the team reviewed deployment strategies")
	ha.SetCorpus(corpus)

	result := ha.AnalyzeText("alice discussed search results with bob about distributed systems")

	// Should find alice, bob, and search as entities.
	foundAgent := 0
	foundSkill := 0
	for _, e := range result.Entities {
		switch e.Type {
		case EntityAgent:
			foundAgent++
			if e.Confidence != 1.0 {
				t.Errorf("expected confidence 1.0 for agent, got %f", e.Confidence)
			}
		case EntitySkill:
			foundSkill++
		}
	}
	if foundAgent < 2 {
		t.Errorf("expected at least 2 agent entities, got %d", foundAgent)
	}
	if foundSkill < 1 {
		t.Errorf("expected at least 1 skill entity, got %d", foundSkill)
	}

	// Should have keywords.
	if len(result.Keywords) == 0 {
		t.Error("expected keywords")
	}

	// Should have tags.
	if len(result.Tags) == 0 {
		t.Error("expected tags")
	}
}

func TestHeuristicAnalyzer_SummarizeSignals(t *testing.T) {
	g := newTestGraph(t)
	ha := NewHeuristicAnalyzer(g)

	signals := []ScoredSignal{
		{NodeID: "n:1", Label: "Message", Summary: "msg1", Keywords: []Keyword{{Term: "distributed", Score: 0.5}}},
		{NodeID: "n:2", Label: "Message", Summary: "msg2", Keywords: []Keyword{{Term: "distributed", Score: 0.4}}},
		{NodeID: "n:3", Label: "Conversation", Summary: "conv1", Entities: []Entity{{Name: "alice", Type: EntityAgent}}},
	}

	summary := ha.SummarizeSignals(signals)
	if !strings.Contains(summary, "3 signals") {
		t.Errorf("expected '3 signals' in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "message") {
		t.Errorf("expected 'message' in summary, got: %s", summary)
	}
}

func TestHeuristicAnalyzer_SummarizeSignals_Empty(t *testing.T) {
	g := newTestGraph(t)
	ha := NewHeuristicAnalyzer(g)

	summary := ha.SummarizeSignals(nil)
	if summary != "No activity in window." {
		t.Errorf("expected 'No activity in window.', got: %s", summary)
	}
}

func TestHeuristicAnalyzer_DetectPatterns(t *testing.T) {
	g := newTestGraph(t)
	ha := NewHeuristicAnalyzer(g)

	signals := []ScoredSignal{
		{
			NodeID:   "n:1",
			Entities: []Entity{{Name: "alice", Type: EntityAgent}, {Name: "bob", Type: EntityAgent}},
			Keywords: []Keyword{{Term: "coordination", Score: 0.5}},
		},
		{
			NodeID:   "n:2",
			Entities: []Entity{{Name: "alice", Type: EntityAgent}, {Name: "bob", Type: EntityAgent}},
			Keywords: []Keyword{{Term: "coordination", Score: 0.4}},
		},
		{
			NodeID:   "n:3",
			Entities: []Entity{{Name: "alice", Type: EntityAgent}},
			Keywords: []Keyword{{Term: "research", Score: 0.3}},
		},
	}

	patterns := ha.DetectPatterns(signals)
	if len(patterns) == 0 {
		t.Fatal("expected patterns")
	}

	// Should detect alice + bob co-occurrence.
	found := false
	for _, p := range patterns {
		if strings.Contains(p.Theme, "alice") && strings.Contains(p.Theme, "bob") {
			found = true
			if p.Occurrences < 2 {
				t.Errorf("expected at least 2 occurrences, got %d", p.Occurrences)
			}
		}
	}
	if !found {
		t.Error("expected alice+bob co-occurrence pattern")
	}
}

func TestHeuristicAnalyzer_DetectPatterns_Empty(t *testing.T) {
	g := newTestGraph(t)
	ha := NewHeuristicAnalyzer(g)
	patterns := ha.DetectPatterns(nil)
	if patterns != nil {
		t.Errorf("expected nil patterns for empty input, got %v", patterns)
	}
}

func TestHeuristicAnalyzer_ComputeTrend(t *testing.T) {
	g := newTestGraph(t)
	ha := NewHeuristicAnalyzer(g)

	tests := []struct {
		name     string
		current  []Pattern
		prior    []Pattern
		expected string
	}{
		{
			name:     "both empty",
			current:  nil,
			prior:    nil,
			expected: "stable",
		},
		{
			name:     "new patterns, no prior",
			current:  []Pattern{{Theme: "a"}, {Theme: "b"}},
			prior:    nil,
			expected: "rising",
		},
		{
			name:     "no current, had prior",
			current:  nil,
			prior:    []Pattern{{Theme: "a"}, {Theme: "b"}},
			expected: "declining",
		},
		{
			name:     "identical sets",
			current:  []Pattern{{Theme: "a"}, {Theme: "b"}, {Theme: "c"}},
			prior:    []Pattern{{Theme: "a"}, {Theme: "b"}, {Theme: "c"}},
			expected: "stable",
		},
		{
			name:     "high overlap",
			current:  []Pattern{{Theme: "a"}, {Theme: "b"}, {Theme: "c"}, {Theme: "d"}},
			prior:    []Pattern{{Theme: "a"}, {Theme: "b"}, {Theme: "c"}, {Theme: "e"}},
			expected: "stable",
		},
		{
			name:     "moderate overlap",
			current:  []Pattern{{Theme: "a"}, {Theme: "b"}, {Theme: "c"}, {Theme: "d"}},
			prior:    []Pattern{{Theme: "a"}, {Theme: "b"}, {Theme: "e"}, {Theme: "f"}},
			expected: "shifting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ha.ComputeTrend(tt.current, tt.prior)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestHeuristicAnalyzer_AutoTag_MaxTags(t *testing.T) {
	g := newTestGraph(t)
	ha := NewHeuristicAnalyzer(g)

	// Create lots of entities and keywords.
	entities := make([]Entity, 10)
	for i := range entities {
		entities[i] = Entity{Name: strings.Repeat("x", i+3), Type: EntityTopic, Confidence: 0.5}
	}
	keywords := make([]Keyword, 10)
	for i := range keywords {
		keywords[i] = Keyword{Term: strings.Repeat("y", i+3), Score: float64(10 - i)}
	}

	tags := ha.autoTag(entities, keywords)
	if len(tags) > 8 {
		t.Errorf("expected at most 8 tags, got %d", len(tags))
	}
}

func TestHeuristicAnalyzer_RefreshKnowledge(t *testing.T) {
	g := newTestGraph(t)
	ha := NewHeuristicAnalyzer(g)

	// Initially no agents.
	if len(ha.agentNames) != 0 {
		t.Errorf("expected 0 agent names, got %d", len(ha.agentNames))
	}

	// Add agent and refresh.
	g.UpsertNode("n:Agent:charlie", "Agent", graph.Properties{"name": "charlie"}) //nolint:errcheck
	ha.RefreshKnowledge()

	if !ha.agentNames["charlie"] {
		t.Error("expected 'charlie' in agent names after refresh")
	}
}

func TestNewCycleRunnerWithAnalyzer(t *testing.T) {
	g := newTestGraph(t)
	ha := NewHeuristicAnalyzer(g)
	cr := NewCycleRunnerWithAnalyzer(g, ha)
	if cr.analyzer == nil {
		t.Error("expected analyzer to be set")
	}
}
