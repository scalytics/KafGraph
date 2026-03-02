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

package graph

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memStorage is an in-memory Storage implementation for testing.
type memStorage struct {
	nodes map[NodeID]*Node
	edges map[EdgeID]*Edge
}

func newMemStorage() *memStorage {
	return &memStorage{
		nodes: make(map[NodeID]*Node),
		edges: make(map[EdgeID]*Edge),
	}
}

func (m *memStorage) PutNode(n *Node) error      { m.nodes[n.ID] = n; return nil }
func (m *memStorage) PutEdge(e *Edge) error      { m.edges[e.ID] = e; return nil }
func (m *memStorage) DeleteEdge(id EdgeID) error { delete(m.edges, id); return nil }
func (m *memStorage) Close() error               { return nil }

func (m *memStorage) GetNode(id NodeID) (*Node, error) {
	n, ok := m.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %s not found", id)
	}
	return n, nil
}

func (m *memStorage) GetEdge(id EdgeID) (*Edge, error) {
	e, ok := m.edges[id]
	if !ok {
		return nil, fmt.Errorf("edge %s not found", id)
	}
	return e, nil
}

func (m *memStorage) DeleteNode(id NodeID) error {
	delete(m.nodes, id)
	for eid, e := range m.edges {
		if e.FromID == id || e.ToID == id {
			delete(m.edges, eid)
		}
	}
	return nil
}

func (m *memStorage) NodesByLabel(label string) ([]*Node, error) {
	var result []*Node
	for _, n := range m.nodes {
		if n.Label == label {
			result = append(result, n)
		}
	}
	return result, nil
}

func (m *memStorage) EdgesByNode(id NodeID) ([]*Edge, error) {
	var result []*Edge
	for _, e := range m.edges {
		if e.FromID == id || e.ToID == id {
			result = append(result, e)
		}
	}
	return result, nil
}

func TestCreateAndGetNode(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	node, err := g.CreateNode("Agent", Properties{"name": "alice"})
	require.NoError(t, err)
	require.NotNil(t, node)

	assert.Equal(t, "Agent", node.Label)
	assert.Equal(t, "alice", node.Properties["name"])
	assert.False(t, node.CreatedAt.IsZero())

	got, err := g.GetNode(node.ID)
	require.NoError(t, err)
	assert.Equal(t, node.ID, got.ID)
}

func TestDeleteNode(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	node, err := g.CreateNode("Agent", nil)
	require.NoError(t, err)

	err = g.DeleteNode(node.ID)
	require.NoError(t, err)

	_, err = g.GetNode(node.ID)
	assert.Error(t, err)
}

func TestCreateEdge(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	a, _ := g.CreateNode("Agent", Properties{"name": "alice"})
	b, _ := g.CreateNode("Agent", Properties{"name": "bob"})

	edge, err := g.CreateEdge("KNOWS", a.ID, b.ID, Properties{"since": "2026"})
	require.NoError(t, err)
	assert.Equal(t, "KNOWS", edge.Label)
	assert.Equal(t, a.ID, edge.FromID)
	assert.Equal(t, b.ID, edge.ToID)
}

func TestCreateEdgeInvalidNode(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	a, _ := g.CreateNode("Agent", nil)
	_, err := g.CreateEdge("KNOWS", a.ID, "nonexistent", nil)
	assert.Error(t, err)
}

func TestGetEdge(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	a, _ := g.CreateNode("Agent", nil)
	b, _ := g.CreateNode("Agent", nil)

	edge, _ := g.CreateEdge("KNOWS", a.ID, b.ID, nil)

	got, err := g.GetEdge(edge.ID)
	require.NoError(t, err)
	assert.Equal(t, edge.ID, got.ID)
	assert.Equal(t, "KNOWS", got.Label)
}

func TestDeleteEdge(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	a, _ := g.CreateNode("Agent", nil)
	b, _ := g.CreateNode("Agent", nil)

	edge, _ := g.CreateEdge("KNOWS", a.ID, b.ID, nil)

	err := g.DeleteEdge(edge.ID)
	require.NoError(t, err)

	_, err = g.GetEdge(edge.ID)
	assert.Error(t, err)
}

func TestDeleteNodeRemovesEdges(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	a, _ := g.CreateNode("Agent", nil)
	b, _ := g.CreateNode("Agent", nil)

	g.CreateEdge("KNOWS", a.ID, b.ID, nil)

	err := g.DeleteNode(a.ID)
	require.NoError(t, err)

	edges, err := g.Neighbors(b.ID)
	require.NoError(t, err)
	assert.Len(t, edges, 0)
}

