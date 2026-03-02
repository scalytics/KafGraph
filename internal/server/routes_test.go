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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
)

func newTestServer() *HTTPServer {
	return NewHTTPServer(":0", graph.New(newMemStorage()))
}

func doRequest(srv *HTTPServer, method, path, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	srv.server.Handler.ServeHTTP(rr, req)
	return rr
}

// --- Node tests ---

func TestCreateNode(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodPost, "/api/v1/nodes",
		`{"label":"Agent","properties":{"name":"alice"}}`)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var node graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &node))
	assert.Equal(t, "Agent", node.Label)
	assert.Equal(t, "alice", node.Properties["name"])
	assert.NotEmpty(t, node.ID)
}

func TestCreateNodeBadJSON(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodPost, "/api/v1/nodes", `{bad}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateNodeMissingLabel(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodPost, "/api/v1/nodes", `{"properties":{}}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetNode(t *testing.T) {
	srv := newTestServer()

	// Create
	rr := doRequest(srv, http.MethodPost, "/api/v1/nodes",
		`{"label":"Agent","properties":{"name":"alice"}}`)
	require.Equal(t, http.StatusCreated, rr.Code)

	var created graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))

	// Get
	rr = doRequest(srv, http.MethodGet, "/api/v1/nodes/"+string(created.ID), "")
	assert.Equal(t, http.StatusOK, rr.Code)

	var got graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, created.ID, got.ID)
}

func TestGetNodeNotFound(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodGet, "/api/v1/nodes/nonexistent", "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDeleteNode(t *testing.T) {
	srv := newTestServer()

	// Create
	rr := doRequest(srv, http.MethodPost, "/api/v1/nodes", `{"label":"Agent"}`)
	require.Equal(t, http.StatusCreated, rr.Code)

	var created graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))

	// Delete
	rr = doRequest(srv, http.MethodDelete, "/api/v1/nodes/"+string(created.ID), "")
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Verify gone
	rr = doRequest(srv, http.MethodGet, "/api/v1/nodes/"+string(created.ID), "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestListNodesByLabel(t *testing.T) {
	srv := newTestServer()

	doRequest(srv, http.MethodPost, "/api/v1/nodes",
		`{"label":"Agent","properties":{"name":"alice"}}`)
	doRequest(srv, http.MethodPost, "/api/v1/nodes",
		`{"label":"Agent","properties":{"name":"bob"}}`)
	doRequest(srv, http.MethodPost, "/api/v1/nodes",
		`{"label":"Message","properties":{"text":"hello"}}`)

	rr := doRequest(srv, http.MethodGet, "/api/v1/nodes?label=Agent", "")
	assert.Equal(t, http.StatusOK, rr.Code)

	var nodes []graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &nodes))
	assert.Len(t, nodes, 2)
}

func TestListNodesMissingLabel(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodGet, "/api/v1/nodes", "")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestListNodesEmptyResult(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodGet, "/api/v1/nodes?label=Nothing", "")
	assert.Equal(t, http.StatusOK, rr.Code)

	var nodes []graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &nodes))
	assert.Len(t, nodes, 0)
}

// --- Edge tests ---

func createTwoNodes(t *testing.T, srv *HTTPServer) (graph.NodeID, graph.NodeID) {
	t.Helper()

	rr := doRequest(srv, http.MethodPost, "/api/v1/nodes",
		`{"label":"Agent","properties":{"name":"alice"}}`)
	require.Equal(t, http.StatusCreated, rr.Code)
	var a graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &a))

	rr = doRequest(srv, http.MethodPost, "/api/v1/nodes",
		`{"label":"Agent","properties":{"name":"bob"}}`)
	require.Equal(t, http.StatusCreated, rr.Code)
	var b graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &b))

	return a.ID, b.ID
}

