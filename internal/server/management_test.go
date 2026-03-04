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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/config"
	"github.com/scalytics/kafgraph/internal/graph"
)

func newTestMgmtServer() *HTTPServer {
	g := graph.New(newMemStorage())
	cfg := &config.Config{
		Host:          "0.0.0.0",
		Port:          7474,
		BoltPort:      7687,
		DataDir:       "./data",
		StorageEngine: "badger",
		LogLevel:      "info",
		S3: config.S3Config{
			Endpoint:  "localhost:9000",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Bucket:    "test",
		},
	}
	return NewHTTPServer(":0", g, WithConfig(cfg))
}

func doMgmtRequest(srv *HTTPServer, method, path string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	srv.server.Handler.ServeHTTP(rr, req)
	return rr
}

// --- Info Endpoint ---

func TestMgmtInfo(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/info")

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, config.Version, body["version"])
	assert.NotEmpty(t, body["uptime"])
	assert.NotEmpty(t, body["goVersion"])
	assert.NotEmpty(t, body["os"])
	assert.NotEmpty(t, body["arch"])
	assert.Equal(t, "badger", body["storageEngine"])
}

// --- Storage Endpoint ---

func TestMgmtStorage(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/storage")

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "badger", body["engine"])
}

// --- Graph Stats ---

func TestMgmtGraphStatsEmpty(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/stats/graph")

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	nodes := body["nodes"].(map[string]any)
	assert.Equal(t, float64(0), nodes["total"])

	edges := body["edges"].(map[string]any)
	assert.Equal(t, float64(0), edges["total"])
}

func TestMgmtGraphStatsWithData(t *testing.T) {
	g := graph.New(newMemStorage())
	_, _ = g.CreateNode("Agent", graph.Properties{"name": "alice"})
	_, _ = g.CreateNode("Agent", graph.Properties{"name": "bob"})
	_, _ = g.CreateNode("Message", graph.Properties{"text": "hello"})

	cfg := &config.Config{StorageEngine: "badger"}
	srv := NewHTTPServer(":0", g, WithConfig(cfg))
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/stats/graph")

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	nodes := body["nodes"].(map[string]any)
	assert.Equal(t, float64(3), nodes["total"])

	byLabel := nodes["byLabel"].(map[string]any)
	assert.Equal(t, float64(2), byLabel["Agent"])
	assert.Equal(t, float64(1), byLabel["Message"])
}

// --- Config Endpoint ---

func TestMgmtConfig(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/config")

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	// Verify S3 secrets are redacted
	s3 := body["s3"].(map[string]any)
	assert.Equal(t, "***REDACTED***", s3["access_key"])
	assert.Equal(t, "***REDACTED***", s3["secret_key"])
	assert.Equal(t, "localhost:9000", s3["endpoint"])
}

func TestMgmtConfigNotAvailable(t *testing.T) {
	srv := NewHTTPServer(":0", graph.New(newMemStorage()))
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/config")

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// --- Cluster Endpoint ---

func TestMgmtClusterSingleNode(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/cluster")

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, false, body["enabled"])
}

// --- Graph Explore ---

func TestMgmtGraphExploreByLabel(t *testing.T) {
	g := graph.New(newMemStorage())
	n1, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	n2, _ := g.CreateNode("Message", graph.Properties{"text": "hello"})
	_, _ = g.CreateEdge("AUTHORED", n1.ID, n2.ID, nil)

	cfg := &config.Config{StorageEngine: "badger"}
	srv := NewHTTPServer(":0", g, WithConfig(cfg))

	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/graph/explore?label=Agent&limit=10")
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	nodes := body["nodes"].([]any)
	assert.GreaterOrEqual(t, len(nodes), 1)
}

func TestMgmtGraphExploreByNode(t *testing.T) {
	g := graph.New(newMemStorage())
	n1, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	n2, _ := g.CreateNode("Message", graph.Properties{"text": "hello"})
	_, _ = g.CreateEdge("AUTHORED", n1.ID, n2.ID, nil)

	cfg := &config.Config{StorageEngine: "badger"}
	srv := NewHTTPServer(":0", g, WithConfig(cfg))

	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/graph/explore?nodeId="+string(n1.ID)+"&depth=1")
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	nodes := body["nodes"].([]any)
	assert.Len(t, nodes, 2) // focal + neighbor

	edges := body["edges"].([]any)
	assert.Len(t, edges, 1)
}