func TestNodesByLabel(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	g.CreateNode("Agent", Properties{"name": "alice"})
	g.CreateNode("Agent", Properties{"name": "bob"})
	g.CreateNode("Message", Properties{"text": "hello"})

	agents, err := g.NodesByLabel("Agent")
	require.NoError(t, err)
	assert.Len(t, agents, 2)
}

func TestNeighbors(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	a, _ := g.CreateNode("Agent", nil)
	b, _ := g.CreateNode("Agent", nil)
	c, _ := g.CreateNode("Agent", nil)

	g.CreateEdge("KNOWS", a.ID, b.ID, nil)
	g.CreateEdge("KNOWS", a.ID, c.ID, nil)

	edges, err := g.Neighbors(a.ID)
	require.NoError(t, err)
	assert.Len(t, edges, 2)
}

func TestUpsertNodeCreate(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	node, err := g.UpsertNode("n:Agent:alice", "Agent", Properties{"name": "alice"})
	require.NoError(t, err)
	assert.Equal(t, NodeID("n:Agent:alice"), node.ID)
	assert.Equal(t, "Agent", node.Label)
	assert.Equal(t, "alice", node.Properties["name"])
}

func TestUpsertNodeMerge(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	_, err := g.UpsertNode("n:Agent:alice", "Agent", Properties{"name": "alice"})
	require.NoError(t, err)

	node, err := g.UpsertNode("n:Agent:alice", "Agent", Properties{"role": "leader"})
	require.NoError(t, err)
	assert.Equal(t, "alice", node.Properties["name"], "existing property preserved")
	assert.Equal(t, "leader", node.Properties["role"], "new property added")
}

func TestUpsertNodeIdempotent(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	n1, err := g.UpsertNode("n:Agent:alice", "Agent", Properties{"name": "alice"})
	require.NoError(t, err)

	n2, err := g.UpsertNode("n:Agent:alice", "Agent", Properties{"name": "alice"})
	require.NoError(t, err)
	assert.Equal(t, n1.ID, n2.ID)
	assert.Equal(t, n1.Properties["name"], n2.Properties["name"])
}

func TestUpsertNodeNilProps(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	node, err := g.UpsertNode("n:Agent:bob", "Agent", nil)
	require.NoError(t, err)
	assert.NotNil(t, node.Properties)
}

func TestUpsertEdgeCreate(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	edge, err := g.UpsertEdge("e:KNOWS:abc", "KNOWS", "n:Agent:alice", "n:Agent:bob", Properties{"since": "2026"})
	require.NoError(t, err)
	assert.Equal(t, EdgeID("e:KNOWS:abc"), edge.ID)
	assert.Equal(t, "KNOWS", edge.Label)
	assert.Equal(t, NodeID("n:Agent:alice"), edge.FromID)
	assert.Equal(t, NodeID("n:Agent:bob"), edge.ToID)
}

func TestUpsertEdgeNoEndpointCheck(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	// Should succeed even though endpoints don't exist
	_, err := g.UpsertEdge("e:KNOWS:abc", "KNOWS", "n:Agent:nonexistent1", "n:Agent:nonexistent2", nil)
	assert.NoError(t, err)
}

func TestUpsertEdgeMerge(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	_, err := g.UpsertEdge("e:KNOWS:abc", "KNOWS", "n:Agent:alice", "n:Agent:bob", Properties{"since": "2026"})
	require.NoError(t, err)

	edge, err := g.UpsertEdge("e:KNOWS:abc", "KNOWS", "n:Agent:alice", "n:Agent:bob", Properties{"weight": 5})
	require.NoError(t, err)
	assert.Equal(t, "2026", edge.Properties["since"], "existing property preserved")
	assert.Equal(t, 5, edge.Properties["weight"], "new property added")
}

func TestUpsertEdgeIdempotent(t *testing.T) {
	g := New(newMemStorage())
	defer g.Close()

	e1, err := g.UpsertEdge("e:KNOWS:abc", "KNOWS", "n:Agent:alice", "n:Agent:bob", nil)
	require.NoError(t, err)

	e2, err := g.UpsertEdge("e:KNOWS:abc", "KNOWS", "n:Agent:alice", "n:Agent:bob", nil)
	require.NoError(t, err)
	assert.Equal(t, e1.ID, e2.ID)
}
