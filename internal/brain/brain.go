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

package brain

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/search"
)

// ReflectionRunner is the interface for delegating reflection cycles
// to the reflect package. Defined here to avoid import cycles.
type ReflectionRunner interface {
	ExecuteForBrain(ctx context.Context, agentID string, windowHours int) (*ReflectCycleResult, error)
}

// ReflectCycleRequest describes a reflection cycle request from brain tools.
type ReflectCycleRequest struct {
	AgentID     string
	WindowHours int
}

// ReflectCycleResult holds the outcome of a delegated reflection cycle.
type ReflectCycleResult struct {
	CycleID         string
	LearningSignals []NodeSummary
	Summary         string
}

// Service implements the seven brain tools.
type Service struct {
	graph     *graph.Graph
	fullText  search.FullTextSearcher
	vector    search.VectorSearcher
	reflector ReflectionRunner
}

// SetReflectionRunner injects a reflection runner for delegation.
// When set, Reflect() delegates to the runner for deterministic IDs
// and heuristic scoring. When nil, falls back to existing behavior.
func (s *Service) SetReflectionRunner(r ReflectionRunner) {
	s.reflector = r
}

// NewService creates a brain tool service.
func NewService(g *graph.Graph, ft search.FullTextSearcher, vs search.VectorSearcher) *Service {
	return &Service{graph: g, fullText: ft, vector: vs}
}

// --- brain_search ---

// SearchInput is the input for brain_search.
type SearchInput struct {
	Query     string     `json:"query"`
	Scope     string     `json:"scope"`
	Limit     int        `json:"limit"`
	TimeRange *TimeRange `json:"timeRange,omitempty"`
}

// TimeRange constrains queries to a time window.
type TimeRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// SearchResult is a single search hit.
type SearchResult struct {
	NodeID  string         `json:"nodeId"`
	Type    string         `json:"type"`
	Content string         `json:"content"`
	Score   float64        `json:"score"`
	Props   map[string]any `json:"properties,omitempty"`
}

// SearchOutput is the output for brain_search.
type SearchOutput struct {
	Results []SearchResult `json:"results"`
}

// Search performs semantic search across the knowledge graph.
func (s *Service) Search(in SearchInput) (*SearchOutput, error) {
	if in.Limit <= 0 {
		in.Limit = 10
	}

	out := &SearchOutput{Results: []SearchResult{}}

	// Try full-text search across known label/property pairs
	labels := []string{"Message", "SharedMemory", "LearningSignal", "Conversation"}
	propForLabel := map[string]string{
		"Message":        "text",
		"SharedMemory":   "value",
		"LearningSignal": "summary",
		"Conversation":   "description",
	}

	if s.fullText != nil {
		for _, label := range labels {
			prop := propForLabel[label]
			results, err := s.fullText.Search(label, prop, in.Query, in.Limit)
			if err != nil {
				continue
			}
			for _, r := range results {
				node, err := s.graph.GetNode(r.NodeID)
				if err != nil {
					continue
				}
				if in.TimeRange != nil && !inTimeRange(node.CreatedAt, in.TimeRange) {
					continue
				}
				content := fmt.Sprint(node.Properties[prop])
				out.Results = append(out.Results, SearchResult{
					NodeID:  string(r.NodeID),
					Type:    node.Label,
					Content: content,
					Score:   r.Score,
					Props:   map[string]any(node.Properties),
				})
			}
		}
	}

	// Sort by score descending
	sort.Slice(out.Results, func(i, j int) bool {
		return out.Results[i].Score > out.Results[j].Score
	})

	if len(out.Results) > in.Limit {
		out.Results = out.Results[:in.Limit]
	}
	return out, nil
}

// --- brain_recall ---

// RecallInput is the input for brain_recall.
type RecallInput struct {
	AgentID            string `json:"agentId"`
	Depth              string `json:"depth"`
	IncludeTeamContext bool   `json:"includeTeamContext"`
}

// RecallOutput is the output for brain_recall.
type RecallOutput struct {
	Context RecallContext `json:"context"`
}

