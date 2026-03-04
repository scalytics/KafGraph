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
	"testing"

	"github.com/scalytics/kafgraph/internal/graph"
)

func TestValidateDataFlows_AllPass(t *testing.T) {
	g := newTestGraph(t)

	// Create required entities.
	cat, _ := g.CreateNode("DataCategory", graph.Properties{"name": "Contact", "isSpecial": false})
	lb, _ := g.CreateNode("LegalBasis", graph.Properties{"name": "Consent", "article": "Art. 6(1)(a)"})
	pa, _ := g.CreateNode("ProcessingActivity", graph.Properties{"name": "PA1", "status": "active"})

	// Create a well-formed data flow.
	flow, _ := g.CreateNode("DataFlow", graph.Properties{"name": "Good Flow", "transferType": "internal"})
	_, _ = g.CreateEdge("FROM_ACTIVITY", flow.ID, pa.ID, nil)
	_, _ = g.CreateEdge("CARRIES", flow.ID, cat.ID, nil)
	_, _ = g.CreateEdge("GOVERNED_BY", flow.ID, lb.ID, nil)

	results, err := ValidateDataFlows(g, "")
	if err != nil {
		t.Fatalf("ValidateDataFlows: %v", err)
	}

	// Should have 1 flow result + 0 activity warnings (PA has a flow).
	flowResults := 0
	for _, r := range results {
		if r.FlowName == "Good Flow" {
			flowResults++
			if r.Overall != EvalPass {
				t.Errorf("expected overall pass for Good Flow, got %s", r.Overall)
			}
		}
	}
	if flowResults != 1 {
		t.Errorf("expected 1 flow result for Good Flow, got %d", flowResults)
	}
}

func TestValidateDataFlows_MissingCategories(t *testing.T) {
	g := newTestGraph(t)

	lb, _ := g.CreateNode("LegalBasis", graph.Properties{"name": "Consent"})

	// Flow without CARRIES edge.
	flow, _ := g.CreateNode("DataFlow", graph.Properties{"name": "No Categories", "transferType": "internal"})
	_, _ = g.CreateEdge("GOVERNED_BY", flow.ID, lb.ID, nil)

	results, err := ValidateDataFlows(g, "")
	if err != nil {
		t.Fatalf("ValidateDataFlows: %v", err)
	}

	for _, r := range results {
		if r.FlowName != "No Categories" {
			continue
		}
		if r.Overall != EvalFail {
			t.Errorf("expected overall fail, got %s", r.Overall)
		}
		foundFlow001 := false
		for _, c := range r.Checks {
			if c.RuleID == "GDPR-FLOW-001" && c.Status == EvalFail {
				foundFlow001 = true
			}
		}
		if !foundFlow001 {
			t.Error("expected GDPR-FLOW-001 fail check")
		}
	}
}

func TestValidateDataFlows_MissingLegalBasis(t *testing.T) {
	g := newTestGraph(t)

	cat, _ := g.CreateNode("DataCategory", graph.Properties{"name": "Contact"})

	// Flow without GOVERNED_BY edge.
	flow, _ := g.CreateNode("DataFlow", graph.Properties{"name": "No Legal Basis", "transferType": "internal"})
	_, _ = g.CreateEdge("CARRIES", flow.ID, cat.ID, nil)

	results, err := ValidateDataFlows(g, "")
	if err != nil {
		t.Fatalf("ValidateDataFlows: %v", err)
	}

	for _, r := range results {
		if r.FlowName != "No Legal Basis" {
			continue
		}
		if r.Overall != EvalFail {
			t.Errorf("expected overall fail, got %s", r.Overall)
		}
		foundFlow002 := false
		for _, c := range r.Checks {
			if c.RuleID == "GDPR-FLOW-002" && c.Status == EvalFail {
				foundFlow002 = true
			}
		}
		if !foundFlow002 {
			t.Error("expected GDPR-FLOW-002 fail check")
		}
	}
}

