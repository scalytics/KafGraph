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

	"github.com/scalytics/kafgraph/internal/brain"
	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/query"
	"github.com/scalytics/kafgraph/internal/storage"
)

func newBadgerTestStorage(t *testing.T) *storage.BadgerStorage {
	t.Helper()
	s, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

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

// --- Query endpoint tests ---

func TestQueryNotAvailable(t *testing.T) {
	srv := newTestServer() // no executor
	rr := doRequest(srv, http.MethodPost, "/api/v1/query",
		`{"cypher":"MATCH (n:Agent) RETURN n"}`)
	assert.Equal(t, http.StatusNotImplemented, rr.Code)
}

func TestQueryBadJSON(t *testing.T) {
	store := newBadgerTestStorage(t)
	g := graph.New(store)
	exec := query.NewExecutor(g, nil, nil)
	srv := NewHTTPServer(":0", g, exec)
	rr := doRequest(srv, http.MethodPost, "/api/v1/query", `{bad}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestQueryEmptyCypher(t *testing.T) {
	store := newBadgerTestStorage(t)
	g := graph.New(store)
	exec := query.NewExecutor(g, nil, nil)
	srv := NewHTTPServer(":0", g, exec)
	rr := doRequest(srv, http.MethodPost, "/api/v1/query", `{"cypher":""}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestQueryWithExecutor(t *testing.T) {
	store := newBadgerTestStorage(t)
	g := graph.New(store)

	// Create test data
	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})

	exec := query.NewExecutor(g, nil, nil)
	srv := NewHTTPServer(":0", g, exec)

	rr := doRequest(srv, http.MethodPost, "/api/v1/query",
		`{"cypher":"MATCH (n:Agent) RETURN n"}`)
	assert.Equal(t, http.StatusOK, rr.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	columns := result["columns"].([]any)
	assert.Len(t, columns, 1)
	assert.Equal(t, "n", columns[0])

	rows := result["rows"].([]any)
	assert.Len(t, rows, 2)
}

func TestQueryWithFilter(t *testing.T) {
	store := newBadgerTestStorage(t)
	g := graph.New(store)

	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})

	exec := query.NewExecutor(g, nil, nil)
	srv := NewHTTPServer(":0", g, exec)

	rr := doRequest(srv, http.MethodPost, "/api/v1/query",
		`{"cypher":"MATCH (n:Agent) WHERE n.name = 'alice' RETURN n"}`)
	assert.Equal(t, http.StatusOK, rr.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	rows := result["rows"].([]any)
	assert.Len(t, rows, 1)
}

// --- Brain tool HTTP tests ---

func newBrainTestServer(t *testing.T) *HTTPServer {
	t.Helper()
	store := newBadgerTestStorage(t)
	g := graph.New(store)
	bs := brain.NewService(g, nil, nil)
	return NewHTTPServer(":0", g, WithBrain(bs))
}

func TestToolNotAvailable(t *testing.T) {
	srv := newTestServer() // no brain service
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/brain_search",
		`{"query":"test"}`)
	assert.Equal(t, http.StatusNotImplemented, rr.Code)
}

func TestToolUnknown(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/unknown_tool", `{}`)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestToolSearch(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/brain_search",
		`{"query":"hello","limit":5}`)
	assert.Equal(t, http.StatusOK, rr.Code)

	var out brain.SearchOutput
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.NotNil(t, out.Results)
}

func TestToolRecall(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/brain_recall",
		`{"agentId":"agent1","depth":"shallow"}`)
	assert.Equal(t, http.StatusOK, rr.Code)

	var out brain.RecallOutput
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.NotNil(t, out.Context.ActiveConversations)
}

func TestToolCapture(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/brain_capture",
		`{"content":"test insight","type":"insight","tags":["test"]}`)
	assert.Equal(t, http.StatusCreated, rr.Code)

	var out brain.CaptureOutput
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.NotEmpty(t, out.NodeID)
}