func TestCreateEdge(t *testing.T) {
	srv := newTestServer()
	fromID, toID := createTwoNodes(t, srv)

	body := `{"label":"KNOWS","fromId":"` + string(fromID) + `","toId":"` + string(toID) + `"}`
	rr := doRequest(srv, http.MethodPost, "/api/v1/edges", body)
	assert.Equal(t, http.StatusCreated, rr.Code)

	var edge graph.Edge
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &edge))
	assert.Equal(t, "KNOWS", edge.Label)
	assert.Equal(t, fromID, edge.FromID)
	assert.Equal(t, toID, edge.ToID)
}

func TestCreateEdgeBadJSON(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodPost, "/api/v1/edges", `{bad}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateEdgeMissingFields(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodPost, "/api/v1/edges", `{"label":"KNOWS"}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateEdgeMissingNode(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodPost, "/api/v1/nodes", `{"label":"Agent"}`)
	require.Equal(t, http.StatusCreated, rr.Code)
	var a graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &a))

	body := `{"label":"KNOWS","fromId":"` + string(a.ID) + `","toId":"nonexistent"}`
	rr = doRequest(srv, http.MethodPost, "/api/v1/edges", body)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetEdge(t *testing.T) {
	srv := newTestServer()
	fromID, toID := createTwoNodes(t, srv)

	body := `{"label":"KNOWS","fromId":"` + string(fromID) + `","toId":"` + string(toID) + `"}`
	rr := doRequest(srv, http.MethodPost, "/api/v1/edges", body)
	require.Equal(t, http.StatusCreated, rr.Code)

	var created graph.Edge
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))

	rr = doRequest(srv, http.MethodGet, "/api/v1/edges/"+string(created.ID), "")
	assert.Equal(t, http.StatusOK, rr.Code)

	var got graph.Edge
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, created.ID, got.ID)
}

func TestGetEdgeNotFound(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodGet, "/api/v1/edges/nonexistent", "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDeleteEdge(t *testing.T) {
	srv := newTestServer()
	fromID, toID := createTwoNodes(t, srv)

	body := `{"label":"KNOWS","fromId":"` + string(fromID) + `","toId":"` + string(toID) + `"}`
	rr := doRequest(srv, http.MethodPost, "/api/v1/edges", body)
	require.Equal(t, http.StatusCreated, rr.Code)

	var created graph.Edge
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))

	rr = doRequest(srv, http.MethodDelete, "/api/v1/edges/"+string(created.ID), "")
	assert.Equal(t, http.StatusNoContent, rr.Code)

	rr = doRequest(srv, http.MethodGet, "/api/v1/edges/"+string(created.ID), "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestNodeEdges(t *testing.T) {
	srv := newTestServer()
	fromID, toID := createTwoNodes(t, srv)

	body := `{"label":"KNOWS","fromId":"` + string(fromID) + `","toId":"` + string(toID) + `"}`
	doRequest(srv, http.MethodPost, "/api/v1/edges", body)

	rr := doRequest(srv, http.MethodGet, "/api/v1/nodes/"+string(fromID)+"/edges", "")
	assert.Equal(t, http.StatusOK, rr.Code)

	var edges []graph.Edge
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &edges))
	assert.Len(t, edges, 1)
}

func TestNodeEdgesEmpty(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodPost, "/api/v1/nodes", `{"label":"Agent"}`)
	require.Equal(t, http.StatusCreated, rr.Code)
	var node graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &node))

	rr = doRequest(srv, http.MethodGet, "/api/v1/nodes/"+string(node.ID)+"/edges", "")
	assert.Equal(t, http.StatusOK, rr.Code)

	var edges []graph.Edge
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &edges))
	assert.Len(t, edges, 0)
}

// --- Tool schema test ---

func TestListTools(t *testing.T) {
	srv := newTestServer()

	rr := doRequest(srv, http.MethodGet, "/api/v1/tools", "")
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string][]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Len(t, body["tools"], 7)
	assert.Contains(t, body["tools"], "brain_search")
}
