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

// PlanNode is the interface for all execution plan nodes.
type PlanNode interface {
	planNode()
}

// ScanByLabel scans all nodes with a given label.
type ScanByLabel struct {
	Variable string
	Label    string
	Props    map[string]Expr // optional inline property filter
}

func (*ScanByLabel) planNode() {}

// ExpandOut follows outgoing edges from a source variable.
type ExpandOut struct {
	Source    PlanNode
	SrcVar    string
	EdgeVar   string
	EdgeLabel string
	DestVar   string
	DestLabel string
}

func (*ExpandOut) planNode() {}

// ExpandIn follows incoming edges to a source variable.
type ExpandIn struct {
	Source    PlanNode
	SrcVar    string
	EdgeVar   string
	EdgeLabel string
	DestVar   string
	DestLabel string
}

func (*ExpandIn) planNode() {}

// Filter applies a predicate expression.
type Filter struct {
	Source PlanNode
	Expr   Expr
}

func (*Filter) planNode() {}

// Project selects output columns.
type Project struct {
	Source PlanNode
	Items  []ReturnItem
}

func (*Project) planNode() {}

// Sort orders results.
type Sort struct {
	Source PlanNode
	Items  []OrderItem
}

func (*Sort) planNode() {}

// LimitOffset limits and/or offsets results.
type LimitOffset struct {
	Source PlanNode
	Limit  *int
	Skip   *int
}

func (*LimitOffset) planNode() {}

// CreateNodePlan creates a new node.
type CreateNodePlan struct {
	Variable string
	Label    string
	Props    map[string]Expr
}

func (*CreateNodePlan) planNode() {}

// CreateEdgePlan creates a new edge.
type CreateEdgePlan struct {
	SrcVar    string
	DestVar   string
	EdgeLabel string
}

func (*CreateEdgePlan) planNode() {}

// MergeNodePlan merges (upsert) a node.
type MergeNodePlan struct {
	Variable string
	Label    string
	Props    map[string]Expr
}

func (*MergeNodePlan) planNode() {}

// SetPropertyPlan sets a property on a variable.
type SetPropertyPlan struct {
	Source PlanNode
	Items  []SetItem
}

func (*SetPropertyPlan) planNode() {}

// DeletePlan deletes nodes by variable.
type DeletePlan struct {
	Source    PlanNode
	Variables []string
}

func (*DeletePlan) planNode() {}

// ProcedureCallPlan executes a registered procedure.
type ProcedureCallPlan struct {
	Procedure string
	Args      []Expr
	YieldVars []string
}

func (*ProcedureCallPlan) planNode() {}
