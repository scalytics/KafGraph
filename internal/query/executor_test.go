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

package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/search"
	"github.com/scalytics/kafgraph/internal/storage"
)

// mockFullText is a mock FullTextSearcher for testing.
type mockFullText struct {
	results []search.TextSearchResult
}

func (m *mockFullText) Index(_ *graph.Node) error   { return nil }
func (m *mockFullText) Remove(_ graph.NodeID) error { return nil }
func (m *mockFullText) Close() error                { return nil }
func (m *mockFullText) Search(_, _, _ string, _ int) ([]search.TextSearchResult, error) {
	return m.results, nil
}

// mockVector is a mock VectorSearcher for testing.
type mockVector struct {
	results []search.VectorSearchResult
}

func (m *mockVector) Index(_, _ string, _ graph.NodeID, _ []float32) error { return nil }
func (m *mockVector) Remove(_ graph.NodeID) error                          { return nil }
func (m *mockVector) Close() error                                         { return nil }
func (m *mockVector) Search(_, _ string, _ []float32, _ int) ([]search.VectorSearchResult, error) {
	return m.results, nil
}

func newTestExecutor(t *testing.T) (*Executor, *graph.Graph) {
	t.Helper()
	dir := t.TempDir()
	s, err := storage.NewBadgerStorage(dir)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	g := graph.New(s)
	exec := NewExecutor(g, nil, nil)
	return exec, g
}

func TestExecuteMatchReturn(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})

	rs, err := exec.Execute("MATCH (n:Agent) RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 2)
	assert.Equal(t, []string{"n"}, rs.Columns)
}

func TestExecuteMatchWhereReturn(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})

	rs, err := exec.Execute("MATCH (n:Agent) WHERE n.name = 'alice' RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
}

func TestExecuteMatchWhereParam(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})

	rs, err := exec.Execute("MATCH (n:Agent) WHERE n.name = $name RETURN n",
		map[string]any{"name": "alice"})
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
}

func TestExecuteMatchWhereContains(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Message", graph.Properties{"text": "hello world"})
	g.CreateNode("Message", graph.Properties{"text": "goodbye world"})

	rs, err := exec.Execute("MATCH (n:Message) WHERE n.text CONTAINS 'hello' RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
}

func TestExecuteMatchWhereAnd(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice", "role": "leader"})
	g.CreateNode("Agent", graph.Properties{"name": "bob", "role": "member"})

	rs, err := exec.Execute("MATCH (n:Agent) WHERE n.name = 'alice' AND n.role = 'leader' RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
}

func TestExecuteMatchWhereOr(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})
	g.CreateNode("Agent", graph.Properties{"name": "charlie"})

	rs, err := exec.Execute("MATCH (n:Agent) WHERE n.name = 'alice' OR n.name = 'bob' RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 2)
}

func TestExecuteMatchWhereNot(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})

	rs, err := exec.Execute("MATCH (n:Agent) WHERE NOT n.name = 'alice' RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
}

func TestExecuteMatchReturnProperty(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})

	rs, err := exec.Execute("MATCH (n:Agent) RETURN n.name AS name", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
	assert.Equal(t, "alice", rs.Rows[0]["name"])
}

func TestExecuteCountStar(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})

	rs, err := exec.Execute("MATCH (n:Agent) RETURN count(*)", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
	assert.Equal(t, int64(2), rs.Rows[0]["count(*)"])
}

func TestExecuteOrderBy(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "bob"})
	g.CreateNode("Agent", graph.Properties{"name": "alice"})

	rs, err := exec.Execute("MATCH (n:Agent) RETURN n.name AS name ORDER BY n.name ASC", nil)
	require.NoError(t, err)
	require.Len(t, rs.Rows, 2)
	assert.Equal(t, "alice", rs.Rows[0]["name"])
	assert.Equal(t, "bob", rs.Rows[1]["name"])
}

func TestExecuteLimit(t *testing.T) {
	exec, g := newTestExecutor(t)

	for i := range 5 {
		g.CreateNode("Agent", graph.Properties{"idx": i})
	}

	rs, err := exec.Execute("MATCH (n:Agent) RETURN n LIMIT 2", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 2)
}

