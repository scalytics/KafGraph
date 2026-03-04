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
	"fmt"
	"strings"
	"time"
)

// RegisterGDPRRules adds all hardcoded GDPR rules to the engine.
func RegisterGDPRRules(e *Engine) {
	rules := []Rule{
		&gdprSetup001{},
		&gdprRopa001{},
		&gdprRopa002{},
		&gdprRopa003{},
		&gdprRopa004{},
		&gdprDSR001{},
		&gdprDSR002{},
		&gdprBreach001{},
		&gdprBreach002{},
		&gdprDPIA001{},
		&gdprDPIA002{},
		&gdprProc001{},
		&gdprEvidence001{},
	}
	for _, r := range rules {
		e.RegisterRule(r)
	}
}

// --- GDPR-SETUP-001: OrgSetup exists with DPO info ---

type gdprSetup001 struct{}

func (r *gdprSetup001) ID() string        { return "GDPR-SETUP-001" }
func (r *gdprSetup001) Framework() Framework { return FrameworkGDPR }
func (r *gdprSetup001) Module() string     { return "setup" }
func (r *gdprSetup001) Article() string    { return "Art. 37" }
func (r *gdprSetup001) Title() string      { return "DPO designation required" }
func (r *gdprSetup001) Severity() Severity { return SeverityCritical }

func (r *gdprSetup001) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	nodes, err := g.NodesByLabel("OrgSetup")
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return []RuleResult{{
			RuleID:   r.ID(),
			Status:   EvalFail,
			Details:  "No OrgSetup node found — DPO must be designated",
			Severity: r.Severity(),
		}}, nil
	}
	for _, n := range nodes {
		dpoName, _ := n.Properties["dpoName"].(string)
		dpoEmail, _ := n.Properties["dpoEmail"].(string)
		if dpoName == "" || dpoEmail == "" {
			return []RuleResult{{
				RuleID:   r.ID(),
				Status:   EvalFail,
				NodeID:   n.ID,
				Details:  "OrgSetup missing dpoName or dpoEmail",
				Severity: r.Severity(),
			}}, nil
		}
	}
	return []RuleResult{{
		RuleID:   r.ID(),
		Status:   EvalPass,
		Details:  "DPO is designated with name and email",
		Severity: r.Severity(),
	}}, nil
}

// --- GDPR-ROPA-001: Every ProcessingActivity has legalBasis ---

type gdprRopa001 struct{}

func (r *gdprRopa001) ID() string        { return "GDPR-ROPA-001" }
func (r *gdprRopa001) Framework() Framework { return FrameworkGDPR }
func (r *gdprRopa001) Module() string     { return "ropa" }
func (r *gdprRopa001) Article() string    { return "Art. 6" }
func (r *gdprRopa001) Title() string      { return "Legal basis required for all processing activities" }
func (r *gdprRopa001) Severity() Severity { return SeverityCritical }

func (r *gdprRopa001) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	return checkPropertyNotEmpty(g, r.ID(), r.Severity(), "ProcessingActivity", "legalBasis")
}

// --- GDPR-ROPA-002: Every ProcessingActivity has retentionPeriod ---

type gdprRopa002 struct{}

func (r *gdprRopa002) ID() string        { return "GDPR-ROPA-002" }
func (r *gdprRopa002) Framework() Framework { return FrameworkGDPR }
func (r *gdprRopa002) Module() string     { return "ropa" }
func (r *gdprRopa002) Article() string    { return "Art. 30" }
func (r *gdprRopa002) Title() string      { return "Retention period required for processing activities" }
func (r *gdprRopa002) Severity() Severity { return SeverityHigh }

func (r *gdprRopa002) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	return checkPropertyNotEmpty(g, r.ID(), r.Severity(), "ProcessingActivity", "retentionPeriod")
}

// --- GDPR-ROPA-003: Every ProcessingActivity has PROCESSES_CATEGORY edge ---

type gdprRopa003 struct{}

func (r *gdprRopa003) ID() string        { return "GDPR-ROPA-003" }
func (r *gdprRopa003) Framework() Framework { return FrameworkGDPR }
func (r *gdprRopa003) Module() string     { return "ropa" }
func (r *gdprRopa003) Article() string    { return "Art. 30" }
func (r *gdprRopa003) Title() string      { return "Data categories must be documented per activity" }
func (r *gdprRopa003) Severity() Severity { return SeverityMedium }

func (r *gdprRopa003) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	return checkHasEdge(g, r.ID(), r.Severity(), "ProcessingActivity", "PROCESSES_CATEGORY")
}

// --- GDPR-ROPA-004: Every ProcessingActivity has PROTECTED_BY edge (TOM) ---

type gdprRopa004 struct{}

