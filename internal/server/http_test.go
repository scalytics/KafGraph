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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/cluster"
	"github.com/scalytics/kafgraph/internal/compliance"
	"github.com/scalytics/kafgraph/internal/config"
	"github.com/scalytics/kafgraph/internal/graph"
)

func TestHealthz(t *testing.T) {
	srv := NewHTTPServer(":0", graph.New(newMemStorage()))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	srv.server.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestReadyz(t *testing.T) {
	srv := NewHTTPServer(":0", graph.New(newMemStorage()))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	srv.server.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestReadyzNotReady(t *testing.T) {
	srv := NewHTTPServer(":0", nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	srv.server.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestVersion(t *testing.T) {
	srv := NewHTTPServer(":0", graph.New(newMemStorage()))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	srv.server.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, config.Version, body["version"])
	assert.Equal(t, config.GitCommit, body["commit"])
	assert.Equal(t, config.BuildDate, body["built"])
}

// memStorage is an in-memory Storage for HTTP handler tests.
type memStorage struct {
	nodes map[graph.NodeID]*graph.Node
	edges map[graph.EdgeID]*graph.Edge
}

func newMemStorage() *memStorage {
	return &memStorage{
		nodes: make(map[graph.NodeID]*graph.Node),
		edges: make(map[graph.EdgeID]*graph.Edge),
	}
}

func (m *memStorage) PutNode(n *graph.Node) error { m.nodes[n.ID] = n; return nil }
func (m *memStorage) PutEdge(e *graph.Edge) error { m.edges[e.ID] = e; return nil }
func (m *memStorage) DeleteEdge(id graph.EdgeID) error {
	delete(m.edges, id)
	return nil
}
func (m *memStorage) Close() error { return nil }

func (m *memStorage) GetNode(id graph.NodeID) (*graph.Node, error) {
	n, ok := m.nodes[id]
	if !ok {
		return nil, &notFoundError{msg: "node " + string(id) + " not found"}
	}
	return n, nil
}

func (m *memStorage) GetEdge(id graph.EdgeID) (*graph.Edge, error) {
	e, ok := m.edges[id]
	if !ok {
		return nil, &notFoundError{msg: "edge " + string(id) + " not found"}
	}
	return e, nil
}

func (m *memStorage) DeleteNode(id graph.NodeID) error {
	delete(m.nodes, id)
	for eid, e := range m.edges {
		if e.FromID == id || e.ToID == id {
			delete(m.edges, eid)
		}
	}
	return nil
}

func (m *memStorage) NodesByLabel(label string) ([]*graph.Node, error) {
	var result []*graph.Node
	for _, n := range m.nodes {
		if n.Label == label {
			result = append(result, n)
		}
	}
	return result, nil
}

func (m *memStorage) EdgesByNode(id graph.NodeID) ([]*graph.Edge, error) {
	var result []*graph.Edge
	for _, e := range m.edges {
		if e.FromID == id || e.ToID == id {
			result = append(result, e)
		}
	}
	return result, nil
}

type notFoundError struct{ msg string }

func (e *notFoundError) Error() string { return e.msg }

func TestHTTPServerServeAndShutdown(t *testing.T) {
	srv := NewHTTPServer("127.0.0.1:0", graph.New(newMemStorage()))

	go srv.Serve() //nolint:errcheck

	err := srv.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestHTTPServerHandler(t *testing.T) {
	srv := NewHTTPServer(":0", graph.New(newMemStorage()))
	h := srv.Handler()
	assert.NotNil(t, h)

	// Use the handler via httptest
	ts := httptest.NewServer(h)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServerOptionsAndReadJSON(t *testing.T) {
	opts := &serverOpts{}
	var exec cluster.QueryExecutor
	var membership *cluster.Membership
	var partMap *cluster.PartitionMap
	var compEngine *compliance.Engine

	WithExecutor(exec)(opts)
	WithBrain(nil)(opts)
	WithConfig(nil)(opts)
	WithMembership(membership)(opts)
	WithPartitionMap(partMap)(opts)
	WithCompliance(compEngine)(opts)

	assert.Nil(t, opts.exec)
	assert.Nil(t, opts.brain)
	assert.Nil(t, opts.cfg)
	assert.Nil(t, opts.membership)
	assert.Nil(t, opts.partMap)
	assert.Nil(t, opts.compEngine)

	req := httptest.NewRequest(http.MethodPost, "/unused", nil)
	req.Body = nil
	err := readJSON(req, &map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty request body")
}
