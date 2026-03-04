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
	resp, err := http.Get(ts.URL + "/api/v2/compliance/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
