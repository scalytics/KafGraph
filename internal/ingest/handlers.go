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
	"context"
	"encoding/json"
	"fmt"

	"github.com/scalytics/kafgraph/internal/graph"
)

// HandleAnnounce processes agent lifecycle events (join, leave, heartbeat).
func HandleAnnounce(_ context.Context, g *graph.Graph, env *GroupEnvelope, _ SourceOffset) error {
	var p AnnouncePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("handle announce: %w", err)
	}

	agentID := p.AgentID
	if agentID == "" {
		agentID = env.SenderID
	}

	_, err := g.UpsertNode(AgentNodeID(agentID), "Agent", graph.Properties{
		"agentName": p.AgentName,
		"action":    p.Action,
		"groupName": p.GroupName,
		"senderID":  env.SenderID,
	})
	return err
}

// HandleRequest processes task requests, creating Conversation and Message nodes.
func HandleRequest(_ context.Context, g *graph.Graph, env *GroupEnvelope, src SourceOffset) error {
	var p TaskRequestPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("handle request: %w", err)
	}

	senderNodeID := AgentNodeID(env.SenderID)
	if _, err := g.UpsertNode(senderNodeID, "Agent", graph.Properties{"senderID": env.SenderID}); err != nil {
		return fmt.Errorf("handle request: upsert sender: %w", err)
	}

	convNodeID := ConversationNodeID(env.CorrelationID)
	if _, err := g.UpsertNode(convNodeID, "Conversation", graph.Properties{
		"correlationID": env.CorrelationID,
	}); err != nil {
		return fmt.Errorf("handle request: upsert conversation: %w", err)
	}

	msgNodeID := MessageNodeID(src)
	if _, err := g.UpsertNode(msgNodeID, "Message", graph.Properties{
		"text":     p.RequestText,
		"taskID":   p.TaskID,
		"target":   p.TargetAgent,
		"senderID": env.SenderID,
	}); err != nil {
		return fmt.Errorf("handle request: upsert message: %w", err)
	}

	authoredID := DeterministicEdgeID("AUTHORED", senderNodeID, msgNodeID)
	if _, err := g.UpsertEdge(authoredID, "AUTHORED", senderNodeID, msgNodeID, nil); err != nil {
		return fmt.Errorf("handle request: upsert authored edge: %w", err)
	}

	belongsID := DeterministicEdgeID("BELONGS_TO", msgNodeID, convNodeID)
	if _, err := g.UpsertEdge(belongsID, "BELONGS_TO", msgNodeID, convNodeID, nil); err != nil {
		return fmt.Errorf("handle request: upsert belongs_to edge: %w", err)
	}

	return nil
}

// HandleResponse processes task responses, creating a Message with REPLIED_TO edge.
func HandleResponse(_ context.Context, g *graph.Graph, env *GroupEnvelope, src SourceOffset) error {
	var p TaskResponsePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("handle response: %w", err)
	}

	senderNodeID := AgentNodeID(env.SenderID)
	if _, err := g.UpsertNode(senderNodeID, "Agent", graph.Properties{"senderID": env.SenderID}); err != nil {
		return fmt.Errorf("handle response: upsert sender: %w", err)
	}

	convNodeID := ConversationNodeID(env.CorrelationID)
	if _, err := g.UpsertNode(convNodeID, "Conversation", graph.Properties{
		"correlationID": env.CorrelationID,
	}); err != nil {
		return fmt.Errorf("handle response: upsert conversation: %w", err)
	}

	msgNodeID := MessageNodeID(src)
	if _, err := g.UpsertNode(msgNodeID, "Message", graph.Properties{
		"text":      p.ResponseText,
		"taskID":    p.TaskID,
		"inReplyTo": p.InReplyTo,
		"senderID":  env.SenderID,
	}); err != nil {
		return fmt.Errorf("handle response: upsert message: %w", err)
	}

	authoredID := DeterministicEdgeID("AUTHORED", senderNodeID, msgNodeID)
	if _, err := g.UpsertEdge(authoredID, "AUTHORED", senderNodeID, msgNodeID, nil); err != nil {
		return fmt.Errorf("handle response: upsert authored edge: %w", err)
	}

	belongsID := DeterministicEdgeID("BELONGS_TO", msgNodeID, convNodeID)
	if _, err := g.UpsertEdge(belongsID, "BELONGS_TO", msgNodeID, convNodeID, nil); err != nil {
		return fmt.Errorf("handle response: upsert belongs_to edge: %w", err)
	}

	return nil
}

// HandleTaskStatus enriches a Conversation node with task status.
func HandleTaskStatus(_ context.Context, g *graph.Graph, env *GroupEnvelope, _ SourceOffset) error {
	var p TaskStatusPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("handle task_status: %w", err)
	}

	convNodeID := ConversationNodeID(env.CorrelationID)
	_, err := g.UpsertNode(convNodeID, "Conversation", graph.Properties{
		"taskStatus": p.Status,
		"taskID":     p.TaskID,
	})
	return err
}

