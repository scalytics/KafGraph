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
	"strconv"
	"strings"
	"time"

	"github.com/scalytics/kafgraph/internal/compliance"
	"github.com/scalytics/kafgraph/internal/graph"
)

// registerComplianceInspectionRoutes wires inspection and remediation routes.
func registerComplianceInspectionRoutes(mux *http.ServeMux, g *graph.Graph) {
	base := "/api/v2/compliance/inspections"

	mux.HandleFunc("GET "+base, handleInspectionList(g))
	mux.HandleFunc("POST "+base, handleInspectionCreate(g))
	mux.HandleFunc("GET "+base+"/", handleInspectionDetail(g, base+"/"))
	mux.HandleFunc("PUT "+base+"/", handleInspectionUpdate(g, base+"/"))

	// Sign-off: POST /api/v2/compliance/inspections/{id}/sign-off
	mux.HandleFunc("POST /api/v2/compliance/inspections/{id}/sign-off", handleInspectionSignOff(g))

	// Findings
	mux.HandleFunc("POST /api/v2/compliance/inspections/{id}/findings", handleFindingCreate(g))
	mux.HandleFunc("GET /api/v2/compliance/findings/", handleFindingDetail(g))
	mux.HandleFunc("PUT /api/v2/compliance/findings/", handleFindingUpdate(g))

	// Remediation
	mux.HandleFunc("POST /api/v2/compliance/findings/{id}/remediation", handleRemediationCreate(g))
	mux.HandleFunc("PUT /api/v2/compliance/remediation/", handleRemediationUpdate(g))

	// Audit trail
	mux.HandleFunc("GET /api/v2/compliance/events", handleComplianceEvents(g))
}

func handleInspectionList(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodes, err := g.NodesByLabel("Inspection")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		status := r.URL.Query().Get("status")
		items := make([]map[string]any, 0, len(nodes))
		for _, n := range nodes {
			if status != "" {
				if s, _ := n.Properties["status"].(string); s != status {
					continue
				}
			}
			item := nodeToJSON(n)
			// Count findings.
			edges, _ := g.Neighbors(n.ID)
			findingCount := 0
			for _, e := range edges {
				if e.Label == "HAS_FINDING" {
					findingCount++
				}
			}
			item["findingCount"] = findingCount
			items = append(items, item)
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	}
}

func handleInspectionCreate(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Props        map[string]any `json:"properties"`
			ScopeNodeIDs []string       `json:"scopeNodeIds"`
			ScanID       string         `json:"scanId"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if body.Props == nil {
			body.Props = map[string]any{}
		}
		node, err := compliance.CreateInspection(g, graph.Properties(body.Props), body.ScopeNodeIDs, body.ScanID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, nodeToJSON(node))
	}
}

func handleInspectionDetail(g *graph.Graph, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := graph.NodeID(strings.TrimPrefix(r.URL.Path, prefix))
		// Avoid matching sign-off route.
		if strings.Contains(string(nodeID), "/") {
			writeError(w, http.StatusBadRequest, "invalid inspection ID")
			return
		}
		if nodeID == "" {
			writeError(w, http.StatusBadRequest, "inspection ID required")
			return
		}
		node, err := g.GetNode(nodeID)
		if err != nil {
			writeError(w, http.StatusNotFound, "inspection not found")
			return
		}

		result := nodeToJSON(node)

		// Collect findings.
		edges, _ := g.Neighbors(nodeID)
		var findings []map[string]any
		var scopeNodes []map[string]any
		var basedOnScan map[string]any
		for _, e := range edges {
			switch e.Label {
			case "HAS_FINDING":
				if fn, err := g.GetNode(e.ToID); err == nil {
					finding := nodeToJSON(fn)
					// Count remediations for this finding.
					fEdges, _ := g.Neighbors(fn.ID)
					remCount := 0
					for _, fe := range fEdges {
						if fe.Label == "REMEDIATED_BY" {
							remCount++
						}
					}
					finding["remediationCount"] = remCount
					findings = append(findings, finding)
				}
			case "INSPECTS":
				if sn, err := g.GetNode(e.ToID); err == nil {
					scopeNodes = append(scopeNodes, nodeToJSON(sn))
				}
			case "BASED_ON":
				if bn, err := g.GetNode(e.ToID); err == nil {
					basedOnScan = nodeToJSON(bn)
				}
			}
		}
		result["findings"] = findings
		result["scope"] = scopeNodes
		result["basedOnScan"] = basedOnScan

		writeJSON(w, http.StatusOK, result)
	}
}

func handleInspectionUpdate(g *graph.Graph, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := graph.NodeID(strings.TrimPrefix(r.URL.Path, prefix))
		if strings.Contains(string(nodeID), "/") {
			writeError(w, http.StatusBadRequest, "invalid inspection ID")
			return
		}
		if nodeID == "" {
			writeError(w, http.StatusBadRequest, "inspection ID required")
			return
		}
		var props graph.Properties
		if err := readJSON(r, &props); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		props["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
		node, err := g.UpsertNode(nodeID, "Inspection", props)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		compliance.LogEvent(g, "inspection_updated", "", "Inspection updated", string(nodeID))
		writeJSON(w, http.StatusOK, nodeToJSON(node))
	}
}

func handleInspectionSignOff(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "inspection ID required")
			return
		}
		var body struct {
			ApproverID string `json:"approverId"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if body.ApproverID == "" {
			writeError(w, http.StatusBadRequest, "approverId required")
			return
		}
		if err := compliance.SignOffInspection(g, graph.NodeID(id), body.ApproverID); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "signed_off"})
	}
}

