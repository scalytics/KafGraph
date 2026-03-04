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
)

func TestParseMatchNode(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) RETURN n")
	require.NoError(t, err)
	require.Len(t, stmt.Clauses, 2)

	m := stmt.Clauses[0].(*MatchClause)
	assert.Len(t, m.Patterns, 1)
	assert.Equal(t, "n", m.Patterns[0].Nodes[0].Variable)
	assert.Equal(t, "Agent", m.Patterns[0].Nodes[0].Label)
}

func TestParseMatchNodeWithProps(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent {name: 'alice'}) RETURN n")
	require.NoError(t, err)

	m := stmt.Clauses[0].(*MatchClause)
	np := m.Patterns[0].Nodes[0]
	assert.Equal(t, "Agent", np.Label)
	require.Contains(t, np.Properties, "name")
	lit := np.Properties["name"].(*LiteralExpr)
	assert.Equal(t, "alice", lit.Value)
}

func TestParseMatchRelationshipRight(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent)-[:KNOWS]->(m:Agent) RETURN n, m")
	require.NoError(t, err)

	m := stmt.Clauses[0].(*MatchClause)
	pat := m.Patterns[0]
	assert.Len(t, pat.Nodes, 2)
	assert.Len(t, pat.Edges, 1)
	assert.Equal(t, "KNOWS", pat.Edges[0].Label)
	assert.Equal(t, EdgeRight, pat.Edges[0].Direction)
}

func TestParseMatchRelationshipLeft(t *testing.T) {
	stmt, err := Parse("MATCH (n)<-[:EDGE]-(m) RETURN n")
	require.NoError(t, err)

	m := stmt.Clauses[0].(*MatchClause)
	assert.Equal(t, EdgeLeft, m.Patterns[0].Edges[0].Direction)
	assert.Equal(t, "EDGE", m.Patterns[0].Edges[0].Label)
}

func TestParseWhere(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) WHERE n.name = 'alice' RETURN n")
	require.NoError(t, err)
	require.Len(t, stmt.Clauses, 3)

	w := stmt.Clauses[1].(*WhereClause)
	bin := w.Expr.(*BinaryExpr)
	assert.Equal(t, "=", bin.Op)
	prop := bin.Left.(*PropertyExpr)
	assert.Equal(t, "n", prop.Variable)
	assert.Equal(t, "name", prop.Property)
}

func TestParseWhereContains(t *testing.T) {
	stmt, err := Parse("MATCH (n:Message) WHERE n.text CONTAINS 'hello' RETURN n")
	require.NoError(t, err)

	w := stmt.Clauses[1].(*WhereClause)
	bin := w.Expr.(*BinaryExpr)
	assert.Equal(t, "CONTAINS", bin.Op)
}

func TestParseWhereAnd(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) WHERE n.name = 'alice' AND n.role = 'leader' RETURN n")
	require.NoError(t, err)

	w := stmt.Clauses[1].(*WhereClause)
	bin := w.Expr.(*BinaryExpr)
	assert.Equal(t, "AND", bin.Op)
}

func TestParseWhereOr(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) WHERE n.name = 'alice' OR n.name = 'bob' RETURN n")
	require.NoError(t, err)

	w := stmt.Clauses[1].(*WhereClause)
	bin := w.Expr.(*BinaryExpr)
	assert.Equal(t, "OR", bin.Op)
}

func TestParseWhereNot(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) WHERE NOT n.active = true RETURN n")
	require.NoError(t, err)

	w := stmt.Clauses[1].(*WhereClause)
	unary := w.Expr.(*UnaryExpr)
	assert.Equal(t, "NOT", unary.Op)
}

func TestParseReturnAlias(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) RETURN n.name AS agentName")
	require.NoError(t, err)

	r := stmt.Clauses[1].(*ReturnClause)
	assert.Len(t, r.Items, 1)
	assert.Equal(t, "agentName", r.Items[0].Alias)
	prop := r.Items[0].Expr.(*PropertyExpr)
	assert.Equal(t, "name", prop.Property)
}

func TestParseReturnCountStar(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) RETURN count(*)")
	require.NoError(t, err)

	r := stmt.Clauses[1].(*ReturnClause)
	fc := r.Items[0].Expr.(*FuncCallExpr)
	assert.Equal(t, "count", fc.Name)
	assert.Len(t, fc.Args, 1)
	_, ok := fc.Args[0].(*StarExpr)
	assert.True(t, ok)
}

