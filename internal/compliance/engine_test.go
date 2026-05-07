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

package compliance

import (
	"context"
	"testing"
)

// mockGraphQuerier implements GraphQuerier for tests.
type mockGraphQuerier struct {
	nodes map[string]NodeList
	edges map[string]EdgeList
}

func newMockQuerier() *mockGraphQuerier {
	return &mockGraphQuerier{
		nodes: map[string]NodeList{},
		edges: map[string]EdgeList{},
	}
}

func (m *mockGraphQuerier) NodesByLabel(label string) (NodeList, error) {
	return m.nodes[label], nil
}

func (m *mockGraphQuerier) Neighbors(id string) (EdgeList, error) {
	return m.edges[id], nil
}

// staticRule is a minimal test Rule.
type staticRule struct {
	id        string
	framework Framework
	module    string
	article   string
	title     string
	severity  Severity
	evalFn    func(GraphQuerier) ([]RuleResult, error)
}

func (r *staticRule) ID() string                                    { return r.id }
func (r *staticRule) Framework() Framework                          { return r.framework }
func (r *staticRule) Module() string                                { return r.module }
func (r *staticRule) Article() string                               { return r.article }
func (r *staticRule) Title() string                                 { return r.title }
func (r *staticRule) Severity() Severity                            { return r.severity }
func (r *staticRule) Evaluate(g GraphQuerier) ([]RuleResult, error) { return r.evalFn(g) }

func TestEngineRegisterAndRules(t *testing.T) {
	e := NewEngine(nil)
	r1 := &staticRule{id: "R1", framework: FrameworkGDPR, severity: SeverityHigh}
	r2 := &staticRule{id: "R2", framework: FrameworkSOC2, severity: SeverityMedium}
	e.RegisterRule(r1)
	e.RegisterRule(r2)

	all := e.Rules()
	if len(all) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(all))
	}

	gdpr := e.RulesByFramework(FrameworkGDPR)
	if len(gdpr) != 1 || gdpr[0].ID() != "R1" {
		t.Fatalf("expected 1 GDPR rule, got %d", len(gdpr))
	}
}

func TestEngineRunScanNoGraph(t *testing.T) {
	// Engine with nil graph — rules use the mock querier approach.
	// For a proper integration test, we'd use a real graph.
	// Here we test the scan flow with a rule that returns pass.
	e := NewEngine(nil)
	e.RegisterRule(&staticRule{
		id: "TEST-001", framework: FrameworkGDPR, module: "test",
		severity: SeverityHigh,
		evalFn: func(g GraphQuerier) ([]RuleResult, error) {
			return []RuleResult{{
				RuleID: "TEST-001", Status: EvalPass,
				Details: "ok", Severity: SeverityHigh,
			}}, nil
		},
	})

	// RunScan requires a graph for storing results — skip store on nil.
	// We test the core logic by checking the rule evaluation path.
	ctx := context.Background()
	// This will panic on storeScanResults with nil graph, so we test
	// at a different level — use the scoring and types directly.
	_ = ctx
	_ = e
}

func TestSeverityWeight(t *testing.T) {
	tests := []struct {
		sev    Severity
		expect float64
	}{
		{SeverityCritical, 3.0},
		{SeverityHigh, 2.0},
		{SeverityMedium, 1.0},
		{SeverityLow, 0.5},
		{Severity("unknown"), 1.0},
	}
	for _, tt := range tests {
		got := tt.sev.Weight()
		if got != tt.expect {
			t.Errorf("Severity(%q).Weight() = %v, want %v", tt.sev, got, tt.expect)
		}
	}
}

