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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
)

func newTestProcessor(reader *MemoryReader) (*Processor, *graph.Graph) {
	g := newTestGraph()
	cfg := ProcessorConfig{
		PollInterval: 10 * time.Millisecond,
		BatchSize:    100,
		Namespace:    "test",
	}
	p := NewProcessor(reader, g, cfg)
	return p, g
}

func announceJSON(agentID, name, action string) []byte {
	env := GroupEnvelope{
		Type:     TypeAnnounce,
		SenderID: agentID,
		Payload:  mustMarshal(AnnouncePayload{AgentID: agentID, AgentName: name, Action: action, GroupName: "team-1"}),
	}
	data, _ := json.Marshal(env)
	return data
}

func requestJSON(senderID, corrID, text string) []byte {
	env := GroupEnvelope{
		Type:          TypeRequest,
		SenderID:      senderID,
		CorrelationID: corrID,
		Payload:       mustMarshal(TaskRequestPayload{TaskID: "task-1", RequestText: text, TargetAgent: "agent-2"}),
	}
	data, _ := json.Marshal(env)
	return data
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func TestProcessorSingleRecord(t *testing.T) {
	reader := NewMemoryReader()
	reader.AddRecord("group.chat", 0, 0, announceJSON("agent-1", "alice", "join"))

	proc, setup := newTestProcessor(reader)
	defer setup.Close()

	rec := Record{
		Source: SourceOffset{Topic: "group.chat", Partition: 0, Offset: 0},
		Value:  announceJSON("agent-1", "alice", "join"),
	}

	err := proc.ProcessRecord(context.Background(), rec)
	require.NoError(t, err)

	node, err := setup.GetNode(AgentNodeID("agent-1"))
	require.NoError(t, err)
	assert.Equal(t, "alice", node.Properties["agentName"])
}

func TestProcessorBatch(t *testing.T) {
	reader := NewMemoryReader()
	reader.AddRecord("group.chat", 0, 0, announceJSON("agent-1", "alice", "join"))
	reader.AddRecord("group.chat", 0, 1, announceJSON("agent-2", "bob", "join"))
	reader.AddRecord("group.chat", 0, 2, requestJSON("agent-1", "corr-1", "hello bob"))

	proc, setup := newTestProcessor(reader)
	defer setup.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Run will process all records in one poll cycle
	_ = proc.Run(ctx)

	// Verify all three records were processed
	_, err := setup.GetNode(AgentNodeID("agent-1"))
	require.NoError(t, err)
	_, err = setup.GetNode(AgentNodeID("agent-2"))
	require.NoError(t, err)
	_, err = setup.GetNode(ConversationNodeID("corr-1"))
	require.NoError(t, err)
}

func TestProcessorResume(t *testing.T) {
	reader := NewMemoryReader()
	reader.AddRecord("group.chat", 0, 0, announceJSON("agent-1", "alice", "join"))
	reader.AddRecord("group.chat", 0, 1, announceJSON("agent-2", "bob", "join"))

	proc, setup := newTestProcessor(reader)
	defer setup.Close()

	// Process first record via poll
	ctx1, cancel1 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel1()
	_ = proc.Run(ctx1)

	// Verify checkpoint was committed
	offset, err := proc.checkpoint.Load("group.chat", 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), offset, "checkpoint should be at last processed offset")

	// Add more records
	reader.AddRecord("group.chat", 0, 2, announceJSON("agent-3", "charlie", "join"))

	// Resume — should only process offset 2 (not re-process 0 and 1)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	_ = proc.Run(ctx2)

	_, err = setup.GetNode(AgentNodeID("agent-3"))
	require.NoError(t, err)
}

func TestProcessorIdempotentReplay(t *testing.T) {
	reader := NewMemoryReader()
	reader.AddRecord("group.chat", 0, 0, announceJSON("agent-1", "alice", "join"))

	proc, setup := newTestProcessor(reader)
	defer setup.Close()

	rec := Record{
		Source: SourceOffset{Topic: "group.chat", Partition: 0, Offset: 0},
		Value:  announceJSON("agent-1", "alice", "join"),
	}

	// Process same record twice
	require.NoError(t, proc.ProcessRecord(context.Background(), rec))
	require.NoError(t, proc.ProcessRecord(context.Background(), rec))

	// Should still be one agent node
	agents, err := setup.NodesByLabel("Agent")
	require.NoError(t, err)
	assert.Len(t, agents, 1)
}

func TestProcessorBadRecordSkipped(t *testing.T) {
	reader := NewMemoryReader()
	reader.AddRecord("group.chat", 0, 0, []byte(`not valid json`))
	reader.AddRecord("group.chat", 0, 1, announceJSON("agent-1", "alice", "join"))

	proc, setup := newTestProcessor(reader)
	defer setup.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = proc.Run(ctx)

	// Bad record skipped, good record processed
	_, err := setup.GetNode(AgentNodeID("agent-1"))
	require.NoError(t, err)

	// Checkpoint should advance past both records
	offset, err := proc.checkpoint.Load("group.chat", 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), offset)
}

func TestProcessorCancelStops(t *testing.T) {
	reader := NewMemoryReader()
	proc, setup := newTestProcessor(reader)
	defer setup.Close()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- proc.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("processor did not stop within timeout")
	}
}

func TestProcessorMultiplePartitions(t *testing.T) {
	reader := NewMemoryReader()
	reader.AddRecord("group.chat", 0, 0, announceJSON("agent-1", "alice", "join"))
	reader.AddRecord("group.chat", 1, 0, announceJSON("agent-2", "bob", "join"))

	proc, setup := newTestProcessor(reader)
	defer setup.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = proc.Run(ctx)

	_, err := setup.GetNode(AgentNodeID("agent-1"))
	require.NoError(t, err)
	_, err = setup.GetNode(AgentNodeID("agent-2"))
	require.NoError(t, err)
}

func TestProcessRecordInvalidEnvelope(t *testing.T) {
	reader := NewMemoryReader()
	proc, setup := newTestProcessor(reader)
	defer setup.Close()

	rec := Record{
		Source: SourceOffset{Topic: "t", Partition: 0, Offset: 0},
		Value:  []byte(`not json`),
	}

	err := proc.ProcessRecord(context.Background(), rec)
	assert.Error(t, err)
}
