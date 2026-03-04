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
	"os"
	"path/filepath"
	"testing"
)

func TestParseYAMLRule(t *testing.T) {
	data := []byte(`
id: GDPR-CUSTOM-001
framework: gdpr
module: ropa
article: "Art. 30"
title: "Retention period required"
severity: high
check:
  nodeLabel: ProcessingActivity
  requiredProperties:
    - retentionPeriod
  condition: "property_not_empty"
`)
	def, err := ParseYAMLRule(data)
	if err != nil {
		t.Fatal(err)
	}
	if def.RuleID != "GDPR-CUSTOM-001" {
		t.Errorf("expected id GDPR-CUSTOM-001, got %s", def.RuleID)
	}
	if def.SeverityStr != "high" {
		t.Errorf("expected severity high, got %s", def.SeverityStr)
	}
	if def.Check.NodeLabel != "ProcessingActivity" {
		t.Errorf("expected nodeLabel ProcessingActivity, got %s", def.Check.NodeLabel)
	}
	if len(def.Check.RequiredProperties) != 1 || def.Check.RequiredProperties[0] != "retentionPeriod" {
		t.Errorf("unexpected requiredProperties: %v", def.Check.RequiredProperties)
	}
}

func TestParseYAMLRule_NoID(t *testing.T) {
	data := []byte(`framework: gdpr`)
	_, err := ParseYAMLRule(data)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestLoadYAMLRules_Dir(t *testing.T) {
	dir := t.TempDir()

	ruleYAML := `
id: TEST-YAML-001
framework: gdpr
module: test
article: "Art. 1"
title: "Test rule"
severity: medium
check:
  nodeLabel: TestNode
  requiredProperties:
    - name
  condition: property_not_empty
`
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(ruleYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-YAML file should be ignored.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := NewEngine(nil)
	if err := e.LoadYAMLRules(dir); err != nil {
		t.Fatal(err)
	}
	rules := e.Rules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ID() != "TEST-YAML-001" {
		t.Errorf("expected TEST-YAML-001, got %s", rules[0].ID())
	}
}

func TestLoadYAMLRules_NonexistentDir(t *testing.T) {
	e := NewEngine(nil)
	if err := e.LoadYAMLRules("/nonexistent/path"); err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got %v", err)
	}
}

func TestLoadYAMLRules_EmptyDir(t *testing.T) {
	e := NewEngine(nil)
	if err := e.LoadYAMLRules(""); err != nil {
		t.Fatalf("expected nil error for empty dir, got %v", err)
	}
}

func TestYAMLRuleEvaluate_PropertyNotEmpty(t *testing.T) {
	def := &YAMLRuleDef{
		RuleID:       "Y-001",
		FrameworkStr: "gdpr",
		ModuleStr:    "ropa",
		SeverityStr:  "high",
	}
	def.Check.NodeLabel = "ProcessingActivity"
	def.Check.RequiredProperties = []string{"retentionPeriod"}
	def.Check.Condition = "property_not_empty"

	rule := def.ToRule()

	q := newMockQuerier()
	q.nodes["ProcessingActivity"] = NodeList{
		{ID: "n:PA:1", Properties: map[string]any{"retentionPeriod": "3 years"}},
		{ID: "n:PA:2", Properties: map[string]any{"name": "test"}},
	}

	results, err := rule.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != EvalPass {
		t.Errorf("PA:1 should pass, got %s", results[0].Status)
	}
	if results[1].Status != EvalFail {
		t.Errorf("PA:2 should fail, got %s", results[1].Status)
	}
}
