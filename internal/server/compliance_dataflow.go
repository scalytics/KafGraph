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

	"github.com/scalytics/kafgraph/internal/compliance"
	"github.com/scalytics/kafgraph/internal/graph"
)

// registerComplianceDataFlowRoutes wires data flow CRUD and validation routes.
func registerComplianceDataFlowRoutes(mux *http.ServeMux, g *graph.Graph) {
	base := "/api/v2/compliance/gdpr/data-flows"

	mux.HandleFunc("GET "+base, handleDataFlowList(g))
	mux.HandleFunc("POST "+base, handleDataFlowCreate(g))
	mux.HandleFunc("GET "+base+"/map", handleDataFlowMap(g))
	mux.HandleFunc("POST "+base+"/validate", handleDataFlowValidate(g))
	mux.HandleFunc("GET "+base+"/", handleDataFlowDetail(g, base+"/"))
	mux.HandleFunc("PUT "+base+"/", handleDataFlowUpdate(g, base+"/"))
	mux.HandleFunc("DELETE "+base+"/", handleDataFlowDelete(g, base+"/"))
}

func handleDataFlowList(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		nodes, err := g.NodesByLabel("DataFlow")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items := make([]map[string]any, 0, len(nodes))
		for _, n := range nodes {
			item := nodeToJSON(n)
			// Enrich with edge summaries.
			edges, _ := g.Neighbors(n.ID)
			var from, to, categories []string
			for _, e := range edges {
				switch e.Label {
				case "FROM_ACTIVITY":
					if an, err := g.GetNode(e.ToID); err == nil {
						name, _ := an.Properties["name"].(string)
						from = append(from, name)
					}
				case "TO_ACTIVITY", "TO_PROCESSOR":
					if tn, err := g.GetNode(e.ToID); err == nil {
						name, _ := tn.Properties["name"].(string)
						to = append(to, name)
					}
				case "CARRIES":
					if cn, err := g.GetNode(e.ToID); err == nil {
						name, _ := cn.Properties["name"].(string)
						categories = append(categories, name)
					}
				}
			}
			item["fromNames"] = from
			item["toNames"] = to
			item["categoryNames"] = categories
			items = append(items, item)
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	}
}

