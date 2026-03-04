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

	"github.com/scalytics/kafgraph/internal/compliance"
	"github.com/scalytics/kafgraph/internal/graph"
)

// registerComplianceRoutes wires all compliance API routes onto the mux.
func registerComplianceRoutes(mux *http.ServeMux, g *graph.Graph, engine *compliance.Engine) {
	if engine == nil {
		return
	}

	mux.HandleFunc("GET /api/v2/compliance/frameworks", handleComplianceFrameworks(g))
	mux.HandleFunc("GET /api/v2/compliance/rules", handleComplianceRules(engine))
	mux.HandleFunc("POST /api/v2/compliance/scan", handleComplianceScan(engine))
	mux.HandleFunc("GET /api/v2/compliance/scans", handleComplianceScans(g))
	mux.HandleFunc("GET /api/v2/compliance/scans/", handleComplianceScanDetail(g))
	mux.HandleFunc("GET /api/v2/compliance/score", handleComplianceScore(g, engine))
	mux.HandleFunc("GET /api/v2/compliance/dashboard", handleComplianceDashboard(g, engine))

	// Register GDPR-specific CRUD routes
	registerComplianceGDPRRoutes(mux, g)

	// Register inspection and data flow routes
	registerComplianceInspectionRoutes(mux, g)
	registerComplianceDataFlowRoutes(mux, g)
}

func handleComplianceFrameworks(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		nodes, err := g.NodesByLabel("ComplianceFramework")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items := make([]map[string]any, 0, len(nodes))
		for _, n := range nodes {
			items = append(items, nodeToJSON(n))
		}
		writeJSON(w, http.StatusOK, map[string]any{"frameworks": items})
	}
}

func handleComplianceRules(engine *compliance.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fw := r.URL.Query().Get("framework")
		mod := r.URL.Query().Get("module")

		rules := engine.Rules()
		type ruleJSON struct {
			ID        string `json:"id"`
			Framework string `json:"framework"`
			Module    string `json:"module"`
			Article   string `json:"article"`
			Title     string `json:"title"`
			Severity  string `json:"severity"`
		}
		var items []ruleJSON
		for _, rule := range rules {
			if fw != "" && string(rule.Framework()) != fw {
				continue
			}
			if mod != "" && rule.Module() != mod {
				continue
			}
			items = append(items, ruleJSON{
				ID:        rule.ID(),
				Framework: string(rule.Framework()),
				Module:    rule.Module(),
				Article:   rule.Article(),
				Title:     rule.Title(),
				Severity:  string(rule.Severity()),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"rules": items, "total": len(items)})
	}
}

func handleComplianceScan(engine *compliance.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req compliance.ScanRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := engine.RunScan(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleComplianceScans(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		nodes, err := g.NodesByLabel("ComplianceScan")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items := make([]map[string]any, 0, len(nodes))
		for _, n := range nodes {
			items = append(items, nodeToJSON(n))
		}
		writeJSON(w, http.StatusOK, map[string]any{"scans": items, "total": len(items)})
	}
}

func handleComplianceScanDetail(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract scanId from path: /api/v2/compliance/scans/{scanId}
		path := r.URL.Path
		scanID := strings.TrimPrefix(path, "/api/v2/compliance/scans/")
		if scanID == "" {
			writeError(w, http.StatusBadRequest, "scan ID required")
			return
		}

		// Find the scan node.
		scans, err := g.NodesByLabel("ComplianceScan")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var scanNode *graph.Node
		for _, s := range scans {
			if sID, ok := s.Properties["scanId"].(string); ok && sID == scanID {
				scanNode = s
				break
			}
		}
		if scanNode == nil {
			writeError(w, http.StatusNotFound, "scan not found")
			return
		}

		// Collect evaluations linked to this scan via PART_OF_SCAN edges.
		evals, _ := g.NodesByLabel("ComplianceEvaluation")
		var scanEvals []map[string]any
		for _, ev := range evals {
			edges, _ := g.Neighbors(ev.ID)
			for _, edge := range edges {
				if edge.Label == "PART_OF_SCAN" && edge.ToID == scanNode.ID {
					scanEvals = append(scanEvals, nodeToJSON(ev))
					break
				}
			}
		}

		result := nodeToJSON(scanNode)
		result["evaluations"] = scanEvals
		writeJSON(w, http.StatusOK, result)
	}
}

func handleComplianceScore(g *graph.Graph, engine *compliance.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// Get latest scan per framework.
		scans, _ := g.NodesByLabel("ComplianceScan")
		latest := map[string]map[string]any{}
		for _, s := range scans {
			fw, _ := s.Properties["framework"].(string)
			if fw == "" {
				continue
			}
			existing, found := latest[fw]
			if !found {
				latest[fw] = nodeToJSON(s)
				continue
			}
			// Compare completedAt.
			newTime, _ := s.Properties["completedAt"].(string)
			oldTime, _ := existing["completedAt"].(string)
			if newTime > oldTime {
				latest[fw] = nodeToJSON(s)
			}
		}

		var scores []map[string]any
		for fw, scan := range latest {
			scores = append(scores, map[string]any{
				"framework":  fw,
				"score":      scan["score"],
				"passCount":  scan["passCount"],
				"failCount":  scan["failCount"],
				"lastScanAt": scan["completedAt"],
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"scores": scores})
	}
}

func handleComplianceDashboard(g *graph.Graph, engine *compliance.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// Frameworks.
		fwNodes, _ := g.NodesByLabel("ComplianceFramework")
		frameworks := make([]map[string]any, 0, len(fwNodes))
		for _, n := range fwNodes {
			frameworks = append(frameworks, nodeToJSON(n))
		}

		// Latest scan.
		scans, _ := g.NodesByLabel("ComplianceScan")
		var latestScan map[string]any
		var latestTime string
		for _, s := range scans {
			t, _ := s.Properties["completedAt"].(string)
			if t > latestTime {
				latestTime = t
				latestScan = nodeToJSON(s)
			}
		}

		// Module scores from latest scan evaluations.
		moduleScores := map[string]map[string]int{}
		if latestScan != nil {
			evals, _ := g.NodesByLabel("ComplianceEvaluation")
			for _, ev := range evals {
				ruleID, _ := ev.Properties["ruleId"].(string)
				status, _ := ev.Properties["status"].(string)
				// Derive module from rule ID prefix (e.g., GDPR-ROPA-001 → ropa).
				mod := moduleFromRuleID(ruleID)
				if moduleScores[mod] == nil {
					moduleScores[mod] = map[string]int{}
				}
				moduleScores[mod][status]++
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"frameworks":   frameworks,
			"latestScan":   latestScan,
			"totalRules":   len(engine.Rules()),
			"moduleScores": moduleScores,
		})
	}
}

func moduleFromRuleID(ruleID string) string {
	// e.g., "GDPR-ROPA-001" → "ropa", "GDPR-SETUP-001" → "setup"
	parts := strings.Split(ruleID, "-")
	if len(parts) >= 2 {
		return strings.ToLower(parts[1])
	}
	return "unknown"
}