func (r *gdprRopa004) ID() string        { return "GDPR-ROPA-004" }
func (r *gdprRopa004) Framework() Framework { return FrameworkGDPR }
func (r *gdprRopa004) Module() string     { return "ropa" }
func (r *gdprRopa004) Article() string    { return "Art. 32" }
func (r *gdprRopa004) Title() string      { return "Technical/organizational measures required" }
func (r *gdprRopa004) Severity() Severity { return SeverityHigh }

func (r *gdprRopa004) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	return checkHasEdge(g, r.ID(), r.Severity(), "ProcessingActivity", "PROTECTED_BY")
}

// --- GDPR-DSR-001: No DSR is overdue ---

type gdprDSR001 struct{}

func (r *gdprDSR001) ID() string        { return "GDPR-DSR-001" }
func (r *gdprDSR001) Framework() Framework { return FrameworkGDPR }
func (r *gdprDSR001) Module() string     { return "dsr" }
func (r *gdprDSR001) Article() string    { return "Art. 12" }
func (r *gdprDSR001) Title() string      { return "No DSR requests overdue" }
func (r *gdprDSR001) Severity() Severity { return SeverityCritical }

func (r *gdprDSR001) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	nodes, err := g.NodesByLabel("DataSubjectRequest")
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	var results []RuleResult
	for _, n := range nodes {
		status, _ := n.Properties["status"].(string)
		if status == "completed" || status == "closed" {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: n.ID,
				Details: "DSR completed", Severity: r.Severity(),
			})
			continue
		}
		deadlineStr, _ := n.Properties["deadline"].(string)
		if deadlineStr == "" {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalWarning, NodeID: n.ID,
				Details: "DSR has no deadline set", Severity: r.Severity(),
			})
			continue
		}
		deadline, err := time.Parse(time.RFC3339, deadlineStr)
		if err != nil {
			deadline, err = time.Parse("2006-01-02", deadlineStr)
			if err != nil {
				continue
			}
		}
		if now.After(deadline) {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalFail, NodeID: n.ID,
				Details: fmt.Sprintf("DSR overdue (deadline: %s)", deadlineStr),
				Severity: r.Severity(),
			})
		} else {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: n.ID,
				Details: "DSR within deadline", Severity: r.Severity(),
			})
		}
	}
	return results, nil
}

// --- GDPR-DSR-002: Completed DSRs have responseDetails ---

type gdprDSR002 struct{}

func (r *gdprDSR002) ID() string        { return "GDPR-DSR-002" }
func (r *gdprDSR002) Framework() Framework { return FrameworkGDPR }
func (r *gdprDSR002) Module() string     { return "dsr" }
func (r *gdprDSR002) Article() string    { return "Art. 15-22" }
func (r *gdprDSR002) Title() string      { return "Completed DSRs must have response details" }
func (r *gdprDSR002) Severity() Severity { return SeverityHigh }

func (r *gdprDSR002) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	nodes, err := g.NodesByLabel("DataSubjectRequest")
	if err != nil {
		return nil, err
	}
	var results []RuleResult
	for _, n := range nodes {
		status, _ := n.Properties["status"].(string)
		if status != "completed" {
			continue
		}
		resp, _ := n.Properties["responseDetails"].(string)
		if resp == "" {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalFail, NodeID: n.ID,
				Details: "Completed DSR missing responseDetails",
				Severity: r.Severity(),
			})
		} else {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: n.ID,
				Details: "DSR has response details", Severity: r.Severity(),
			})
		}
	}
	return results, nil
}

// --- GDPR-BREACH-001: High/critical breaches notified within 72h ---

type gdprBreach001 struct{}

func (r *gdprBreach001) ID() string        { return "GDPR-BREACH-001" }
func (r *gdprBreach001) Framework() Framework { return FrameworkGDPR }
func (r *gdprBreach001) Module() string     { return "breach" }
func (r *gdprBreach001) Article() string    { return "Art. 33" }
func (r *gdprBreach001) Title() string      { return "Authority notification within 72 hours" }
func (r *gdprBreach001) Severity() Severity { return SeverityCritical }

func (r *gdprBreach001) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	nodes, err := g.NodesByLabel("DataBreach")
	if err != nil {
		return nil, err
	}
	var results []RuleResult
	for _, n := range nodes {
		sev, _ := n.Properties["severity"].(string)
		if sev != "high" && sev != "critical" {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: n.ID,
				Details: "Low/medium breach — 72h rule not mandatory",
				Severity: r.Severity(),
			})
			continue
		}
		discoveredStr, _ := n.Properties["discoveredAt"].(string)
		notifiedStr, _ := n.Properties["authorityNotifiedAt"].(string)
		if notifiedStr == "" {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalFail, NodeID: n.ID,
				Details: "High/critical breach not yet notified to authority",
				Severity: r.Severity(),
			})
			continue
		}
		discovered, err1 := time.Parse(time.RFC3339, discoveredStr)
		notified, err2 := time.Parse(time.RFC3339, notifiedStr)
		if err1 != nil || err2 != nil {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalWarning, NodeID: n.ID,
				Details: "Cannot parse breach timestamps",
				Severity: r.Severity(),
			})
			continue
		}
		if notified.Sub(discovered) > 72*time.Hour {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalFail, NodeID: n.ID,
				Details: fmt.Sprintf("Notification took %s (> 72h)", notified.Sub(discovered)),
				Severity: r.Severity(),
			})
		} else {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: n.ID,
				Details: "Authority notified within 72h",
				Severity: r.Severity(),
			})
		}
	}
	return results, nil
}