// HandleSkillRequest creates Message, Skill, and USES_SKILL edge.
func HandleSkillRequest(_ context.Context, g *graph.Graph, env *GroupEnvelope, src SourceOffset) error {
	var p SkillRequestPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("handle skill_request: %w", err)
	}

	senderNodeID := AgentNodeID(env.SenderID)
	if _, err := g.UpsertNode(senderNodeID, "Agent", graph.Properties{"senderID": env.SenderID}); err != nil {
		return fmt.Errorf("handle skill_request: upsert sender: %w", err)
	}

	convNodeID := ConversationNodeID(env.CorrelationID)
	if _, err := g.UpsertNode(convNodeID, "Conversation", graph.Properties{
		"correlationID": env.CorrelationID,
	}); err != nil {
		return fmt.Errorf("handle skill_request: upsert conversation: %w", err)
	}

	msgNodeID := MessageNodeID(src)
	if _, err := g.UpsertNode(msgNodeID, "Message", graph.Properties{
		"skillName":  p.SkillName,
		"parameters": string(p.Parameters),
		"senderID":   env.SenderID,
	}); err != nil {
		return fmt.Errorf("handle skill_request: upsert message: %w", err)
	}

	skillNodeID := SkillNodeID(p.SkillName)
	if _, err := g.UpsertNode(skillNodeID, "Skill", graph.Properties{"skillName": p.SkillName}); err != nil {
		return fmt.Errorf("handle skill_request: upsert skill: %w", err)
	}

	authoredID := DeterministicEdgeID("AUTHORED", senderNodeID, msgNodeID)
	if _, err := g.UpsertEdge(authoredID, "AUTHORED", senderNodeID, msgNodeID, nil); err != nil {
		return fmt.Errorf("handle skill_request: upsert authored edge: %w", err)
	}

	belongsID := DeterministicEdgeID("BELONGS_TO", msgNodeID, convNodeID)
	if _, err := g.UpsertEdge(belongsID, "BELONGS_TO", msgNodeID, convNodeID, nil); err != nil {
		return fmt.Errorf("handle skill_request: upsert belongs_to edge: %w", err)
	}

	usesID := DeterministicEdgeID("USES_SKILL", msgNodeID, skillNodeID)
	if _, err := g.UpsertEdge(usesID, "USES_SKILL", msgNodeID, skillNodeID, nil); err != nil {
		return fmt.Errorf("handle skill_request: upsert uses_skill edge: %w", err)
	}

	return nil
}

// HandleSkillResponse delegates to HandleResponse (same graph shape).
func HandleSkillResponse(ctx context.Context, g *graph.Graph, env *GroupEnvelope, src SourceOffset) error {
	var p SkillResponsePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("handle skill_response: %w", err)
	}

	resp := TaskResponsePayload{
		ResponseText: p.Result,
		InReplyTo:    p.InReplyTo,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("handle skill_response: marshal: %w", err)
	}
	env.Payload = data
	return HandleResponse(ctx, g, env, src)
}

// HandleMemory creates SharedMemory node with SHARED_BY and REFERENCES edges.
func HandleMemory(_ context.Context, g *graph.Graph, env *GroupEnvelope, src SourceOffset) error {
	var p MemoryPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("handle memory: %w", err)
	}

	memNodeID := SharedMemoryNodeID(src)
	if _, err := g.UpsertNode(memNodeID, "SharedMemory", graph.Properties{
		"key":   p.Key,
		"value": p.Value,
	}); err != nil {
		return fmt.Errorf("handle memory: upsert memory node: %w", err)
	}

	senderNodeID := AgentNodeID(env.SenderID)
	if _, err := g.UpsertNode(senderNodeID, "Agent", graph.Properties{"senderID": env.SenderID}); err != nil {
		return fmt.Errorf("handle memory: upsert sender: %w", err)
	}

	sharedByID := DeterministicEdgeID("SHARED_BY", memNodeID, senderNodeID)
	if _, err := g.UpsertEdge(sharedByID, "SHARED_BY", memNodeID, senderNodeID, nil); err != nil {
		return fmt.Errorf("handle memory: upsert shared_by edge: %w", err)
	}

	for _, ref := range p.References {
		refNodeID := AgentNodeID(ref)
		if _, err := g.UpsertNode(refNodeID, "Agent", graph.Properties{"senderID": ref}); err != nil {
			return fmt.Errorf("handle memory: upsert ref agent: %w", err)
		}

		refEdgeID := DeterministicEdgeID("REFERENCES", memNodeID, refNodeID)
		if _, err := g.UpsertEdge(refEdgeID, "REFERENCES", memNodeID, refNodeID, nil); err != nil {
			return fmt.Errorf("handle memory: upsert references edge: %w", err)
		}
	}

	return nil
}