func handleFindingCreate(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inspectionID := r.PathValue("id")
		if inspectionID == "" {
			writeError(w, http.StatusBadRequest, "inspection ID required")
			return
		}
		var body struct {
			Props           map[string]any `json:"properties"`
			AffectedNodeIDs []string       `json:"affectedNodeIds"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if body.Props == nil {
			body.Props = map[string]any{}
		}
		node, err := compliance.CreateFinding(g, graph.NodeID(inspectionID), graph.Properties(body.Props), body.AffectedNodeIDs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, nodeToJSON(node))
	}
}

func handleFindingDetail(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := graph.NodeID(strings.TrimPrefix(r.URL.Path, "/api/v2/compliance/findings/"))
		if nodeID == "" {
			writeError(w, http.StatusBadRequest, "finding ID required")
			return
		}
		node, err := g.GetNode(nodeID)
		if err != nil {
			writeError(w, http.StatusNotFound, "finding not found")
			return
		}
		result := nodeToJSON(node)

		// Collect edges.
		edges, _ := g.Neighbors(nodeID)
		var remediations []map[string]any
		var affected []map[string]any
		var evidence []map[string]any
		for _, e := range edges {
			switch e.Label {
			case "REMEDIATED_BY":
				if rn, err := g.GetNode(e.ToID); err == nil {
					remediations = append(remediations, nodeToJSON(rn))
				}
			case "AFFECTS":
				if an, err := g.GetNode(e.ToID); err == nil {
					affected = append(affected, nodeToJSON(an))
				}
			case "EVIDENCED_BY":
				if en, err := g.GetNode(e.ToID); err == nil {
					evidence = append(evidence, nodeToJSON(en))
				}
			}
		}
		result["remediations"] = remediations
		result["affected"] = affected
		result["evidence"] = evidence

		writeJSON(w, http.StatusOK, result)
	}
}

func handleFindingUpdate(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := graph.NodeID(strings.TrimPrefix(r.URL.Path, "/api/v2/compliance/findings/"))
		if nodeID == "" {
			writeError(w, http.StatusBadRequest, "finding ID required")
			return
		}
		var props graph.Properties
		if err := readJSON(r, &props); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		props["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
		node, err := g.UpsertNode(nodeID, "InspectionFinding", props)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		compliance.LogEvent(g, "finding_updated", "", "Finding updated", string(nodeID))
		writeJSON(w, http.StatusOK, nodeToJSON(node))
	}
}

func handleRemediationCreate(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		findingID := r.PathValue("id")
		if findingID == "" {
			writeError(w, http.StatusBadRequest, "finding ID required")
			return
		}
		var props graph.Properties
		if err := readJSON(r, &props); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		node, err := compliance.CreateRemediation(g, graph.NodeID(findingID), props)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, nodeToJSON(node))
	}
}

func handleRemediationUpdate(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := graph.NodeID(strings.TrimPrefix(r.URL.Path, "/api/v2/compliance/remediation/"))
		if nodeID == "" {
			writeError(w, http.StatusBadRequest, "remediation ID required")
			return
		}
		var props graph.Properties
		if err := readJSON(r, &props); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		props["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
		node, err := g.UpsertNode(nodeID, "RemediationAction", props)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		compliance.LogEvent(g, "remediation_updated", "", "Remediation updated", string(nodeID))
		writeJSON(w, http.StatusOK, nodeToJSON(node))
	}
}

func handleComplianceEvents(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodes, err := g.NodesByLabel("ComplianceEvent")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Sort by timestamp descending (newest first).
		items := make([]map[string]any, 0, len(nodes))
		for _, n := range nodes {
			items = append(items, nodeToJSON(n))
		}
		// Simple reverse — nodes are created in order so newest is last.
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}

		// Pagination.
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil && v > 0 {
				limit = v
			}
		}
		if len(items) > limit {
			items = items[:limit]
		}

		writeJSON(w, http.StatusOK, map[string]any{"events": items, "total": len(nodes)})
	}
}
