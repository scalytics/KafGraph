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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSrc = SourceOffset{Topic: "group.chat", Partition: 0, Offset: 1}

func makeEnvelope(t *testing.T, typ EnvelopeType, senderID, corrID string, payload any) *GroupEnvelope {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	return &GroupEnvelope{
		Type:          typ,
		CorrelationID: corrID,
		SenderID:      senderID,
		Payload:       data,
	}
}

func TestHandleAnnounce(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeAnnounce, "agent-1", "corr-1", AnnouncePayload{
		AgentID: "agent-1", AgentName: "alice", Action: "join", GroupName: "team-1",
	})

	err := HandleAnnounce(context.Background(), g, env, testSrc)
	require.NoError(t, err)

	node, err := g.GetNode(AgentNodeID("agent-1"))
	require.NoError(t, err)
	assert.Equal(t, "Agent", node.Label)
	assert.Equal(t, "alice", node.Properties["agentName"])
	assert.Equal(t, "join", node.Properties["action"])
}

func TestHandleAnnounceInvalidJSON(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := &GroupEnvelope{Type: TypeAnnounce, SenderID: "s", Payload: json.RawMessage(`bad`)}
	err := HandleAnnounce(context.Background(), g, env, testSrc)
	assert.Error(t, err)
}

func TestHandleAnnounceFallbackSenderID(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeAnnounce, "sender-1", "", AnnouncePayload{
		AgentName: "bob", Action: "heartbeat",
	})

	err := HandleAnnounce(context.Background(), g, env, testSrc)
	require.NoError(t, err)

	// Should use SenderID since AgentID is empty
	_, err = g.GetNode(AgentNodeID("sender-1"))
	require.NoError(t, err)
}

func TestHandleRequest(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeRequest, "agent-1", "corr-1", TaskRequestPayload{
		TaskID: "task-1", RequestText: "do something", TargetAgent: "agent-2",
	})

	err := HandleRequest(context.Background(), g, env, testSrc)
	require.NoError(t, err)

	// Agent, Conversation, Message should exist
	_, err = g.GetNode(AgentNodeID("agent-1"))
	require.NoError(t, err)

	_, err = g.GetNode(ConversationNodeID("corr-1"))
	require.NoError(t, err)

	msg, err := g.GetNode(MessageNodeID(testSrc))
	require.NoError(t, err)
	assert.Equal(t, "do something", msg.Properties["text"])
}

func TestHandleRequestInvalidJSON(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := &GroupEnvelope{Type: TypeRequest, SenderID: "s", CorrelationID: "c", Payload: json.RawMessage(`bad`)}
	err := HandleRequest(context.Background(), g, env, testSrc)
	assert.Error(t, err)
}

func TestHandleResponse(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	src := SourceOffset{Topic: "group.chat", Partition: 0, Offset: 2}
	env := makeEnvelope(t, TypeResponse, "agent-2", "corr-1", TaskResponsePayload{
		TaskID: "task-1", ResponseText: "done", InReplyTo: "msg-1",
	})

	err := HandleResponse(context.Background(), g, env, src)
	require.NoError(t, err)

	msg, err := g.GetNode(MessageNodeID(src))
	require.NoError(t, err)
	assert.Equal(t, "done", msg.Properties["text"])
	assert.Equal(t, "msg-1", msg.Properties["inReplyTo"])
}

func TestHandleResponseInvalidJSON(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := &GroupEnvelope{Type: TypeResponse, SenderID: "s", CorrelationID: "c", Payload: json.RawMessage(`bad`)}
	err := HandleResponse(context.Background(), g, env, testSrc)
	assert.Error(t, err)
}

func TestHandleTaskStatus(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeTaskStatus, "agent-1", "corr-1", TaskStatusPayload{
		TaskID: "task-1", Status: "completed",
	})

	err := HandleTaskStatus(context.Background(), g, env, testSrc)
	require.NoError(t, err)

	conv, err := g.GetNode(ConversationNodeID("corr-1"))
	require.NoError(t, err)
	assert.Equal(t, "completed", conv.Properties["taskStatus"])
}

func TestHandleSkillRequest(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeSkillRequest, "agent-1", "corr-1", SkillRequestPayload{
		SkillName: "brain_search", Parameters: json.RawMessage(`{"query":"test"}`),
	})

	err := HandleSkillRequest(context.Background(), g, env, testSrc)
	require.NoError(t, err)

	// Skill node should exist
	skill, err := g.GetNode(SkillNodeID("brain_search"))
	require.NoError(t, err)
	assert.Equal(t, "Skill", skill.Label)
}

