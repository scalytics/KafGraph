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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
)

func newTestGraph() *graph.Graph {
	return graph.New(newMemStorage())
}

func TestRouterDispatch(t *testing.T) {
	r := NewRouter()
	g := newTestGraph()
	defer g.Close()

	payload, _ := json.Marshal(AnnouncePayload{
		AgentID:   "agent-1",
		AgentName: "alice",
		Action:    "join",
		GroupName: "team-1",
	})

	env := &GroupEnvelope{
		Type:          TypeAnnounce,
		CorrelationID: "corr-1",
		SenderID:      "agent-1",
		Payload:       payload,
	}

	err := r.Route(context.Background(), g, env, SourceOffset{Topic: "t", Partition: 0, Offset: 0})
	require.NoError(t, err)

	// Verify agent node was created
	node, err := g.GetNode(AgentNodeID("agent-1"))
	require.NoError(t, err)
	assert.Equal(t, "Agent", node.Label)
}

func TestRouterUnknownType(t *testing.T) {
	r := NewRouter()
	g := newTestGraph()
	defer g.Close()

	env := &GroupEnvelope{
		Type:     "unknown_type",
		SenderID: "agent-1",
		Payload:  json.RawMessage(`{}`),
	}

	err := r.Route(context.Background(), g, env, SourceOffset{})
	assert.NoError(t, err, "unknown types should be skipped, not error")
}

func TestRouterErrorPropagation(t *testing.T) {
	r := NewRouter()
	g := newTestGraph()
	defer g.Close()

	// Send announce with invalid JSON payload to trigger unmarshal error
	env := &GroupEnvelope{
		Type:     TypeAnnounce,
		SenderID: "agent-1",
		Payload:  json.RawMessage(`not valid json`),
	}

	err := r.Route(context.Background(), g, env, SourceOffset{})
	assert.Error(t, err, "handler errors should propagate through router")
}

// memStorage is an in-memory graph.Storage for testing.
type memStorage struct {
	nodes map[graph.NodeID]*graph.Node
	edges map[graph.EdgeID]*graph.Edge
}

func newMemStorage() *memStorage {
	return &memStorage{
		nodes: make(map[graph.NodeID]*graph.Node),
		edges: make(map[graph.EdgeID]*graph.Edge),
	}
}

func (m *memStorage) PutNode(n *graph.Node) error      { m.nodes[n.ID] = n; return nil }
func (m *memStorage) PutEdge(e *graph.Edge) error      { m.edges[e.ID] = e; return nil }
func (m *memStorage) DeleteEdge(id graph.EdgeID) error { delete(m.edges, id); return nil }
func (m *memStorage) Close() error                     { return nil }

func (m *memStorage) GetNode(id graph.NodeID) (*graph.Node, error) {
	n, ok := m.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %s not found", id)
	}
	return n, nil
}

func (m *memStorage) GetEdge(id graph.EdgeID) (*graph.Edge, error) {
	e, ok := m.edges[id]
	if !ok {
		return nil, fmt.Errorf("edge %s not found", id)
	}
	return e, nil
}

func (m *memStorage) DeleteNode(id graph.NodeID) error {
	delete(m.nodes, id)
	for eid, e := range m.edges {
		if e.FromID == id || e.ToID == id {
			delete(m.edges, eid)
		}
	}
	return nil
}

func (m *memStorage) NodesByLabel(label string) ([]*graph.Node, error) {
	var result []*graph.Node
	for _, n := range m.nodes {
		if n.Label == label {
			result = append(result, n)
		}
	}
	return result, nil
}

func (m *memStorage) EdgesByNode(id graph.NodeID) ([]*graph.Edge, error) {
	var result []*graph.Edge
	for _, e := range m.edges {
		if e.FromID == id || e.ToID == id {
			result = append(result, e)
		}
	}
	return result, nil
}
