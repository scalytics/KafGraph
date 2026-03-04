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

package compliance

import "time"

// Framework identifies a compliance framework.
type Framework string

const (
	FrameworkGDPR  Framework = "gdpr"
	FrameworkSOC2  Framework = "soc2"
	FrameworkAIAct Framework = "ai_act"
)

// Severity levels for compliance rules.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// SeverityWeight returns the numeric weight for score calculation.
func (s Severity) Weight() float64 {
	switch s {
	case SeverityCritical:
		return 3.0
	case SeverityHigh:
		return 2.0
	case SeverityMedium:
		return 1.0
	case SeverityLow:
		return 0.5
	default:
		return 1.0
	}
}

// EvalStatus is the result status of a rule evaluation.
type EvalStatus string

const (
	EvalPass    EvalStatus = "pass"
	EvalFail    EvalStatus = "fail"
	EvalWarning EvalStatus = "warning"
	EvalNA      EvalStatus = "na"
)

// Rule is the interface every compliance check must satisfy.
type Rule interface {
	ID() string
	Framework() Framework
	Module() string
	Article() string
	Title() string
	Severity() Severity
	Evaluate(g GraphQuerier) ([]RuleResult, error)
}

// GraphQuerier abstracts the graph operations needed by compliance rules.
type GraphQuerier interface {
	NodesByLabel(label string) (NodeList, error)
	Neighbors(id string) (EdgeList, error)
}

// NodeItem represents a graph node for compliance evaluation.
type NodeItem struct {
	ID         string
	Label      string
	Properties map[string]any
}

// NodeList is a slice of NodeItem.
type NodeList []NodeItem

// EdgeItem represents a graph edge for compliance evaluation.
type EdgeItem struct {
	ID    string
	Label string
	From  string
	To    string
}

// EdgeList is a slice of EdgeItem.
type EdgeList []EdgeItem

// RuleResult is the outcome of evaluating a single rule against one entity.
type RuleResult struct {
	RuleID   string     `json:"ruleId"`
	Status   EvalStatus `json:"status"`
	NodeID   string     `json:"nodeId,omitempty"`
	Details  string     `json:"details"`
	Severity Severity   `json:"severity"`
}

// ScanRequest specifies what to scan.
type ScanRequest struct {
	Framework Framework `json:"framework"`
	Module    string    `json:"module,omitempty"`
}

// ScanResult is the aggregated result of a compliance scan.
type ScanResult struct {
	ScanID      string       `json:"scanId"`
	Framework   Framework    `json:"framework"`
	TriggeredBy string       `json:"triggeredBy"`
	StartedAt   time.Time    `json:"startedAt"`
	CompletedAt time.Time    `json:"completedAt"`
	PassCount   int          `json:"passCount"`
	FailCount   int          `json:"failCount"`
	WarningCount int         `json:"warningCount"`
	NACount     int          `json:"naCount"`
	Score       float64      `json:"score"`
	Evaluations []RuleResult `json:"evaluations"`
}

// ScoreSummary holds per-framework compliance scores.
type ScoreSummary struct {
	Framework   Framework `json:"framework"`
	Score       float64   `json:"score"`
	PassCount   int       `json:"passCount"`
	FailCount   int       `json:"failCount"`
	TotalRules  int       `json:"totalRules"`
	LastScanAt  string    `json:"lastScanAt,omitempty"`
}

// DashboardData aggregates data for the compliance dashboard.
type DashboardData struct {
	Frameworks []ScoreSummary `json:"frameworks"`
	RecentScan *ScanResult    `json:"recentScan,omitempty"`
	TotalRules int            `json:"totalRules"`
}