func TestHandleSkillResponse(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	src := SourceOffset{Topic: "group.chat", Partition: 0, Offset: 3}
	env := makeEnvelope(t, TypeSkillResponse, "agent-1", "corr-1", SkillResponsePayload{
		SkillName: "brain_search", Result: "found 3 results", InReplyTo: "msg-1",
	})

	err := HandleSkillResponse(context.Background(), g, env, src)
	require.NoError(t, err)

	msg, err := g.GetNode(MessageNodeID(src))
	require.NoError(t, err)
	assert.Equal(t, "found 3 results", msg.Properties["text"])
}

func TestHandleMemory(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeMemory, "agent-1", "corr-1", MemoryPayload{
		Key: "decision-1", Value: "use BadgerDB", References: []string{"agent-2"},
	})

	err := HandleMemory(context.Background(), g, env, testSrc)
	require.NoError(t, err)

	mem, err := g.GetNode(SharedMemoryNodeID(testSrc))
	require.NoError(t, err)
	assert.Equal(t, "SharedMemory", mem.Label)
	assert.Equal(t, "decision-1", mem.Properties["key"])
}

func TestHandleTrace(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeTrace, "agent-1", "corr-1", TracePayload{
		SpanID: "span-1", Operation: "search", DurationMs: 42.5,
	})

	err := HandleTrace(context.Background(), g, env, testSrc)
	require.NoError(t, err)

	conv, err := g.GetNode(ConversationNodeID("corr-1"))
	require.NoError(t, err)
	assert.Equal(t, "span-1", conv.Properties["traceSpanID"])
	assert.Equal(t, 42.5, conv.Properties["traceDurationMs"])
}

func TestHandleAudit(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeAudit, "agent-1", "corr-1", AuditPayload{
		Action: "read", Resource: "/data/secret", Outcome: "allowed",
	})

	err := HandleAudit(context.Background(), g, env, testSrc)
	require.NoError(t, err)

	audit, err := g.GetNode(AuditEventNodeID(testSrc))
	require.NoError(t, err)
	assert.Equal(t, "AuditEvent", audit.Label)
	assert.Equal(t, "read", audit.Properties["action"])
}

func TestHandleRoster(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeRoster, "agent-1", "", RosterPayload{
		AgentID: "agent-1", Skills: []string{"brain_search", "brain_recall"},
	})

	err := HandleRoster(context.Background(), g, env, testSrc)
	require.NoError(t, err)

	// Both skill nodes should exist
	_, err = g.GetNode(SkillNodeID("brain_search"))
	require.NoError(t, err)
	_, err = g.GetNode(SkillNodeID("brain_recall"))
	require.NoError(t, err)
}

func TestHandleOrchestrator(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeOrchestrator, "orchestrator", "corr-1", OrchestratorPayload{
		Action: "delegate", FromAgent: "leader", ToAgent: "worker", TaskID: "task-1",
	})

	err := HandleOrchestrator(context.Background(), g, env, testSrc)
	require.NoError(t, err)

	// Both agent nodes should exist
	_, err = g.GetNode(AgentNodeID("leader"))
	require.NoError(t, err)
	_, err = g.GetNode(AgentNodeID("worker"))
	require.NoError(t, err)
}

func TestHandleOrchestratorReport(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeOrchestrator, "orchestrator", "corr-1", OrchestratorPayload{
		Action: "report", FromAgent: "worker", ToAgent: "leader", TaskID: "task-1",
	})

	err := HandleOrchestrator(context.Background(), g, env, testSrc)
	require.NoError(t, err)
}

func TestHandleOrchestratorUnknownAction(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeOrchestrator, "orchestrator", "corr-1", OrchestratorPayload{
		Action: "unknown",
	})

	err := HandleOrchestrator(context.Background(), g, env, testSrc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}

func TestHandleRequestIdempotent(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	env := makeEnvelope(t, TypeRequest, "agent-1", "corr-1", TaskRequestPayload{
		TaskID: "task-1", RequestText: "do something",
	})

	// Process same record twice
	err := HandleRequest(context.Background(), g, env, testSrc)
	require.NoError(t, err)
	err = HandleRequest(context.Background(), g, env, testSrc)
	require.NoError(t, err)

	// Should still have exactly one message node (same ID)
	msg, err := g.GetNode(MessageNodeID(testSrc))
	require.NoError(t, err)
	assert.Equal(t, "Message", msg.Label)

	// Conversation should have exactly one node
	nodes, err := g.NodesByLabel("Conversation")
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
}