func TestValidateDataFlows_InternationalWithoutSafeguard(t *testing.T) {
	g := newTestGraph(t)

	cat, _ := g.CreateNode("DataCategory", graph.Properties{"name": "Contact"})
	lb, _ := g.CreateNode("LegalBasis", graph.Properties{"name": "Contract"})

	// International flow without safeguard.
	flow, _ := g.CreateNode("DataFlow", graph.Properties{
		"name":         "Unsafe International",
		"transferType": "international",
	})
	_, _ = g.CreateEdge("CARRIES", flow.ID, cat.ID, nil)
	_, _ = g.CreateEdge("GOVERNED_BY", flow.ID, lb.ID, nil)

	results, err := ValidateDataFlows(g, "")
	if err != nil {
		t.Fatalf("ValidateDataFlows: %v", err)
	}

	for _, r := range results {
		if r.FlowName != "Unsafe International" {
			continue
		}
		if r.Overall != EvalFail {
			t.Errorf("expected overall fail, got %s", r.Overall)
		}
		foundFlow003 := false
		for _, c := range r.Checks {
			if c.RuleID == "GDPR-FLOW-003" && c.Status == EvalFail {
				foundFlow003 = true
			}
		}
		if !foundFlow003 {
			t.Error("expected GDPR-FLOW-003 fail check")
		}
	}
}

func TestValidateDataFlows_InternationalWithSafeguard(t *testing.T) {
	g := newTestGraph(t)

	cat, _ := g.CreateNode("DataCategory", graph.Properties{"name": "Contact"})
	lb, _ := g.CreateNode("LegalBasis", graph.Properties{"name": "Contract"})

	flow, _ := g.CreateNode("DataFlow", graph.Properties{
		"name":         "Safe International",
		"transferType": "international",
		"safeguard":    "SCC",
	})
	_, _ = g.CreateEdge("CARRIES", flow.ID, cat.ID, nil)
	_, _ = g.CreateEdge("GOVERNED_BY", flow.ID, lb.ID, nil)

	results, err := ValidateDataFlows(g, "")
	if err != nil {
		t.Fatalf("ValidateDataFlows: %v", err)
	}

	for _, r := range results {
		if r.FlowName != "Safe International" {
			continue
		}
		if r.Overall != EvalPass {
			t.Errorf("expected overall pass, got %s", r.Overall)
		}
	}
}

func TestValidateDataFlows_ActivityWithoutFlow(t *testing.T) {
	g := newTestGraph(t)

	// Active activity with no DataFlow pointing to it.
	_, _ = g.CreateNode("ProcessingActivity", graph.Properties{"name": "Orphan PA", "status": "active"})

	results, err := ValidateDataFlows(g, "")
	if err != nil {
		t.Fatalf("ValidateDataFlows: %v", err)
	}

	foundWarning := false
	for _, r := range results {
		if r.FlowName == "Orphan PA" && r.Overall == EvalWarning {
			foundWarning = true
			for _, c := range r.Checks {
				if c.RuleID != "GDPR-FLOW-005" {
					t.Errorf("expected GDPR-FLOW-005 check, got %s", c.RuleID)
				}
			}
		}
	}
	if !foundWarning {
		t.Error("expected GDPR-FLOW-005 warning for orphan activity")
	}
}

func TestValidateDataFlows_CreatesValidationNodes(t *testing.T) {
	g := newTestGraph(t)

	cat, _ := g.CreateNode("DataCategory", graph.Properties{"name": "Test"})
	lb, _ := g.CreateNode("LegalBasis", graph.Properties{"name": "Consent"})
	flow, _ := g.CreateNode("DataFlow", graph.Properties{"name": "Flow1", "transferType": "internal"})
	_, _ = g.CreateEdge("CARRIES", flow.ID, cat.ID, nil)
	_, _ = g.CreateEdge("GOVERNED_BY", flow.ID, lb.ID, nil)

	_, err := ValidateDataFlows(g, "")
	if err != nil {
		t.Fatalf("ValidateDataFlows: %v", err)
	}

	// Verify DataFlowValidation node was created.
	valNodes, _ := g.NodesByLabel("DataFlowValidation")
	if len(valNodes) != 1 {
		t.Fatalf("expected 1 DataFlowValidation node, got %d", len(valNodes))
	}

	// Verify VALIDATES edge.
	edges, _ := g.Neighbors(valNodes[0].ID)
	foundValidates := false
	for _, e := range edges {
		if e.Label == "VALIDATES" && e.ToID == flow.ID {
			foundValidates = true
		}
	}
	if !foundValidates {
		t.Error("expected VALIDATES edge from validation to flow")
	}
}

