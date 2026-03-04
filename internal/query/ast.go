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

// Statement is the top-level AST node for a Cypher query.
type Statement struct {
	Clauses []Clause
}

// Clause is implemented by all clause types.
type Clause interface {
	clauseNode()
}

// MatchClause represents a MATCH clause with one or more patterns.
type MatchClause struct {
	Patterns []Pattern
}

func (*MatchClause) clauseNode() {}

// WhereClause represents a WHERE clause.
type WhereClause struct {
	Expr Expr
}

func (*WhereClause) clauseNode() {}

// ReturnClause represents a RETURN clause.
type ReturnClause struct {
	Items   []ReturnItem
	OrderBy []OrderItem
	Limit   *int
	Skip    *int
}

func (*ReturnClause) clauseNode() {}

// ReturnItem is a single item in a RETURN clause.
type ReturnItem struct {
	Expr  Expr
	Alias string
}

// OrderItem is a single ORDER BY item.
type OrderItem struct {
	Expr Expr
	Desc bool
}

// CreateClause represents a CREATE clause.
type CreateClause struct {
	Patterns []Pattern
}

func (*CreateClause) clauseNode() {}

// MergeClause represents a MERGE clause.
type MergeClause struct {
	Pattern Pattern
}

func (*MergeClause) clauseNode() {}

// SetClause represents a SET clause.
type SetClause struct {
	Items []SetItem
}

func (*SetClause) clauseNode() {}

// SetItem is a single property assignment.
type SetItem struct {
	Variable string
	Property string
	Value    Expr
}

// DeleteClause represents a DELETE clause.
type DeleteClause struct {
	Variables []string
}

func (*DeleteClause) clauseNode() {}

// CallClause represents a CALL procedure clause.
type CallClause struct {
	Procedure string
	Args      []Expr
	YieldVars []string
}

func (*CallClause) clauseNode() {}

// Pattern represents a graph pattern (node or node-relationship-node).
type Pattern struct {
	Nodes []NodePattern
	Edges []EdgePattern
}

// NodePattern represents a node in a pattern: (var:Label {props}).
type NodePattern struct {
	Variable   string
	Label      string
	Properties map[string]Expr
}

// EdgePattern represents a relationship in a pattern: -[:LABEL]-> or <-[:LABEL]-.
type EdgePattern struct {
	Variable  string
	Label     string
	Direction EdgeDirection
}

// EdgeDirection indicates the direction of a relationship pattern.
type EdgeDirection int

const (
	// EdgeRight indicates a right-directed relationship: (a)-[:X]->(b).
	EdgeRight EdgeDirection = iota
	// EdgeLeft indicates a left-directed relationship: (a)<-[:X]-(b).
	EdgeLeft
)

// Expr is implemented by all expression types.
type Expr interface {
	exprNode()
}

// IdentExpr represents a variable reference: n
type IdentExpr struct {
	Name string
}

func (*IdentExpr) exprNode() {}

// PropertyExpr represents a property access: n.name
type PropertyExpr struct {
	Variable string
	Property string
}

func (*PropertyExpr) exprNode() {}

// LiteralExpr represents a literal value: 42, 'hello', true, null
type LiteralExpr struct {
	Value any
}

func (*LiteralExpr) exprNode() {}

// ParamExpr represents a parameter: $name
type ParamExpr struct {
	Name string
}

func (*ParamExpr) exprNode() {}

// BinaryExpr represents a binary operation: a = b, a AND b
type BinaryExpr struct {
	Left  Expr
	Op    string
	Right Expr
}

func (*BinaryExpr) exprNode() {}

// UnaryExpr represents a unary operation: NOT x
type UnaryExpr struct {
	Op   string
	Expr Expr
}

func (*UnaryExpr) exprNode() {}

// FuncCallExpr represents a function call: count(*)
type FuncCallExpr struct {
	Name string
	Args []Expr
}

func (*FuncCallExpr) exprNode() {}

// StarExpr represents the * wildcard.
type StarExpr struct{}

func (*StarExpr) exprNode() {}

// ListExpr represents a list literal: [1, 2, 3]
type ListExpr struct {
	Elements []Expr
}

func (*ListExpr) exprNode() {}
