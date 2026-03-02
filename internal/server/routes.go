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

	"github.com/scalytics/kafgraph/internal/graph"
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
func registerRoutes(mux *http.ServeMux, g *graph.Graph) {
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

	// Tool schema (placeholder)
	mux.HandleFunc("GET /api/v1/tools", handleListTools())
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
