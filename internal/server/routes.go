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
	"net/http"
	"strings"

	"github.com/scalytics/kafgraph/internal/brain"
	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/query"
)

// brainTools is the static list of brain tool names (Phase 3 placeholder).
var brainTools = []string{
	"brain_search",
	"brain_recall",
	"brain_capture",
	"brain_recent",
	"brain_patterns",
	"brain_reflect",
	"brain_feedback",
}

// registerRoutes wires all REST CRUD routes onto the given mux.
func registerRoutes(mux *http.ServeMux, g *graph.Graph, exec *query.Executor, bs *brain.Service) {
	// Node endpoints
	mux.HandleFunc("POST /api/v1/nodes", handleCreateNode(g))
	mux.HandleFunc("GET /api/v1/nodes/{id}", handleGetNode(g))
	mux.HandleFunc("DELETE /api/v1/nodes/{id}", handleDeleteNode(g))
	mux.HandleFunc("GET /api/v1/nodes", handleListNodes(g))

	// Edge endpoints
	mux.HandleFunc("POST /api/v1/edges", handleCreateEdge(g))
	mux.HandleFunc("GET /api/v1/edges/{id}", handleGetEdge(g))
	mux.HandleFunc("DELETE /api/v1/edges/{id}", handleDeleteEdge(g))

	// Node neighbors
	mux.HandleFunc("GET /api/v1/nodes/{id}/edges", handleNodeEdges(g))

	// Tool schema
	mux.HandleFunc("GET /api/v1/tools", handleListTools())

	// Query endpoint
	mux.HandleFunc("POST /api/v1/query", handleQuery(exec))

	// Cycle endpoints (Phase 6)
	mux.HandleFunc("GET /api/v1/cycles", handleListCycles(g))
	mux.HandleFunc("POST /api/v1/cycles/{id}/waive", handleWaiveCycle(g))

	// Brain tool endpoint
	mux.HandleFunc("POST /api/v1/tools/{toolName}", handleToolExec(bs))
}

// --- Node handlers ---

type createNodeRequest struct {
	Label      string           `json:"label"`
	Properties graph.Properties `json:"properties"`
}

func handleCreateNode(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createNodeRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.Label == "" {
			writeError(w, http.StatusBadRequest, "label is required")
			return
		}

		node, err := g.CreateNode(req.Label, req.Properties)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, node)
	}
}

func handleGetNode(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		node, err := g.GetNode(graph.NodeID(id))
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, node)
	}
}

func handleDeleteNode(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if err := g.DeleteNode(graph.NodeID(id)); err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListNodes(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		label := r.URL.Query().Get("label")
		if label == "" {
			writeError(w, http.StatusBadRequest, "label query parameter is required")
			return
		}

		nodes, err := g.NodesByLabel(label)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if nodes == nil {
			nodes = []*graph.Node{}
		}
		writeJSON(w, http.StatusOK, nodes)
	}
}

// --- Edge handlers ---

type createEdgeRequest struct {
	Label      string           `json:"label"`
	FromID     string           `json:"fromId"`
	ToID       string           `json:"toId"`
	Properties graph.Properties `json:"properties"`
}

func handleCreateEdge(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createEdgeRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.Label == "" || req.FromID == "" || req.ToID == "" {
			writeError(w, http.StatusBadRequest, "label, fromId, and toId are required")
			return
		}

		edge, err := g.CreateEdge(req.Label, graph.NodeID(req.FromID), graph.NodeID(req.ToID), req.Properties)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, edge)
	}
}

func handleGetEdge(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		edge, err := g.GetEdge(graph.EdgeID(id))
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, edge)
	}
}

func handleDeleteEdge(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if err := g.DeleteEdge(graph.EdgeID(id)); err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleNodeEdges(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		edges, err := g.Neighbors(graph.NodeID(id))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if edges == nil {
			edges = []*graph.Edge{}
		}
		writeJSON(w, http.StatusOK, edges)
	}
}

// --- Tool schema handler ---

func handleListTools() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"tools": brainTools,
		})
	}
}

// --- Query handler ---

