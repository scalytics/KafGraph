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
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/server"
	"github.com/scalytics/kafgraph/internal/storage"
)

// TestE2EGraphCRUD exercises the full graph lifecycle with an embedded BadgerDB.
func TestE2EGraphCRUD(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	// 1. Create nodes
	agent, err := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	require.NoError(t, err)

	conv, err := g.CreateNode("Conversation", graph.Properties{"topic": "planning"})
	require.NoError(t, err)

	msg, err := g.CreateNode("Message", graph.Properties{"text": "hello"})
	require.NoError(t, err)

	// 2. Create edges
	authored, err := g.CreateEdge("AUTHORED", agent.ID, msg.ID, nil)
	require.NoError(t, err)

	_, err = g.CreateEdge("BELONGS_TO", msg.ID, conv.ID, nil)
	require.NoError(t, err)

	// 3. Query by label
	agents, err := g.NodesByLabel("Agent")
	require.NoError(t, err)
	assert.Len(t, agents, 1)
	assert.Equal(t, "alice", agents[0].Properties["name"])

	// 4. Query neighbors
	edges, err := g.Neighbors(agent.ID)
	require.NoError(t, err)
	assert.Len(t, edges, 1)
	assert.Equal(t, authored.ID, edges[0].ID)

	// 5. Delete and verify cleanup
	err = g.DeleteNode(agent.ID)
	require.NoError(t, err)

	_, err = g.GetNode(agent.ID)
	assert.Error(t, err)

	edges, err = g.Neighbors(msg.ID)
	require.NoError(t, err)
	// AUTHORED edge should be removed (agent deleted), only BELONGS_TO remains
	assert.Len(t, edges, 1)
}

// TestE2EBoltHandshake validates the Bolt v4 handshake with a real TCP connection.
func TestE2EBoltHandshake(t *testing.T) {
	srv, err := server.NewBoltServer("127.0.0.1:0")
	require.NoError(t, err)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.Serve(ctx) //nolint:errcheck

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	defer conn.Close()

	// Send handshake
	binary.Write(conn, binary.BigEndian, server.BoltMagic)
	binary.Write(conn, binary.BigEndian, server.BoltVersion4_4)
	binary.Write(conn, binary.BigEndian, uint32(0))
	binary.Write(conn, binary.BigEndian, uint32(0))
	binary.Write(conn, binary.BigEndian, uint32(0))

	// Read response
	var negotiated uint32
	err = binary.Read(conn, binary.BigEndian, &negotiated)
	require.NoError(t, err)
	assert.Equal(t, server.BoltVersion4_4, negotiated)
}

// TestE2EHTTPGraphCRUD exercises the HTTP REST API end-to-end with a real BadgerDB.
func TestE2EHTTPGraphCRUD(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	srv := server.NewHTTPServer(":0", g)
	ts := httptest.NewServer(srv.Handler())

	defer ts.Close()

	// Create nodes
	resp, err := http.Post(ts.URL+"/api/v1/nodes", "application/json",
		strings.NewReader(`{"label":"Agent","properties":{"name":"alice"}}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var alice graph.Node
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&alice))
	resp.Body.Close()

	resp, err = http.Post(ts.URL+"/api/v1/nodes", "application/json",
		strings.NewReader(`{"label":"Agent","properties":{"name":"bob"}}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var bob graph.Node
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&bob))
	resp.Body.Close()

	// Create edge
	edgeBody := `{"label":"KNOWS","fromId":"` + string(alice.ID) + `","toId":"` + string(bob.ID) + `"}`
	resp, err = http.Post(ts.URL+"/api/v1/edges", "application/json",
		strings.NewReader(edgeBody))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var edge graph.Edge
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&edge))
	resp.Body.Close()

	// Get node
	resp, err = http.Get(ts.URL + "/api/v1/nodes/" + string(alice.ID))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// List by label
	resp, err = http.Get(ts.URL + "/api/v1/nodes?label=Agent")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var agents []graph.Node
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&agents))
	resp.Body.Close()
	assert.Len(t, agents, 2)

	// Get neighbors
	resp, err = http.Get(ts.URL + "/api/v1/nodes/" + string(alice.ID) + "/edges")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var edges []graph.Edge
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&edges))
	resp.Body.Close()
	assert.Len(t, edges, 1)

	// Delete node
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/nodes/"+string(alice.ID), nil)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify gone
	resp, err = http.Get(ts.URL + "/api/v1/nodes/" + string(alice.ID))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// Health check
	resp, err = http.Get(ts.URL + "/healthz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Tools
	resp, err = http.Get(ts.URL + "/api/v1/tools")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// TestE2EBrainToolAPI validates the Brain Tool HTTP API.
func TestE2EBrainToolAPI(t *testing.T) {
	// TODO: start HTTP server, call /api/v1/tools/brain_search
	t.Skip("E2E scaffold — implement in Phase 3")
}
