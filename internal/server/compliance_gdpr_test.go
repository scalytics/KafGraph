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