func TestMgmtGraphExploreMissingParams(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/graph/explore")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMgmtGraphExploreNodeNotFound(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/graph/explore?nodeId=nonexistent")

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// --- Graph Search ---

func TestMgmtGraphSearch(t *testing.T) {
	g := graph.New(newMemStorage())
	_, _ = g.CreateNode("Agent", graph.Properties{"name": "alice"})
	_, _ = g.CreateNode("Agent", graph.Properties{"name": "bob"})

	cfg := &config.Config{StorageEngine: "badger"}
	srv := NewHTTPServer(":0", g, WithConfig(cfg))

	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/graph/search?q=alice")
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	nodes := body["nodes"].([]any)
	assert.Len(t, nodes, 1)
}

func TestMgmtGraphSearchMissingQuery(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/graph/search")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- Reflection Summary ---

func TestMgmtReflectSummaryEmpty(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/reflect/summary")

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, float64(0), body["totalCycles"])
	assert.Equal(t, float64(0), body["totalSignals"])
}

// --- Reflection Cycles ---

func TestMgmtReflectCyclesEmpty(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/reflect/cycles?limit=10")

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, float64(0), body["total"])
}

// --- Activity ---

func TestMgmtActivityEmpty(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/activity?hours=24")

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	events := body["events"].([]any)
	assert.Len(t, events, 0)
}

func TestMgmtActivityWithData(t *testing.T) {
	g := graph.New(newMemStorage())
	_, _ = g.CreateNode("Agent", graph.Properties{"name": "alice"})

	cfg := &config.Config{StorageEngine: "badger"}
	srv := NewHTTPServer(":0", g, WithConfig(cfg))

	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/activity?hours=24&limit=10")
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	events := body["events"].([]any)
	assert.Len(t, events, 1)
}

// --- Skills by Agent ---

func TestMgmtSkillsByAgentEmpty(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/skills/by-agent")

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	agents := body["agents"].([]any)
	assert.Len(t, agents, 0)

	roster := body["skillRoster"].([]any)
	assert.Len(t, roster, 0)
}

func TestMgmtSkillsByAgentWithData(t *testing.T) {
	g := graph.New(newMemStorage())

	// Create agents
	alice, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	bob, _ := g.CreateNode("Agent", graph.Properties{"name": "bob"})

	// Create skills
	search, _ := g.CreateNode("Skill", graph.Properties{"skillName": "web_search"})
	summarize, _ := g.CreateNode("Skill", graph.Properties{"skillName": "summarize"})

	// Declare skills via HAS_SKILL edges
	_, _ = g.CreateEdge("HAS_SKILL", alice.ID, search.ID, graph.Properties{"declaredAt": "2026-03-01T00:00:00Z"})
	_, _ = g.CreateEdge("HAS_SKILL", alice.ID, summarize.ID, graph.Properties{"declaredAt": "2026-03-01T00:00:00Z"})
	_, _ = g.CreateEdge("HAS_SKILL", bob.ID, search.ID, graph.Properties{"declaredAt": "2026-03-01T00:00:00Z"})

	// Create messages with AUTHORED + USES_SKILL edges
	msg1, _ := g.CreateNode("Message", graph.Properties{"skillName": "web_search"})
	_, _ = g.CreateEdge("AUTHORED", alice.ID, msg1.ID, nil)
	_, _ = g.CreateEdge("USES_SKILL", msg1.ID, search.ID, nil)

	msg2, _ := g.CreateNode("Message", graph.Properties{"skillName": "web_search"})
	_, _ = g.CreateEdge("AUTHORED", alice.ID, msg2.ID, nil)
	_, _ = g.CreateEdge("USES_SKILL", msg2.ID, search.ID, nil)

	cfg := &config.Config{StorageEngine: "badger"}
	srv := NewHTTPServer(":0", g, WithConfig(cfg))

	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/skills/by-agent?window=day")
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	agents := body["agents"].([]any)
	assert.Len(t, agents, 2) // alice, bob

	// Find alice
	var aliceEntry map[string]any
	for _, a := range agents {
		entry := a.(map[string]any)
		if entry["name"] == "alice" {
			aliceEntry = entry
			break
		}
	}
	require.NotNil(t, aliceEntry)
	assert.Len(t, aliceEntry["declaredSkills"].([]any), 2)
	assert.Len(t, aliceEntry["skillUsage"].([]any), 1)

	usage := aliceEntry["skillUsage"].([]any)[0].(map[string]any)
	assert.Equal(t, "web_search", usage["skill"])
	assert.Equal(t, float64(2), usage["totalUses"])

	// Check skill roster
	roster := body["skillRoster"].([]any)
	assert.GreaterOrEqual(t, len(roster), 2)
}

