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

package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/scalytics/kafgraph/internal/graph"
)

func TestGDPRSetupCRUD(t *testing.T) {
	ts, _, _ := setupComplianceTestServer(t)

	// GET setup — should be empty initially.
	resp, err := http.Get(ts.URL + "/api/v2/compliance/gdpr/setup")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// PUT setup
	body := `{"orgName":"TestOrg","dpoName":"Jane","dpoEmail":"jane@test.com"}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v2/compliance/gdpr/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on PUT, got %d", resp.StatusCode)
	}

	// GET setup — should now return org data.
	resp, err = http.Get(ts.URL + "/api/v2/compliance/gdpr/setup")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var result map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&result)
	props, _ := result["properties"].(map[string]any)
	if props["orgName"] != "TestOrg" {
		t.Errorf("expected TestOrg, got %v", props["orgName"])
	}
}

func TestGDPRRoPACRUD(t *testing.T) {
	ts, _, _ := setupComplianceTestServer(t)

	// Create processing activity.
	body := `{"name":"Analytics","purpose":"Product improvement","legalBasis":"legitimate_interest"}`
	resp, err := http.Post(ts.URL+"/api/v2/compliance/gdpr/ropa", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	// List processing activities.
	resp2, err := http.Get(ts.URL + "/api/v2/compliance/gdpr/ropa")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	var list map[string]any
	_ = json.NewDecoder(resp2.Body).Decode(&list)
	total, _ := list["total"].(float64)
	if total != 1 {
		t.Fatalf("expected 1 activity, got %v", total)
	}
}

func TestGDPRDSRSLA(t *testing.T) {
	ts, _, g := setupComplianceTestServer(t)

	// Create a DSR node directly.
	_, _ = g.CreateNode("DataSubjectRequest", map[string]any{
		"requestType": "access",
		"status":      "pending",
		"deadline":    "2099-12-31T00:00:00Z",
	})

	resp, err := http.Get(ts.URL + "/api/v2/compliance/gdpr/dsr/sla")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var result map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&result)
	total, _ := result["total"].(float64)
	if total != 1 {
		t.Fatalf("expected 1 SLA item, got %v", total)
	}
}

func TestGDPRDetailUpdateAndDelete(t *testing.T) {
	ts, _, g := setupComplianceTestServer(t)

	lb, err := g.CreateNode("LegalBasis", graph.Properties{"name": "Contract"})
	if err != nil {
		t.Fatal(err)
	}
	sm, err := g.CreateNode("SecurityMeasure", graph.Properties{"name": "Encryption"})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := g.CreateNode("DataCategory", graph.Properties{"name": "Employee Data"})
	if err != nil {
		t.Fatal(err)
	}

	ropaBody := `{"name":"Analytics","purpose":"Insights","legalBasisId":"` + string(lb.ID) + `","securityMeasureId":"` + string(sm.ID) + `","categoryIds":["` + string(cat.ID) + `"]}`
	resp, err := http.Post(ts.URL+"/api/v2/compliance/gdpr/ropa", "application/json", strings.NewReader(ropaBody))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var created map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	ropaID, _ := created["id"].(string)
	if ropaID == "" {
		t.Fatal("expected ropa ID")
	}

	detailResp, err := http.Get(ts.URL + "/api/v2/compliance/gdpr/ropa/" + ropaID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = detailResp.Body.Close() }()
	var detail map[string]any
	if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if len(detail["edges"].([]any)) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(detail["edges"].([]any)))
	}

	req, _ := http.NewRequest("PUT", ts.URL+"/api/v2/compliance/gdpr/ropa/"+ropaID, strings.NewReader(`{"name":"Analytics v2"}`))
	req.Header.Set("Content-Type", "application/json")
	updateResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = updateResp.Body.Close() }()
	var updated map[string]any
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	props, _ := updated["properties"].(map[string]any)
	if props["name"] != "Analytics v2" {
		t.Fatalf("expected updated name, got %v", props["name"])
	}

	evidenceResp, err := http.Post(ts.URL+"/api/v2/compliance/gdpr/evidence", "application/json", strings.NewReader(`{"title":"Audit Trail"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = evidenceResp.Body.Close() }()
	var evidence map[string]any
	if err := json.NewDecoder(evidenceResp.Body).Decode(&evidence); err != nil {
		t.Fatal(err)
	}
	evidenceID, _ := evidence["id"].(string)
	if evidenceID == "" {
		t.Fatal("expected evidence ID")
	}

	delReq, _ := http.NewRequest("DELETE", ts.URL+"/api/v2/compliance/gdpr/evidence/"+evidenceID, nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = delResp.Body.Close() }()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", delResp.StatusCode)
	}
}
