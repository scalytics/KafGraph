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
	"time"
)

func TestGDPRSetup001_NoOrgSetup(t *testing.T) {
	q := newMockQuerier()
	rule := &gdprSetup001{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != EvalFail {
		t.Fatalf("expected 1 fail result, got %v", results)
	}
}

func TestGDPRSetup001_WithDPO(t *testing.T) {
	q := newMockQuerier()
	q.nodes["OrgSetup"] = NodeList{{
		ID: "n:OrgSetup:1", Label: "OrgSetup",
		Properties: map[string]any{"dpoName": "Jane", "dpoEmail": "jane@example.com"},
	}}
	rule := &gdprSetup001{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != EvalPass {
		t.Fatalf("expected 1 pass result, got %v", results)
	}
}

func TestGDPRSetup001_MissingEmail(t *testing.T) {
	q := newMockQuerier()
	q.nodes["OrgSetup"] = NodeList{{
		ID: "n:OrgSetup:1", Label: "OrgSetup",
		Properties: map[string]any{"dpoName": "Jane"},
	}}
	rule := &gdprSetup001{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != EvalFail {
		t.Fatalf("expected fail for missing email, got %v", results)
	}
}

func TestGDPRRopa001_MissingLegalBasis(t *testing.T) {
	q := newMockQuerier()
	q.nodes["ProcessingActivity"] = NodeList{
		{ID: "n:PA:1", Label: "ProcessingActivity", Properties: map[string]any{"name": "Analytics"}},
		{ID: "n:PA:2", Label: "ProcessingActivity", Properties: map[string]any{"name": "Marketing", "legalBasis": "consent"}},
	}
	rule := &gdprRopa001{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != EvalFail || results[1].Status != EvalPass {
		t.Fatalf("expected [fail, pass], got [%s, %s]", results[0].Status, results[1].Status)
	}
}

func TestGDPRDSR001_Overdue(t *testing.T) {
	yesterday := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	tomorrow := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	q := newMockQuerier()
	q.nodes["DataSubjectRequest"] = NodeList{
		{ID: "n:DSR:1", Properties: map[string]any{"status": "pending", "deadline": yesterday}},
		{ID: "n:DSR:2", Properties: map[string]any{"status": "pending", "deadline": tomorrow}},
		{ID: "n:DSR:3", Properties: map[string]any{"status": "completed"}},
	}
	rule := &gdprDSR001{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Status != EvalFail {
		t.Errorf("DSR:1 should be fail (overdue), got %s", results[0].Status)
	}
	if results[1].Status != EvalPass {
		t.Errorf("DSR:2 should be pass (within deadline), got %s", results[1].Status)
	}
	if results[2].Status != EvalPass {
		t.Errorf("DSR:3 should be pass (completed), got %s", results[2].Status)
	}
}

func TestGDPRBreach001_NotifiedWithin72h(t *testing.T) {
	discovered := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	notified := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)

	q := newMockQuerier()
	q.nodes["DataBreach"] = NodeList{{
		ID: "n:DB:1", Properties: map[string]any{
			"severity":            "critical",
			"discoveredAt":        discovered,
			"authorityNotifiedAt": notified,
		},
	}}
	rule := &gdprBreach001{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != EvalPass {
		t.Fatalf("expected pass, got %v", results)
	}
}

func TestGDPRBreach001_NotNotified(t *testing.T) {
	q := newMockQuerier()
	q.nodes["DataBreach"] = NodeList{{
		ID: "n:DB:1", Properties: map[string]any{
			"severity":     "high",
			"discoveredAt": time.Now().Add(-96 * time.Hour).Format(time.RFC3339),
		},
	}}
	rule := &gdprBreach001{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != EvalFail {
		t.Fatalf("expected fail, got %v", results)
	}
}

func TestGDPRProc001_SignedContract(t *testing.T) {
	q := newMockQuerier()
	q.nodes["DataProcessor"] = NodeList{
		{ID: "n:DP:1", Properties: map[string]any{"name": "CloudCo", "contractStatus": "signed"}},
		{ID: "n:DP:2", Properties: map[string]any{"name": "AnalyticsCo", "contractStatus": "pending"}},
	}
	rule := &gdprProc001{}
	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != EvalPass {
		t.Errorf("DP:1 should pass, got %s", results[0].Status)
	}
	if results[1].Status != EvalFail {
		t.Errorf("DP:2 should fail, got %s", results[1].Status)
	}
}

func TestRegisterGDPRRules(t *testing.T) {
	e := NewEngine(nil)
	RegisterGDPRRules(e)
	rules := e.Rules()
	if len(rules) != 13 {
		t.Fatalf("expected 13 GDPR rules, got %d", len(rules))
	}
}

func TestGDPRWrapperRulesAndEvidence(t *testing.T) {
	q := newMockQuerier()
	q.nodes["ProcessingActivity"] = NodeList{
		{ID: "n:PA:1", Label: "ProcessingActivity", Properties: map[string]any{"retentionPeriod": "30d", "riskLevel": "high"}},
		{ID: "n:PA:2", Label: "ProcessingActivity", Properties: map[string]any{}},
	}
	q.nodes["DataSubjectRequest"] = NodeList{
		{ID: "n:DSR:1", Properties: map[string]any{"status": "completed", "responseDetails": "emailed data export"}},
		{ID: "n:DSR:2", Properties: map[string]any{"status": "completed"}},
	}
	q.nodes["DPIA"] = NodeList{
		{ID: "n:DPIA:1", Properties: map[string]any{}},
		{ID: "n:DPIA:2", Properties: map[string]any{}},
	}
	q.nodes["ChecklistItem"] = NodeList{
		{ID: "n:CL:1", Properties: map[string]any{"status": "compliant"}},
		{ID: "n:CL:2", Properties: map[string]any{"status": "compliant"}},
	}
	q.edges["n:PA:1"] = EdgeList{
		{Label: "PROCESSES_CATEGORY", To: "n:Cat:1"},
		{Label: "PROTECTED_BY", To: "n:SM:1"},
	}
	q.edges["n:DPIA:1"] = EdgeList{{Label: "DPIA_FOR", To: "n:PA:1"}}
	q.edges["n:DPIA:2"] = EdgeList{{Label: "HAS_RISK", To: "n:Risk:1"}}
	q.edges["n:CL:1"] = EdgeList{{Label: "EVIDENCED_BY", To: "n:EV:1"}}

	tests := []struct {
		name      string
		rule      Rule
		wantCount int
		statuses  []EvalStatus
	}{
		{"ropa002", &gdprRopa002{}, 2, []EvalStatus{EvalPass, EvalFail}},
		{"ropa003", &gdprRopa003{}, 2, []EvalStatus{EvalPass, EvalFail}},
		{"ropa004", &gdprRopa004{}, 2, []EvalStatus{EvalPass, EvalFail}},
		{"dsr002", &gdprDSR002{}, 2, []EvalStatus{EvalPass, EvalFail}},
		{"dpia001", &gdprDPIA001{}, 1, []EvalStatus{EvalPass}},
		{"dpia002", &gdprDPIA002{}, 2, []EvalStatus{EvalFail, EvalPass}},
		{"evidence001", &gdprEvidence001{}, 2, []EvalStatus{EvalPass, EvalFail}},
	}

	for _, tt := range tests {
		results, err := tt.rule.Evaluate(q)
		if err != nil {
			t.Fatalf("%s Evaluate: %v", tt.name, err)
		}
		if len(results) != tt.wantCount {
			t.Fatalf("%s expected %d results, got %d", tt.name, tt.wantCount, len(results))
		}
		for i, status := range tt.statuses {
			if results[i].Status != status {
				t.Fatalf("%s result[%d] = %s, want %s", tt.name, i, results[i].Status, status)
			}
		}
	}
}

func TestGDPRBreach002AndFlow004Evaluate(t *testing.T) {
	q := newMockQuerier()
	q.nodes["DataBreach"] = NodeList{
		{ID: "n:Breach:1", Properties: map[string]any{}},
		{ID: "n:Breach:2", Properties: map[string]any{"subjectsNotifiedAt": time.Now().UTC().Format(time.RFC3339)}},
	}
	q.edges["n:Breach:2"] = EdgeList{{Label: "BREACH_INVOLVES", To: "n:Cat:1"}}

	breachResults, err := (&gdprBreach002{}).Evaluate(q)
	if err != nil {
		t.Fatalf("gdprBreach002 Evaluate: %v", err)
	}
	if len(breachResults) != 2 {
		t.Fatalf("expected 2 breach results, got %d", len(breachResults))
	}
	if breachResults[0].Status != EvalPass || breachResults[1].Status != EvalPass {
		t.Fatalf("unexpected breach statuses: %+v", breachResults)
	}

	q.nodes["DataFlow"] = NodeList{
		{ID: "n:Flow:1", Properties: map[string]any{"legalBasis": "legitimate_interest"}},
	}
	q.edges["n:Flow:1"] = EdgeList{{Label: "CARRIES", To: "n:Cat:2"}}

	flowResults, err := (&gdprFlow004{}).Evaluate(q)
	if err != nil {
		t.Fatalf("gdprFlow004 Evaluate: %v", err)
	}
	if len(flowResults) != 0 {
		t.Fatalf("expected no flow004 results, got %d", len(flowResults))
	}
}