func TestExecuteSkip(t *testing.T) {
	exec, g := newTestExecutor(t)

	for i := range 5 {
		g.CreateNode("Agent", graph.Properties{"idx": i})
	}

	rs, err := exec.Execute("MATCH (n:Agent) RETURN n LIMIT 2 SKIP 3", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 2)
}

func TestExecuteRelationship(t *testing.T) {
	exec, g := newTestExecutor(t)

	alice, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	bob, _ := g.CreateNode("Agent", graph.Properties{"name": "bob"})
	g.CreateEdge("KNOWS", alice.ID, bob.ID, nil)

	rs, err := exec.Execute("MATCH (n:Agent)-[:KNOWS]->(m:Agent) RETURN n, m", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
}

func TestExecuteIncomingRelationship(t *testing.T) {
	exec, g := newTestExecutor(t)

	alice, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	bob, _ := g.CreateNode("Agent", graph.Properties{"name": "bob"})
	g.CreateEdge("FOLLOWS", alice.ID, bob.ID, nil)

	rs, err := exec.Execute("MATCH (n:Agent)<-[:FOLLOWS]-(m:Agent) RETURN n, m", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
}

func TestExecuteCreate(t *testing.T) {
	exec, g := newTestExecutor(t)

	rs, err := exec.Execute("CREATE (n:Agent {name: 'alice'})", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)

	// Verify node was created
	nodes, _ := g.NodesByLabel("Agent")
	assert.Len(t, nodes, 1)
	assert.Equal(t, "alice", nodes[0].Properties["name"])
}

func TestExecuteMergeCreate(t *testing.T) {
	exec, g := newTestExecutor(t)

	rs, err := exec.Execute("MERGE (n:Agent {name: 'alice'})", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)

	nodes, _ := g.NodesByLabel("Agent")
	assert.Len(t, nodes, 1)
}

func TestExecuteMergeExisting(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})

	_, err := exec.Execute("MERGE (n:Agent {name: 'alice'})", nil)
	require.NoError(t, err)

	nodes, _ := g.NodesByLabel("Agent")
	assert.Len(t, nodes, 1) // should not create duplicate
}

func TestExecuteDelete(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})

	_, err := exec.Execute("MATCH (n:Agent) DELETE n", nil)
	require.NoError(t, err)

	nodes, _ := g.NodesByLabel("Agent")
	assert.Len(t, nodes, 0)
}

func TestExecuteSet(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})

	_, err := exec.Execute("MATCH (n:Agent) SET n.role = 'leader'", nil)
	require.NoError(t, err)

	nodes, _ := g.NodesByLabel("Agent")
	require.Len(t, nodes, 1)
	assert.Equal(t, "leader", nodes[0].Properties["role"])
}

func TestExecuteMatchWithProps(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})

	rs, err := exec.Execute("MATCH (n:Agent {name: 'alice'}) RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
}

func TestExecuteWhereComparison(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice", "age": 30})
	g.CreateNode("Agent", graph.Properties{"name": "bob", "age": 25})
	g.CreateNode("Agent", graph.Properties{"name": "charlie", "age": 35})

	// Greater than
	rs, err := exec.Execute("MATCH (n:Agent) WHERE n.age > 28 RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 2) // alice(30) and charlie(35)

	// Less than
	rs, err = exec.Execute("MATCH (n:Agent) WHERE n.age < 28 RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1) // bob(25)

	// Not equal
	rs, err = exec.Execute("MATCH (n:Agent) WHERE n.name <> 'alice' RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 2) // bob and charlie
}

func TestExecuteOrderByDesc(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})
	g.CreateNode("Agent", graph.Properties{"name": "charlie"})

	rs, err := exec.Execute("MATCH (n:Agent) RETURN n.name AS name ORDER BY n.name DESC", nil)
	require.NoError(t, err)
	require.Len(t, rs.Rows, 3)
	assert.Equal(t, "charlie", rs.Rows[0]["name"])
	assert.Equal(t, "bob", rs.Rows[1]["name"])
	assert.Equal(t, "alice", rs.Rows[2]["name"])
}

func TestExecuteCountWithAlias(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})

	rs, err := exec.Execute("MATCH (n:Agent) RETURN count(*) AS total", nil)
	require.NoError(t, err)
	require.Len(t, rs.Rows, 1)
	assert.Equal(t, int64(2), rs.Rows[0]["total"])
}