func TestMgmtSkillsByAgentWindowParam(t *testing.T) {
	srv := newTestMgmtServer()

	for _, w := range []string{"hour", "day", "week"} {
		rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/skills/by-agent?window="+w)
		assert.Equal(t, http.StatusOK, rr.Code, "window=%s", w)
	}
}

func TestMgmtSkillsByAgentHistory(t *testing.T) {
	g := graph.New(newMemStorage())

	// Create agent and skills
	formatter, _ := g.CreateNode("Agent", graph.Properties{"name": "formatter"})
	asciiDoc, _ := g.CreateNode("Skill", graph.Properties{"skillName": "ascii_doc"})
	proofread, _ := g.CreateNode("Skill", graph.Properties{"skillName": "proofread"})
	formatHTML, _ := g.CreateNode("Skill", graph.Properties{"skillName": "format_html"})

	// Version 1: initial roster — ascii_doc + proofread
	_, _ = g.CreateEdge("HAS_SKILL", formatter.ID, asciiDoc.ID, graph.Properties{
		"declaredAt": "2026-03-01T09:00:06Z", "rosterVersion": 1,
	})
	_, _ = g.CreateEdge("HAS_SKILL", formatter.ID, proofread.ID, graph.Properties{
		"declaredAt": "2026-03-01T09:00:06Z", "rosterVersion": 1,
	})
	// Version 2: gained format_html
	_, _ = g.CreateEdge("HAS_SKILL", formatter.ID, formatHTML.ID, graph.Properties{
		"declaredAt": "2026-03-01T09:02:18Z", "rosterVersion": 2,
	})

	cfg := &config.Config{StorageEngine: "badger"}
	srv := NewHTTPServer(":0", g, WithConfig(cfg))

	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/skills/by-agent")
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	agents := body["agents"].([]any)
	require.Len(t, agents, 1)

	entry := agents[0].(map[string]any)
	assert.Equal(t, "formatter", entry["name"])

	// All 3 skills should be active
	assert.Len(t, entry["declaredSkills"].([]any), 3)

	// History should have 3 entries, sorted by declaredAt
	history := entry["skillHistory"].([]any)
	require.Len(t, history, 3)

	h0 := history[0].(map[string]any)
	assert.Equal(t, "2026-03-01T09:00:06Z", h0["declaredAt"])
	assert.Equal(t, true, h0["active"])
	assert.Equal(t, float64(1), h0["rosterVersion"])

	h2 := history[2].(map[string]any)
	assert.Equal(t, "format_html", h2["skill"])
	assert.Equal(t, "2026-03-01T09:02:18Z", h2["declaredAt"])
	assert.Equal(t, float64(2), h2["rosterVersion"])
	assert.Equal(t, true, h2["active"])
}