func TestParseOrderBy(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) RETURN n ORDER BY n.name DESC")
	require.NoError(t, err)

	r := stmt.Clauses[1].(*ReturnClause)
	require.Len(t, r.OrderBy, 1)
	assert.True(t, r.OrderBy[0].Desc)
}

func TestParseLimitSkip(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) RETURN n LIMIT 10 SKIP 5")
	require.NoError(t, err)

	r := stmt.Clauses[1].(*ReturnClause)
	require.NotNil(t, r.Limit)
	assert.Equal(t, 10, *r.Limit)
	require.NotNil(t, r.Skip)
	assert.Equal(t, 5, *r.Skip)
}

func TestParseCreate(t *testing.T) {
	stmt, err := Parse("CREATE (n:Agent {name: 'alice'})")
	require.NoError(t, err)
	require.Len(t, stmt.Clauses, 1)

	c := stmt.Clauses[0].(*CreateClause)
	assert.Len(t, c.Patterns, 1)
	assert.Equal(t, "Agent", c.Patterns[0].Nodes[0].Label)
}

func TestParseCreateRelationship(t *testing.T) {
	stmt, err := Parse("CREATE (n)-[:KNOWS]->(m)")
	require.NoError(t, err)

	c := stmt.Clauses[0].(*CreateClause)
	assert.Len(t, c.Patterns[0].Edges, 1)
	assert.Equal(t, "KNOWS", c.Patterns[0].Edges[0].Label)
}

func TestParseMerge(t *testing.T) {
	stmt, err := Parse("MERGE (n:Agent {name: 'alice'})")
	require.NoError(t, err)

	m := stmt.Clauses[0].(*MergeClause)
	assert.Equal(t, "Agent", m.Pattern.Nodes[0].Label)
}

func TestParseSet(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) SET n.name = 'bob'")
	require.NoError(t, err)

	s := stmt.Clauses[1].(*SetClause)
	require.Len(t, s.Items, 1)
	assert.Equal(t, "n", s.Items[0].Variable)
	assert.Equal(t, "name", s.Items[0].Property)
}

func TestParseDelete(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) DELETE n")
	require.NoError(t, err)

	d := stmt.Clauses[1].(*DeleteClause)
	assert.Equal(t, []string{"n"}, d.Variables)
}

func TestParseCall(t *testing.T) {
	stmt, err := Parse("CALL kafgraph.fullTextSearch('Message', 'text', 'hello') YIELD node, score")
	require.NoError(t, err)

	c := stmt.Clauses[0].(*CallClause)
	assert.Equal(t, "kafgraph.fullTextSearch", c.Procedure)
	assert.Len(t, c.Args, 3)
	assert.Equal(t, []string{"node", "score"}, c.YieldVars)
}

func TestParseParam(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) WHERE n.name = $name RETURN n")
	require.NoError(t, err)

	w := stmt.Clauses[1].(*WhereClause)
	bin := w.Expr.(*BinaryExpr)
	param := bin.Right.(*ParamExpr)
	assert.Equal(t, "name", param.Name)
}

func TestParseEmptyStatement(t *testing.T) {
	_, err := Parse("")
	assert.Error(t, err)
}

func TestParseInvalidSyntax(t *testing.T) {
	_, err := Parse("MATCH RETURN")
	assert.Error(t, err)
}

func TestParseWhereComparisonOps(t *testing.T) {
	for _, op := range []string{"<>", "<", ">", "<=", ">="} {
		stmt, err := Parse("MATCH (n:Agent) WHERE n.age " + op + " 30 RETURN n")
		require.NoError(t, err, "op: %s", op)
		w := stmt.Clauses[1].(*WhereClause)
		bin := w.Expr.(*BinaryExpr)
		assert.Equal(t, op, bin.Op, "op: %s", op)
	}
}

func TestParseWhereIn(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) WHERE n.name IN ['alice', 'bob'] RETURN n")
	require.NoError(t, err)
	w := stmt.Clauses[1].(*WhereClause)
	bin := w.Expr.(*BinaryExpr)
	assert.Equal(t, "IN", bin.Op)
	list := bin.Right.(*ListExpr)
	assert.Len(t, list.Elements, 2)
}