func TestExecuteWhereGteAndLte(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice", "score": 80})
	g.CreateNode("Agent", graph.Properties{"name": "bob", "score": 90})
	g.CreateNode("Agent", graph.Properties{"name": "charlie", "score": 70})

	// >= 80
	rs, err := exec.Execute("MATCH (n:Agent) WHERE n.score >= 80 RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 2) // alice(80) and bob(90)

	// <= 80
	rs, err = exec.Execute("MATCH (n:Agent) WHERE n.score <= 80 RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 2) // alice(80) and charlie(70)
}

func TestExecuteWhereIn(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})
	g.CreateNode("Agent", graph.Properties{"name": "charlie"})

	rs, err := exec.Execute("MATCH (n:Agent) WHERE n.name IN ['alice', 'bob'] RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 2)
}

func TestExecuteReturnStar(t *testing.T) {
	exec, g := newTestExecutor(t)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})

	rs, err := exec.Execute("MATCH (n:Agent) RETURN *", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
}

func TestExecuteCreateAndRelationship(t *testing.T) {
	exec, g := newTestExecutor(t)

	// Use UpsertNode with simple IDs that are valid Cypher identifiers
	g.UpsertNode("alice", "Agent", graph.Properties{"name": "alice"})
	g.UpsertNode("bob", "Agent", graph.Properties{"name": "bob"})

	// Create edge via Cypher (variable names map to node IDs)
	_, err := exec.Execute("CREATE (alice)-[:KNOWS]->(bob)", nil)
	require.NoError(t, err)

	edges, _ := g.Neighbors("alice")
	assert.Len(t, edges, 1)
}

func TestExecuteEmptyResult(t *testing.T) {
	exec, _ := newTestExecutor(t)

	rs, err := exec.Execute("MATCH (n:NonExistent) RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 0)
}

func TestExecuteInvalidCypher(t *testing.T) {
	exec, _ := newTestExecutor(t)
	_, err := exec.Execute("INVALID QUERY", nil)
	assert.Error(t, err)
}

func newTestExecutorWithSearch(t *testing.T) (*Executor, *graph.Graph) {
	t.Helper()
	dir := t.TempDir()
	s, err := storage.NewBadgerStorage(dir)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	g := graph.New(s)

	node, _ := g.CreateNode("Message", graph.Properties{"text": "hello world"})

	ft := &mockFullText{
		results: []search.TextSearchResult{{NodeID: node.ID, Score: 0.9}},
	}
	vs := &mockVector{
		results: []search.VectorSearchResult{{NodeID: node.ID, Score: 0.85}},
	}
	exec := NewExecutor(g, ft, vs)
	return exec, g
}

func TestExecuteCallFullTextSearch(t *testing.T) {
	exec, _ := newTestExecutorWithSearch(t)

	rs, err := exec.Execute(
		"CALL kafgraph.fullTextSearch('Message', 'text', 'hello') YIELD node, score", nil)
	require.NoError(t, err)
	require.Len(t, rs.Rows, 1)
	assert.Equal(t, []string{"node", "score"}, rs.Columns)
	assert.Equal(t, 0.9, rs.Rows[0]["score"])
}

func TestExecuteCallVectorSearch(t *testing.T) {
	exec, _ := newTestExecutorWithSearch(t)

	rs, err := exec.Execute(
		"CALL kafgraph.vectorSearch('Message', 'embedding', [1.0, 0.0, 0.0], 5) YIELD node, score", nil)
	require.NoError(t, err)
	require.Len(t, rs.Rows, 1)
	assert.Equal(t, 0.85, rs.Rows[0]["score"])
}

func TestExecuteCallUnknownProcedure(t *testing.T) {
	exec, _ := newTestExecutor(t)
	_, err := exec.Execute("CALL unknown.proc('arg')", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown procedure")
}

func TestExecuteSetMultipleProps(t *testing.T) {
	exec, g := newTestExecutor(t)
	g.CreateNode("Agent", graph.Properties{"name": "alice"})

	_, err := exec.Execute("MATCH (n:Agent) SET n.role = 'leader', n.level = 'senior'", nil)
	require.NoError(t, err)

	nodes, _ := g.NodesByLabel("Agent")
	require.Len(t, nodes, 1)
	assert.Equal(t, "leader", nodes[0].Properties["role"])
	assert.Equal(t, "senior", nodes[0].Properties["level"])
}