func TestValidateDataFlows_WithInspectionLink(t *testing.T) {
	g := newTestGraph(t)

	insp, _ := g.CreateNode("Inspection", graph.Properties{"title": "Test Inspection"})
	flow, _ := g.CreateNode("DataFlow", graph.Properties{"name": "Flow1", "transferType": "internal"})
	cat, _ := g.CreateNode("DataCategory", graph.Properties{"name": "Test"})
	_, _ = g.CreateEdge("CARRIES", flow.ID, cat.ID, nil)
	lb, _ := g.CreateNode("LegalBasis", graph.Properties{"name": "Consent"})
	_, _ = g.CreateEdge("GOVERNED_BY", flow.ID, lb.ID, nil)

	_, err := ValidateDataFlows(g, string(insp.ID))
	if err != nil {
		t.Fatalf("ValidateDataFlows: %v", err)
	}

	// Verify PART_OF_INSPECTION edge.
	valNodes, _ := g.NodesByLabel("DataFlowValidation")
	if len(valNodes) == 0 {
		t.Fatal("expected DataFlowValidation node")
	}
	edges, _ := g.Neighbors(valNodes[0].ID)
	foundPartOf := false
	for _, e := range edges {
		if e.Label == "PART_OF_INSPECTION" {
			foundPartOf = true
		}
	}
	if !foundPartOf {
		t.Error("expected PART_OF_INSPECTION edge")
	}
}

func TestDataFlowValidationResult_SummaryText(t *testing.T) {
	r := &DataFlowValidationResult{
		FlowID:   "f1",
		FlowName: "Test",
		Checks: []ValidationCheck{
			{RuleID: "R1", Status: EvalPass},
			{RuleID: "R2", Status: EvalFail},
			{RuleID: "R3", Status: EvalPass},
			{RuleID: "R4", Status: EvalNA},
		},
	}
	got := r.SummaryText()
	if got != "2 pass, 1 fail" {
		t.Errorf("SummaryText() = %q, want %q", got, "2 pass, 1 fail")
	}
}

// --- Engine rule tests ---

