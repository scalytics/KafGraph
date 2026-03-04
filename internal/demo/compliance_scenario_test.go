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
	"testing"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/storage"
)

func TestSeedComplianceScenario(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	g := graph.New(store)
	defer func() { _ = g.Close() }()

	result, err := SeedComplianceScenario(g)
	if err != nil {
		t.Fatalf("SeedComplianceScenario: %v", err)
	}

	if result.OrgSetup != 1 {
		t.Errorf("expected 1 OrgSetup, got %d", result.OrgSetup)
	}
	if result.DataCategories != 5 {
		t.Errorf("expected 5 DataCategories, got %d", result.DataCategories)
	}
	if result.LegalBases != 3 {
		t.Errorf("expected 3 LegalBases, got %d", result.LegalBases)
	}
	if result.SecurityMeasures != 4 {
		t.Errorf("expected 4 SecurityMeasures, got %d", result.SecurityMeasures)
	}
	if result.ProcessingActivities != 4 {
		t.Errorf("expected 4 ProcessingActivities, got %d", result.ProcessingActivities)
	}
	if result.DSRs != 2 {
		t.Errorf("expected 2 DSRs, got %d", result.DSRs)
	}
	if result.Breaches != 1 {
		t.Errorf("expected 1 breach, got %d", result.Breaches)
	}
	if result.DPIAs != 1 {
		t.Errorf("expected 1 DPIA, got %d", result.DPIAs)
	}
	if result.Processors != 2 {
		t.Errorf("expected 2 processors, got %d", result.Processors)
	}
	if result.ChecklistItems != 10 {
		t.Errorf("expected 10 checklist items, got %d", result.ChecklistItems)
	}
	if result.Evidence != 3 {
		t.Errorf("expected 3 evidence, got %d", result.Evidence)
	}
	if result.DataFlows != 3 {
		t.Errorf("expected 3 data flows, got %d", result.DataFlows)
	}
	if result.Inspections != 1 {
		t.Errorf("expected 1 inspection, got %d", result.Inspections)
	}
	if result.Findings != 2 {
		t.Errorf("expected 2 findings, got %d", result.Findings)
	}
	if result.Remediations != 1 {
		t.Errorf("expected 1 remediation, got %d", result.Remediations)
	}
	if result.ComplianceEvents != 5 {
		t.Errorf("expected 5 compliance events, got %d", result.ComplianceEvents)
	}

	// Verify nodes exist in graph.
	nodes, _ := g.NodesByLabel("ProcessingActivity")
	if len(nodes) != 4 {
		t.Errorf("expected 4 ProcessingActivity nodes, got %d", len(nodes))
	}

	// Verify data flow nodes and edges.
	flowNodes, _ := g.NodesByLabel("DataFlow")
	if len(flowNodes) != 3 {
		t.Errorf("expected 3 DataFlow nodes, got %d", len(flowNodes))
	}

	// Verify inspection nodes.
	inspNodes, _ := g.NodesByLabel("Inspection")
	if len(inspNodes) != 1 {
		t.Errorf("expected 1 Inspection node, got %d", len(inspNodes))
	}

	// Verify finding nodes.
	findingNodes, _ := g.NodesByLabel("InspectionFinding")
	if len(findingNodes) != 2 {
		t.Errorf("expected 2 InspectionFinding nodes, got %d", len(findingNodes))
	}

	// Verify remediation nodes.
	remNodes, _ := g.NodesByLabel("RemediationAction")
	if len(remNodes) != 1 {
		t.Errorf("expected 1 RemediationAction node, got %d", len(remNodes))
	}

	// Verify compliance event nodes.
	eventNodes, _ := g.NodesByLabel("ComplianceEvent")
	if len(eventNodes) != 5 {
		t.Errorf("expected 5 ComplianceEvent nodes, got %d", len(eventNodes))
	}
}