func TestToolCaptureEmptyContent(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/brain_capture",
		`{"content":""}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestToolRecent(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/brain_recent",
		`{"windowHours":24}`)
	assert.Equal(t, http.StatusOK, rr.Code)

	var out brain.RecentOutput
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.NotNil(t, out.Activity)
}

func TestToolPatterns(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/brain_patterns",
		`{"minOccurrences":2}`)
	assert.Equal(t, http.StatusOK, rr.Code)

	var out brain.PatternsOutput
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.NotNil(t, out.Patterns)
}

func TestToolReflect(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/brain_reflect",
		`{"agentId":"agent1","windowHours":24}`)
	assert.Equal(t, http.StatusOK, rr.Code)

	var out brain.ReflectOutput
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.NotEmpty(t, out.CycleID)
	assert.Equal(t, "PENDING", out.HumanFeedbackStatus)
}

func TestToolFeedback(t *testing.T) {
	srv := newBrainTestServer(t)

	// First create a reflection cycle
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/brain_reflect",
		`{"agentId":"agent1"}`)
	require.Equal(t, http.StatusOK, rr.Code)
	var ref brain.ReflectOutput
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &ref))

	// Submit feedback
	rr = doRequest(srv, http.MethodPost, "/api/v1/tools/brain_feedback",
		`{"cycleId":"`+ref.CycleID+`","feedbackType":"confirm","scores":{"impact":0.8},"comment":"good"}`)
	assert.Equal(t, http.StatusOK, rr.Code)

	var out brain.FeedbackOutput
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.NotEmpty(t, out.FeedbackID)
	assert.Equal(t, "RECEIVED", out.CycleStatus)
}

func TestToolFeedbackInvalid(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/brain_feedback",
		`{"cycleId":"nonexistent"}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestToolBadJSON(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodPost, "/api/v1/tools/brain_search", `{bad}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- Cycle endpoint tests (Phase 6) ---

func TestListCyclesEmpty(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodGet, "/api/v1/cycles", "")
	assert.Equal(t, http.StatusOK, rr.Code)

	var cycles []graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &cycles))
	assert.Len(t, cycles, 0)
}

func TestListCyclesWithFilter(t *testing.T) {
	store := newBadgerTestStorage(t)
	g := graph.New(store)
	bs := brain.NewService(g, nil, nil)
	srv := NewHTTPServer(":0", g, WithBrain(bs))

	// Create cycles with different statuses
	g.UpsertNode("n:cycle:1", "ReflectionCycle", graph.Properties{
		"humanFeedbackStatus": "NEEDS_FEEDBACK",
		"agentId":             "alice",
	})
	g.UpsertNode("n:cycle:2", "ReflectionCycle", graph.Properties{
		"humanFeedbackStatus": "REQUESTED",
		"agentId":             "bob",
	})
	g.UpsertNode("n:cycle:3", "ReflectionCycle", graph.Properties{
		"humanFeedbackStatus": "NEEDS_FEEDBACK",
		"agentId":             "bob",
	})

	// Filter by status
	rr := doRequest(srv, http.MethodGet, "/api/v1/cycles?status=NEEDS_FEEDBACK", "")
	assert.Equal(t, http.StatusOK, rr.Code)
	var cycles []graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &cycles))
	assert.Len(t, cycles, 2)

	// Filter by agentId
	rr = doRequest(srv, http.MethodGet, "/api/v1/cycles?agentId=bob", "")
	assert.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &cycles))
	assert.Len(t, cycles, 2)

	// Filter by both
	rr = doRequest(srv, http.MethodGet, "/api/v1/cycles?status=NEEDS_FEEDBACK&agentId=bob", "")
	assert.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &cycles))
	assert.Len(t, cycles, 1)
}

func TestWaiveCycleSuccess(t *testing.T) {
	store := newBadgerTestStorage(t)
	g := graph.New(store)
	bs := brain.NewService(g, nil, nil)
	srv := NewHTTPServer(":0", g, WithBrain(bs))

	g.UpsertNode("n:cycle:1", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "NEEDS_FEEDBACK",
	})

	rr := doRequest(srv, http.MethodPost, "/api/v1/cycles/n:cycle:1/waive", "")
	assert.Equal(t, http.StatusOK, rr.Code)

	var node graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &node))
	assert.Equal(t, "WAIVED", node.Properties["humanFeedbackStatus"])
}

func TestWaiveCycleFromRequested(t *testing.T) {
	store := newBadgerTestStorage(t)
	g := graph.New(store)
	bs := brain.NewService(g, nil, nil)
	srv := NewHTTPServer(":0", g, WithBrain(bs))

	g.UpsertNode("n:cycle:1", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "REQUESTED",
	})

	rr := doRequest(srv, http.MethodPost, "/api/v1/cycles/n:cycle:1/waive", "")
	assert.Equal(t, http.StatusOK, rr.Code)

	var node graph.Node
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &node))
	assert.Equal(t, "WAIVED", node.Properties["humanFeedbackStatus"])
}

func TestWaiveCycleNotFound(t *testing.T) {
	srv := newBrainTestServer(t)
	rr := doRequest(srv, http.MethodPost, "/api/v1/cycles/nonexistent/waive", "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestWaiveCycleWrongStatus(t *testing.T) {
	store := newBadgerTestStorage(t)
	g := graph.New(store)
	bs := brain.NewService(g, nil, nil)
	srv := NewHTTPServer(":0", g, WithBrain(bs))

	g.UpsertNode("n:cycle:1", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "PENDING",
	})

	rr := doRequest(srv, http.MethodPost, "/api/v1/cycles/n:cycle:1/waive", "")
	assert.Equal(t, http.StatusConflict, rr.Code)
}