type queryRequest struct {
	Cypher string         `json:"cypher"`
	Params map[string]any `json:"params"`
}

func handleQuery(exec *query.Executor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if exec == nil {
			writeError(w, http.StatusNotImplemented, "query engine not available")
			return
		}
		var req queryRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.Cypher == "" {
			writeError(w, http.StatusBadRequest, "cypher query is required")
			return
		}
		rs, err := exec.Execute(req.Cypher, req.Params)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Convert rows to [][]any for JSON output
		rows := make([][]any, len(rs.Rows))
		for i, row := range rs.Rows {
			rowData := make([]any, len(rs.Columns))
			for j, col := range rs.Columns {
				rowData[j] = row[col]
			}
			rows[i] = rowData
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"columns": rs.Columns,
			"rows":    rows,
		})
	}
}

// --- Cycle handlers (Phase 6) ---

func handleListCycles(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cycles, err := g.NodesByLabel("ReflectionCycle")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if cycles == nil {
			cycles = []*graph.Node{}
		}

		statusFilter := r.URL.Query().Get("status")
		agentFilter := r.URL.Query().Get("agentId")

		var filtered []*graph.Node
		for _, c := range cycles {
			if statusFilter != "" {
				fbStatus, _ := c.Properties["humanFeedbackStatus"].(string)
				if fbStatus != statusFilter {
					continue
				}
			}
			if agentFilter != "" {
				agentID, _ := c.Properties["agentId"].(string)
				if agentID != agentFilter {
					continue
				}
			}
			filtered = append(filtered, c)
		}
		if filtered == nil {
			filtered = []*graph.Node{}
		}
		writeJSON(w, http.StatusOK, filtered)
	}
}

func handleWaiveCycle(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		node, err := g.GetNode(graph.NodeID(id))
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeError(w, http.StatusNotFound, "cycle not found: "+id)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if node.Label != "ReflectionCycle" {
			writeError(w, http.StatusBadRequest, "node is not a ReflectionCycle")
			return
		}

		fbStatus, _ := node.Properties["humanFeedbackStatus"].(string)
		if fbStatus != "NEEDS_FEEDBACK" && fbStatus != "REQUESTED" {
			writeError(w, http.StatusConflict, "cycle status is "+fbStatus+", expected NEEDS_FEEDBACK or REQUESTED")
			return
		}

		if _, err := g.UpsertNode(graph.NodeID(id), "ReflectionCycle", graph.Properties{
			"humanFeedbackStatus": "WAIVED",
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		updated, _ := g.GetNode(graph.NodeID(id))
		writeJSON(w, http.StatusOK, updated)
	}
}

// --- Brain tool handler ---

func handleToolExec(bs *brain.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bs == nil {
			writeError(w, http.StatusNotImplemented, "brain tools not available")
			return
		}
		toolName := r.PathValue("toolName")
		switch toolName {
		case "brain_search":
			var in brain.SearchInput
			if err := readJSON(r, &in); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
				return
			}
			out, err := bs.Search(in)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, out)

		case "brain_recall":
			var in brain.RecallInput
			if err := readJSON(r, &in); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
				return
			}
			out, err := bs.Recall(in)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, out)

		case "brain_capture":
			var in brain.CaptureInput
			if err := readJSON(r, &in); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
				return
			}
			out, err := bs.Capture(in)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, out)

		case "brain_recent":
			var in brain.RecentInput
			if err := readJSON(r, &in); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
				return
			}
			out, err := bs.Recent(in)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, out)

		case "brain_patterns":
			var in brain.PatternsInput
			if err := readJSON(r, &in); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
				return
			}
			out, err := bs.Patterns(in)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, out)

		case "brain_reflect":
			var in brain.ReflectInput
			if err := readJSON(r, &in); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
				return
			}
			out, err := bs.Reflect(in)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, out)

		case "brain_feedback":
			var in brain.FeedbackInput
			if err := readJSON(r, &in); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
				return
			}
			out, err := bs.Feedback(in)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, out)

		default:
			writeError(w, http.StatusNotFound, "unknown tool: "+toolName)
		}
	}
}
