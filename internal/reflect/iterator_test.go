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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/storage"
)

func newTestGraph(t *testing.T) *graph.Graph {
	t.Helper()
	s, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return graph.New(s)
}

func TestNodesInWindowEmpty(t *testing.T) {
	g := newTestGraph(t)
	it := NewHistoricIterator(g)
	now := time.Now()
	nodes, err := it.NodesInWindow([]string{"Message"}, now.Add(-1*time.Hour), now)
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestNodesInWindowFiltersTime(t *testing.T) {
	g := newTestGraph(t)
	it := NewHistoricIterator(g)

	// Create nodes (they get CreatedAt = now)
	g.CreateNode("Message", graph.Properties{"text": "in window"})
	g.CreateNode("Message", graph.Properties{"text": "also in window"})

	now := time.Now()
	nodes, err := it.NodesInWindow([]string{"Message"}, now.Add(-1*time.Hour), now.Add(1*time.Hour))
	require.NoError(t, err)
	assert.Len(t, nodes, 2)
}

func TestNodesInWindowFiltersLabel(t *testing.T) {
	g := newTestGraph(t)
	it := NewHistoricIterator(g)

	g.CreateNode("Message", graph.Properties{"text": "msg"})
	g.CreateNode("Agent", graph.Properties{"name": "alice"})

	now := time.Now()
	nodes, err := it.NodesInWindow([]string{"Message"}, now.Add(-1*time.Hour), now.Add(1*time.Hour))
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "Message", nodes[0].Label)
}

func TestNodesInWindowMultipleLabels(t *testing.T) {
	g := newTestGraph(t)
	it := NewHistoricIterator(g)

	g.CreateNode("Message", graph.Properties{"text": "msg"})
	g.CreateNode("Conversation", graph.Properties{"description": "conv"})

	now := time.Now()
	nodes, err := it.NodesInWindow([]string{"Message", "Conversation"}, now.Add(-1*time.Hour), now.Add(1*time.Hour))
	require.NoError(t, err)
	assert.Len(t, nodes, 2)
}

func TestAgentNodesInWindowEmpty(t *testing.T) {
	g := newTestGraph(t)
	it := NewHistoricIterator(g)

	now := time.Now()
	nodes, err := it.AgentNodesInWindow("n:Agent:alice", []string{"Message"}, now.Add(-1*time.Hour), now.Add(1*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestAgentNodesInWindowWithEdges(t *testing.T) {
	g := newTestGraph(t)
	it := NewHistoricIterator(g)

	agent, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	msg1, _ := g.CreateNode("Message", graph.Properties{"text": "hello"})
	msg2, _ := g.CreateNode("Message", graph.Properties{"text": "world"})
	g.CreateNode("Message", graph.Properties{"text": "unlinked"})

	g.CreateEdge("AUTHORED", agent.ID, msg1.ID, nil)
	g.CreateEdge("AUTHORED", agent.ID, msg2.ID, nil)

	now := time.Now()
	nodes, err := it.AgentNodesInWindow(agent.ID, []string{"Message"}, now.Add(-1*time.Hour), now.Add(1*time.Hour))
	require.NoError(t, err)
	assert.Len(t, nodes, 2)
}

func TestAgentNodesInWindowLabelFilter(t *testing.T) {
	g := newTestGraph(t)
	it := NewHistoricIterator(g)

	agent, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	msg, _ := g.CreateNode("Message", graph.Properties{"text": "hello"})
	conv, _ := g.CreateNode("Conversation", graph.Properties{"description": "chat"})

	g.CreateEdge("AUTHORED", agent.ID, msg.ID, nil)
	g.CreateEdge("BELONGS_TO", conv.ID, agent.ID, nil)

	now := time.Now()
	nodes, err := it.AgentNodesInWindow(agent.ID, []string{"Message"}, now.Add(-1*time.Hour), now.Add(1*time.Hour))
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "Message", nodes[0].Label)
}

func TestAgentNodesInWindowDeduplicates(t *testing.T) {
	g := newTestGraph(t)
	it := NewHistoricIterator(g)

	agent, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	msg, _ := g.CreateNode("Message", graph.Properties{"text": "hello"})

	// Two edges to the same node
	g.CreateEdge("AUTHORED", agent.ID, msg.ID, nil)
	g.CreateEdge("LINKS_TO", agent.ID, msg.ID, nil)

	now := time.Now()
	nodes, err := it.AgentNodesInWindow(agent.ID, []string{"Message"}, now.Add(-1*time.Hour), now.Add(1*time.Hour))
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
}