func TestMgmtSkillsByAgentHistoryRemoved(t *testing.T) {
	g := graph.New(newMemStorage())

	agent, _ := g.CreateNode("Agent", graph.Properties{"name": "tester"})
	skillA, _ := g.CreateNode("Skill", graph.Properties{"skillName": "skill_a"})
	skillB, _ := g.CreateNode("Skill", graph.Properties{"skillName": "skill_b"})

	// skill_a declared then removed
	_, _ = g.CreateEdge("HAS_SKILL", agent.ID, skillA.ID, graph.Properties{
		"declaredAt": "2026-03-01T09:00:00Z", "rosterVersion": 1,
		"removedAt": "2026-03-01T10:00:00Z",
	})
	// skill_b active
	_, _ = g.CreateEdge("HAS_SKILL", agent.ID, skillB.ID, graph.Properties{
		"declaredAt": "2026-03-01T09:00:00Z", "rosterVersion": 1,
	})

	cfg := &config.Config{StorageEngine: "badger"}
	srv := NewHTTPServer(":0", g, WithConfig(cfg))

	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/skills/by-agent")
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	agents := body["agents"].([]any)
	entry := agents[0].(map[string]any)

	// Only skill_b should be in declaredSkills (active)
	assert.Len(t, entry["declaredSkills"].([]any), 1)
	assert.Equal(t, "skill_b", entry["declaredSkills"].([]any)[0])

	// History should have both (including removed)
	history := entry["skillHistory"].([]any)
	require.Len(t, history, 2)

	// Find the removed one
	var removedEntry map[string]any
	for _, h := range history {
		he := h.(map[string]any)
		if he["skill"] == "skill_a" {
			removedEntry = he
		}
	}
	require.NotNil(t, removedEntry)
	assert.Equal(t, false, removedEntry["active"])
	assert.Equal(t, "2026-03-01T10:00:00Z", removedEntry["removedAt"])
}

func TestMgmtSkillsByAgentFilter(t *testing.T) {
	g := graph.New(newMemStorage())

	alice, _ := g.CreateNode("Agent", graph.Properties{"name": "alice"})
	bob, _ := g.CreateNode("Agent", graph.Properties{"name": "bob"})
	skill, _ := g.CreateNode("Skill", graph.Properties{"skillName": "search"})

	_, _ = g.CreateEdge("HAS_SKILL", alice.ID, skill.ID, graph.Properties{"declaredAt": "2026-03-01T00:00:00Z"})
	_, _ = g.CreateEdge("HAS_SKILL", bob.ID, skill.ID, graph.Properties{"declaredAt": "2026-03-01T00:00:00Z"})

	cfg := &config.Config{StorageEngine: "badger"}
	srv := NewHTTPServer(":0", g, WithConfig(cfg))

	// Without filter — both agents
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/skills/by-agent")
	assert.Equal(t, http.StatusOK, rr.Code)
	var all map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &all))
	assert.Len(t, all["agents"].([]any), 2)

	// With filter — only alice
	rr = doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/skills/by-agent?agent=alice")
	assert.Equal(t, http.StatusOK, rr.Code)
	var filtered map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &filtered))
	agents := filtered["agents"].([]any)
	assert.Len(t, agents, 1)
	assert.Equal(t, "alice", agents[0].(map[string]any)["name"])

	// Case-insensitive filter
	rr = doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/skills/by-agent?agent=Alice")
	assert.Equal(t, http.StatusOK, rr.Code)
	var ciBody map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &ciBody))
	assert.Len(t, ciBody["agents"].([]any), 1)

	// Non-existent agent
	rr = doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/skills/by-agent?agent=nobody")
	assert.Equal(t, http.StatusOK, rr.Code)
	var empty map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &empty))
	assert.Len(t, empty["agents"].([]any), 0)
}

// --- CORS Middleware ---

func TestCORSHeaders(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/api/v2/mgmt/info")

	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSPreflight(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodOptions, "/api/v2/mgmt/info")

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}

// --- Static File Serving ---

func TestStaticFileServing(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "KafGraph")
}

func TestStaticCSSServing(t *testing.T) {
	srv := newTestMgmtServer()
	rr := doMgmtRequest(srv, http.MethodGet, "/css/tokens.css")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "--color-accent")
}

// --- sanitizeConfig ---

func TestSanitizeConfig(t *testing.T) {
	cfg := &config.Config{
		Host:          "0.0.0.0",
		Port:          7474,
		StorageEngine: "badger",
		S3: config.S3Config{
			Endpoint:  "localhost:9000",
			AccessKey: "secret-access",
			SecretKey: "secret-key",
			Bucket:    "test",
		},
	}

	result := sanitizeConfig(cfg)

	s3, ok := result["s3"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "***REDACTED***", s3["access_key"])
	assert.Equal(t, "***REDACTED***", s3["secret_key"])
	assert.Equal(t, "localhost:9000", s3["endpoint"])
	assert.Equal(t, "test", s3["bucket"])

	// Verify main fields
	assert.Equal(t, "0.0.0.0", result["host"])
	assert.Equal(t, 7474, result["port"])
	assert.Equal(t, "badger", result["storage_engine"])
}
