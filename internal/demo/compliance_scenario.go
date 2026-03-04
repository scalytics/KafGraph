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

package demo

import (
	"fmt"
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
)

// ComplianceResult summarizes what was seeded.
type ComplianceResult struct {
	OrgSetup             int
	DataCategories       int
	LegalBases           int
	SecurityMeasures     int
	ProcessingActivities int
	DSRs                 int
	Breaches             int
	DPIAs                int
	Processors           int
	ChecklistItems       int
	Evidence             int
	DataFlows            int
	Inspections          int
	Findings             int
	Remediations         int
	ComplianceEvents     int
}

// SeedComplianceScenario creates demo compliance data directly in the graph.
func SeedComplianceScenario(g *graph.Graph) (*ComplianceResult, error) {
	result := &ComplianceResult{}
	now := time.Now().UTC()

	// --- OrgSetup ---
	_, err := g.CreateNode("OrgSetup", graph.Properties{
		"orgName":              "Scalytics GmbH",
		"dpoName":              "Dr. Maria Schmidt",
		"dpoEmail":             "dpo@scalytics.io",
		"supervisoryAuthority": "BfDI (Germany)",
		"country":              "DE",
		"createdAt":            now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("create OrgSetup: %w", err)
	}
	result.OrgSetup = 1

	// --- Data Categories ---
	categories := []struct {
		name      string
		desc      string
		isSpecial bool
	}{
		{"Contact Data", "Name, email, phone number", false},
		{"Usage Analytics", "Feature usage, session data", false},
		{"Payment Data", "Credit card details, billing address", false},
		{"Health Data", "Employee health records", true},
		{"Communication Content", "Agent conversation transcripts", false},
	}
	catNodes := make([]*graph.Node, len(categories))
	for i, cat := range categories {
		n, err := g.CreateNode("DataCategory", graph.Properties{
			"name":        cat.name,
			"description": cat.desc,
			"isSpecial":   cat.isSpecial,
			"createdAt":   now.Format(time.RFC3339),
		})
		if err != nil {
			return nil, fmt.Errorf("create DataCategory %s: %w", cat.name, err)
		}
		catNodes[i] = n
	}
	result.DataCategories = len(categories)

	// --- Legal Bases ---
	legalBases := []struct {
		name    string
		article string
	}{
		{"Consent", "Art. 6(1)(a)"},
		{"Contract Performance", "Art. 6(1)(b)"},
		{"Legitimate Interest", "Art. 6(1)(f)"},
	}
	lbNodes := make([]*graph.Node, len(legalBases))
	for i, lb := range legalBases {
		n, err := g.CreateNode("LegalBasis", graph.Properties{
			"name":      lb.name,
			"article":   lb.article,
			"createdAt": now.Format(time.RFC3339),
		})
		if err != nil {
			return nil, fmt.Errorf("create LegalBasis: %w", err)
		}
		lbNodes[i] = n
	}
	result.LegalBases = len(legalBases)

	// --- Security Measures ---
	measures := []struct {
		name   string
		typ    string
		status string
	}{
		{"AES-256 Encryption at Rest", "technical", "active"},
		{"TLS 1.3 in Transit", "technical", "active"},
		{"Role-Based Access Control", "organizational", "active"},
		{"Annual Security Training", "organizational", "planned"},
	}
	smNodes := make([]*graph.Node, len(measures))
	for i, m := range measures {
		n, err := g.CreateNode("SecurityMeasure", graph.Properties{
			"name":      m.name,
			"type":      m.typ,
			"status":    m.status,
			"createdAt": now.Format(time.RFC3339),
		})
		if err != nil {
			return nil, fmt.Errorf("create SecurityMeasure: %w", err)
		}
		smNodes[i] = n
	}
	result.SecurityMeasures = len(measures)

	// --- Processing Activities ---
	activities := []struct {
		name            string
		purpose         string
		legalBasis      string
		retentionPeriod string
		riskLevel       string
		lbIdx           int
		catIdxs         []int
		smIdx           int
	}{
		{
			name: "Agent Conversation Processing", purpose: "Multi-agent collaboration",
			legalBasis: "legitimate_interest", retentionPeriod: "2 years",
			riskLevel: "high", lbIdx: 2, catIdxs: []int{4}, smIdx: 0,
		},
		{
			name: "User Analytics", purpose: "Product improvement",
			legalBasis: "consent", retentionPeriod: "1 year",
			riskLevel: "medium", lbIdx: 0, catIdxs: []int{1}, smIdx: 2,
		},
		{
			name: "Billing Processing", purpose: "Subscription management",
			legalBasis: "contract", retentionPeriod: "7 years",
			riskLevel: "low", lbIdx: 1, catIdxs: []int{0, 2}, smIdx: 0,
		},
		{
			name: "Employee Health Monitoring", purpose: "Workplace safety",
			legalBasis: "consent", retentionPeriod: "", // Missing — triggers rule failure
			riskLevel: "high", lbIdx: 0, catIdxs: []int{3}, smIdx: -1, // No TOM — triggers rule failure
		},
	}
	paNodes := make([]*graph.Node, len(activities))
	for i, a := range activities {
		props := graph.Properties{
			"name":       a.name,
			"purpose":    a.purpose,
			"legalBasis": a.legalBasis,
			"riskLevel":  a.riskLevel,
			"status":     "active",
			"createdAt":  now.Format(time.RFC3339),
		}
		if a.retentionPeriod != "" {
			props["retentionPeriod"] = a.retentionPeriod
		}
		n, err := g.CreateNode("ProcessingActivity", props)
		if err != nil {
			return nil, fmt.Errorf("create ProcessingActivity: %w", err)
		}
		paNodes[i] = n

		// Edges
		_, _ = g.CreateEdge("HAS_LEGAL_BASIS", n.ID, lbNodes[a.lbIdx].ID, nil)
		for _, ci := range a.catIdxs {
			_, _ = g.CreateEdge("PROCESSES_CATEGORY", n.ID, catNodes[ci].ID, nil)
		}
		if a.smIdx >= 0 {
			_, _ = g.CreateEdge("PROTECTED_BY", n.ID, smNodes[a.smIdx].ID, nil)
		}
	}
	result.ProcessingActivities = len(activities)

	// --- Data Subject Requests ---
	dsrs := []struct {
		requestType string
		status      string
		deadline    time.Time
		completed   bool
	}{
		{"access", "in_progress", now.Add(20 * 24 * time.Hour), false},         // Active, within SLA
		{"erasure", "pending", now.Add(-5 * 24 * time.Hour), false},             // Overdue!
	}
	for _, d := range dsrs {
		props := graph.Properties{
			"requestType": d.requestType,
			"status":      d.status,
			"receivedAt":  now.Add(-10 * 24 * time.Hour).Format(time.RFC3339),
			"deadline":    d.deadline.Format(time.RFC3339),
			"createdAt":   now.Format(time.RFC3339),
		}
		n, err := g.CreateNode("DataSubjectRequest", props)
		if err != nil {
			return nil, fmt.Errorf("create DSR: %w", err)
		}
		// Link to first activity.
		_, _ = g.CreateEdge("DSR_FOR_ACTIVITY", n.ID, paNodes[0].ID, nil)
	}
	result.DSRs = len(dsrs)

	// --- Data Breach ---
	breachNode, err := g.CreateNode("DataBreach", graph.Properties{
		"title":               "API Key Exposure Incident",
		"severity":            "high",
		"discoveredAt":        now.Add(-48 * time.Hour).Format(time.RFC3339),
		"authorityNotifiedAt": now.Add(-24 * time.Hour).Format(time.RFC3339), // Within 72h
		"status":              "investigating",
		"createdAt":           now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("create DataBreach: %w", err)
	}
	_, _ = g.CreateEdge("BREACH_AFFECTS", breachNode.ID, paNodes[0].ID, nil)
	_, _ = g.CreateEdge("BREACH_INVOLVES", breachNode.ID, catNodes[4].ID, nil)
	result.Breaches = 1

	// --- DPIA ---
	dpiaNode, err := g.CreateNode("DPIA", graph.Properties{
		"title":       "Agent Conversation Processing DPIA",
		"status":      "completed",
		"overallRisk": "medium",
		"createdAt":   now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("create DPIA: %w", err)
	}
	_, _ = g.CreateEdge("DPIA_FOR", dpiaNode.ID, paNodes[0].ID, nil)

	// DPIA risks
	risks := []struct {
		desc       string
		likelihood int
		impact     int
	}{
		{"Unauthorized access to conversation data", 2, 4},
		{"Data retention beyond necessity", 3, 3},
	}
	for _, risk := range risks {
		riskNode, err := g.CreateNode("DPIARisk", graph.Properties{
			"description": risk.desc,
			"likelihood":  risk.likelihood,
			"impact":      risk.impact,
			"riskScore":   risk.likelihood * risk.impact,
			"createdAt":   now.Format(time.RFC3339),
		})
		if err != nil {
			continue
		}
		_, _ = g.CreateEdge("HAS_RISK", dpiaNode.ID, riskNode.ID, nil)
		_, _ = g.CreateEdge("MITIGATED_BY", riskNode.ID, smNodes[0].ID, nil)
	}
	result.DPIAs = 1

	// --- Data Processors ---
	processors := []struct {
		name           string
		country        string
		contractStatus string
		sccStatus      string
	}{
		{"CloudProvider Inc.", "US", "signed", "active"},
		{"AnalyticsCo Ltd.", "IE", "pending", "not_required"},
	}
	for _, p := range processors {
		n, err := g.CreateNode("DataProcessor", graph.Properties{
			"name":           p.name,
			"country":        p.country,
			"contractStatus": p.contractStatus,
			"sccStatus":      p.sccStatus,
			"createdAt":      now.Format(time.RFC3339),
		})
		if err != nil {
			continue
		}
		_, _ = g.CreateEdge("PROCESSES_FOR", n.ID, paNodes[0].ID, nil)
	}
	result.Processors = len(processors)

	// --- Checklist Items ---
	articles := []struct {
		article     string
		requirement string
		status      string
	}{
		{"Art. 5", "Principles of processing", "compliant"},
		{"Art. 6", "Lawful basis", "compliant"},
		{"Art. 7", "Conditions for consent", "compliant"},
		{"Art. 12", "Transparent communication", "partial"},
		{"Art. 13", "Information to be provided", "compliant"},
		{"Art. 15", "Right of access", "compliant"},
		{"Art. 17", "Right to erasure", "partial"},
		{"Art. 25", "Data protection by design", "non_compliant"},
		{"Art. 30", "Records of processing", "compliant"},
		{"Art. 32", "Security of processing", "compliant"},
	}
	var compliantItems []*graph.Node
	for _, a := range articles {
		n, err := g.CreateNode("ChecklistItem", graph.Properties{
			"article":     a.article,
			"requirement": a.requirement,
			"status":      a.status,
			"createdAt":   now.Format(time.RFC3339),
		})
		if err != nil {
			continue
		}
		if a.status == "compliant" {
			compliantItems = append(compliantItems, n)
		}
	}
	result.ChecklistItems = len(articles)

	// --- Evidence ---
	evidenceDocs := []struct {
		title   string
		docType string
		fileRef string
	}{
		{"Privacy Policy v3.2", "policy", "docs/privacy-policy.pdf"},
		{"DPA with CloudProvider", "contract", "legal/dpa-cloudprovider.pdf"},
		{"Security Audit Report Q4", "audit", "security/audit-q4-2025.pdf"},
	}
	for i, e := range evidenceDocs {
		n, err := g.CreateNode("Evidence", graph.Properties{
			"title":      e.title,
			"type":       e.docType,
			"fileRef":    e.fileRef,
			"uploadedAt": now.Format(time.RFC3339),
			"createdAt":  now.Format(time.RFC3339),
		})
		if err != nil {
			continue
		}
		// Link evidence to compliant checklist items.
		if i < len(compliantItems) {
			_, _ = g.CreateEdge("EVIDENCED_BY", compliantItems[i].ID, n.ID, nil)
		}
	}
	result.Evidence = len(evidenceDocs)

	// --- Data Flows ---
	dataFlows := []struct {
		name         string
		transferType string
		safeguard    string
		legalBasis   string
		fromPA       int    // index into paNodes
		toPA         int    // -1 if to processor
		toProc       int    // index into procNodes; -1 if to activity
		catIdxs      []int  // data category indices
		lbIdx        int    // legal basis index
	}{
		{
			name: "Internal Analytics Pipeline", transferType: "internal",
			fromPA: 1, toPA: 0, toProc: -1,
			catIdxs: []int{1}, lbIdx: 0,
		},
		{
			name: "Cloud Processing Transfer", transferType: "international",
			safeguard: "SCC", legalBasis: "contract",
			fromPA: 0, toPA: -1, toProc: 0,
			catIdxs: []int{4}, lbIdx: 1,
		},
		{
			name: "Billing to Analytics", transferType: "internal",
			fromPA: 2, toPA: 1, toProc: -1,
			catIdxs: []int{0, 2}, lbIdx: 2,
		},
	}
	// Collect processor nodes for edge creation.
	procNodes, _ := g.NodesByLabel("DataProcessor")
	for _, df := range dataFlows {
		props := graph.Properties{
			"name":         df.name,
			"transferType": df.transferType,
			"createdAt":    now.Format(time.RFC3339),
		}
		if df.safeguard != "" {
			props["safeguard"] = df.safeguard
		}
		if df.legalBasis != "" {
			props["legalBasis"] = df.legalBasis
		}
		flowNode, err := g.CreateNode("DataFlow", props)
		if err != nil {
			return nil, fmt.Errorf("create DataFlow %s: %w", df.name, err)
		}
		_, _ = g.CreateEdge("FROM_ACTIVITY", flowNode.ID, paNodes[df.fromPA].ID, nil)
		if df.toPA >= 0 {
			_, _ = g.CreateEdge("TO_ACTIVITY", flowNode.ID, paNodes[df.toPA].ID, nil)
		}
		if df.toProc >= 0 && df.toProc < len(procNodes) {
			_, _ = g.CreateEdge("TO_PROCESSOR", flowNode.ID, procNodes[df.toProc].ID, nil)
		}
		for _, ci := range df.catIdxs {
			_, _ = g.CreateEdge("CARRIES", flowNode.ID, catNodes[ci].ID, nil)
		}
		_, _ = g.CreateEdge("GOVERNED_BY", flowNode.ID, lbNodes[df.lbIdx].ID, nil)
	}
	result.DataFlows = len(dataFlows)

	// --- Inspection with findings and remediation ---
	inspNode, err := g.CreateNode("Inspection", graph.Properties{
		"title":       "Q1 2026 GDPR Compliance Inspection",
		"inspectorId": "dpo-maria",
		"status":      "in_progress",
		"createdAt":   now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("create Inspection: %w", err)
	}
	// Scope: first two processing activities.
	_, _ = g.CreateEdge("INSPECTS", inspNode.ID, paNodes[0].ID, nil)
	_, _ = g.CreateEdge("INSPECTS", inspNode.ID, paNodes[1].ID, nil)
	result.Inspections = 1

	// Finding 1: remediated
	finding1, err := g.CreateNode("InspectionFinding", graph.Properties{
		"title":       "Missing retention period for Employee Health Monitoring",
		"severity":    "high",
		"status":      "remediated",
		"targetDate":  now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
		"createdAt":   now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("create finding 1: %w", err)
	}
	_, _ = g.CreateEdge("HAS_FINDING", inspNode.ID, finding1.ID, nil)
	_, _ = g.CreateEdge("AFFECTS", finding1.ID, paNodes[3].ID, nil)

	// Remediation for finding 1
	remAction, err := g.CreateNode("RemediationAction", graph.Properties{
		"title":       "Add 5-year retention period to Employee Health Monitoring",
		"assignee":    "compliance-team",
		"status":      "completed",
		"verifiedBy":  "dpo-maria",
		"completedAt": now.Add(-2 * 24 * time.Hour).Format(time.RFC3339),
		"createdAt":   now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("create remediation: %w", err)
	}
	_, _ = g.CreateEdge("REMEDIATED_BY", finding1.ID, remAction.ID, nil)

	// Finding 2: still open
	finding2, err := g.CreateNode("InspectionFinding", graph.Properties{
		"title":      "No TOM assigned to Employee Health Monitoring",
		"severity":   "high",
		"status":     "open",
		"targetDate": now.Add(14 * 24 * time.Hour).Format(time.RFC3339),
		"createdAt":  now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("create finding 2: %w", err)
	}
	_, _ = g.CreateEdge("HAS_FINDING", inspNode.ID, finding2.ID, nil)
	_, _ = g.CreateEdge("AFFECTS", finding2.ID, paNodes[3].ID, nil)
	result.Findings = 2
	result.Remediations = 1

	// --- Compliance Events ---
	events := []struct {
		eventType string
		actor     string
		details   string
		relatedID string
	}{
		{"inspection_created", "dpo-maria", "Q1 2026 inspection started", string(inspNode.ID)},
		{"finding_opened", "dpo-maria", "Missing retention period finding", string(finding1.ID)},
		{"finding_opened", "dpo-maria", "Missing TOM finding", string(finding2.ID)},
		{"remediation_created", "compliance-team", "Retention period remediation", string(remAction.ID)},
		{"finding_resolved", "dpo-maria", "Retention period finding remediated", string(finding1.ID)},
	}
	for _, evt := range events {
		evtNode, err := g.CreateNode("ComplianceEvent", graph.Properties{
			"eventType":     evt.eventType,
			"actor":         evt.actor,
			"details":       evt.details,
			"relatedNodeId": evt.relatedID,
			"timestamp":     now.Format(time.RFC3339),
		})
		if err != nil {
			continue
		}
		if evt.relatedID != "" {
			_, _ = g.CreateEdge("RELATES_TO", evtNode.ID, graph.NodeID(evt.relatedID), nil)
		}
	}
	result.ComplianceEvents = len(events)

	return result, nil
}
