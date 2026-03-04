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

package demo_test

import (
	"context"
	"testing"

	"github.com/scalytics/kafgraph/internal/demo"
	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/ingest"
	"github.com/scalytics/kafgraph/internal/storage"
)

func TestBlogTeamScenario(t *testing.T) {
	records := demo.BlogTeamScenario()

	if got := len(records); got != 47 {
		t.Fatalf("expected 47 records, got %d", got)
	}

	// Verify all records have the expected topic and sequential offsets.
	for i, rec := range records {
		if rec.Source.Topic != demo.Topic {
			t.Errorf("record %d: topic = %q, want %q", i, rec.Source.Topic, demo.Topic)
		}
		if rec.Source.Partition != demo.Partition {
			t.Errorf("record %d: partition = %d, want %d", i, rec.Source.Partition, demo.Partition)
		}
		if rec.Source.Offset != int64(i) {
			t.Errorf("record %d: offset = %d, want %d", i, rec.Source.Offset, i)
		}
	}
}

func TestBlogTeamScenarioIngestion(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewBadgerStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	g := graph.New(store)
	defer func() { _ = g.Close() }()

	// Load records into MemoryReader and process them.
	reader := ingest.NewMemoryReader()
	records := demo.BlogTeamScenario()
	for _, rec := range records {
		reader.AddRecord(rec.Source.Topic, rec.Source.Partition, rec.Source.Offset, rec.Value)
	}

	proc := ingest.NewProcessor(reader, g, ingest.ProcessorConfig{
		PollInterval: 50_000_000, // 50ms — just needs one tick
		BatchSize:    100,
		Namespace:    "demo-test",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500_000_000) // 500ms
	defer cancel()
	_ = proc.Run(ctx)

	// Verify node counts by label.
	assertNodeCount(t, g, "Agent", 4)
	assertNodeCount(t, g, "Conversation", 1)
	assertNodeCount(t, g, "Skill", 9)          // web_search, summarize, deep_search, rewrite, tone_check, citation_check, ascii_doc, proofread, format_html
	assertNodeCount(t, g, "SharedMemory", 3)   // research-findings, editorial-notes, final-blog
	assertNodeCount(t, g, "AuditEvent", 4)     // 3 task_completed + 1 pipeline_completed
	assertMinNodeCount(t, g, "Message", 15)    // requests + responses + skill_requests + skill_responses
}

func TestRunReflections(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewBadgerStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	g := graph.New(store)
	defer func() { _ = g.Close() }()

	// First seed conversation data.
	reader := ingest.NewMemoryReader()
	records := demo.BlogTeamScenario()
	for _, rec := range records {
		reader.AddRecord(rec.Source.Topic, rec.Source.Partition, rec.Source.Offset, rec.Value)
	}
	proc := ingest.NewProcessor(reader, g, ingest.ProcessorConfig{
		PollInterval: 50_000_000,
		BatchSize:    100,
		Namespace:    "demo-refl-test",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 500_000_000)
	defer cancel()
	_ = proc.Run(ctx)

	// Run reflections.
	result, err := demo.RunReflections(context.Background(), g)
	if err != nil {
		t.Fatalf("RunReflections: %v", err)
	}

	if result.CyclesRun != 4 {
		t.Errorf("CyclesRun = %d, want 4", result.CyclesRun)
	}
	if result.FeedbackGiven != 1 {
		t.Errorf("FeedbackGiven = %d, want 1", result.FeedbackGiven)
	}

	// Verify reflection nodes exist in the graph.
	assertNodeCount(t, g, "ReflectionCycle", 4)
	assertMinNodeCount(t, g, "LearningSignal", 1)
	assertNodeCount(t, g, "HumanFeedback", 1)
}

func assertNodeCount(t *testing.T, g *graph.Graph, label string, want int) {
	t.Helper()
	nodes, err := g.NodesByLabel(label)
	if err != nil {
		t.Fatalf("NodesByLabel(%q): %v", label, err)
	}
	if got := len(nodes); got != want {
		t.Errorf("%s nodes: got %d, want %d", label, got, want)
	}
}

func assertMinNodeCount(t *testing.T, g *graph.Graph, label string, min int) {
	t.Helper()
	nodes, err := g.NodesByLabel(label)
	if err != nil {
		t.Fatalf("NodesByLabel(%q): %v", label, err)
	}
	if got := len(nodes); got < min {
		t.Errorf("%s nodes: got %d, want >= %d", label, got, min)
	}
}
