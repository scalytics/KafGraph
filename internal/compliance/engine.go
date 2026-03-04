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

package compliance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
)

// Engine manages compliance rules and runs scans, storing results in the graph.
type Engine struct {
	graph   *graph.Graph
	rules   []Rule
	mu      sync.RWMutex
	scanSeq int
}

// NewEngine creates a compliance engine backed by the given graph.
func NewEngine(g *graph.Graph) *Engine {
	return &Engine{graph: g}
}

// RegisterRule adds a rule to the engine.
func (e *Engine) RegisterRule(r Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, r)
}

// Rules returns a copy of all registered rules.
func (e *Engine) Rules() []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Rule, len(e.rules))
	copy(out, e.rules)
	return out
}

// RulesByFramework returns rules filtered by framework.
func (e *Engine) RulesByFramework(fw Framework) []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var out []Rule
	for _, r := range e.rules {
		if r.Framework() == fw {
			out = append(out, r)
		}
	}
	return out
}

// RunScan evaluates all matching rules and stores results as graph nodes.
func (e *Engine) RunScan(ctx context.Context, req ScanRequest) (*ScanResult, error) {
	e.mu.Lock()
	e.scanSeq++
	scanID := fmt.Sprintf("scan-%d", e.scanSeq)
	e.mu.Unlock()

	start := time.Now().UTC()
	querier := &graphAdapter{g: e.graph}

	// Filter rules by request.
	e.mu.RLock()
	var rules []Rule
	for _, r := range e.rules {
		if req.Framework != "" && r.Framework() != req.Framework {
			continue
		}
		if req.Module != "" && r.Module() != req.Module {
			continue
		}
		rules = append(rules, r)
	}
	e.mu.RUnlock()

	var allResults []RuleResult
	for _, r := range rules {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		results, err := r.Evaluate(querier)
		if err != nil {
			allResults = append(allResults, RuleResult{
				RuleID:   r.ID(),
				Status:   EvalWarning,
				Details:  fmt.Sprintf("evaluation error: %v", err),
				Severity: r.Severity(),
			})
			continue
		}
		if len(results) == 0 {
			allResults = append(allResults, RuleResult{
				RuleID:   r.ID(),
				Status:   EvalNA,
				Details:  "no applicable data",
				Severity: r.Severity(),
			})
			continue
		}
		allResults = append(allResults, results...)
	}

	completed := time.Now().UTC()

	result := &ScanResult{
		ScanID:      scanID,
		Framework:   req.Framework,
		TriggeredBy: "api",
		StartedAt:   start,
		CompletedAt: completed,
		Evaluations: allResults,
	}

	// Count results.
	for _, ev := range allResults {
		switch ev.Status {
		case EvalPass:
			result.PassCount++
		case EvalFail:
			result.FailCount++
		case EvalWarning:
			result.WarningCount++
		case EvalNA:
			result.NACount++
		}
	}

	result.Score = CalculateScore(allResults)

	// Store scan results in the graph.
	if err := e.storeScanResults(result); err != nil {
		return result, fmt.Errorf("store scan results: %w", err)
	}

	return result, nil
}

// storeScanResults persists a ScanResult as graph nodes and edges.
func (e *Engine) storeScanResults(result *ScanResult) error {
	// Create ComplianceScan node.
	scanNode, err := e.graph.CreateNode("ComplianceScan", graph.Properties{
		"scanId":      result.ScanID,
		"framework":   string(result.Framework),
		"triggeredBy": result.TriggeredBy,
		"startedAt":   result.StartedAt.Format(time.RFC3339),
		"completedAt": result.CompletedAt.Format(time.RFC3339),
		"passCount":   result.PassCount,
		"failCount":   result.FailCount,
		"warningCount": result.WarningCount,
		"score":       result.Score,
	})
	if err != nil {
		return fmt.Errorf("create scan node: %w", err)
	}

	// Link scan to framework node if it exists.
	fwNodes, _ := e.graph.NodesByLabel("ComplianceFramework")
	for _, fn := range fwNodes {
		if name, ok := fn.Properties["name"].(string); ok && Framework(name) == result.Framework {
			_, _ = e.graph.CreateEdge("SCOPED_TO", scanNode.ID, fn.ID, nil)
			break
		}
	}

	// Create evaluation nodes.
	for _, ev := range result.Evaluations {
		evalNode, err := e.graph.CreateNode("ComplianceEvaluation", graph.Properties{
			"ruleId":      ev.RuleID,
			"status":      string(ev.Status),
			"evaluatedAt": result.CompletedAt.Format(time.RFC3339),
			"details":     ev.Details,
			"severity":    string(ev.Severity),
			"nodeId":      ev.NodeID,
		})
		if err != nil {
			continue
		}
		// PART_OF_SCAN edge
		_, _ = e.graph.CreateEdge("PART_OF_SCAN", evalNode.ID, scanNode.ID, nil)

		// EVALUATED_BY edge to ComplianceRule node (if exists).
		ruleNodes, _ := e.graph.NodesByLabel("ComplianceRule")
		for _, rn := range ruleNodes {
			if rID, ok := rn.Properties["ruleId"].(string); ok && rID == ev.RuleID {
				_, _ = e.graph.CreateEdge("EVALUATED_BY", rn.ID, evalNode.ID, nil)
				break
			}
		}
	}

	return nil
}

