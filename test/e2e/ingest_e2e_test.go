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

//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/ingest"
	"github.com/scalytics/kafgraph/internal/storage"
)

// TestE2EIngestPipeline exercises the full ingestion pipeline:
// announce → request → response, then verifies the resulting graph structure.
func TestE2EIngestPipeline(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	reader := ingest.NewMemoryReader()

	// Simulate a sequence of KafClaw messages
	reader.AddRecord("group.chat", 0, 0,
		envelopeBytes(t, "announce", "agent-1", "corr-1", ingest.AnnouncePayload{
			AgentID: "agent-1", AgentName: "alice", Action: "join", GroupName: "team-1",
		}))

	reader.AddRecord("group.chat", 0, 1,
		envelopeBytes(t, "announce", "agent-2", "corr-1", ingest.AnnouncePayload{
			AgentID: "agent-2", AgentName: "bob", Action: "join", GroupName: "team-1",
		}))

	reader.AddRecord("group.chat", 0, 2,
		envelopeBytes(t, "request", "agent-1", "corr-1", ingest.TaskRequestPayload{
			TaskID: "task-1", RequestText: "hello bob, please search", TargetAgent: "agent-2",
		}))

	reader.AddRecord("group.chat", 0, 3,
		envelopeBytes(t, "response", "agent-2", "corr-1", ingest.TaskResponsePayload{
			TaskID: "task-1", ResponseText: "found 3 results", InReplyTo: "msg-1",
		}))

	reader.AddRecord("group.chat", 0, 4,
		envelopeBytes(t, "task_status", "agent-1", "corr-1", ingest.TaskStatusPayload{
			TaskID: "task-1", Status: "completed",
		}))

	// Run processor
	proc := ingest.NewProcessor(reader, g, ingest.ProcessorConfig{
		PollInterval: 10 * time.Millisecond,
		BatchSize:    100,
		Namespace:    "e2e-test",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = proc.Run(ctx)

	// Verify graph structure
	// 1. Two agent nodes
	agents, err := g.NodesByLabel("Agent")
	require.NoError(t, err)
	assert.Len(t, agents, 2, "should have 2 agent nodes")

	// 2. One conversation node
	convs, err := g.NodesByLabel("Conversation")
	require.NoError(t, err)
	assert.Len(t, convs, 1, "should have 1 conversation node")
	assert.Equal(t, "completed", convs[0].Properties["taskStatus"])

	// 3. Two message nodes (request + response)
	msgs, err := g.NodesByLabel("Message")
	require.NoError(t, err)
	assert.Len(t, msgs, 2, "should have 2 message nodes")

	// 4. Agent nodes have correct names
	alice, err := g.GetNode(ingest.AgentNodeID("agent-1"))
	require.NoError(t, err)
	assert.Equal(t, "alice", alice.Properties["agentName"])

	bob, err := g.GetNode(ingest.AgentNodeID("agent-2"))
	require.NoError(t, err)
	assert.Equal(t, "bob", bob.Properties["agentName"])

	// 5. Conversation has edges from both messages
	conv, err := g.GetNode(ingest.ConversationNodeID("corr-1"))
	require.NoError(t, err)
	convEdges, err := g.Neighbors(conv.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(convEdges), 2, "conversation should have at least 2 edges (BELONGS_TO)")

	// 6. Request message has correct text
	reqSrc := ingest.SourceOffset{Topic: "group.chat", Partition: 0, Offset: 2}
	reqMsg, err := g.GetNode(ingest.MessageNodeID(reqSrc))
	require.NoError(t, err)
	assert.Equal(t, "hello bob, please search", reqMsg.Properties["text"])

	// 7. Response message has correct text
	respSrc := ingest.SourceOffset{Topic: "group.chat", Partition: 0, Offset: 3}
	respMsg, err := g.GetNode(ingest.MessageNodeID(respSrc))
	require.NoError(t, err)
	assert.Equal(t, "found 3 results", respMsg.Properties["text"])

	// 8. Alice authored the request
	aliceEdges, err := g.Neighbors(alice.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(aliceEdges), 1, "alice should have at least 1 AUTHORED edge")
}

func envelopeBytes(t *testing.T, typ, senderID, corrID string, payload any) []byte {
	t.Helper()
	p, err := json.Marshal(payload)
	require.NoError(t, err)

	env := ingest.GroupEnvelope{
		Type:          ingest.EnvelopeType(typ),
		CorrelationID: corrID,
		SenderID:      senderID,
		Timestamp:     time.Now().UTC(),
		Payload:       p,
	}
	data, err := json.Marshal(env)
	require.NoError(t, err)
	return data
}
