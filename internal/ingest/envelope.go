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

package ingest

import (
	"encoding/json"
	"fmt"
	"time"
)

// EnvelopeType identifies the kind of KafClaw GroupEnvelope.
type EnvelopeType string

// KafClaw envelope types.
const (
	TypeAnnounce      EnvelopeType = "announce"
	TypeRequest       EnvelopeType = "request"
	TypeResponse      EnvelopeType = "response"
	TypeTaskStatus    EnvelopeType = "task_status"
	TypeSkillRequest  EnvelopeType = "skill_request"
	TypeSkillResponse EnvelopeType = "skill_response"
	TypeMemory        EnvelopeType = "memory"
	TypeTrace         EnvelopeType = "trace"
	TypeAudit         EnvelopeType = "audit"
	TypeRoster        EnvelopeType = "roster"
	TypeOrchestrator  EnvelopeType = "orchestrator"
	TypeHumanFeedback EnvelopeType = "human_feedback"
)

// GroupEnvelope is the wire format for KafClaw agent messages.
type GroupEnvelope struct {
	Type          EnvelopeType    `json:"Type"`
	CorrelationID string          `json:"CorrelationID"`
	SenderID      string          `json:"SenderID"`
	Timestamp     time.Time       `json:"Timestamp"`
	Payload       json.RawMessage `json:"Payload"`
}

// ParseEnvelope decodes a JSON-encoded GroupEnvelope.
func ParseEnvelope(data []byte) (*GroupEnvelope, error) {
	var env GroupEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse envelope: %w", err)
	}
	if env.Type == "" {
		return nil, fmt.Errorf("parse envelope: missing Type field")
	}
	return &env, nil
}

// AnnouncePayload contains agent lifecycle events (join, leave, heartbeat).
type AnnouncePayload struct {
	AgentID   string `json:"AgentID"`
	AgentName string `json:"AgentName"`
	Action    string `json:"Action"` // "join", "leave", "heartbeat"
	GroupName string `json:"GroupName"`
}

// TaskRequestPayload contains a task request from one agent to another.
type TaskRequestPayload struct {
	TaskID      string `json:"TaskID"`
	RequestText string `json:"RequestText"`
	TargetAgent string `json:"TargetAgent"`
}

// TaskResponsePayload contains a response to a task request.
type TaskResponsePayload struct {
	TaskID       string `json:"TaskID"`
	ResponseText string `json:"ResponseText"`
	InReplyTo    string `json:"InReplyTo"`
}

// TaskStatusPayload contains task status updates.
type TaskStatusPayload struct {
	TaskID string `json:"TaskID"`
	Status string `json:"Status"` // "pending", "in_progress", "completed", "failed"
}

// SkillRequestPayload contains a skill invocation request.
type SkillRequestPayload struct {
	SkillName  string          `json:"SkillName"`
	Parameters json.RawMessage `json:"Parameters"`
}

// SkillResponsePayload contains the result of a skill invocation.
type SkillResponsePayload struct {
	SkillName string `json:"SkillName"`
	Result    string `json:"Result"`
	InReplyTo string `json:"InReplyTo"`
}

// MemoryPayload contains shared memory entries.
type MemoryPayload struct {
	Key        string   `json:"Key"`
	Value      string   `json:"Value"`
	References []string `json:"References"`
}

// TracePayload contains timing and trace information.
type TracePayload struct {
	SpanID     string  `json:"SpanID"`
	Operation  string  `json:"Operation"`
	DurationMs float64 `json:"DurationMs"`
}

// AuditPayload contains audit events.
type AuditPayload struct {
	Action   string `json:"Action"`
	Resource string `json:"Resource"`
	Outcome  string `json:"Outcome"`
}

// RosterPayload contains skill manifest information.
type RosterPayload struct {
	AgentID string   `json:"AgentID"`
	Skills  []string `json:"Skills"`
	Version int      `json:"Version,omitempty"` // roster version counter (REQ-009)
	Action  string   `json:"Action,omitempty"`  // "full", "add", "remove" (REQ-009)
}

// OrchestratorPayload contains delegation and reporting events.
type OrchestratorPayload struct {
	Action    string `json:"Action"` // "delegate", "report"
	FromAgent string `json:"FromAgent"`
	ToAgent   string `json:"ToAgent"`
	TaskID    string `json:"TaskID"`
}

// HumanFeedbackPayload contains human feedback on a reflection cycle.
type HumanFeedbackPayload struct {
	CycleID           string  `json:"CycleID"`
	FeedbackType      string  `json:"FeedbackType"` // "positive", "negative", "neutral"
	Comment           string  `json:"Comment"`
	Impact            float64 `json:"Impact"`
	Relevance         float64 `json:"Relevance"`
	ValueContribution float64 `json:"ValueContribution"`
	ReviewerID        string  `json:"ReviewerID"`
}
