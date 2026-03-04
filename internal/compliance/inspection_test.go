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
	"github.com/scalytics/kafgraph/internal/storage"
)

func newTestGraph(t *testing.T) *graph.Graph {
	t.Helper()
	store, err := storage.NewBadgerStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	g := graph.New(store)
	t.Cleanup(func() { _ = g.Close() })
	return g
}

func TestCreateInspection(t *testing.T) {
	g := newTestGraph(t)

	// Create a scope node.
	pa, _ := g.CreateNode("ProcessingActivity", graph.Properties{"name": "Test Activity"})

	node, err := CreateInspection(g, graph.Properties{
		"title":       "Test Inspection",
		"inspectorId": "inspector-1",
	}, []string{string(pa.ID)}, "")
	if err != nil {
		t.Fatalf("CreateInspection: %v", err)
	}

	if node.Label != "Inspection" {
		t.Errorf("expected label Inspection, got %s", node.Label)
	}
	if s, _ := node.Properties["status"].(string); s != "draft" {
		t.Errorf("expected status draft, got %s", s)
	}

	// Verify INSPECTS edge exists.
	edges, _ := g.Neighbors(node.ID)
	found := false
	for _, e := range edges {
		if e.Label == "INSPECTS" && e.ToID == pa.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected INSPECTS edge to processing activity")
	}

	// Verify audit event was created.
	events, _ := g.NodesByLabel("ComplianceEvent")
	if len(events) == 0 {
		t.Error("expected at least one ComplianceEvent")
	}
}

func TestCreateInspectionWithScan(t *testing.T) {
	g := newTestGraph(t)

	// Create a scan node.
	scan, _ := g.CreateNode("ComplianceScan", graph.Properties{"scanId": "scan-1"})

	node, err := CreateInspection(g, graph.Properties{"title": "Scan-based inspection"}, nil, "scan-1")
	if err != nil {
		t.Fatalf("CreateInspection: %v", err)
	}

	edges, _ := g.Neighbors(node.ID)
	foundBasedOn := false
	for _, e := range edges {
		if e.Label == "BASED_ON" && e.ToID == scan.ID {
			foundBasedOn = true
		}
	}
	if !foundBasedOn {
		t.Error("expected BASED_ON edge to scan")
	}
}

func TestCreateFinding(t *testing.T) {
	g := newTestGraph(t)

	insp, _ := CreateInspection(g, graph.Properties{"title": "Test"}, nil, "")
	pa, _ := g.CreateNode("ProcessingActivity", graph.Properties{"name": "PA1"})

	finding, err := CreateFinding(g, insp.ID, graph.Properties{
		"title":    "Test Finding",
		"severity": "high",
	}, []string{string(pa.ID)})
	if err != nil {
		t.Fatalf("CreateFinding: %v", err)
	}

	if finding.Label != "InspectionFinding" {
		t.Errorf("expected label InspectionFinding, got %s", finding.Label)
	}
	if s, _ := finding.Properties["status"].(string); s != "open" {
		t.Errorf("expected status open, got %s", s)
	}

	// Verify HAS_FINDING edge from inspection.
	edges, _ := g.Neighbors(insp.ID)
	foundHF := false
	for _, e := range edges {
		if e.Label == "HAS_FINDING" && e.ToID == finding.ID {
			foundHF = true
		}
	}
	if !foundHF {
		t.Error("expected HAS_FINDING edge from inspection to finding")
	}

	// Verify AFFECTS edge.
	findEdges, _ := g.Neighbors(finding.ID)
	foundAffects := false
	for _, e := range findEdges {
		if e.Label == "AFFECTS" && e.ToID == pa.ID {
			foundAffects = true
		}
	}
	if !foundAffects {
		t.Error("expected AFFECTS edge from finding to processing activity")
	}
}

func TestCreateRemediation(t *testing.T) {
	g := newTestGraph(t)

	insp, _ := CreateInspection(g, graph.Properties{"title": "Test"}, nil, "")
	finding, _ := CreateFinding(g, insp.ID, graph.Properties{"title": "F1"}, nil)

	rem, err := CreateRemediation(g, finding.ID, graph.Properties{
		"title":    "Fix it",
		"assignee": "team-a",
	})
	if err != nil {
		t.Fatalf("CreateRemediation: %v", err)
	}

	if rem.Label != "RemediationAction" {
		t.Errorf("expected label RemediationAction, got %s", rem.Label)
	}
	if s, _ := rem.Properties["status"].(string); s != "pending" {
		t.Errorf("expected status pending, got %s", s)
	}

	// Verify REMEDIATED_BY edge.
	edges, _ := g.Neighbors(finding.ID)
	foundRem := false
	for _, e := range edges {
		if e.Label == "REMEDIATED_BY" && e.ToID == rem.ID {
			foundRem = true
		}
	}
	if !foundRem {
		t.Error("expected REMEDIATED_BY edge from finding to remediation")
	}
}

func TestSignOffInspection_Success(t *testing.T) {
	g := newTestGraph(t)

	insp, _ := CreateInspection(g, graph.Properties{"title": "Test"}, nil, "")

	// Add a remediated finding (not open).
	finding, _ := CreateFinding(g, insp.ID, graph.Properties{
		"title":  "Resolved",
		"status": "remediated",
	}, nil)
	_ = finding

	err := SignOffInspection(g, insp.ID, "approver-1")
	if err != nil {
		t.Fatalf("SignOffInspection: %v", err)
	}

	// Verify status updated.
	updated, _ := g.GetNode(insp.ID)
	if s, _ := updated.Properties["status"].(string); s != "signed_off" {
		t.Errorf("expected status signed_off, got %s", s)
	}
	if a, _ := updated.Properties["approverId"].(string); a != "approver-1" {
		t.Errorf("expected approverId approver-1, got %s", a)
	}
}

func TestSignOffInspection_BlockedByOpenFinding(t *testing.T) {
	g := newTestGraph(t)

	insp, _ := CreateInspection(g, graph.Properties{"title": "Test"}, nil, "")

	// Add an open finding.
	_, _ = CreateFinding(g, insp.ID, graph.Properties{
		"title":  "Still open",
		"status": "open",
	}, nil)

	err := SignOffInspection(g, insp.ID, "approver-1")
	if err == nil {
		t.Fatal("expected error when signing off with open findings")
	}
	if want := "cannot sign off"; !contains(err.Error(), want) {
		t.Errorf("expected error containing %q, got %q", want, err.Error())
	}
}

func TestLogEvent(t *testing.T) {
	g := newTestGraph(t)

	// Create a target node so the RELATES_TO edge can be established.
	target, _ := g.CreateNode("ProcessingActivity", graph.Properties{"name": "Target"})

	LogEvent(g, "test_event", "actor-1", "Test details", string(target.ID))

	events, _ := g.NodesByLabel("ComplianceEvent")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt := events[0]
	if et, _ := evt.Properties["eventType"].(string); et != "test_event" {
		t.Errorf("expected eventType test_event, got %s", et)
	}
	if actor, _ := evt.Properties["actor"].(string); actor != "actor-1" {
		t.Errorf("expected actor actor-1, got %s", actor)
	}

	// Verify RELATES_TO edge.
	edges, _ := g.Neighbors(evt.ID)
	foundRelates := false
	for _, e := range edges {
		if e.Label == "RELATES_TO" {
			foundRelates = true
		}
	}
	if !foundRelates {
		t.Error("expected RELATES_TO edge")
	}
}

func TestLogEvent_NilGraph(t *testing.T) {
	// Should not panic.
	LogEvent(nil, "test", "", "details", "")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
