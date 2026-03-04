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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/config"
	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/server"
	"github.com/scalytics/kafgraph/internal/storage"
)

// TestE2EManagementInfo tests the management info endpoint with real BadgerDB storage.
func TestE2EManagementInfo(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	cfg := &config.Config{
		StorageEngine: "badger",
		DataDir:       t.TempDir(),
	}
	srv := server.NewHTTPServer(":0", g, server.WithConfig(cfg))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v2/mgmt/info")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "badger", body["storageEngine"])
	assert.NotEmpty(t, body["goVersion"])
}

// TestE2EManagementGraphExplore tests graph exploration with real storage.
func TestE2EManagementGraphExplore(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	// Create test data
	agent, err := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	require.NoError(t, err)

	msg, err := g.CreateNode("Message", graph.Properties{"text": "hello"})
	require.NoError(t, err)

	_, err = g.CreateEdge("AUTHORED", agent.ID, msg.ID, nil)
	require.NoError(t, err)

	cfg := &config.Config{StorageEngine: "badger"}
	srv := server.NewHTTPServer(":0", g, server.WithConfig(cfg))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Explore by node
	resp, err := http.Get(ts.URL + "/api/v2/mgmt/graph/explore?nodeId=" + string(agent.ID) + "&depth=1")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	nodes := body["nodes"].([]any)
	assert.Len(t, nodes, 2) // agent + message

	edges := body["edges"].([]any)
	assert.Len(t, edges, 1)
}

// TestE2EManagementGraphStats tests graph statistics with real storage.
func TestE2EManagementGraphStats(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	_, _ = g.CreateNode("Agent", graph.Properties{"name": "alice"})
	_, _ = g.CreateNode("Agent", graph.Properties{"name": "bob"})
	_, _ = g.CreateNode("Message", graph.Properties{"text": "hello"})

	cfg := &config.Config{StorageEngine: "badger"}
	srv := server.NewHTTPServer(":0", g, server.WithConfig(cfg))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v2/mgmt/stats/graph")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	nodes := body["nodes"].(map[string]any)
	assert.Equal(t, float64(3), nodes["total"])

	byLabel := nodes["byLabel"].(map[string]any)
	assert.Equal(t, float64(2), byLabel["Agent"])
	assert.Equal(t, float64(1), byLabel["Message"])
}

// TestE2EManagementUIServed tests that the embedded UI is served.
func TestE2EManagementUIServed(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	cfg := &config.Config{StorageEngine: "badger"}
	srv := server.NewHTTPServer(":0", g, server.WithConfig(cfg))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2EManagementSearch tests graph search with real storage.
func TestE2EManagementSearch(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	_, _ = g.CreateNode("Agent", graph.Properties{"name": "alice"})
	_, _ = g.CreateNode("Agent", graph.Properties{"name": "bob"})

	cfg := &config.Config{StorageEngine: "badger"}
	srv := server.NewHTTPServer(":0", g, server.WithConfig(cfg))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v2/mgmt/graph/search?q=alice")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	nodes := body["nodes"].([]any)
	assert.Len(t, nodes, 1)
}
