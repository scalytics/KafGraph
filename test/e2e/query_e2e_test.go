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

//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/query"
	"github.com/scalytics/kafgraph/internal/server"
	"github.com/scalytics/kafgraph/internal/storage"
)

// TestE2EQueryPipeline exercises the full query pipeline:
// ingest data via graph API → query via Cypher → verify results.
func TestE2EQueryPipeline(t *testing.T) {
	// Setup
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	exec := query.NewExecutor(g, nil, nil)
	srv := server.NewHTTPServer(":0", g, exec)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Create test data via REST API
	post := func(path, body string) *http.Response {
		resp, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
		require.NoError(t, err)
		return resp
	}

	resp := post("/api/v1/nodes", `{"label":"Agent","properties":{"name":"alice","role":"leader"}}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var alice graph.Node
	json.NewDecoder(resp.Body).Decode(&alice)
	resp.Body.Close()

	resp = post("/api/v1/nodes", `{"label":"Agent","properties":{"name":"bob","role":"member"}}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var bob graph.Node
	json.NewDecoder(resp.Body).Decode(&bob)
	resp.Body.Close()

	resp = post("/api/v1/nodes", `{"label":"Message","properties":{"text":"hello from alice"}}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var msg graph.Node
	json.NewDecoder(resp.Body).Decode(&msg)
	resp.Body.Close()

	// Create edges
	edgeBody := `{"label":"AUTHORED","fromId":"` + string(alice.ID) + `","toId":"` + string(msg.ID) + `"}`
	resp = post("/api/v1/edges", edgeBody)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	edgeBody = `{"label":"KNOWS","fromId":"` + string(alice.ID) + `","toId":"` + string(bob.ID) + `"}`
	resp = post("/api/v1/edges", edgeBody)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// 2. Query via Cypher endpoint
	doQuery := func(cypher string) map[string]any {
		t.Helper()
		body := `{"cypher":"` + cypher + `"}`
		resp, err := http.Post(ts.URL+"/api/v1/query", "application/json", strings.NewReader(body))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		return result
	}

	// Test: MATCH all agents
	result := doQuery("MATCH (n:Agent) RETURN n")
	rows := result["rows"].([]any)
	assert.Len(t, rows, 2)

	// Test: MATCH with WHERE filter
	result = doQuery("MATCH (n:Agent) WHERE n.name = 'alice' RETURN n")
	rows = result["rows"].([]any)
	assert.Len(t, rows, 1)

	// Test: count(*)
	result = doQuery("MATCH (n:Agent) RETURN count(*)")
	rows = result["rows"].([]any)
	require.Len(t, rows, 1)
	row := rows[0].([]any)
	assert.Equal(t, float64(2), row[0]) // JSON numbers are float64

	// Test: relationship traversal
	result = doQuery("MATCH (n:Agent)-[:KNOWS]->(m:Agent) RETURN n, m")
	rows = result["rows"].([]any)
	assert.Len(t, rows, 1)

	// Test: MATCH relationship with direction
	result = doQuery("MATCH (n:Agent)-[:AUTHORED]->(m:Message) RETURN n, m")
	rows = result["rows"].([]any)
	assert.Len(t, rows, 1)

	// Test: Query endpoint returns 501 without executor
	srvNoExec := server.NewHTTPServer(":0", g)
	tsNoExec := httptest.NewServer(srvNoExec.Handler())
	defer tsNoExec.Close()

	resp, err = http.Post(tsNoExec.URL+"/api/v1/query", "application/json",
		strings.NewReader(`{"cypher":"MATCH (n:Agent) RETURN n"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	resp.Body.Close()
}