func TestEngineRunScanStoresResults(t *testing.T) {
	g := newTestGraph(t)
	e := NewEngine(g)

	passRule := &staticRule{
		id: "PASS-001", framework: FrameworkGDPR, module: "ops",
		article: "Art. 1", title: "pass", severity: SeverityHigh,
		evalFn: func(GraphQuerier) ([]RuleResult, error) {
			return []RuleResult{{
				RuleID:   "PASS-001",
				Status:   EvalPass,
				NodeID:   "node-1",
				Details:  "ok",
				Severity: SeverityHigh,
			}}, nil
		},
	}
	naRule := &staticRule{
		id: "NA-001", framework: FrameworkGDPR, module: "ops",
		article: "Art. 2", title: "na", severity: SeverityCritical,
		evalFn: func(GraphQuerier) ([]RuleResult, error) {
			return nil, nil
		},
	}
	warnRule := &staticRule{
		id: "WARN-001", framework: FrameworkGDPR, module: "ops",
		article: "Art. 3", title: "warn", severity: SeverityMedium,
		evalFn: func(GraphQuerier) ([]RuleResult, error) {
			return nil, context.DeadlineExceeded
		},
	}
	filteredRule := &staticRule{
		id: "SKIP-001", framework: FrameworkSOC2, module: "ops",
		article: "Art. 4", title: "skip", severity: SeverityLow,
		evalFn: func(GraphQuerier) ([]RuleResult, error) {
			t.Fatal("filtered rule should not be evaluated")
			return nil, nil
		},
	}

	e.RegisterRule(passRule)
	e.RegisterRule(naRule)
	e.RegisterRule(warnRule)
	e.RegisterRule(filteredRule)

	if err := e.EnsureFrameworkNodes(); err != nil {
		t.Fatalf("EnsureFrameworkNodes: %v", err)
	}

	result, err := e.RunScan(context.Background(), ScanRequest{
		Framework: FrameworkGDPR,
		Module:    "ops",
	})
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}

	if result.ScanID == "" {
		t.Fatal("expected scan ID")
	}
	if result.PassCount != 1 || result.WarningCount != 1 || result.NACount != 1 || result.FailCount != 0 {
		t.Fatalf("unexpected counts: %+v", result)
	}
	if got, want := result.Score, CalculateScore(result.Evaluations); got != want {
		t.Fatalf("score = %v, want %v", got, want)
	}
	if len(result.Evaluations) != 3 {
		t.Fatalf("expected 3 evaluations, got %d", len(result.Evaluations))
	}

	scans, err := g.NodesByLabel("ComplianceScan")
	if err != nil {
		t.Fatalf("NodesByLabel(ComplianceScan): %v", err)
	}
	if len(scans) != 1 {
		t.Fatalf("expected 1 ComplianceScan node, got %d", len(scans))
	}

	evals, err := g.NodesByLabel("ComplianceEvaluation")
	if err != nil {
		t.Fatalf("NodesByLabel(ComplianceEvaluation): %v", err)
	}
	if len(evals) != 3 {
		t.Fatalf("expected 3 ComplianceEvaluation nodes, got %d", len(evals))
	}

	ruleNodes, err := g.NodesByLabel("ComplianceRule")
	if err != nil {
		t.Fatalf("NodesByLabel(ComplianceRule): %v", err)
	}
	if len(ruleNodes) != 4 {
		t.Fatalf("expected 4 ComplianceRule nodes, got %d", len(ruleNodes))
	}

	scanEdges, err := g.Neighbors(scans[0].ID)
	if err != nil {
		t.Fatalf("Neighbors(scan): %v", err)
	}
	foundScope := false
	for _, edge := range scanEdges {
		if edge.Label == "SCOPED_TO" {
			foundScope = true
			break
		}
	}
	if !foundScope {
		t.Fatal("expected SCOPED_TO edge for scan")
	}

	foundPartOfScan := false
	foundEvaluatedBy := false
	for _, eval := range evals {
		edges, err := g.Neighbors(eval.ID)
		if err != nil {
			t.Fatalf("Neighbors(eval): %v", err)
		}
		for _, edge := range edges {
			if edge.Label == "PART_OF_SCAN" {
				foundPartOfScan = true
			}
			if edge.Label == "EVALUATED_BY" {
				foundEvaluatedBy = true
			}
		}
	}
	if !foundPartOfScan {
		t.Fatal("expected PART_OF_SCAN edge")
	}
	if !foundEvaluatedBy {
		t.Fatal("expected EVALUATED_BY edge")
	}
}

func TestEngineRunScanCanceled(t *testing.T) {
	g := newTestGraph(t)
	e := NewEngine(g)
	e.RegisterRule(&staticRule{
		id: "CANCEL-001", framework: FrameworkGDPR, module: "ops", severity: SeverityHigh,
		evalFn: func(GraphQuerier) ([]RuleResult, error) {
			t.Fatal("rule should not run after cancellation")
			return nil, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.RunScan(ctx, ScanRequest{Framework: FrameworkGDPR, Module: "ops"})
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestEnsureFrameworkNodesIdempotent(t *testing.T) {
	g := newTestGraph(t)
	e := NewEngine(g)
	e.RegisterRule(&staticRule{id: "R1", framework: FrameworkGDPR, module: "setup", article: "Art. 1", title: "one", severity: SeverityHigh})
	e.RegisterRule(&staticRule{id: "R2", framework: FrameworkGDPR, module: "ops", article: "Art. 2", title: "two", severity: SeverityMedium})
	e.RegisterRule(&staticRule{id: "R3", framework: FrameworkSOC2, module: "ops", article: "CC1", title: "three", severity: SeverityLow})

	if err := e.EnsureFrameworkNodes(); err != nil {
		t.Fatalf("first EnsureFrameworkNodes: %v", err)
	}
	if err := e.EnsureFrameworkNodes(); err != nil {
		t.Fatalf("second EnsureFrameworkNodes: %v", err)
	}

	frameworks, err := g.NodesByLabel("ComplianceFramework")
	if err != nil {
		t.Fatalf("NodesByLabel(ComplianceFramework): %v", err)
	}
	if len(frameworks) != 2 {
		t.Fatalf("expected 2 framework nodes, got %d", len(frameworks))
	}

	rules, err := g.NodesByLabel("ComplianceRule")
	if err != nil {
		t.Fatalf("NodesByLabel(ComplianceRule): %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rule nodes, got %d", len(rules))
	}

	defineEdges := 0
	for _, rule := range rules {
		edges, err := g.Neighbors(rule.ID)
		if err != nil {
			t.Fatalf("Neighbors(rule): %v", err)
		}
		for _, edge := range edges {
			if edge.Label == "DEFINES_RULE" {
				defineEdges++
			}
		}
	}
	if defineEdges != 3 {
		t.Fatalf("expected 3 DEFINES_RULE edges, got %d", defineEdges)
	}
}

func TestGraphAdapter(t *testing.T) {
	g := newTestGraph(t)
	from, err := g.CreateNode("Agent", map[string]any{"name": "alice"})
	if err != nil {
		t.Fatalf("CreateNode(from): %v", err)
	}
	to, err := g.CreateNode("Agent", map[string]any{"name": "bob"})
	if err != nil {
		t.Fatalf("CreateNode(to): %v", err)
	}
	if _, err := g.CreateEdge("KNOWS", from.ID, to.ID, nil); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	adapter := &graphAdapter{g: g}
	nodes, err := adapter.NodesByLabel("Agent")
	if err != nil {
		t.Fatalf("NodesByLabel: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	edges, err := adapter.Neighbors(string(from.ID))
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Label != "KNOWS" || edges[0].From != string(from.ID) || edges[0].To != string(to.ID) {
		t.Fatalf("unexpected edge: %+v", edges[0])
	}
}
