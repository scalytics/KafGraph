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
	"fmt"
	"strings"
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
)

// CycleRunner executes a single reflection cycle.
type CycleRunner struct {
	graph    *graph.Graph
	iterator *HistoricIterator
	scorer   *Scorer
	nowFunc  func() time.Time
}

// NewCycleRunner creates a new cycle runner.
func NewCycleRunner(g *graph.Graph) *CycleRunner {
	return &CycleRunner{
		graph:    g,
		iterator: NewHistoricIterator(g),
		scorer:   NewScorer(g),
		nowFunc:  time.Now,
	}
}

// Execute runs a single reflection cycle.
func (cr *CycleRunner) Execute(ctx context.Context, req CycleRequest) (*CycleResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cycleID := CycleNodeID(req.Type, req.AgentID, req.WindowStart)

	// 1. Upsert ReflectionCycle node with status RUNNING
	_, err := cr.graph.UpsertNode(cycleID, "ReflectionCycle", graph.Properties{
		"agentId":             req.AgentID,
		"cycleType":           string(req.Type),
		"windowStart":         req.WindowStart.Format(time.RFC3339),
		"windowEnd":           req.WindowEnd.Format(time.RFC3339),
		"startedAt":           cr.nowFunc().Format(time.RFC3339),
		"status":              "RUNNING",
		"humanFeedbackStatus": "PENDING",
	})
	if err != nil {
		return nil, fmt.Errorf("upsert cycle node: %w", err)
	}

	// 2. Link cycle to agent
	agentNodeID := graph.NodeID(fmt.Sprintf("n:Agent:%s", req.AgentID))
	edgeID := ScoreEdgeID("TRIGGERED_REFLECTION", agentNodeID, cycleID)
	cr.graph.UpsertEdge(edgeID, "TRIGGERED_REFLECTION", agentNodeID, cycleID, nil) //nolint:errcheck,gosec // best-effort linking

	// 3. Gather nodes in window
	labels := []string{"Message", "Conversation", "LearningSignal"}
	nodes, err := cr.iterator.AgentNodesInWindow(agentNodeID, labels, req.WindowStart, req.WindowEnd)
	if err != nil {
		return nil, fmt.Errorf("iterate agent nodes: %w", err)
	}

	// 4. Find conversation context for scoring
	convMap := make(map[graph.NodeID]*graph.Node)
	for _, n := range nodes {
		if n.Label == "Conversation" {
			convMap[n.ID] = n
		}
	}

	// 5. Score each node and create LearningSignal nodes
	var signals []ScoredSignal
	var summaryParts []string
	for _, n := range nodes {
		// Find conversation for this node
		var conv *graph.Node
		for _, c := range convMap {
			conv = c
			break // use first conversation as context
		}

		scored := cr.scorer.ScoreNode(n, conv)
		signals = append(signals, scored)

		// Create deterministic LearningSignal node
		sigID := SignalNodeID(req.Type, req.AgentID, req.WindowStart, n.ID)
		cr.graph.UpsertNode(sigID, "LearningSignal", graph.Properties{ //nolint:errcheck,gosec // best-effort
			"sourceNodeId":      string(n.ID),
			"impact":            scored.Impact,
			"relevance":         scored.Relevance,
			"valueContribution": scored.ValueContribution,
			"summary":           scored.Summary,
			"cycleType":         string(req.Type),
			"agentId":           req.AgentID,
		})

		// Link signal to cycle
		linkID := ScoreEdgeID("LINKS_TO", sigID, cycleID)
		cr.graph.UpsertEdge(linkID, "LINKS_TO", sigID, cycleID, graph.Properties{ //nolint:errcheck,gosec // best-effort
			"impact":            scored.Impact,
			"relevance":         scored.Relevance,
			"valueContribution": scored.ValueContribution,
		})

		summaryParts = append(summaryParts, scored.Summary)
	}

	// 6. For weekly/monthly: load prior cycles and aggregate scores
	if req.Type == CycleWeekly || req.Type == CycleMonthly {
		cr.aggregatePriorCycles(req, cycleID)
	}

	// 7. Update cycle status to COMPLETED
	cr.graph.UpsertNode(cycleID, "ReflectionCycle", graph.Properties{ //nolint:errcheck,gosec // best-effort
		"status":      "COMPLETED",
		"completedAt": cr.nowFunc().Format(time.RFC3339),
	})

	summary := "No activity in window."
	if len(summaryParts) > 0 {
		summary = fmt.Sprintf("Found %d signals: %s",
			len(summaryParts), strings.Join(summaryParts, "; "))
	}

	return &CycleResult{
		CycleID:         cycleID,
		Status:          "COMPLETED",
		LearningSignals: signals,
		Summary:         summary,
	}, nil
}

// aggregatePriorCycles loads prior daily/weekly cycles within the window
// and creates CONTRIBUTED_VALUE edges with averaged scores.
func (cr *CycleRunner) aggregatePriorCycles(req CycleRequest, cycleID graph.NodeID) {
	priorLabel := "ReflectionCycle"
	priors, err := cr.graph.NodesByLabel(priorLabel)
	if err != nil {
		return
	}

	var targetType string
	switch req.Type {
	case CycleWeekly:
		targetType = string(CycleDaily)
	case CycleMonthly:
		targetType = string(CycleWeekly)
	default:
		return
	}

	for _, prior := range priors {
		ct, _ := prior.Properties["cycleType"].(string)
		if ct != targetType {
			continue
		}
		wsStr, _ := prior.Properties["windowStart"].(string)
		ws, err := time.Parse(time.RFC3339, wsStr)
		if err != nil {
			continue
		}
		if ws.Before(req.WindowStart) || !ws.Before(req.WindowEnd) {
			continue
		}
		edgeID := ScoreEdgeID("ROLLUP_OF", cycleID, prior.ID)
		cr.graph.UpsertEdge(edgeID, "ROLLUP_OF", cycleID, prior.ID, nil) //nolint:errcheck,gosec // best-effort
	}
}

// ExecuteForBrain adapts CycleRunner for the brain.ReflectionRunner interface.
func (cr *CycleRunner) ExecuteForBrain(ctx context.Context, agentID string, windowHours int) (*CycleResult, error) {
	now := cr.nowFunc()
	ws := DailyWindowStart(now)
	we := now
	if windowHours > 0 {
		we = ws.Add(time.Duration(windowHours) * time.Hour)
		if we.After(now) {
			we = now
		}
	}

	return cr.Execute(ctx, CycleRequest{
		Type:        CycleDaily,
		AgentID:     agentID,
		WindowStart: ws,
		WindowEnd:   we,
	})
}
