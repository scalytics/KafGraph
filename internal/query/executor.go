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
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/search"
)

// Executor executes query plans against the graph.
type Executor struct {
	graph    *graph.Graph
	storage  graph.IndexedStorage
	fullText search.FullTextSearcher
	vector   search.VectorSearcher
	procs    *ProcedureRegistry
}

// NewExecutor creates a new query executor.
func NewExecutor(g *graph.Graph, ft search.FullTextSearcher, vs search.VectorSearcher) *Executor {
	e := &Executor{
		graph:    g,
		fullText: ft,
		vector:   vs,
		procs:    NewProcedureRegistry(),
	}

	// Extract IndexedStorage if available.
	if is, ok := g.StorageBackend().(graph.IndexedStorage); ok {
		e.storage = is
	}

	// Register built-in procedures.
	e.registerBuiltins()
	return e
}

// Execute parses and executes a Cypher query.
func (e *Executor) Execute(cypher string, params map[string]any) (*ResultSet, error) {
	stmt, err := Parse(cypher)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	plan, err := Plan(stmt)
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}
	return e.executePlan(plan, params)
}

func (e *Executor) executePlan(plan PlanNode, params map[string]any) (*ResultSet, error) {
	switch p := plan.(type) {
	case *ScanByLabel:
		return e.execScan(p, params)
	case *ExpandOut:
		return e.execExpandOut(p, params)
	case *ExpandIn:
		return e.execExpandIn(p, params)
	case *Filter:
		return e.execFilter(p, params)
	case *Project:
		return e.execProject(p, params)
	case *Sort:
		return e.execSort(p, params)
	case *LimitOffset:
		return e.execLimitOffset(p, params)
	case *CreateNodePlan:
		return e.execCreateNode(p, params)
	case *CreateEdgePlan:
		return e.execCreateEdge(p, params)
	case *MergeNodePlan:
		return e.execMergeNode(p, params)
	case *SetPropertyPlan:
		return e.execSetProperty(p, params)
	case *DeletePlan:
		return e.execDelete(p, params)
	case *ProcedureCallPlan:
		return e.execProcedure(p, params)
	default:
		return nil, fmt.Errorf("unsupported plan node: %T", plan)
	}
}

func (e *Executor) execScan(p *ScanByLabel, _ map[string]any) (*ResultSet, error) {
	nodes, err := e.graph.NodesByLabel(p.Label)
	if err != nil {
		return nil, err
	}

	rs := &ResultSet{Columns: []string{p.Variable}}
	for _, node := range nodes {
		// Apply inline property filter if present.
		if len(p.Props) > 0 && !matchProps(node.Properties, p.Props) {
			continue
		}
		rs.Rows = append(rs.Rows, Row{p.Variable: nodeToMap(node)})
	}
	return rs, nil
}

func (e *Executor) execExpandOut(p *ExpandOut, params map[string]any) (*ResultSet, error) {
	source, err := e.executePlan(p.Source, params)
	if err != nil {
		return nil, err
	}

	rs := &ResultSet{}
	for _, row := range source.Rows {
		srcNode := row[p.SrcVar]
		srcMap, ok := srcNode.(map[string]any)
		if !ok {
			continue
		}
		nodeID := graph.NodeID(fmt.Sprint(srcMap["id"]))

		var edgeIDs []graph.EdgeID
		if e.storage != nil {
			edgeIDs, _ = e.storage.OutgoingEdgeIDs(nodeID)
		}

		for _, eid := range edgeIDs {
			edge, err := e.graph.GetEdge(eid)
			if err != nil {
				continue
			}
			if p.EdgeLabel != "" && edge.Label != p.EdgeLabel {
				continue
			}
			destNode, err := e.graph.GetNode(edge.ToID)
			if err != nil {
				continue
			}
			if p.DestLabel != "" && destNode.Label != p.DestLabel {
				continue
			}
			newRow := copyRow(row)
			if p.EdgeVar != "" {
				newRow[p.EdgeVar] = edgeToMap(edge)
			}
			newRow[p.DestVar] = nodeToMap(destNode)
			rs.Rows = append(rs.Rows, newRow)
		}
	}
	// Build columns from first row
	if len(rs.Rows) > 0 {
		for k := range rs.Rows[0] {
			rs.Columns = append(rs.Columns, k)
		}
	}
	return rs, nil
}

