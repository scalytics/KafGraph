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
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
)

// registerComplianceGDPRRoutes wires all GDPR-specific CRUD routes.
func registerComplianceGDPRRoutes(mux *http.ServeMux, g *graph.Graph) {
	base := "/api/v2/compliance/gdpr"

	// Setup
	mux.HandleFunc("GET "+base+"/setup", handleGDPRGet(g, "OrgSetup"))
	mux.HandleFunc("PUT "+base+"/setup", handleGDPRUpsertSingleton(g, "OrgSetup"))

	// Data categories
	mux.HandleFunc("GET "+base+"/data-categories", handleGDPRList(g, "DataCategory"))
	mux.HandleFunc("POST "+base+"/data-categories", handleGDPRCreate(g, "DataCategory"))

	// Legal bases (read-only reference data)
	mux.HandleFunc("GET "+base+"/legal-bases", handleGDPRList(g, "LegalBasis"))

	// Security measures
	mux.HandleFunc("GET "+base+"/security-measures", handleGDPRList(g, "SecurityMeasure"))
	mux.HandleFunc("POST "+base+"/security-measures", handleGDPRCreate(g, "SecurityMeasure"))

	// RoPA
	mux.HandleFunc("GET "+base+"/ropa", handleGDPRList(g, "ProcessingActivity"))
	mux.HandleFunc("POST "+base+"/ropa", handleGDPRCreateWithEdges(g, "ProcessingActivity"))
	mux.HandleFunc("GET "+base+"/ropa/", handleGDPRGetByID(g, "ProcessingActivity", base+"/ropa/"))
	mux.HandleFunc("PUT "+base+"/ropa/", handleGDPRUpdate(g, "ProcessingActivity", base+"/ropa/"))
	mux.HandleFunc("DELETE "+base+"/ropa/", handleGDPRDelete(g, base+"/ropa/"))

	// DSR
	mux.HandleFunc("GET "+base+"/dsr", handleGDPRList(g, "DataSubjectRequest"))
	mux.HandleFunc("POST "+base+"/dsr", handleGDPRCreate(g, "DataSubjectRequest"))
	mux.HandleFunc("PUT "+base+"/dsr/", handleGDPRUpdate(g, "DataSubjectRequest", base+"/dsr/"))
	mux.HandleFunc("GET "+base+"/dsr/sla", handleGDPRDSRSLA(g))

	// Breaches
	mux.HandleFunc("GET "+base+"/breaches", handleGDPRList(g, "DataBreach"))
	mux.HandleFunc("POST "+base+"/breaches", handleGDPRCreate(g, "DataBreach"))
	mux.HandleFunc("PUT "+base+"/breaches/", handleGDPRUpdate(g, "DataBreach", base+"/breaches/"))

	// DPIA
	mux.HandleFunc("GET "+base+"/dpia", handleGDPRList(g, "DPIA"))
	mux.HandleFunc("POST "+base+"/dpia", handleGDPRCreate(g, "DPIA"))
	mux.HandleFunc("PUT "+base+"/dpia/", handleGDPRUpdate(g, "DPIA", base+"/dpia/"))

	// Processors
	mux.HandleFunc("GET "+base+"/processors", handleGDPRList(g, "DataProcessor"))
	mux.HandleFunc("POST "+base+"/processors", handleGDPRCreate(g, "DataProcessor"))
	mux.HandleFunc("PUT "+base+"/processors/", handleGDPRUpdate(g, "DataProcessor", base+"/processors/"))

	// Checklist
	mux.HandleFunc("GET "+base+"/checklist", handleGDPRList(g, "ChecklistItem"))
	mux.HandleFunc("PUT "+base+"/checklist/", handleGDPRUpdate(g, "ChecklistItem", base+"/checklist/"))

	// Evidence
	mux.HandleFunc("GET "+base+"/evidence", handleGDPRList(g, "Evidence"))
	mux.HandleFunc("POST "+base+"/evidence", handleGDPRCreate(g, "Evidence"))
	mux.HandleFunc("DELETE "+base+"/evidence/", handleGDPRDelete(g, base+"/evidence/"))
}

// handleGDPRList returns all nodes of the given label.
func handleGDPRList(g *graph.Graph, label string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		nodes, err := g.NodesByLabel(label)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items := make([]map[string]any, 0, len(nodes))
		for _, n := range nodes {
			items = append(items, nodeToJSON(n))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	}
}

// handleGDPRGet returns the first node of the given label (singleton).
func handleGDPRGet(g *graph.Graph, label string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		nodes, err := g.NodesByLabel(label)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if len(nodes) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{})
			return
		}
		writeJSON(w, http.StatusOK, nodeToJSON(nodes[0]))
	}
}

// handleGDPRCreate creates a new node with the given label.
func handleGDPRCreate(g *graph.Graph, label string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var props graph.Properties
		if err := readJSON(r, &props); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		props["createdAt"] = time.Now().UTC().Format(time.RFC3339)
		node, err := g.CreateNode(label, props)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, nodeToJSON(node))
	}
}