func handleDataFlowCreate(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Props         map[string]any `json:"properties"`
			FromActivity  string         `json:"fromActivityId"`
			ToActivity    string         `json:"toActivityId"`
			ToProcessor   string         `json:"toProcessorId"`
			CategoryIDs   []string       `json:"categoryIds"`
			LegalBasisID  string         `json:"legalBasisId"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if body.Props == nil {
			body.Props = map[string]any{}
		}
		body.Props["createdAt"] = time.Now().UTC().Format(time.RFC3339)

		node, err := g.CreateNode("DataFlow", graph.Properties(body.Props))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create edges.
		if body.FromActivity != "" {
			_, _ = g.CreateEdge("FROM_ACTIVITY", node.ID, graph.NodeID(body.FromActivity), nil)
		}
		if body.ToActivity != "" {
			_, _ = g.CreateEdge("TO_ACTIVITY", node.ID, graph.NodeID(body.ToActivity), nil)
		}
		if body.ToProcessor != "" {
			_, _ = g.CreateEdge("TO_PROCESSOR", node.ID, graph.NodeID(body.ToProcessor), nil)
		}
		for _, catID := range body.CategoryIDs {
			_, _ = g.CreateEdge("CARRIES", node.ID, graph.NodeID(catID), nil)
		}
		if body.LegalBasisID != "" {
			_, _ = g.CreateEdge("GOVERNED_BY", node.ID, graph.NodeID(body.LegalBasisID), nil)
		}

		compliance.LogEvent(g, "dataflow_created", "", "DataFlow created: "+string(node.ID), string(node.ID))
		writeJSON(w, http.StatusCreated, nodeToJSON(node))
	}
}

func handleDataFlowDetail(g *graph.Graph, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := graph.NodeID(strings.TrimPrefix(r.URL.Path, prefix))
		if nodeID == "" {
			writeError(w, http.StatusBadRequest, "data flow ID required")
			return
		}
		node, err := g.GetNode(nodeID)
		if err != nil {
			writeError(w, http.StatusNotFound, "data flow not found")
			return
		}
		result := nodeToJSON(node)

		edges, _ := g.Neighbors(nodeID)
		edgeList := make([]map[string]any, 0, len(edges))
		for _, e := range edges {
			edgeList = append(edgeList, edgeToJSON(e))
		}
		result["edges"] = edgeList

		writeJSON(w, http.StatusOK, result)
	}
}

func handleDataFlowUpdate(g *graph.Graph, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := graph.NodeID(strings.TrimPrefix(r.URL.Path, prefix))
		if nodeID == "" {
			writeError(w, http.StatusBadRequest, "data flow ID required")
			return
		}
		var props graph.Properties
		if err := readJSON(r, &props); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		props["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
		node, err := g.UpsertNode(nodeID, "DataFlow", props)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, nodeToJSON(node))
	}
}

func handleDataFlowDelete(g *graph.Graph, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := graph.NodeID(strings.TrimPrefix(r.URL.Path, prefix))
		if nodeID == "" {
			writeError(w, http.StatusBadRequest, "data flow ID required")
			return
		}
		if err := g.DeleteNode(nodeID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func handleDataFlowMap(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		flows, _ := g.NodesByLabel("DataFlow")
		activities, _ := g.NodesByLabel("ProcessingActivity")
		processors, _ := g.NodesByLabel("DataProcessor")

		// Build nodes for the map.
		type mapNode struct {
			ID    string `json:"id"`
			Label string `json:"label"`
			Name  string `json:"name"`
			Type  string `json:"type"`
		}
		type mapEdge struct {
			From         string `json:"from"`
			To           string `json:"to"`
			FlowName     string `json:"flowName"`
			TransferType string `json:"transferType"`
		}

		nodeMap := map[string]bool{}
		var nodes []mapNode
		var edges []mapEdge

		// Add activities.
		for _, a := range activities {
			name, _ := a.Properties["name"].(string)
			nodes = append(nodes, mapNode{ID: string(a.ID), Label: a.Label, Name: name, Type: "activity"})
			nodeMap[string(a.ID)] = true
		}
		// Add processors.
		for _, p := range processors {
			name, _ := p.Properties["name"].(string)
			nodes = append(nodes, mapNode{ID: string(p.ID), Label: p.Label, Name: name, Type: "processor"})
			nodeMap[string(p.ID)] = true
		}

		// Build edges from flows.
		for _, f := range flows {
			fEdges, _ := g.Neighbors(f.ID)
			flowName, _ := f.Properties["name"].(string)
			transferType, _ := f.Properties["transferType"].(string)
			var fromID string
			var toIDs []string
			for _, e := range fEdges {
				switch e.Label {
				case "FROM_ACTIVITY":
					fromID = string(e.ToID)
				case "TO_ACTIVITY", "TO_PROCESSOR":
					toIDs = append(toIDs, string(e.ToID))
				}
			}
			for _, toID := range toIDs {
				edges = append(edges, mapEdge{From: fromID, To: toID, FlowName: flowName, TransferType: transferType})
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"nodes": nodes,
			"edges": edges,
		})
	}
}

func handleDataFlowValidate(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			InspectionID string `json:"inspectionId"`
		}
		// Body is optional.
		_ = readJSON(r, &body)

		results, err := compliance.ValidateDataFlows(g, body.InspectionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Summarize.
		pass, fail, warn := 0, 0, 0
		for _, r := range results {
			switch r.Overall {
			case compliance.EvalPass:
				pass++
			case compliance.EvalFail:
				fail++
			case compliance.EvalWarning:
				warn++
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"results":  results,
			"total":    len(results),
			"pass":     pass,
			"fail":     fail,
			"warnings": warn,
		})
	}
}