// --- GDPR-BREACH-002: Breaches affecting special categories → subjects notified ---

type gdprBreach002 struct{}

func (r *gdprBreach002) ID() string        { return "GDPR-BREACH-002" }
func (r *gdprBreach002) Framework() Framework { return FrameworkGDPR }
func (r *gdprBreach002) Module() string     { return "breach" }
func (r *gdprBreach002) Article() string    { return "Art. 34" }
func (r *gdprBreach002) Title() string      { return "Data subjects notified for special category breaches" }
func (r *gdprBreach002) Severity() Severity { return SeverityCritical }

func (r *gdprBreach002) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	nodes, err := g.NodesByLabel("DataBreach")
	if err != nil {
		return nil, err
	}
	var results []RuleResult
	for _, n := range nodes {
		// Check if breach involves special categories.
		edges, _ := g.Neighbors(n.ID)
		involvesSpecial := false
		for _, e := range edges {
			if e.Label == "BREACH_INVOLVES" {
				involvesSpecial = true
				break
			}
		}
		if !involvesSpecial {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: n.ID,
				Details: "Breach does not involve special categories",
				Severity: r.Severity(),
			})
			continue
		}
		notifiedStr, _ := n.Properties["subjectsNotifiedAt"].(string)
		if notifiedStr == "" {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalFail, NodeID: n.ID,
				Details: "Breach involves special categories but subjects not notified",
				Severity: r.Severity(),
			})
		} else {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: n.ID,
				Details: "Data subjects notified", Severity: r.Severity(),
			})
		}
	}
	return results, nil
}

// --- GDPR-DPIA-001: High-risk activities have DPIA ---

type gdprDPIA001 struct{}

func (r *gdprDPIA001) ID() string        { return "GDPR-DPIA-001" }
func (r *gdprDPIA001) Framework() Framework { return FrameworkGDPR }
func (r *gdprDPIA001) Module() string     { return "dpia" }
func (r *gdprDPIA001) Article() string    { return "Art. 35" }
func (r *gdprDPIA001) Title() string      { return "DPIA required for high-risk processing" }
func (r *gdprDPIA001) Severity() Severity { return SeverityCritical }

func (r *gdprDPIA001) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	activities, err := g.NodesByLabel("ProcessingActivity")
	if err != nil {
		return nil, err
	}
	dpias, _ := g.NodesByLabel("DPIA")

	// Build set of activities covered by DPIAs.
	coveredActivities := map[string]bool{}
	for _, d := range dpias {
		edges, _ := g.Neighbors(d.ID)
		for _, e := range edges {
			if e.Label == "DPIA_FOR" {
				coveredActivities[e.To] = true
			}
		}
	}

	var results []RuleResult
	for _, a := range activities {
		risk, _ := a.Properties["riskLevel"].(string)
		if !strings.EqualFold(risk, "high") {
			continue
		}
		if coveredActivities[a.ID] {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: a.ID,
				Details: "High-risk activity has DPIA", Severity: r.Severity(),
			})
		} else {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalFail, NodeID: a.ID,
				Details: "High-risk activity missing DPIA",
				Severity: r.Severity(),
			})
		}
	}
	return results, nil
}

// --- GDPR-DPIA-002: Every DPIA has at least one risk ---

type gdprDPIA002 struct{}

func (r *gdprDPIA002) ID() string        { return "GDPR-DPIA-002" }
func (r *gdprDPIA002) Framework() Framework { return FrameworkGDPR }
func (r *gdprDPIA002) Module() string     { return "dpia" }
func (r *gdprDPIA002) Article() string    { return "Art. 35" }
func (r *gdprDPIA002) Title() string      { return "Every DPIA must identify risks" }
func (r *gdprDPIA002) Severity() Severity { return SeverityHigh }

