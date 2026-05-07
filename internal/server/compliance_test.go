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
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scalytics/kafgraph/internal/compliance"
	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/storage"
)

func setupComplianceTestServer(t *testing.T) (*httptest.Server, *compliance.Engine, *graph.Graph) {
	t.Helper()
	store, err := storage.NewBadgerStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	g := graph.New(store)
	t.Cleanup(func() { _ = g.Close() })

	engine := compliance.NewEngine(g)
	compliance.RegisterGDPRRules(engine)
	_ = engine.EnsureFrameworkNodes()

	srv := NewHTTPServer(":0", g, WithCompliance(engine))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return ts, engine, g
}

func TestComplianceFrameworks(t *testing.T) {
	ts, _, _ := setupComplianceTestServer(t)
	resp, err := http.Get(ts.URL + "/api/v2/compliance/frameworks")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	frameworks, ok := body["frameworks"].([]any)
	if !ok {
		t.Fatal("expected frameworks array")
	}
	if len(frameworks) == 0 {
		t.Fatal("expected at least 1 framework")
	}
}

func TestComplianceRules(t *testing.T) {
	ts, _, _ := setupComplianceTestServer(t)
	resp, err := http.Get(ts.URL + "/api/v2/compliance/rules?framework=gdpr")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	total, _ := body["total"].(float64)
	if total != 13 {
		t.Fatalf("expected 13 GDPR rules, got %v", total)
	}
}

func TestComplianceScan(t *testing.T) {
	ts, _, _ := setupComplianceTestServer(t)
	body := `{"framework":"gdpr"}`
	resp, err := http.Post(ts.URL+"/api/v2/compliance/scan", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["scanId"] == nil {
		t.Fatal("expected scanId in result")
	}
}

func TestComplianceDashboard(t *testing.T) {
	ts, _, _ := setupComplianceTestServer(t)
	_, err := http.Post(ts.URL+"/api/v2/compliance/scan", "application/json", strings.NewReader(`{"framework":"gdpr","module":"setup"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(ts.URL + "/api/v2/compliance/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["latestScan"] == nil {
		t.Fatal("expected latestScan")
	}
	if body["moduleScores"] == nil {
		t.Fatal("expected moduleScores")
	}
}

func TestComplianceScanListingDetailAndScore(t *testing.T) {
	ts, _, _ := setupComplianceTestServer(t)

	resp, err := http.Post(ts.URL+"/api/v2/compliance/scan", "application/json", strings.NewReader(`{"framework":"gdpr","module":"setup"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var scan map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&scan); err != nil {
		t.Fatal(err)
	}
	scanID, _ := scan["scanId"].(string)
	if scanID == "" {
		t.Fatal("expected scanId")
	}

	listResp, err := http.Get(ts.URL + "/api/v2/compliance/scans")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listResp.Body.Close() }()
	var list map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if total, _ := list["total"].(float64); total < 1 {
		t.Fatalf("expected at least one scan, got %v", total)
	}

	detailResp, err := http.Get(ts.URL + "/api/v2/compliance/scans/" + scanID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = detailResp.Body.Close() }()
	var detail map[string]any
	if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	props, _ := detail["properties"].(map[string]any)
	if props["scanId"] != scanID {
		t.Fatalf("expected detail for %s, got %v", scanID, props["scanId"])
	}
	if _, ok := detail["evaluations"].([]any); !ok {
		t.Fatal("expected evaluations array")
	}

	scoreResp, err := http.Get(ts.URL + "/api/v2/compliance/score")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = scoreResp.Body.Close() }()
	var score map[string]any
	if err := json.NewDecoder(scoreResp.Body).Decode(&score); err != nil {
		t.Fatal(err)
	}
	scores, ok := score["scores"].([]any)
	if !ok || len(scores) == 0 {
		t.Fatal("expected score entries")
	}
}

func TestModuleFromRuleID(t *testing.T) {
	if got := moduleFromRuleID("GDPR-ROPA-001"); got != "ropa" {
		t.Fatalf("expected ropa, got %s", got)
	}
	if got := moduleFromRuleID("BROKEN"); got != "unknown" {
		t.Fatalf("expected unknown, got %s", got)
	}
}
