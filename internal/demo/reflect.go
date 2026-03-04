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

package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/ingest"
	reflectpkg "github.com/scalytics/kafgraph/internal/reflect"
)

// ReflectionResult holds the outcome of the demo reflection step.
type ReflectionResult struct {
	CyclesRun      int
	SignalsCreated int
	FeedbackGiven  int
}

// RunReflections executes daily reflection cycles for each agent in the
// blog-team scenario, then processes human feedback for the first cycle.
// This populates the graph with ReflectionCycle, LearningSignal, and
// HumanFeedback nodes so the management UI reflection dashboard has data.
func RunReflections(ctx context.Context, g *graph.Graph) (*ReflectionResult, error) {
	analyzer := reflectpkg.NewHeuristicAnalyzer(g)
	runner := reflectpkg.NewCycleRunnerWithAnalyzer(g, analyzer)
	result := &ReflectionResult{}

	// Node CreatedAt is set to time.Now() during ingestion, not the envelope
	// timestamp. Use a window covering the current day so the reflection
	// engine finds the nodes we just created.
	now := time.Now().UTC()
	windowStart := reflectpkg.DailyWindowStart(now)
	windowEnd := windowStart.Add(24 * time.Hour)

	agents := []string{Coordinator, Researcher, Editor, Formatter}
	var firstCycleID graph.NodeID

	for _, agentID := range agents {
		cycleResult, err := runner.Execute(ctx, reflectpkg.CycleRequest{
			Type:        reflectpkg.CycleDaily,
			AgentID:     agentID,
			WindowStart: windowStart,
			WindowEnd:   windowEnd,
		})
		if err != nil {
			return nil, fmt.Errorf("reflection for %s: %w", agentID, err)
		}
		result.CyclesRun++
		result.SignalsCreated += len(cycleResult.LearningSignals)

		if firstCycleID == "" {
			firstCycleID = cycleResult.CycleID
		}
	}

	// Submit human feedback on the first cycle (coordinator's) to exercise
	// the feedback pipeline.
	if firstCycleID != "" {
		if err := submitFeedback(ctx, g, firstCycleID); err != nil {
			return nil, fmt.Errorf("submit feedback: %w", err)
		}
		result.FeedbackGiven++
	}

	return result, nil
}

// submitFeedback processes a human_feedback envelope targeting the given cycle.
func submitFeedback(ctx context.Context, g *graph.Graph, cycleID graph.NodeID) error {
	payload := ingest.HumanFeedbackPayload{
		CycleID:           string(cycleID),
		FeedbackType:      "positive",
		Comment:           "Good coordination across the blog writing pipeline. The research phase produced high-quality citations.",
		Impact:            0.85,
		Relevance:         0.90,
		ValueContribution: 0.80,
		ReviewerID:        "demo-reviewer",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	envelope := ingest.GroupEnvelope{
		Type:          ingest.TypeHumanFeedback,
		CorrelationID: CorrelationID,
		SenderID:      "demo-reviewer",
		Timestamp:     time.Now().UTC(),
		Payload:       payloadBytes,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	rec := ingest.Record{
		Source: ingest.SourceOffset{Topic: Topic, Partition: Partition, Offset: 100},
		Value:  data,
	}

	// Process directly via ProcessRecord to avoid poll-loop log noise.
	proc := ingest.NewProcessor(ingest.NewMemoryReader(), g, ingest.ProcessorConfig{
		Namespace: "demo-feedback",
	})
	return proc.ProcessRecord(ctx, rec)
}