func (r *gdprDPIA002) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	dpias, err := g.NodesByLabel("DPIA")
	if err != nil {
		return nil, err
	}
	var results []RuleResult
	for _, d := range dpias {
		edges, _ := g.Neighbors(d.ID)
		hasRisk := false
		for _, e := range edges {
			if e.Label == "HAS_RISK" {
				hasRisk = true
				break
			}
		}
		if hasRisk {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: d.ID,
				Details: "DPIA has identified risks", Severity: r.Severity(),
			})
		} else {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalFail, NodeID: d.ID,
				Details: "DPIA has no identified risks",
				Severity: r.Severity(),
			})
		}
	}
	return results, nil
}

// --- GDPR-PROC-001: Active processors have signed contracts ---

type gdprProc001 struct{}

func (r *gdprProc001) ID() string        { return "GDPR-PROC-001" }
func (r *gdprProc001) Framework() Framework { return FrameworkGDPR }
func (r *gdprProc001) Module() string     { return "processor" }
func (r *gdprProc001) Article() string    { return "Art. 28" }
func (r *gdprProc001) Title() string      { return "Active processors must have signed contracts" }
func (r *gdprProc001) Severity() Severity { return SeverityHigh }

func (r *gdprProc001) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	nodes, err := g.NodesByLabel("DataProcessor")
	if err != nil {
		return nil, err
	}
	var results []RuleResult
	for _, n := range nodes {
		status, _ := n.Properties["contractStatus"].(string)
		if strings.EqualFold(status, "signed") || strings.EqualFold(status, "active") {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: n.ID,
				Details: "Processor has signed contract", Severity: r.Severity(),
			})
		} else {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalFail, NodeID: n.ID,
				Details: fmt.Sprintf("Processor contract status: %s", status),
				Severity: r.Severity(),
			})
		}
	}
	return results, nil
}

// --- GDPR-EVIDENCE-001: Compliant checklist items have evidence ---

type gdprEvidence001 struct{}

func (r *gdprEvidence001) ID() string        { return "GDPR-EVIDENCE-001" }
func (r *gdprEvidence001) Framework() Framework { return FrameworkGDPR }
func (r *gdprEvidence001) Module() string     { return "evidence" }
func (r *gdprEvidence001) Article() string    { return "Art. 5(2)" }
func (r *gdprEvidence001) Title() string      { return "Compliant checklist items must have evidence" }
func (r *gdprEvidence001) Severity() Severity { return SeverityMedium }

func (r *gdprEvidence001) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	nodes, err := g.NodesByLabel("ChecklistItem")
	if err != nil {
		return nil, err
	}
	var results []RuleResult
	for _, n := range nodes {
		status, _ := n.Properties["status"].(string)
		if status != "compliant" {
			continue
		}
		edges, _ := g.Neighbors(n.ID)
		hasEvidence := false
		for _, e := range edges {
			if e.Label == "EVIDENCED_BY" {
				hasEvidence = true
				break
			}
		}
		if hasEvidence {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: n.ID,
				Details: "Checklist item has evidence", Severity: r.Severity(),
			})
		} else {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalFail, NodeID: n.ID,
				Details: "Compliant checklist item has no evidence attached",
				Severity: r.Severity(),
			})
		}
	}
	return results, nil
}

// --- Helper functions ---

func checkPropertyNotEmpty(g GraphQuerier, ruleID string, sev Severity, label, prop string) ([]RuleResult, error) {
	nodes, err := g.NodesByLabel(label)
	if err != nil {
		return nil, err
	}
	var results []RuleResult
	for _, n := range nodes {
		val, _ := n.Properties[prop].(string)
		if val == "" {
			results = append(results, RuleResult{
				RuleID:   ruleID,
				Status:   EvalFail,
				NodeID:   n.ID,
				Details:  fmt.Sprintf("%s missing %s", label, prop),
				Severity: sev,
			})
		} else {
			results = append(results, RuleResult{
				RuleID:   ruleID,
				Status:   EvalPass,
				NodeID:   n.ID,
				Details:  fmt.Sprintf("%s has %s", label, prop),
				Severity: sev,
			})
		}
	}
	return results, nil
}

func checkHasEdge(g GraphQuerier, ruleID string, sev Severity, label, edgeLabel string) ([]RuleResult, error) {
	nodes, err := g.NodesByLabel(label)
	if err != nil {
		return nil, err
	}
	var results []RuleResult
	for _, n := range nodes {
		edges, _ := g.Neighbors(n.ID)
		found := false
		for _, e := range edges {
			if e.Label == edgeLabel {
				found = true
				break
			}
		}
		if found {
			results = append(results, RuleResult{
				RuleID: ruleID, Status: EvalPass, NodeID: n.ID,
				Details: fmt.Sprintf("%s has %s edge", label, edgeLabel),
				Severity: sev,
			})
		} else {
			results = append(results, RuleResult{
				RuleID: ruleID, Status: EvalFail, NodeID: n.ID,
				Details: fmt.Sprintf("%s missing %s edge", label, edgeLabel),
				Severity: sev,
			})
		}
	}
	return results, nil
}
