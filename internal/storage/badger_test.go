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

package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
)

func newTestStorage(t *testing.T) *BadgerStorage {
	t.Helper()
	dir := t.TempDir()
	s, err := NewBadgerStorage(dir)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestBadgerPutGetNode(t *testing.T) {
	s := newTestStorage(t)

	node := &graph.Node{
		ID:         "n:test:1",
		Label:      "Agent",
		Properties: graph.Properties{"name": "alice"},
		CreatedAt:  time.Now().UTC(),
	}

	err := s.PutNode(node)
	require.NoError(t, err)

	got, err := s.GetNode("n:test:1")
	require.NoError(t, err)
	assert.Equal(t, "Agent", got.Label)
	assert.Equal(t, "alice", got.Properties["name"])
}

func TestBadgerGetNodeNotFound(t *testing.T) {
	s := newTestStorage(t)

	_, err := s.GetNode("nonexistent")
	assert.Error(t, err)
}

func TestBadgerDeleteNode(t *testing.T) {
	s := newTestStorage(t)

	node := &graph.Node{ID: "n:test:1", Label: "Agent", CreatedAt: time.Now().UTC()}
	require.NoError(t, s.PutNode(node))

	err := s.DeleteNode("n:test:1")
	require.NoError(t, err)

	_, err = s.GetNode("n:test:1")
	assert.Error(t, err)
}

func TestBadgerPutGetEdge(t *testing.T) {
	s := newTestStorage(t)

	edge := &graph.Edge{
		ID:        "e:test:1",
		Label:     "KNOWS",
		FromID:    "n:a",
		ToID:      "n:b",
		CreatedAt: time.Now().UTC(),
	}

	err := s.PutEdge(edge)
	require.NoError(t, err)

	got, err := s.GetEdge("e:test:1")
	require.NoError(t, err)
	assert.Equal(t, "KNOWS", got.Label)
	assert.Equal(t, graph.NodeID("n:a"), got.FromID)
}

func TestBadgerDeleteEdge(t *testing.T) {
	s := newTestStorage(t)

	edge := &graph.Edge{ID: "e:test:1", Label: "KNOWS", FromID: "n:a", ToID: "n:b", CreatedAt: time.Now().UTC()}
	require.NoError(t, s.PutEdge(edge))

	err := s.DeleteEdge("e:test:1")
	require.NoError(t, err)

	_, err = s.GetEdge("e:test:1")
	assert.Error(t, err)
}

func TestBadgerDeleteNodeRemovesEdges(t *testing.T) {
	s := newTestStorage(t)

	s.PutNode(&graph.Node{ID: "n:a", Label: "Agent", CreatedAt: time.Now().UTC()})
	s.PutNode(&graph.Node{ID: "n:b", Label: "Agent", CreatedAt: time.Now().UTC()})
	s.PutEdge(&graph.Edge{ID: "e:1", Label: "KNOWS", FromID: "n:a", ToID: "n:b", CreatedAt: time.Now().UTC()})

	err := s.DeleteNode("n:a")
	require.NoError(t, err)

	edges, err := s.EdgesByNode("n:b")
	require.NoError(t, err)
	assert.Len(t, edges, 0)
}

func TestBadgerEdgesByNode(t *testing.T) {
	s := newTestStorage(t)

	s.PutNode(&graph.Node{ID: "n:a", Label: "Agent", CreatedAt: time.Now().UTC()})
	s.PutNode(&graph.Node{ID: "n:b", Label: "Agent", CreatedAt: time.Now().UTC()})
	s.PutNode(&graph.Node{ID: "n:c", Label: "Agent", CreatedAt: time.Now().UTC()})

	s.PutEdge(&graph.Edge{ID: "e:1", Label: "KNOWS", FromID: "n:a", ToID: "n:b", CreatedAt: time.Now().UTC()})
	s.PutEdge(&graph.Edge{ID: "e:2", Label: "KNOWS", FromID: "n:a", ToID: "n:c", CreatedAt: time.Now().UTC()})

	edges, err := s.EdgesByNode("n:a")
	require.NoError(t, err)
	assert.Len(t, edges, 2)

	edges, err = s.EdgesByNode("n:b")
	require.NoError(t, err)
	assert.Len(t, edges, 1)
}

func TestBadgerGetEdgeNotFound(t *testing.T) {
	s := newTestStorage(t)

	_, err := s.GetEdge("nonexistent")
	assert.Error(t, err)
}

func TestBadgerNodesByLabel(t *testing.T) {
	s := newTestStorage(t)

	s.PutNode(&graph.Node{ID: "n:1", Label: "Agent", CreatedAt: time.Now().UTC()})
	s.PutNode(&graph.Node{ID: "n:2", Label: "Agent", CreatedAt: time.Now().UTC()})
	s.PutNode(&graph.Node{ID: "n:3", Label: "Message", CreatedAt: time.Now().UTC()})

	agents, err := s.NodesByLabel("Agent")
	require.NoError(t, err)
	assert.Len(t, agents, 2)
}
