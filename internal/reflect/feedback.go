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
	"fmt"
	"sort"
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
)

// Publisher sends messages to an external topic. Defined in the reflect package
// to avoid import cycles — ingest.MemoryPublisher satisfies this via Go
// structural typing.
type Publisher interface {
	Publish(ctx context.Context, topic string, key string, data []byte) error
}

// FeedbackChecker monitors feedback grace periods on reflection cycles.
type FeedbackChecker struct {
	graph        *graph.Graph
	gracePeriod  time.Duration
	nowFunc      func() time.Time
	publisher    Publisher
	requestTopic string
	topN         int
}

// NewFeedbackChecker creates a new feedback checker.
func NewFeedbackChecker(g *graph.Graph, gracePeriod time.Duration) *FeedbackChecker {
	return &FeedbackChecker{
		graph:       g,
		gracePeriod: gracePeriod,
		nowFunc:     time.Now,
		topN:        5,
	}
}

// WithPublisher configures event emission for feedback requests.
func (fc *FeedbackChecker) WithPublisher(p Publisher, topic string, topN int) {
	fc.publisher = p
	fc.requestTopic = topic
	if topN > 0 {
		fc.topN = topN
	}
}

// CheckPending queries ReflectionCycle nodes with status=COMPLETED.
// It handles two transitions:
//  1. PENDING → NEEDS_FEEDBACK (grace period expired)
//  2. NEEDS_FEEDBACK → REQUESTED (emit event via publisher)
func (fc *FeedbackChecker) CheckPending(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	cycles, err := fc.graph.NodesByLabel("ReflectionCycle")
	if err != nil {
		return fmt.Errorf("list reflection cycles: %w", err)
	}

	now := fc.nowFunc()
	for _, cycle := range cycles {
		status, _ := cycle.Properties["status"].(string)
		fbStatus, _ := cycle.Properties["humanFeedbackStatus"].(string)

		if status != "COMPLETED" {
			continue
		}

		switch FeedbackStatus(fbStatus) {
		case FBPending:
			fc.checkGracePeriod(cycle, now)
		case FBNeedsFeedback:
			fc.requestFeedback(ctx, cycle, now)
		}
	}

	return nil
}

// checkGracePeriod transitions PENDING → NEEDS_FEEDBACK when the grace period expires.
func (fc *FeedbackChecker) checkGracePeriod(cycle *graph.Node, now time.Time) {
	completedStr, _ := cycle.Properties["completedAt"].(string)
	if completedStr == "" {
		return
	}
	completedAt, err := time.Parse(time.RFC3339, completedStr)
	if err != nil {
		return
	}

	if now.Sub(completedAt) >= fc.gracePeriod {
		fc.graph.UpsertNode(cycle.ID, "ReflectionCycle", graph.Properties{ //nolint:errcheck,gosec // best-effort
			"humanFeedbackStatus": string(FBNeedsFeedback),
		})
	}
}

// requestFeedback transitions NEEDS_FEEDBACK → REQUESTED by emitting a
// FeedbackRequestEvent via the configured publisher.
func (fc *FeedbackChecker) requestFeedback(ctx context.Context, cycle *graph.Node, now time.Time) {
	if fc.publisher == nil {
		return
	}

	agentID, _ := cycle.Properties["agentId"].(string)
	cycleType, _ := cycle.Properties["cycleType"].(string)
	windowStart, _ := cycle.Properties["windowStart"].(string)
	windowEnd, _ := cycle.Properties["windowEnd"].(string)

	topSignals := fc.gatherTopSignals(cycle.ID, fc.topN)

	event := FeedbackRequestEvent{
		CycleID:     string(cycle.ID),
		AgentID:     agentID,
		CycleType:   cycleType,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		TopSignals:  topSignals,
		RequestedAt: now.Format(time.RFC3339),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	if err := fc.publisher.Publish(ctx, fc.requestTopic, string(cycle.ID), data); err != nil {
		return
	}

	fc.graph.UpsertNode(cycle.ID, "ReflectionCycle", graph.Properties{ //nolint:errcheck,gosec // best-effort
		"humanFeedbackStatus": string(FBRequested),
		"feedbackRequestedAt": now.Format(time.RFC3339),
	})
}

// gatherTopSignals finds LearningSignal nodes linked to the cycle, sorted by impact desc.
func (fc *FeedbackChecker) gatherTopSignals(cycleID graph.NodeID, n int) []SignalSummary {
	edges, err := fc.graph.Neighbors(cycleID)
	if err != nil {
		return nil
	}

	type signalWithScore struct {
		summary SignalSummary
		impact  float64
	}

	var signals []signalWithScore
	for _, edge := range edges {
		// Find edges pointing TO this cycle (LINKS_TO signal→cycle)
		if edge.ToID != cycleID {
			continue
		}
		node, err := fc.graph.GetNode(edge.FromID)
		if err != nil || node.Label != "LearningSignal" {
			continue
		}

		impact, _ := node.Properties["impact"].(float64)
		relevance, _ := node.Properties["relevance"].(float64)
		valueContribution, _ := node.Properties["valueContribution"].(float64)
		summary, _ := node.Properties["summary"].(string)
		label, _ := node.Properties["label"].(string)

		signals = append(signals, signalWithScore{
			summary: SignalSummary{
				SignalID:          string(node.ID),
				Label:             label,
				Summary:           summary,
				Impact:            impact,
				Relevance:         relevance,
				ValueContribution: valueContribution,
			},
			impact: impact,
		})
	}

	sort.Slice(signals, func(i, j int) bool {
		return signals[i].impact > signals[j].impact
	})

	if len(signals) > n {
		signals = signals[:n]
	}

	result := make([]SignalSummary, len(signals))
	for i, s := range signals {
		result[i] = s.summary
	}
	return result
}