// handleGDPRCreateWithEdges creates a node and optional edges from properties.
func handleGDPRCreateWithEdges(g *graph.Graph, label string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Extract edge targets before creating node.
		edgeTargets := map[string]string{}
		for _, key := range []string{"legalBasisId", "securityMeasureId"} {
			if v, ok := body[key].(string); ok && v != "" {
				edgeTargets[key] = v
				delete(body, key)
			}
		}
		// Category IDs for PROCESSES_CATEGORY edges.
		var categoryIDs []string
		if cats, ok := body["categoryIds"].([]any); ok {
			for _, c := range cats {
				if s, ok := c.(string); ok {
					categoryIDs = append(categoryIDs, s)
				}
			}
			delete(body, "categoryIds")
		}

		props := graph.Properties(body)
		props["createdAt"] = time.Now().UTC().Format(time.RFC3339)

		node, err := g.CreateNode(label, props)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create edges.
		if lbID, ok := edgeTargets["legalBasisId"]; ok {
			_, _ = g.CreateEdge("HAS_LEGAL_BASIS", node.ID, graph.NodeID(lbID), nil)
		}
		if smID, ok := edgeTargets["securityMeasureId"]; ok {
			_, _ = g.CreateEdge("PROTECTED_BY", node.ID, graph.NodeID(smID), nil)
		}
		for _, catID := range categoryIDs {
			_, _ = g.CreateEdge("PROCESSES_CATEGORY", node.ID, graph.NodeID(catID), nil)
		}

		writeJSON(w, http.StatusCreated, nodeToJSON(node))
	}
}

// handleGDPRUpsertSingleton creates or updates a singleton node.
func handleGDPRUpsertSingleton(g *graph.Graph, label string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var props graph.Properties
		if err := readJSON(r, &props); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		props["updatedAt"] = time.Now().UTC().Format(time.RFC3339)

		// Find existing.
		nodes, _ := g.NodesByLabel(label)
		var node *graph.Node
		var err error
		if len(nodes) > 0 {
			node, err = g.UpsertNode(nodes[0].ID, label, props)
		} else {
			props["createdAt"] = time.Now().UTC().Format(time.RFC3339)
			node, err = g.CreateNode(label, props)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, nodeToJSON(node))
	}
}

// handleGDPRGetByID returns a node by its graph ID extracted from the URL path.
func handleGDPRGetByID(g *graph.Graph, label, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := graph.NodeID(strings.TrimPrefix(r.URL.Path, prefix))
		if nodeID == "" {
			writeError(w, http.StatusBadRequest, "node ID required")
			return
		}
		node, err := g.GetNode(nodeID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		// Include neighbors for context.
		edges, _ := g.Neighbors(nodeID)
		edgeList := make([]map[string]any, 0, len(edges))
		for _, e := range edges {
			edgeList = append(edgeList, edgeToJSON(e))
		}
		result := nodeToJSON(node)
		result["edges"] = edgeList
		writeJSON(w, http.StatusOK, result)
	}
}

// handleGDPRUpdate updates a node by ID.
func handleGDPRUpdate(g *graph.Graph, label, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := graph.NodeID(strings.TrimPrefix(r.URL.Path, prefix))
		if nodeID == "" {
			writeError(w, http.StatusBadRequest, "node ID required")
			return
		}
		var props graph.Properties
		if err := readJSON(r, &props); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		props["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
		node, err := g.UpsertNode(nodeID, label, props)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, nodeToJSON(node))
	}
}

// handleGDPRDelete deletes a node by ID.
func handleGDPRDelete(g *graph.Graph, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := graph.NodeID(strings.TrimPrefix(r.URL.Path, prefix))
		if nodeID == "" {
			writeError(w, http.StatusBadRequest, "node ID required")
			return
		}
		if err := g.DeleteNode(nodeID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// handleGDPRDSRSLA returns SLA status for all active DSRs.
func handleGDPRDSRSLA(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		nodes, err := g.NodesByLabel("DataSubjectRequest")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		now := time.Now().UTC()
		type slaItem struct {
			NodeID      string `json:"nodeId"`
			RequestType string `json:"requestType"`
			Status      string `json:"status"`
			Deadline    string `json:"deadline"`
			DaysLeft    int    `json:"daysLeft"`
			Overdue     bool   `json:"overdue"`
		}
		var items []slaItem
		for _, n := range nodes {
			status, _ := n.Properties["status"].(string)
			if status == "completed" || status == "closed" {
				continue
			}
			deadlineStr, _ := n.Properties["deadline"].(string)
			reqType, _ := n.Properties["requestType"].(string)
			item := slaItem{
				NodeID:      string(n.ID),
				RequestType: reqType,
				Status:      status,
				Deadline:    deadlineStr,
			}
			if deadlineStr != "" {
				if deadline, err := time.Parse(time.RFC3339, deadlineStr); err == nil {
					days := int(deadline.Sub(now).Hours() / 24)
					item.DaysLeft = days
					item.Overdue = days < 0
				} else if deadline, err := time.Parse("2006-01-02", deadlineStr); err == nil {
					days := int(deadline.Sub(now).Hours() / 24)
					item.DaysLeft = days
					item.Overdue = days < 0
				}
			}
			items = append(items, item)
		}
		writeJSON(w, http.StatusOK, map[string]any{"sla": items, "total": len(items)})
	}
}