func TestParseNegativeNumber(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) WHERE n.score > -10 RETURN n")
	require.NoError(t, err)
	w := stmt.Clauses[1].(*WhereClause)
	bin := w.Expr.(*BinaryExpr)
	lit := bin.Right.(*LiteralExpr)
	assert.Equal(t, int64(-10), lit.Value)
}

func TestParseNegativeFloat(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) WHERE n.score > -3.14 RETURN n")
	require.NoError(t, err)
	w := stmt.Clauses[1].(*WhereClause)
	bin := w.Expr.(*BinaryExpr)
	lit := bin.Right.(*LiteralExpr)
	assert.InDelta(t, -3.14, lit.Value.(float64), 0.001)
}

func TestParseTrueFalseNull(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) WHERE n.active = true RETURN n")
	require.NoError(t, err)
	w := stmt.Clauses[1].(*WhereClause)
	bin := w.Expr.(*BinaryExpr)
	lit := bin.Right.(*LiteralExpr)
	assert.Equal(t, true, lit.Value)

	stmt, err = Parse("MATCH (n:Agent) WHERE n.active = false RETURN n")
	require.NoError(t, err)
	w = stmt.Clauses[1].(*WhereClause)
	bin = w.Expr.(*BinaryExpr)
	lit = bin.Right.(*LiteralExpr)
	assert.Equal(t, false, lit.Value)

	stmt, err = Parse("MATCH (n:Agent) WHERE n.val = null RETURN n")
	require.NoError(t, err)
	w = stmt.Clauses[1].(*WhereClause)
	bin = w.Expr.(*BinaryExpr)
	lit = bin.Right.(*LiteralExpr)
	assert.Nil(t, lit.Value)
}

func TestParseParenthesizedExpr(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) WHERE (n.name = 'alice') RETURN n")
	require.NoError(t, err)
	w := stmt.Clauses[1].(*WhereClause)
	bin := w.Expr.(*BinaryExpr)
	assert.Equal(t, "=", bin.Op)
}

func TestParseReturnStar(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) RETURN *")
	require.NoError(t, err)
	r := stmt.Clauses[1].(*ReturnClause)
	require.Len(t, r.Items, 1)
	_, ok := r.Items[0].Expr.(*StarExpr)
	assert.True(t, ok)
}

func TestParseReturnMultipleItems(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) RETURN n.name AS name, n.age AS age")
	require.NoError(t, err)
	r := stmt.Clauses[1].(*ReturnClause)
	assert.Len(t, r.Items, 2)
	assert.Equal(t, "name", r.Items[0].Alias)
	assert.Equal(t, "age", r.Items[1].Alias)
}

func TestParseDeleteMultipleVars(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent)-[r:KNOWS]->(m:Agent) DELETE n, m")
	require.NoError(t, err)
	d := stmt.Clauses[1].(*DeleteClause)
	assert.Equal(t, []string{"n", "m"}, d.Variables)
}

func TestParseSetMultiple(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) SET n.name = 'alice', n.role = 'leader'")
	require.NoError(t, err)
	s := stmt.Clauses[1].(*SetClause)
	assert.Len(t, s.Items, 2)
}

func TestParseOrderByASC(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) RETURN n ORDER BY n.name ASC")
	require.NoError(t, err)
	r := stmt.Clauses[1].(*ReturnClause)
	require.Len(t, r.OrderBy, 1)
	assert.False(t, r.OrderBy[0].Desc)
}

func TestParseCallNoYield(t *testing.T) {
	stmt, err := Parse("CALL kafgraph.test('arg1')")
	require.NoError(t, err)
	c := stmt.Clauses[0].(*CallClause)
	assert.Equal(t, "kafgraph.test", c.Procedure)
	assert.Len(t, c.Args, 1)
	assert.Nil(t, c.YieldVars)
}

func TestParseFloatLiteral(t *testing.T) {
	stmt, err := Parse("MATCH (n:Agent) WHERE n.score > 3.14 RETURN n")
	require.NoError(t, err)
	w := stmt.Clauses[1].(*WhereClause)
	bin := w.Expr.(*BinaryExpr)
	lit := bin.Right.(*LiteralExpr)
	assert.InDelta(t, 3.14, lit.Value.(float64), 0.001)
}
