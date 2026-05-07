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

import "testing"

func TestRegisterRuleSets(t *testing.T) {
	e := NewEngine(nil)
	RegisterGDPRRules(e)
	RegisterDataFlowRules(e)

	rules := e.Rules()
	if len(rules) != 20 {
		t.Fatalf("expected 20 registered rules, got %d", len(rules))
	}

	if got := len(e.RulesByFramework(FrameworkGDPR)); got != 20 {
		t.Fatalf("expected 20 GDPR rules, got %d", got)
	}
}

func TestBuiltInRuleMetadata(t *testing.T) {
	tests := []struct {
		rule      Rule
		id        string
		framework Framework
		module    string
		article   string
		title     string
		severity  Severity
	}{
		{&gdprSetup001{}, "GDPR-SETUP-001", FrameworkGDPR, "setup", "Art. 37", "DPO designation required", SeverityCritical},
		{&gdprRopa001{}, "GDPR-ROPA-001", FrameworkGDPR, "ropa", "Art. 6", "Legal basis required for all processing activities", SeverityCritical},
		{&gdprRopa002{}, "GDPR-ROPA-002", FrameworkGDPR, "ropa", "Art. 30", "Retention period required for processing activities", SeverityHigh},
		{&gdprRopa003{}, "GDPR-ROPA-003", FrameworkGDPR, "ropa", "Art. 30", "Data categories must be documented per activity", SeverityMedium},
		{&gdprRopa004{}, "GDPR-ROPA-004", FrameworkGDPR, "ropa", "Art. 32", "Technical/organizational measures required", SeverityHigh},
		{&gdprDSR001{}, "GDPR-DSR-001", FrameworkGDPR, "dsr", "Art. 12", "No DSR requests overdue", SeverityCritical},
		{&gdprDSR002{}, "GDPR-DSR-002", FrameworkGDPR, "dsr", "Art. 15-22", "Completed DSRs must have response details", SeverityHigh},
		{&gdprBreach001{}, "GDPR-BREACH-001", FrameworkGDPR, "breach", "Art. 33", "Authority notification within 72 hours", SeverityCritical},
		{&gdprBreach002{}, "GDPR-BREACH-002", FrameworkGDPR, "breach", "Art. 34", "Data subjects notified for special category breaches", SeverityCritical},
		{&gdprDPIA001{}, "GDPR-DPIA-001", FrameworkGDPR, "dpia", "Art. 35", "DPIA required for high-risk processing", SeverityCritical},
		{&gdprDPIA002{}, "GDPR-DPIA-002", FrameworkGDPR, "dpia", "Art. 35", "Every DPIA must identify risks", SeverityHigh},
		{&gdprProc001{}, "GDPR-PROC-001", FrameworkGDPR, "processor", "Art. 28", "Active processors must have signed contracts", SeverityHigh},
		{&gdprEvidence001{}, "GDPR-EVIDENCE-001", FrameworkGDPR, "evidence", "Art. 5(2)", "Compliant checklist items must have evidence", SeverityMedium},
		{&gdprFlow001{}, "GDPR-FLOW-001", FrameworkGDPR, "dataflow", "Art. 30", "Data categories must be documented per data flow", SeverityHigh},
		{&gdprFlow002{}, "GDPR-FLOW-002", FrameworkGDPR, "dataflow", "Art. 6", "Legal basis required for every data flow", SeverityCritical},
		{&gdprFlow003{}, "GDPR-FLOW-003", FrameworkGDPR, "dataflow", "Art. 44-49", "International transfers require adequate safeguards", SeverityCritical},
		{&gdprFlow004{}, "GDPR-FLOW-004", FrameworkGDPR, "dataflow", "Art. 9", "Special category data flows require explicit consent", SeverityCritical},
		{&gdprFlow005{}, "GDPR-FLOW-005", FrameworkGDPR, "dataflow", "Art. 30", "Active processing activities should have data flows defined", SeverityMedium},
		{&gdprInsp001{}, "GDPR-INSP-001", FrameworkGDPR, "inspection", "Art. 5(2)", "No inspection findings overdue", SeverityHigh},
		{&gdprInsp002{}, "GDPR-INSP-002", FrameworkGDPR, "inspection", "Art. 5(2)", "Completed remediations must be verified", SeverityMedium},
	}

	for _, tt := range tests {
		if got := tt.rule.ID(); got != tt.id {
			t.Fatalf("%T ID() = %q, want %q", tt.rule, got, tt.id)
		}
		if got := tt.rule.Framework(); got != tt.framework {
			t.Fatalf("%T Framework() = %q, want %q", tt.rule, got, tt.framework)
		}
		if got := tt.rule.Module(); got != tt.module {
			t.Fatalf("%T Module() = %q, want %q", tt.rule, got, tt.module)
		}
		if got := tt.rule.Article(); got != tt.article {
			t.Fatalf("%T Article() = %q, want %q", tt.rule, got, tt.article)
		}
		if got := tt.rule.Title(); got != tt.title {
			t.Fatalf("%T Title() = %q, want %q", tt.rule, got, tt.title)
		}
		if got := tt.rule.Severity(); got != tt.severity {
			t.Fatalf("%T Severity() = %q, want %q", tt.rule, got, tt.severity)
		}
	}
}

func TestYAMLRuleMetadataAndSeverityFallback(t *testing.T) {
	def := &YAMLRuleDef{
		RuleID:       "YAML-001",
		FrameworkStr: "gdpr",
		ModuleStr:    "yaml",
		ArticleStr:   "Art. 99",
		TitleStr:     "YAML title",
		SeverityStr:  "unknown",
	}

	rule := def.ToRule()
	if rule.ID() != "YAML-001" || rule.Framework() != FrameworkGDPR || rule.Module() != "yaml" {
		t.Fatalf("unexpected yaml rule metadata: %+v", rule)
	}
	if rule.Article() != "Art. 99" || rule.Title() != "YAML title" {
		t.Fatalf("unexpected yaml article/title: %q %q", rule.Article(), rule.Title())
	}
	if rule.Severity() != SeverityMedium {
		t.Fatalf("unexpected default severity: %q", rule.Severity())
	}
}