// RecallContext holds the accumulated agent context.
type RecallContext struct {
	ActiveConversations []NodeSummary `json:"activeConversations"`
	RecentDecisions     []NodeSummary `json:"recentDecisions"`
	PendingFeedback     []NodeSummary `json:"pendingFeedback"`
	UnresolvedThreads   []NodeSummary `json:"unresolvedThreads"`
}

// NodeSummary is a compact node representation.
type NodeSummary struct {
	NodeID    string `json:"nodeId"`
	Type      string `json:"type"`
	Summary   string `json:"summary"`
	Timestamp string `json:"timestamp"`
}

// Recall loads accumulated context for a specific agent.
func (s *Service) Recall(in RecallInput) (*RecallOutput, error) {
	out := &RecallOutput{
		Context: RecallContext{
			ActiveConversations: []NodeSummary{},
			RecentDecisions:     []NodeSummary{},
			PendingFeedback:     []NodeSummary{},
			UnresolvedThreads:   []NodeSummary{},
		},
	}

	agentID := graph.NodeID(in.AgentID)

	// Find conversations connected to this agent
	edges, err := s.graph.Neighbors(agentID)
	if err != nil {
		return out, nil // agent not found → empty context
	}

	limit := 20
	if in.Depth == "shallow" {
		limit = 5
	}

	for _, edge := range edges {
		targetID := edge.ToID
		if targetID == agentID {
			targetID = edge.FromID
		}
		node, err := s.graph.GetNode(targetID)
		if err != nil {
			continue
		}
		summary := summarizeNode(node)

		switch node.Label {
		case "Conversation":
			if len(out.Context.ActiveConversations) < limit {
				out.Context.ActiveConversations = append(out.Context.ActiveConversations, summary)
			}
		case "LearningSignal":
			if len(out.Context.RecentDecisions) < limit {
				out.Context.RecentDecisions = append(out.Context.RecentDecisions, summary)
			}
		case "HumanFeedback":
			if len(out.Context.PendingFeedback) < limit {
				out.Context.PendingFeedback = append(out.Context.PendingFeedback, summary)
			}
		case "Message":
			if len(out.Context.UnresolvedThreads) < limit {
				out.Context.UnresolvedThreads = append(out.Context.UnresolvedThreads, summary)
			}
		}
	}
	return out, nil
}

// --- brain_capture ---

// CaptureInput is the input for brain_capture.
type CaptureInput struct {
	AgentID     string   `json:"agentId"`
	Type        string   `json:"type"`
	Content     string   `json:"content"`
	Tags        []string `json:"tags"`
	LinkedNodes []string `json:"linkedNodes"`
}

// CaptureOutput is the output for brain_capture.
type CaptureOutput struct {
	NodeID   string   `json:"nodeId"`
	LinkedTo []string `json:"linkedTo"`
}

// Capture writes an insight, decision, or observation into the graph.
func (s *Service) Capture(in CaptureInput) (*CaptureOutput, error) {
	if in.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if in.Type == "" {
		in.Type = "observation"
	}

	props := graph.Properties{
		"content":    in.Content,
		"type":       in.Type,
		"agentId":    in.AgentID,
		"tags":       strings.Join(in.Tags, ","),
		"capturedAt": time.Now().Format(time.RFC3339),
	}
	node, err := s.graph.CreateNode("LearningSignal", props)
	if err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}

	// Index for full-text search
	if s.fullText != nil {
		_ = s.fullText.Index(node)
	}

	out := &CaptureOutput{
		NodeID:   string(node.ID),
		LinkedTo: []string{},
	}

	// Link to agent
	if in.AgentID != "" {
		_, err := s.graph.CreateEdge("AUTHORED", graph.NodeID(in.AgentID), node.ID, nil)
		if err == nil {
			out.LinkedTo = append(out.LinkedTo, in.AgentID)
		}
	}

	// Link to specified nodes
	for _, linkedID := range in.LinkedNodes {
		_, err := s.graph.CreateEdge("REFERENCES", node.ID, graph.NodeID(linkedID), nil)
		if err == nil {
			out.LinkedTo = append(out.LinkedTo, linkedID)
		}
	}

	return out, nil
}

