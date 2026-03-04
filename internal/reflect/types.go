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
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
)

// CycleType identifies the cadence of a reflection cycle.
type CycleType string

// Reflection cycle cadences.
const (
	CycleDaily   CycleType = "daily"
	CycleWeekly  CycleType = "weekly"
	CycleMonthly CycleType = "monthly"
)

// CycleRequest describes a reflection cycle to execute.
type CycleRequest struct {
	Type        CycleType
	AgentID     string
	WindowStart time.Time
	WindowEnd   time.Time
}

// ScoredSignal is a graph node with heuristic scores attached.
type ScoredSignal struct {
	NodeID            graph.NodeID
	Label             string
	Summary           string
	Impact            float64
	Relevance         float64
	ValueContribution float64
	// Enriched by Analyzer (nil/empty when analyzer is not set).
	Entities []Entity
	Keywords []Keyword
	Tags     []string
}

// CycleResult holds the outcome of a completed reflection cycle.
type CycleResult struct {
	CycleID         graph.NodeID
	Status          string
	LearningSignals []ScoredSignal
	Summary         string
}

// FeedbackRequestEvent is emitted when a reflection cycle needs human feedback.
type FeedbackRequestEvent struct {
	CycleID     string          `json:"cycleId"`
	AgentID     string          `json:"agentId"`
	CycleType   string          `json:"cycleType"`
	WindowStart string          `json:"windowStart"`
	WindowEnd   string          `json:"windowEnd"`
	TopSignals  []SignalSummary `json:"topSignals"`
	RequestedAt string          `json:"requestedAt"`
}

// SignalSummary is a compact representation of a learning signal for feedback requests.
type SignalSummary struct {
	SignalID          string  `json:"signalId"`
	Label             string  `json:"label"`
	Summary           string  `json:"summary"`
	Impact            float64 `json:"impact"`
	Relevance         float64 `json:"relevance"`
	ValueContribution float64 `json:"valueContribution"`
}