// HandleTrace annotates a Conversation node with timing information.
func HandleTrace(_ context.Context, g *graph.Graph, env *GroupEnvelope, _ SourceOffset) error {
	var p TracePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("handle trace: %w", err)
	}

	convNodeID := ConversationNodeID(env.CorrelationID)
	_, err := g.UpsertNode(convNodeID, "Conversation", graph.Properties{
		"traceSpanID":     p.SpanID,
		"traceOperation":  p.Operation,
		"traceDurationMs": p.DurationMs,
	})
	return err
}

// HandleAudit creates an AuditEvent node linked to the sender agent.
func HandleAudit(_ context.Context, g *graph.Graph, env *GroupEnvelope, src SourceOffset) error {
	var p AuditPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("handle audit: %w", err)
	}

	auditNodeID := AuditEventNodeID(src)
	if _, err := g.UpsertNode(auditNodeID, "AuditEvent", graph.Properties{
		"action":   p.Action,
		"resource": p.Resource,
		"outcome":  p.Outcome,
		"senderID": env.SenderID,
	}); err != nil {
		return fmt.Errorf("handle audit: upsert audit node: %w", err)
	}

	senderNodeID := AgentNodeID(env.SenderID)
	if _, err := g.UpsertNode(senderNodeID, "Agent", graph.Properties{"senderID": env.SenderID}); err != nil {
		return fmt.Errorf("handle audit: upsert sender: %w", err)
	}

	edgeID := DeterministicEdgeID("AUDITED_BY", auditNodeID, senderNodeID)
	_, err := g.UpsertEdge(edgeID, "AUDITED_BY", auditNodeID, senderNodeID, nil)
	return err
}

// HandleRoster creates Skill nodes from an agent's skill manifest.
func HandleRoster(_ context.Context, g *graph.Graph, env *GroupEnvelope, _ SourceOffset) error {
	var p RosterPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("handle roster: %w", err)
	}

	agentID := p.AgentID
	if agentID == "" {
		agentID = env.SenderID
	}

	agentNodeID := AgentNodeID(agentID)
	if _, err := g.UpsertNode(agentNodeID, "Agent", graph.Properties{"senderID": agentID}); err != nil {
		return fmt.Errorf("handle roster: upsert agent: %w", err)
	}

	for _, skill := range p.Skills {
		skillNodeID := SkillNodeID(skill)
		if _, err := g.UpsertNode(skillNodeID, "Skill", graph.Properties{"skillName": skill}); err != nil {
			return fmt.Errorf("handle roster: upsert skill: %w", err)
		}

		edgeID := DeterministicEdgeID("HAS_SKILL", agentNodeID, skillNodeID)
		if _, err := g.UpsertEdge(edgeID, "HAS_SKILL", agentNodeID, skillNodeID, nil); err != nil {
			return fmt.Errorf("handle roster: upsert has_skill edge: %w", err)
		}
	}

	return nil
}

// HandleOrchestrator creates DELEGATES_TO and REPORTS_TO edges between agents.
func HandleOrchestrator(_ context.Context, g *graph.Graph, env *GroupEnvelope, _ SourceOffset) error {
	var p OrchestratorPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("handle orchestrator: %w", err)
	}

	fromNodeID := AgentNodeID(p.FromAgent)
	toNodeID := AgentNodeID(p.ToAgent)

	if _, err := g.UpsertNode(fromNodeID, "Agent", graph.Properties{"senderID": p.FromAgent}); err != nil {
		return fmt.Errorf("handle orchestrator: upsert from agent: %w", err)
	}
	if _, err := g.UpsertNode(toNodeID, "Agent", graph.Properties{"senderID": p.ToAgent}); err != nil {
		return fmt.Errorf("handle orchestrator: upsert to agent: %w", err)
	}

	switch p.Action {
	case "delegate":
		edgeID := DeterministicEdgeID("DELEGATES_TO", fromNodeID, toNodeID)
		_, err := g.UpsertEdge(edgeID, "DELEGATES_TO", fromNodeID, toNodeID, graph.Properties{
			"taskID": p.TaskID,
		})
		return err
	case "report":
		edgeID := DeterministicEdgeID("REPORTS_TO", fromNodeID, toNodeID)
		_, err := g.UpsertEdge(edgeID, "REPORTS_TO", fromNodeID, toNodeID, graph.Properties{
			"taskID": p.TaskID,
		})
		return err
	default:
		return fmt.Errorf("handle orchestrator: unknown action %q", p.Action)
	}
}