// --- brain_recent ---

// RecentInput is the input for brain_recent.
type RecentInput struct {
	AgentID     string   `json:"agentId"`
	WindowHours int      `json:"windowHours"`
	Types       []string `json:"types"`
	Limit       int      `json:"limit"`
}

// ActivityItem is a single recent activity entry.
type ActivityItem struct {
	NodeID    string `json:"nodeId"`
	Type      string `json:"type"`
	Summary   string `json:"summary"`
	Timestamp string `json:"timestamp"`
}

// RecentOutput is the output for brain_recent.
type RecentOutput struct {
	Activity []ActivityItem `json:"activity"`
}

// Recent browses recent activity within a time window.
func (s *Service) Recent(in RecentInput) (*RecentOutput, error) {
	if in.WindowHours <= 0 {
		in.WindowHours = 24
	}
	if in.Limit <= 0 {
		in.Limit = 50
	}

	cutoff := time.Now().Add(-time.Duration(in.WindowHours) * time.Hour)
	out := &RecentOutput{Activity: []ActivityItem{}}

	// Search through requested types (or defaults)
	types := in.Types
	if len(types) == 0 {
		types = []string{"Message", "LearningSignal", "Conversation"}
	}

	for _, label := range types {
		nodes, err := s.graph.NodesByLabel(label)
		if err != nil {
			continue
		}
		for _, node := range nodes {
			if node.CreatedAt.Before(cutoff) {
				continue
			}
			// Filter by agent if specified
			if in.AgentID != "" {
				agentProp, _ := node.Properties["agentId"].(string)
				if agentProp != in.AgentID {
					continue
				}
			}
			out.Activity = append(out.Activity, ActivityItem{
				NodeID:    string(node.ID),
				Type:      node.Label,
				Summary:   extractSummary(node),
				Timestamp: node.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(out.Activity, func(i, j int) bool {
		return out.Activity[i].Timestamp > out.Activity[j].Timestamp
	})

	if len(out.Activity) > in.Limit {
		out.Activity = out.Activity[:in.Limit]
	}
	return out, nil
}

// --- brain_patterns ---

// PatternsInput is the input for brain_patterns.
type PatternsInput struct {
	AgentID        string     `json:"agentId"`
	Scope          string     `json:"scope"`
	MinOccurrences int        `json:"minOccurrences"`
	TimeRange      *TimeRange `json:"timeRange,omitempty"`
}

// PatternItem is a detected pattern.
type PatternItem struct {
	Theme        string   `json:"theme"`
	Occurrences  int      `json:"occurrences"`
	RelatedNodes []string `json:"relatedNodes"`
	Trend        string   `json:"trend"`
}

// PatternsOutput is the output for brain_patterns.
type PatternsOutput struct {
	Patterns []PatternItem `json:"patterns"`
}

// Patterns surfaces recurring themes from the graph.
func (s *Service) Patterns(in PatternsInput) (*PatternsOutput, error) {
	if in.MinOccurrences <= 0 {
		in.MinOccurrences = 3
	}

	out := &PatternsOutput{Patterns: []PatternItem{}}

	// Aggregate tags from LearningSignal nodes
	tagCounts := map[string][]string{} // tag → list of node IDs
	signals, err := s.graph.NodesByLabel("LearningSignal")
	if err != nil {
		return out, nil
	}
	for _, node := range signals {
		if in.TimeRange != nil && !inTimeRange(node.CreatedAt, in.TimeRange) {
			continue
		}
		tagsStr, _ := node.Properties["tags"].(string)
		if tagsStr == "" {
			continue
		}
		for _, tag := range strings.Split(tagsStr, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tagCounts[tag] = append(tagCounts[tag], string(node.ID))
			}
		}
	}

	for tag, nodeIDs := range tagCounts {
		if len(nodeIDs) >= in.MinOccurrences {
			out.Patterns = append(out.Patterns, PatternItem{
				Theme:        tag,
				Occurrences:  len(nodeIDs),
				RelatedNodes: nodeIDs,
				Trend:        "stable",
			})
		}
	}

	// Sort by occurrences descending
	sort.Slice(out.Patterns, func(i, j int) bool {
		return out.Patterns[i].Occurrences > out.Patterns[j].Occurrences
	})
	return out, nil
}

// --- brain_reflect ---

// ReflectInput is the input for brain_reflect.
type ReflectInput struct {
	AgentID     string `json:"agentId"`
	Scope       string `json:"scope"`
	WindowHours int    `json:"windowHours"`
}

// ReflectOutput is the output for brain_reflect.
type ReflectOutput struct {
	CycleID             string        `json:"cycleId"`
	LearningSignals     []NodeSummary `json:"learningSignals"`
	Summary             string        `json:"summary"`
	HumanFeedbackStatus string        `json:"humanFeedbackStatus"`
}

// Reflect triggers an on-demand reflection cycle.
func (s *Service) Reflect(in ReflectInput) (*ReflectOutput, error) {
	if in.WindowHours <= 0 {
		in.WindowHours = 24
	}

	// Delegate to reflection runner if available
	if s.reflector != nil {
		result, err := s.reflector.ExecuteForBrain(context.Background(), in.AgentID, in.WindowHours)
		if err != nil {
			return nil, fmt.Errorf("reflection runner: %w", err)
		}
		return &ReflectOutput{
			CycleID:             result.CycleID,
			LearningSignals:     result.LearningSignals,
			Summary:             result.Summary,
			HumanFeedbackStatus: "PENDING",
		}, nil
	}

	// Fallback: original behavior (no deterministic IDs, no scoring)

	// Create ReflectionCycle node
	cycleProps := graph.Properties{
		"agentId":     in.AgentID,
		"scope":       in.Scope,
		"windowHours": in.WindowHours,
		"startedAt":   time.Now().Format(time.RFC3339),
		"status":      "PENDING",
	}
	cycle, err := s.graph.CreateNode("ReflectionCycle", cycleProps)
	if err != nil {
		return nil, fmt.Errorf("create reflection cycle: %w", err)
	}

	// Link to agent
	if in.AgentID != "" {
		s.graph.CreateEdge("TRIGGERED_REFLECTION", graph.NodeID(in.AgentID), cycle.ID, nil) //nolint:errcheck,gosec // best-effort linking
	}

	// Gather recent learning signals
	cutoff := time.Now().Add(-time.Duration(in.WindowHours) * time.Hour)
	signals, _ := s.graph.NodesByLabel("LearningSignal")
	var summaries []NodeSummary
	var summaryParts []string

	for _, sig := range signals {
		if sig.CreatedAt.Before(cutoff) {
			continue
		}
		summary := summarizeNode(sig)
		summaries = append(summaries, summary)
		summaryParts = append(summaryParts, summary.Summary)

		// Link signal to cycle
		s.graph.CreateEdge("LINKS_TO", sig.ID, cycle.ID, nil) //nolint:errcheck,gosec // best-effort linking
	}

	if summaries == nil {
		summaries = []NodeSummary{}
	}

	cycleSummary := "No learning signals in window."
	if len(summaryParts) > 0 {
		cycleSummary = fmt.Sprintf("Found %d learning signals: %s",
			len(summaryParts), strings.Join(summaryParts, "; "))
	}

	return &ReflectOutput{
		CycleID:             string(cycle.ID),
		LearningSignals:     summaries,
		Summary:             cycleSummary,
		HumanFeedbackStatus: "PENDING",
	}, nil
}

// --- brain_feedback ---

// FeedbackInput is the input for brain_feedback.
type FeedbackInput struct {
	CycleID      string         `json:"cycleId"`
	FeedbackType string         `json:"feedbackType"`
	Scores       FeedbackScores `json:"scores"`
	Comment      string         `json:"comment"`
}

// FeedbackScores holds feedback scoring dimensions.
type FeedbackScores struct {
	Impact            float64 `json:"impact"`
	Relevance         float64 `json:"relevance"`
	ValueContribution float64 `json:"valueContribution"`
}

// FeedbackOutput is the output for brain_feedback.
type FeedbackOutput struct {
	FeedbackID  string `json:"feedbackId"`
	CycleStatus string `json:"cycleStatus"`
}

// Feedback submits human feedback on a reflection cycle.
func (s *Service) Feedback(in FeedbackInput) (*FeedbackOutput, error) {
	if in.CycleID == "" {
		return nil, fmt.Errorf("cycleId is required")
	}

	// Verify cycle exists
	_, err := s.graph.GetNode(graph.NodeID(in.CycleID))
	if err != nil {
		return nil, fmt.Errorf("reflection cycle not found: %s", in.CycleID)
	}

	props := graph.Properties{
		"cycleId":           in.CycleID,
		"feedbackType":      in.FeedbackType,
		"impact":            in.Scores.Impact,
		"relevance":         in.Scores.Relevance,
		"valueContribution": in.Scores.ValueContribution,
		"comment":           in.Comment,
		"submittedAt":       time.Now().Format(time.RFC3339),
	}
	fb, err := s.graph.CreateNode("HumanFeedback", props)
	if err != nil {
		return nil, fmt.Errorf("create feedback: %w", err)
	}

	// Link feedback to cycle
	s.graph.CreateEdge("HAS_FEEDBACK", graph.NodeID(in.CycleID), fb.ID, nil) //nolint:errcheck,gosec // best-effort linking

	// Update cycle status to RECEIVED
	s.graph.UpsertNode(graph.NodeID(in.CycleID), "ReflectionCycle", graph.Properties{ //nolint:errcheck,gosec // best-effort
		"humanFeedbackStatus": "RECEIVED",
	})

	// Apply score overrides to linked signals
	s.applyScoreOverrides(in.CycleID, in.Scores)

	return &FeedbackOutput{
		FeedbackID:  string(fb.ID),
		CycleStatus: "RECEIVED",
	}, nil
}

// applyScoreOverrides updates LINKS_TO edge properties for signals linked to a cycle.
func (s *Service) applyScoreOverrides(cycleID string, scores FeedbackScores) {
	edges, err := s.graph.Neighbors(graph.NodeID(cycleID))
	if err != nil {
		return
	}
	for _, edge := range edges {
		if edge.Label != "LINKS_TO" || edge.ToID != graph.NodeID(cycleID) {
			continue
		}
		props := graph.Properties{}
		if scores.Impact != 0 {
			props["impact"] = scores.Impact
		}
		if scores.Relevance != 0 {
			props["relevance"] = scores.Relevance
		}
		if scores.ValueContribution != 0 {
			props["valueContribution"] = scores.ValueContribution
		}
		if len(props) > 0 {
			s.graph.UpsertEdge(edge.ID, edge.Label, edge.FromID, edge.ToID, props) //nolint:errcheck,gosec // best-effort
		}
	}
}

// --- Helpers ---

func summarizeNode(n *graph.Node) NodeSummary {
	return NodeSummary{
		NodeID:    string(n.ID),
		Type:      n.Label,
		Summary:   extractSummary(n),
		Timestamp: n.CreatedAt.Format(time.RFC3339),
	}
}

func extractSummary(n *graph.Node) string {
	for _, key := range []string{"summary", "content", "text", "description", "value", "name"} {
		if v, ok := n.Properties[key]; ok {
			s := fmt.Sprint(v)
			if len(s) > 200 {
				return s[:200] + "..."
			}
			return s
		}
	}
	return n.Label + ":" + string(n.ID)
}

func inTimeRange(t time.Time, tr *TimeRange) bool {
	if !tr.From.IsZero() && t.Before(tr.From) {
		return false
	}
	if !tr.To.IsZero() && t.After(tr.To) {
		return false
	}
	return true
}
