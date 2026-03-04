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

import "fmt"

// Plan converts a Statement AST into an execution plan.
func Plan(stmt *Statement) (PlanNode, error) {
	var current PlanNode

	for _, clause := range stmt.Clauses {
		switch c := clause.(type) {
		case *MatchClause:
			node, err := planMatch(c)
			if err != nil {
				return nil, err
			}
			current = node

		case *WhereClause:
			if current == nil {
				return nil, fmt.Errorf("WHERE without preceding MATCH")
			}
			current = &Filter{Source: current, Expr: c.Expr}

		case *ReturnClause:
			if current == nil {
				return nil, fmt.Errorf("RETURN without preceding clause")
			}
			// Sort before projection so ORDER BY can reference match variables.
			if len(c.OrderBy) > 0 {
				current = &Sort{Source: current, Items: c.OrderBy}
			}
			current = &Project{Source: current, Items: c.Items}
			if c.Limit != nil || c.Skip != nil {
				current = &LimitOffset{Source: current, Limit: c.Limit, Skip: c.Skip}
			}

		case *CreateClause:
			node, err := planCreate(c)
			if err != nil {
				return nil, err
			}
			current = node

		case *MergeClause:
			current = &MergeNodePlan{
				Variable: c.Pattern.Nodes[0].Variable,
				Label:    c.Pattern.Nodes[0].Label,
				Props:    c.Pattern.Nodes[0].Properties,
			}

		case *SetClause:
			if current == nil {
				return nil, fmt.Errorf("SET without preceding clause")
			}
			current = &SetPropertyPlan{Source: current, Items: c.Items}

		case *DeleteClause:
			if current == nil {
				return nil, fmt.Errorf("DELETE without preceding clause")
			}
			current = &DeletePlan{Source: current, Variables: c.Variables}

		case *CallClause:
			current = &ProcedureCallPlan{
				Procedure: c.Procedure,
				Args:      c.Args,
				YieldVars: c.YieldVars,
			}

		default:
			return nil, fmt.Errorf("unsupported clause type: %T", clause)
		}
	}

	if current == nil {
		return nil, fmt.Errorf("empty plan")
	}
	return current, nil
}

func planMatch(c *MatchClause) (PlanNode, error) {
	if len(c.Patterns) == 0 {
		return nil, fmt.Errorf("MATCH without patterns")
	}

	pat := c.Patterns[0]
	if len(pat.Nodes) == 0 {
		return nil, fmt.Errorf("pattern without nodes")
	}

	scan := &ScanByLabel{
		Variable: pat.Nodes[0].Variable,
		Label:    pat.Nodes[0].Label,
		Props:    pat.Nodes[0].Properties,
	}

	if len(pat.Edges) == 0 {
		return scan, nil
	}

	// Build chain of expands
	var current PlanNode = scan
	for i, edge := range pat.Edges {
		dest := pat.Nodes[i+1]
		if edge.Direction == EdgeLeft {
			current = &ExpandIn{
				Source:    current,
				SrcVar:    pat.Nodes[i].Variable,
				EdgeVar:   edge.Variable,
				EdgeLabel: edge.Label,
				DestVar:   dest.Variable,
				DestLabel: dest.Label,
			}
		} else {
			current = &ExpandOut{
				Source:    current,
				SrcVar:    pat.Nodes[i].Variable,
				EdgeVar:   edge.Variable,
				EdgeLabel: edge.Label,
				DestVar:   dest.Variable,
				DestLabel: dest.Label,
			}
		}
	}
	return current, nil
}

func planCreate(c *CreateClause) (PlanNode, error) {
	if len(c.Patterns) == 0 {
		return nil, fmt.Errorf("CREATE without patterns")
	}
	pat := c.Patterns[0]

	// Simple node creation
	if len(pat.Edges) == 0 {
		return &CreateNodePlan{
			Variable: pat.Nodes[0].Variable,
			Label:    pat.Nodes[0].Label,
			Props:    pat.Nodes[0].Properties,
		}, nil
	}

	// Edge creation
	if len(pat.Edges) == 1 && len(pat.Nodes) == 2 {
		return &CreateEdgePlan{
			SrcVar:    pat.Nodes[0].Variable,
			DestVar:   pat.Nodes[1].Variable,
			EdgeLabel: pat.Edges[0].Label,
		}, nil
	}

	return nil, fmt.Errorf("complex CREATE patterns not yet supported")
}
