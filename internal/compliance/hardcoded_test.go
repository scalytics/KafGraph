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
