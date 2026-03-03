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

package reflect

import (
	"fmt"
	"strings"

	"github.com/scalytics/kafgraph/internal/graph"
)

// Scorer computes heuristic scores for graph nodes.
type Scorer struct {
	graph *graph.Graph
}

// NewScorer creates a new heuristic scorer.
func NewScorer(g *graph.Graph) *Scorer {
	return &Scorer{graph: g}
}

// ScoreNode computes impact, relevance, and valueContribution for a node
// relative to its conversation context.
func (sc *Scorer) ScoreNode(node *graph.Node, convNode *graph.Node) ScoredSignal {
	return ScoredSignal{
		NodeID:            node.ID,
		Label:             node.Label,
		Summary:           extractSummary(node),
		Impact:            sc.impact(node),
		Relevance:         sc.relevance(node, convNode),
		ValueContribution: sc.valueContribution(convNode),
	}
}

// impact counts edges from node and normalizes by a cap of 10.
func (sc *Scorer) impact(node *graph.Node) float64 {
	edges, err := sc.graph.Neighbors(node.ID)
	if err != nil {
		return 0.0
	}
	depth := float64(len(edges))
	if depth > 10 {
		depth = 10
	}
	return depth / 10.0
}

// relevance computes Jaccard word-set similarity between message text
// and conversation description.
func (sc *Scorer) relevance(node *graph.Node, convNode *graph.Node) float64 {
	if convNode == nil {
		return 0.5
	}
	nodeText := extractText(node)
	convText := extractText(convNode)
	if nodeText == "" || convText == "" {
		return 0.5
	}
	return jaccardSimilarity(tokenize(nodeText), tokenize(convText))
}

// valueContribution computes the ratio of messages in a conversation
// that received replies to total messages.
func (sc *Scorer) valueContribution(convNode *graph.Node) float64 {
	if convNode == nil {
		return 0.5
	}
	edges, err := sc.graph.Neighbors(convNode.ID)
	if err != nil {
		return 0.5
	}

	var totalMsgs int
	var repliedMsgs int
	for _, edge := range edges {
		targetID := edge.ToID
		if targetID == convNode.ID {
			targetID = edge.FromID
		}
		node, err := sc.graph.GetNode(targetID)
		if err != nil || node.Label != "Message" {
			continue
		}
		totalMsgs++
		if _, ok := node.Properties["inReplyTo"]; ok {
			repliedMsgs++
		}
	}
	if totalMsgs == 0 {
		return 0.5
	}
	return float64(repliedMsgs) / float64(totalMsgs)
}

// tokenize splits text into a word set.
func tokenize(text string) map[string]bool {
	words := make(map[string]bool)
	for w := range strings.FieldsSeq(strings.ToLower(text)) {
		words[w] = true
	}
	return words
}

// jaccardSimilarity computes |A ∩ B| / |A ∪ B|.
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0.0
	}
	intersection := 0
	for w := range a {
		if b[w] {
			intersection++
		}
	}
	union := len(a)
	for w := range b {
		if !a[w] {
			union++
		}
	}
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

// extractSummary returns a human-readable summary from a node's properties.
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

// extractText returns the primary text content of a node.
func extractText(n *graph.Node) string {
	for _, key := range []string{"text", "content", "summary", "description", "value"} {
		if v, ok := n.Properties[key]; ok {
			return fmt.Sprint(v)
		}
	}
	return ""
}