func TestGDPRFlow001_Rule(t *testing.T) {
	q := newMockQuerier()
	q.nodes["DataFlow"] = NodeList{
		{ID: "flow-1", Properties: map[string]any{"name": "F1"}},
	}
	q.edges["flow-1"] = EdgeList{
		{Label: "CARRIES", To: "cat-1"},
	}

	rule := &gdprFlow001{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != EvalPass {
		t.Errorf("expected 1 pass result, got %v", results)
	}
}

func TestGDPRFlow002_Rule(t *testing.T) {
	q := newMockQuerier()
	q.nodes["DataFlow"] = NodeList{
		{ID: "flow-1", Properties: map[string]any{"name": "F1"}},
	}
	// No GOVERNED_BY edge.
	q.edges["flow-1"] = EdgeList{}

	rule := &gdprFlow002{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != EvalFail {
		t.Errorf("expected 1 fail result, got %v", results)
	}
}

func TestGDPRFlow003_Rule(t *testing.T) {
	q := newMockQuerier()
	q.nodes["DataFlow"] = NodeList{
		{ID: "flow-1", Properties: map[string]any{"name": "F1", "transferType": "international", "safeguard": "SCC"}},
		{ID: "flow-2", Properties: map[string]any{"name": "F2", "transferType": "international"}},
		{ID: "flow-3", Properties: map[string]any{"name": "F3", "transferType": "internal"}},
	}

	rule := &gdprFlow003{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	// flow-1: pass, flow-2: fail, flow-3: not international so skipped
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != EvalPass {
		t.Errorf("flow-1 expected pass, got %s", results[0].Status)
	}
	if results[1].Status != EvalFail {
		t.Errorf("flow-2 expected fail, got %s", results[1].Status)
	}
}

func TestGDPRFlow005_Rule(t *testing.T) {
	q := newMockQuerier()
	q.nodes["DataFlow"] = NodeList{
		{ID: "flow-1", Properties: map[string]any{"name": "F1"}},
	}
	q.edges["flow-1"] = EdgeList{
		{Label: "FROM_ACTIVITY", To: "pa-1"},
	}
	q.nodes["ProcessingActivity"] = NodeList{
		{ID: "pa-1", Properties: map[string]any{"name": "Covered PA", "status": "active"}},
		{ID: "pa-2", Properties: map[string]any{"name": "Uncovered PA", "status": "active"}},
	}

	rule := &gdprFlow005{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// pa-1 should pass, pa-2 should warn.
	statusMap := map[string]EvalStatus{}
	for _, r := range results {
		statusMap[r.NodeID] = r.Status
	}
	if statusMap["pa-1"] != EvalPass {
		t.Errorf("pa-1 expected pass, got %s", statusMap["pa-1"])
	}
	if statusMap["pa-2"] != EvalWarning {
		t.Errorf("pa-2 expected warning, got %s", statusMap["pa-2"])
	}
}

func TestGDPRInsp001_Rule_OverdueFinding(t *testing.T) {
	q := newMockQuerier()
	q.nodes["InspectionFinding"] = NodeList{
		{ID: "f-1", Properties: map[string]any{
			"status":     "open",
			"targetDate": "2020-01-01T00:00:00Z", // well in the past
		}},
	}

	rule := &gdprInsp001{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != EvalFail {
		t.Errorf("expected 1 fail result for overdue finding, got %v", results)
	}
}

func TestGDPRInsp001_Rule_FutureFinding(t *testing.T) {
	q := newMockQuerier()
	q.nodes["InspectionFinding"] = NodeList{
		{ID: "f-1", Properties: map[string]any{
			"status":     "open",
			"targetDate": "2099-01-01T00:00:00Z",
		}},
	}

	rule := &gdprInsp001{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != EvalPass {
		t.Errorf("expected 1 pass result, got %v", results)
	}
}

func TestGDPRInsp002_Rule(t *testing.T) {
	q := newMockQuerier()
	q.nodes["RemediationAction"] = NodeList{
		{ID: "r-1", Properties: map[string]any{"status": "completed", "verifiedBy": "reviewer"}},
		{ID: "r-2", Properties: map[string]any{"status": "completed"}}, // not verified
		{ID: "r-3", Properties: map[string]any{"status": "pending"}},   // not completed, skip
	}

	rule := &gdprInsp002{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (skip pending), got %d", len(results))
	}
	statusMap := map[string]EvalStatus{}
	for _, r := range results {
		statusMap[r.NodeID] = r.Status
	}
	if statusMap["r-1"] != EvalPass {
		t.Errorf("r-1 expected pass, got %s", statusMap["r-1"])
	}
	if statusMap["r-2"] != EvalFail {
		t.Errorf("r-2 expected fail, got %s", statusMap["r-2"])
	}
}

func TestRegisterDataFlowRules(t *testing.T) {
	e := NewEngine(nil)
	RegisterDataFlowRules(e)

	rules := e.Rules()
	if len(rules) != 7 {
		t.Fatalf("expected 7 data flow rules, got %d", len(rules))
	}

	ids := map[string]bool{}
	for _, r := range rules {
		ids[r.ID()] = true
	}
	expected := []string{
		"GDPR-FLOW-001", "GDPR-FLOW-002", "GDPR-FLOW-003",
		"GDPR-FLOW-004", "GDPR-FLOW-005", "GDPR-INSP-001", "GDPR-INSP-002",
	}
	for _, id := range expected {
		if !ids[id] {
			t.Errorf("missing rule %s", id)
		}
	}
}
