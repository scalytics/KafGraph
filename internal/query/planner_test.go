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

func mustParse(t *testing.T, cypher string) *Statement {
	t.Helper()
	stmt, err := Parse(cypher)
	require.NoError(t, err)
	return stmt
}

func TestPlanMatchLabel(t *testing.T) {
	stmt := mustParse(t, "MATCH (n:Agent) RETURN n")
	plan, err := Plan(stmt)
	require.NoError(t, err)

	// Should be LimitOffset or Project wrapping ScanByLabel
	proj, ok := plan.(*Project)
	require.True(t, ok)
	scan, ok := proj.Source.(*ScanByLabel)
	require.True(t, ok)
	assert.Equal(t, "Agent", scan.Label)
	assert.Equal(t, "n", scan.Variable)
}

func TestPlanMatchRelationship(t *testing.T) {
	stmt := mustParse(t, "MATCH (n:Agent)-[:KNOWS]->(m:Agent) RETURN n, m")
	plan, err := Plan(stmt)
	require.NoError(t, err)

	proj := plan.(*Project)
	expand, ok := proj.Source.(*ExpandOut)
	require.True(t, ok)
	assert.Equal(t, "KNOWS", expand.EdgeLabel)
	assert.Equal(t, "m", expand.DestVar)
}

func TestPlanMatchIncoming(t *testing.T) {
	stmt := mustParse(t, "MATCH (n:Agent)<-[:AUTHORED]-(m:Message) RETURN n")
	plan, err := Plan(stmt)
	require.NoError(t, err)

	proj := plan.(*Project)
	expand, ok := proj.Source.(*ExpandIn)
	require.True(t, ok)
	assert.Equal(t, "AUTHORED", expand.EdgeLabel)
}

func TestPlanWhereFilter(t *testing.T) {
	stmt := mustParse(t, "MATCH (n:Agent) WHERE n.name = 'alice' RETURN n")
	plan, err := Plan(stmt)
	require.NoError(t, err)

	proj := plan.(*Project)
	filter, ok := proj.Source.(*Filter)
	require.True(t, ok)
	assert.NotNil(t, filter.Expr)
}

func TestPlanOrderByAndLimit(t *testing.T) {
	stmt := mustParse(t, "MATCH (n:Agent) RETURN n ORDER BY n.name DESC LIMIT 10")
	plan, err := Plan(stmt)
	require.NoError(t, err)

	lo := plan.(*LimitOffset)
	require.NotNil(t, lo.Limit)
	assert.Equal(t, 10, *lo.Limit)

	proj := lo.Source.(*Project)
	sortNode := proj.Source.(*Sort)
	assert.True(t, sortNode.Items[0].Desc)
}

func TestPlanCreate(t *testing.T) {
	stmt := mustParse(t, "CREATE (n:Agent {name: 'alice'})")
	plan, err := Plan(stmt)
	require.NoError(t, err)

	create, ok := plan.(*CreateNodePlan)
	require.True(t, ok)
	assert.Equal(t, "Agent", create.Label)
}

func TestPlanCreateEdge(t *testing.T) {
	stmt := mustParse(t, "CREATE (n)-[:KNOWS]->(m)")
	plan, err := Plan(stmt)
	require.NoError(t, err)

	create, ok := plan.(*CreateEdgePlan)
	require.True(t, ok)
	assert.Equal(t, "KNOWS", create.EdgeLabel)
}

func TestPlanMerge(t *testing.T) {
	stmt := mustParse(t, "MERGE (n:Agent {name: 'alice'})")
	plan, err := Plan(stmt)
	require.NoError(t, err)

	merge, ok := plan.(*MergeNodePlan)
	require.True(t, ok)
	assert.Equal(t, "Agent", merge.Label)
}

func TestPlanSet(t *testing.T) {
	stmt := mustParse(t, "MATCH (n:Agent) SET n.name = 'bob'")
	plan, err := Plan(stmt)
	require.NoError(t, err)

	set, ok := plan.(*SetPropertyPlan)
	require.True(t, ok)
	assert.Len(t, set.Items, 1)
}

func TestPlanCall(t *testing.T) {
	stmt := mustParse(t, "CALL kafgraph.fullTextSearch('Message', 'text', 'hello') YIELD node, score")
	plan, err := Plan(stmt)
	require.NoError(t, err)

	call, ok := plan.(*ProcedureCallPlan)
	require.True(t, ok)
	assert.Equal(t, "kafgraph.fullTextSearch", call.Procedure)
	assert.Len(t, call.Args, 3)
	assert.Equal(t, []string{"node", "score"}, call.YieldVars)
}