func (e *Executor) execExpandIn(p *ExpandIn, params map[string]any) (*ResultSet, error) {
	source, err := e.executePlan(p.Source, params)
	if err != nil {
		return nil, err
	}

	rs := &ResultSet{}
	for _, row := range source.Rows {
		srcNode := row[p.SrcVar]
		srcMap, ok := srcNode.(map[string]any)
		if !ok {
			continue
		}
		nodeID := graph.NodeID(fmt.Sprint(srcMap["id"]))

		var edgeIDs []graph.EdgeID
		if e.storage != nil {
			edgeIDs, _ = e.storage.IncomingEdgeIDs(nodeID)
		}

		for _, eid := range edgeIDs {
			edge, err := e.graph.GetEdge(eid)
			if err != nil {
				continue
			}
			if p.EdgeLabel != "" && edge.Label != p.EdgeLabel {
				continue
			}
			destNode, err := e.graph.GetNode(edge.FromID)
			if err != nil {
				continue
			}
			if p.DestLabel != "" && destNode.Label != p.DestLabel {
				continue
			}
			newRow := copyRow(row)
			if p.EdgeVar != "" {
				newRow[p.EdgeVar] = edgeToMap(edge)
			}
			newRow[p.DestVar] = nodeToMap(destNode)
			rs.Rows = append(rs.Rows, newRow)
		}
	}
	if len(rs.Rows) > 0 {
		for k := range rs.Rows[0] {
			rs.Columns = append(rs.Columns, k)
		}
	}
	return rs, nil
}

func (e *Executor) execFilter(p *Filter, params map[string]any) (*ResultSet, error) {
	source, err := e.executePlan(p.Source, params)
	if err != nil {
		return nil, err
	}

	rs := &ResultSet{Columns: source.Columns}
	for _, row := range source.Rows {
		if evalBool(p.Expr, row, params) {
			rs.Rows = append(rs.Rows, row)
		}
	}
	return rs, nil
}

func (e *Executor) execProject(p *Project, params map[string]any) (*ResultSet, error) {
	source, err := e.executePlan(p.Source, params)
	if err != nil {
		return nil, err
	}

	// Check for aggregation (count(*))
	for _, item := range p.Items {
		if fc, ok := item.Expr.(*FuncCallExpr); ok && strings.ToLower(fc.Name) == "count" {
			col := "count(*)"
			if item.Alias != "" {
				col = item.Alias
			}
			return &ResultSet{
				Columns: []string{col},
				Rows:    []Row{{col: int64(len(source.Rows))}},
			}, nil
		}
	}

	rs := &ResultSet{}
	for _, item := range p.Items {
		name := exprName(item)
		rs.Columns = append(rs.Columns, name)
	}

	for _, row := range source.Rows {
		newRow := Row{}
		for _, item := range p.Items {
			name := exprName(item)
			newRow[name] = evalExpr(item.Expr, row, params)
		}
		rs.Rows = append(rs.Rows, newRow)
	}
	return rs, nil
}