// EnsureFrameworkNodes creates ComplianceFramework and ComplianceRule nodes
// for all registered rules (idempotent via label+property checks).
func (e *Engine) EnsureFrameworkNodes() error {
	e.mu.RLock()
	rules := make([]Rule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	// Collect unique frameworks.
	frameworks := map[Framework]bool{}
	for _, r := range rules {
		frameworks[r.Framework()] = true
	}

	// Create framework nodes.
	for fw := range frameworks {
		existing, _ := e.graph.NodesByLabel("ComplianceFramework")
		found := false
		for _, n := range existing {
			if name, ok := n.Properties["name"].(string); ok && Framework(name) == fw {
				found = true
				break
			}
		}
		if !found {
			_, err := e.graph.CreateNode("ComplianceFramework", graph.Properties{
				"name":    string(fw),
				"version": "1.0",
				"status":  "active",
			})
			if err != nil {
				return fmt.Errorf("create framework node %s: %w", fw, err)
			}
		}
	}

	// Create rule nodes.
	existingRules, _ := e.graph.NodesByLabel("ComplianceRule")
	ruleIDs := map[string]bool{}
	for _, n := range existingRules {
		if rID, ok := n.Properties["ruleId"].(string); ok {
			ruleIDs[rID] = true
		}
	}

	for _, r := range rules {
		if ruleIDs[r.ID()] {
			continue
		}
		ruleNode, err := e.graph.CreateNode("ComplianceRule", graph.Properties{
			"ruleId":    r.ID(),
			"framework": string(r.Framework()),
			"module":    r.Module(),
			"article":   r.Article(),
			"title":     r.Title(),
			"severity":  string(r.Severity()),
		})
		if err != nil {
			continue
		}

		// Link to framework.
		fwNodes, _ := e.graph.NodesByLabel("ComplianceFramework")
		for _, fn := range fwNodes {
			if name, ok := fn.Properties["name"].(string); ok && Framework(name) == r.Framework() {
				_, _ = e.graph.CreateEdge("DEFINES_RULE", fn.ID, ruleNode.ID, nil)
				break
			}
		}
	}

	return nil
}

// graphAdapter wraps *graph.Graph to satisfy GraphQuerier.
type graphAdapter struct {
	g *graph.Graph
}

func (a *graphAdapter) NodesByLabel(label string) (NodeList, error) {
	nodes, err := a.g.NodesByLabel(label)
	if err != nil {
		return nil, err
	}
	out := make(NodeList, len(nodes))
	for i, n := range nodes {
		out[i] = NodeItem{
			ID:         string(n.ID),
			Label:      n.Label,
			Properties: n.Properties,
		}
	}
	return out, nil
}

func (a *graphAdapter) Neighbors(id string) (EdgeList, error) {
	edges, err := a.g.Neighbors(graph.NodeID(id))
	if err != nil {
		return nil, err
	}
	out := make(EdgeList, len(edges))
	for i, e := range edges {
		out[i] = EdgeItem{
			ID:    string(e.ID),
			Label: e.Label,
			From:  string(e.FromID),
			To:    string(e.ToID),
		}
	}
	return out, nil
}
