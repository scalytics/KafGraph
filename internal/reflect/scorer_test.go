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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
)

func TestTokenize(t *testing.T) {
	words := tokenize("Hello World hello")
	assert.True(t, words["hello"])
	assert.True(t, words["world"])
	assert.Len(t, words, 2) // "hello" deduped
}

func TestTokenizeEmpty(t *testing.T) {
	words := tokenize("")
	assert.Empty(t, words)
}

func TestJaccardSimilarityIdentical(t *testing.T) {
	a := map[string]bool{"hello": true, "world": true}
	b := map[string]bool{"hello": true, "world": true}
	assert.Equal(t, 1.0, jaccardSimilarity(a, b))
}

func TestJaccardSimilarityDisjoint(t *testing.T) {
	a := map[string]bool{"hello": true}
	b := map[string]bool{"world": true}
	assert.Equal(t, 0.0, jaccardSimilarity(a, b))
}

func TestJaccardSimilarityPartial(t *testing.T) {
	a := map[string]bool{"hello": true, "world": true}
	b := map[string]bool{"hello": true, "there": true}
	// intersection=1, union=3
	assert.InDelta(t, 1.0/3.0, jaccardSimilarity(a, b), 0.001)
}

func TestJaccardSimilarityBothEmpty(t *testing.T) {
	assert.Equal(t, 0.0, jaccardSimilarity(map[string]bool{}, map[string]bool{}))
}

func TestExtractSummaryFromText(t *testing.T) {
	n := &graph.Node{Label: "Message", Properties: graph.Properties{"text": "hello"}}
	assert.Equal(t, "hello", extractSummary(n))
}

func TestExtractSummaryFromContent(t *testing.T) {
	n := &graph.Node{Label: "Signal", Properties: graph.Properties{"content": "insight"}}
	assert.Equal(t, "insight", extractSummary(n))
}

func TestExtractSummaryFallback(t *testing.T) {
	n := &graph.Node{ID: "test-id", Label: "Unknown", Properties: graph.Properties{}}
	assert.Equal(t, "Unknown:test-id", extractSummary(n))
}

func TestExtractSummaryTruncation(t *testing.T) {
	longText := strings.Repeat("abcde ", 50)
	n := &graph.Node{Label: "Message", Properties: graph.Properties{"text": longText}}
	s := extractSummary(n)
	assert.LessOrEqual(t, len(s), 204) // 200 + "..."
}

func TestExtractText(t *testing.T) {
	n := &graph.Node{Properties: graph.Properties{"text": "hello"}}
	assert.Equal(t, "hello", extractText(n))

	n2 := &graph.Node{Properties: graph.Properties{"content": "world"}}
	assert.Equal(t, "world", extractText(n2))

	n3 := &graph.Node{Properties: graph.Properties{}}
	assert.Equal(t, "", extractText(n3))
}

func TestScorerImpactNoEdges(t *testing.T) {
	g := newTestGraph(t)
	sc := NewScorer(g)

	node, _ := g.CreateNode("Message", graph.Properties{"text": "hello"})
	assert.Equal(t, 0.0, sc.impact(node))
}

func TestScorerImpactWithEdges(t *testing.T) {
	g := newTestGraph(t)
	sc := NewScorer(g)

	node, _ := g.CreateNode("Message", graph.Properties{"text": "hello"})
	for range 5 {
		other, _ := g.CreateNode("Message", graph.Properties{"text": "reply"})
		g.CreateEdge("REPLIED_TO", other.ID, node.ID, nil)
	}

	assert.Equal(t, 0.5, sc.impact(node))
}

func TestScorerImpactCappedAt10(t *testing.T) {
	g := newTestGraph(t)
	sc := NewScorer(g)

	node, _ := g.CreateNode("Message", graph.Properties{"text": "popular"})
	for range 15 {
		other, _ := g.CreateNode("Message", graph.Properties{"text": "reply"})
		g.CreateEdge("REPLIED_TO", other.ID, node.ID, nil)
	}

	assert.Equal(t, 1.0, sc.impact(node))
}

func TestScorerRelevanceNilConv(t *testing.T) {
	g := newTestGraph(t)
	sc := NewScorer(g)

	node := &graph.Node{Properties: graph.Properties{"text": "hello"}}
	assert.Equal(t, 0.5, sc.relevance(node, nil))
}

func TestScorerRelevanceMatching(t *testing.T) {
	g := newTestGraph(t)
	sc := NewScorer(g)

	node := &graph.Node{Properties: graph.Properties{"text": "deploy the service"}}
	conv := &graph.Node{Properties: graph.Properties{"description": "service deployment"}}

	r := sc.relevance(node, conv)
	assert.Greater(t, r, 0.0)
	assert.LessOrEqual(t, r, 1.0)
}

func TestScorerRelevanceNoText(t *testing.T) {
	g := newTestGraph(t)
	sc := NewScorer(g)

	node := &graph.Node{Properties: graph.Properties{}}
	conv := &graph.Node{Properties: graph.Properties{"description": "chat"}}
	assert.Equal(t, 0.5, sc.relevance(node, conv))
}

func TestScorerValueContributionNilConv(t *testing.T) {
	g := newTestGraph(t)
	sc := NewScorer(g)
	assert.Equal(t, 0.5, sc.valueContribution(nil))
}

func TestScorerValueContributionWithReplies(t *testing.T) {
	g := newTestGraph(t)
	sc := NewScorer(g)

	conv, _ := g.CreateNode("Conversation", graph.Properties{"description": "chat"})
	msg1, _ := g.CreateNode("Message", graph.Properties{"text": "hello", "inReplyTo": "something"})
	msg2, _ := g.CreateNode("Message", graph.Properties{"text": "world"})

	g.CreateEdge("BELONGS_TO", msg1.ID, conv.ID, nil)
	g.CreateEdge("BELONGS_TO", msg2.ID, conv.ID, nil)

	vc := sc.valueContribution(conv)
	assert.InDelta(t, 0.5, vc, 0.01) // 1 reply out of 2 messages
}

func TestScorerValueContributionNoMessages(t *testing.T) {
	g := newTestGraph(t)
	sc := NewScorer(g)

	conv, _ := g.CreateNode("Conversation", graph.Properties{"description": "empty"})
	assert.Equal(t, 0.5, sc.valueContribution(conv))
}

func TestScoreNodeIntegration(t *testing.T) {
	g := newTestGraph(t)
	sc := NewScorer(g)

	conv, _ := g.CreateNode("Conversation", graph.Properties{"description": "deploy service"})
	msg, _ := g.CreateNode("Message", graph.Properties{"text": "deploy the service now"})
	g.CreateEdge("BELONGS_TO", msg.ID, conv.ID, nil)

	reply, _ := g.CreateNode("Message", graph.Properties{"text": "done", "inReplyTo": string(msg.ID)})
	g.CreateEdge("BELONGS_TO", reply.ID, conv.ID, nil)

	scored := sc.ScoreNode(msg, conv)
	require.Equal(t, msg.ID, scored.NodeID)
	assert.Equal(t, "Message", scored.Label)
	assert.NotEmpty(t, scored.Summary)
	assert.GreaterOrEqual(t, scored.Impact, 0.0)
	assert.LessOrEqual(t, scored.Impact, 1.0)
	assert.GreaterOrEqual(t, scored.Relevance, 0.0)
	assert.LessOrEqual(t, scored.Relevance, 1.0)
	assert.GreaterOrEqual(t, scored.ValueContribution, 0.0)
	assert.LessOrEqual(t, scored.ValueContribution, 1.0)
}