func (e *Executor) execSort(p *Sort, params map[string]any) (*ResultSet, error) {
	source, err := e.executePlan(p.Source, params)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(source.Rows, func(i, j int) bool {
		for _, item := range p.Items {
			vi := evalExpr(item.Expr, source.Rows[i], params)
			vj := evalExpr(item.Expr, source.Rows[j], params)
			cmp := compareValues(vi, vj)
			if cmp == 0 {
				continue
			}
			if item.Desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
	return source, nil
}

func (e *Executor) execLimitOffset(p *LimitOffset, params map[string]any) (*ResultSet, error) {
	source, err := e.executePlan(p.Source, params)
	if err != nil {
		return nil, err
	}

	start := 0
	if p.Skip != nil {
		start = *p.Skip
	}
	if start > len(source.Rows) {
		start = len(source.Rows)
	}
	source.Rows = source.Rows[start:]

	if p.Limit != nil && *p.Limit < len(source.Rows) {
		source.Rows = source.Rows[:*p.Limit]
	}
	return source, nil
}

func (e *Executor) execCreateNode(p *CreateNodePlan, params map[string]any) (*ResultSet, error) {
	props := graph.Properties{}
	for k, expr := range p.Props {
		props[k] = evalExpr(expr, nil, params)
	}
	node, err := e.graph.CreateNode(p.Label, props)
	if err != nil {
		return nil, err
	}
	col := p.Variable
	if col == "" {
		col = "node"
	}
	return &ResultSet{
		Columns: []string{col},
		Rows:    []Row{{col: nodeToMap(node)}},
	}, nil
}

func (e *Executor) execCreateEdge(p *CreateEdgePlan, _ map[string]any) (*ResultSet, error) {
	// Variables reference existing nodes by variable name as node ID
	edge, err := e.graph.CreateEdge(p.EdgeLabel,
		graph.NodeID(p.SrcVar), graph.NodeID(p.DestVar), nil)
	if err != nil {
		return nil, err
	}
	return &ResultSet{
		Columns: []string{"edge"},
		Rows:    []Row{{"edge": edgeToMap(edge)}},
	}, nil
}

func (e *Executor) execMergeNode(p *MergeNodePlan, params map[string]any) (*ResultSet, error) {
	props := graph.Properties{}
	for k, expr := range p.Props {
		props[k] = evalExpr(expr, nil, params)
	}

	// Find existing by label + props match
	nodes, err := e.graph.NodesByLabel(p.Label)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if matchPropsExact(n.Properties, props) {
			col := p.Variable
			if col == "" {
				col = "node"
			}
			return &ResultSet{
				Columns: []string{col},
				Rows:    []Row{{col: nodeToMap(n)}},
			}, nil
		}
	}

	// Create if not found
	node, err := e.graph.CreateNode(p.Label, props)
	if err != nil {
		return nil, err
	}
	col := p.Variable
	if col == "" {
		col = "node"
	}
	return &ResultSet{
		Columns: []string{col},
		Rows:    []Row{{col: nodeToMap(node)}},
	}, nil
}

func (e *Executor) execSetProperty(p *SetPropertyPlan, params map[string]any) (*ResultSet, error) {
	source, err := e.executePlan(p.Source, params)
	if err != nil {
		return nil, err
	}

	for _, row := range source.Rows {
		for _, item := range p.Items {
			nodeMap, ok := row[item.Variable].(map[string]any)
			if !ok {
				continue
			}
			nodeID := graph.NodeID(fmt.Sprint(nodeMap["id"]))
			node, err := e.graph.GetNode(nodeID)
			if err != nil {
				continue
			}
			val := evalExpr(item.Value, row, params)
			node.Properties[item.Property] = val
			if err := e.graph.StorageBackend().PutNode(node); err != nil {
				return nil, err
			}
		}
	}
	return source, nil
}

func (e *Executor) execDelete(p *DeletePlan, params map[string]any) (*ResultSet, error) {
	source, err := e.executePlan(p.Source, params)
	if err != nil {
		return nil, err
	}

	for _, row := range source.Rows {
		for _, varName := range p.Variables {
			nodeMap, ok := row[varName].(map[string]any)
			if !ok {
				continue
			}
			nodeID := graph.NodeID(fmt.Sprint(nodeMap["id"]))
			if err := e.graph.DeleteNode(nodeID); err != nil {
				return nil, err
			}
		}
	}
	return &ResultSet{Columns: []string{}, Rows: nil}, nil
}

func (e *Executor) execProcedure(p *ProcedureCallPlan, params map[string]any) (*ResultSet, error) {
	var args []any
	for _, expr := range p.Args {
		args = append(args, evalExpr(expr, nil, params))
	}
	rs, err := e.procs.Call(p.Procedure, args)
	if err != nil {
		return nil, err
	}

	// Filter to YIELD columns if specified
	if len(p.YieldVars) > 0 {
		filtered := &ResultSet{Columns: p.YieldVars}
		for _, row := range rs.Rows {
			newRow := Row{}
			for _, col := range p.YieldVars {
				newRow[col] = row[col]
			}
			filtered.Rows = append(filtered.Rows, newRow)
		}
		return filtered, nil
	}
	return rs, nil
}

func (e *Executor) registerBuiltins() {
	if e.fullText != nil {
		e.procs.Register("kafgraph.fullTextSearch", func(args []any) (*ResultSet, error) {
			if len(args) < 3 {
				return nil, fmt.Errorf("fullTextSearch requires 3 args: label, property, query")
			}
			label := fmt.Sprint(args[0])
			property := fmt.Sprint(args[1])
			queryStr := fmt.Sprint(args[2])
			limit := 100
			results, err := e.fullText.Search(label, property, queryStr, limit)
			if err != nil {
				return nil, err
			}
			rs := &ResultSet{Columns: []string{"node", "score"}}
			for _, r := range results {
				node, err := e.graph.GetNode(r.NodeID)
				if err != nil {
					continue
				}
				rs.Rows = append(rs.Rows, Row{"node": nodeToMap(node), "score": r.Score})
			}
			return rs, nil
		})
	}

	if e.vector != nil {
		e.procs.Register("kafgraph.vectorSearch", func(args []any) (*ResultSet, error) {
			if len(args) < 4 {
				return nil, fmt.Errorf("vectorSearch requires 4 args: label, property, vector, k")
			}
			label := fmt.Sprint(args[0])
			property := fmt.Sprint(args[1])
			vecAny, ok := args[2].([]any)
			if !ok {
				return nil, fmt.Errorf("vector argument must be a list")
			}
			vec := make([]float32, len(vecAny))
			for i, v := range vecAny {
				switch val := v.(type) {
				case float64:
					vec[i] = float32(val)
				case int64:
					vec[i] = float32(val)
				default:
					return nil, fmt.Errorf("vector element %d: unexpected type %T", i, v)
				}
			}
			k := 10
			if kVal, ok := args[3].(int64); ok {
				k = int(kVal)
			}
			results, err := e.vector.Search(label, property, vec, k)
			if err != nil {
				return nil, err
			}
			rs := &ResultSet{Columns: []string{"node", "score"}}
			for _, r := range results {
				node, err := e.graph.GetNode(r.NodeID)
				if err != nil {
					continue
				}
				rs.Rows = append(rs.Rows, Row{"node": nodeToMap(node), "score": r.Score})
			}
			return rs, nil
		})
	}
}

// --- Helper functions ---

func nodeToMap(n *graph.Node) map[string]any {
	return map[string]any{
		"id":         string(n.ID),
		"label":      n.Label,
		"properties": map[string]any(n.Properties),
		"createdAt":  formatTime(n.CreatedAt),
	}
}

func edgeToMap(e *graph.Edge) map[string]any {
	return map[string]any{
		"id":         string(e.ID),
		"label":      e.Label,
		"fromId":     string(e.FromID),
		"toId":       string(e.ToID),
		"properties": map[string]any(e.Properties),
		"createdAt":  formatTime(e.CreatedAt),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func copyRow(r Row) Row {
	newRow := Row{}
	for k, v := range r {
		newRow[k] = v
	}
	return newRow
}

func exprName(item ReturnItem) string {
	if item.Alias != "" {
		return item.Alias
	}
	switch e := item.Expr.(type) {
	case *IdentExpr:
		return e.Name
	case *PropertyExpr:
		return e.Variable + "." + e.Property
	case *FuncCallExpr:
		return e.Name + "(*)"
	case *StarExpr:
		return "*"
	}
	return "?"
}

func evalExpr(expr Expr, row Row, params map[string]any) any {
	switch e := expr.(type) {
	case *LiteralExpr:
		return e.Value
	case *IdentExpr:
		if row != nil {
			return row[e.Name]
		}
		return nil
	case *PropertyExpr:
		if row != nil {
			if nodeMap, ok := row[e.Variable].(map[string]any); ok {
				if props, ok := nodeMap["properties"].(graph.Properties); ok {
					return props[e.Property]
				}
				if props, ok := nodeMap["properties"].(map[string]any); ok {
					return props[e.Property]
				}
			}
		}
		return nil
	case *ParamExpr:
		if params != nil {
			return params[e.Name]
		}
		return nil
	case *StarExpr:
		return "*"
	case *ListExpr:
		var result []any
		for _, elem := range e.Elements {
			result = append(result, evalExpr(elem, row, params))
		}
		return result
	}
	return nil
}

func evalBool(expr Expr, row Row, params map[string]any) bool {
	switch e := expr.(type) {
	case *BinaryExpr:
		switch e.Op {
		case "AND":
			return evalBool(e.Left, row, params) && evalBool(e.Right, row, params)
		case "OR":
			return evalBool(e.Left, row, params) || evalBool(e.Right, row, params)
		case "=":
			return fmt.Sprint(evalExpr(e.Left, row, params)) == fmt.Sprint(evalExpr(e.Right, row, params))
		case "<>":
			return fmt.Sprint(evalExpr(e.Left, row, params)) != fmt.Sprint(evalExpr(e.Right, row, params))
		case "<":
			return compareValues(evalExpr(e.Left, row, params), evalExpr(e.Right, row, params)) < 0
		case ">":
			return compareValues(evalExpr(e.Left, row, params), evalExpr(e.Right, row, params)) > 0
		case "<=":
			return compareValues(evalExpr(e.Left, row, params), evalExpr(e.Right, row, params)) <= 0
		case ">=":
			return compareValues(evalExpr(e.Left, row, params), evalExpr(e.Right, row, params)) >= 0
		case "CONTAINS":
			l := fmt.Sprint(evalExpr(e.Left, row, params))
			r := fmt.Sprint(evalExpr(e.Right, row, params))
			return strings.Contains(l, r)
		case "IN":
			l := evalExpr(e.Left, row, params)
			r := evalExpr(e.Right, row, params)
			if list, ok := r.([]any); ok {
				for _, v := range list {
					if fmt.Sprint(l) == fmt.Sprint(v) {
						return true
					}
				}
			}
			return false
		}
	case *UnaryExpr:
		if e.Op == "NOT" {
			return !evalBool(e.Expr, row, params)
		}
	case *LiteralExpr:
		if b, ok := e.Value.(bool); ok {
			return b
		}
	}
	return false
}

func compareValues(a, b any) int {
	as := fmt.Sprint(a)
	bs := fmt.Sprint(b)
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
}

func matchProps(nodeProps graph.Properties, filterProps map[string]Expr) bool {
	for k, expr := range filterProps {
		lit, ok := expr.(*LiteralExpr)
		if !ok {
			continue
		}
		if fmt.Sprint(nodeProps[k]) != fmt.Sprint(lit.Value) {
			return false
		}
	}
	return true
}

func matchPropsExact(nodeProps graph.Properties, props graph.Properties) bool {
	for k, v := range props {
		if fmt.Sprint(nodeProps[k]) != fmt.Sprint(v) {
			return false
		}
	}
	return true
}
