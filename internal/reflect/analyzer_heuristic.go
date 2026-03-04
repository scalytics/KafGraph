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
	"sort"
	"strings"

	"github.com/scalytics/kafgraph/internal/graph"
)

// HeuristicAnalyzer is a pure-Go Analyzer that uses dictionary matching
// and TF-IDF for entity recognition, keyword extraction, auto-tagging,
// summary synthesis, pattern detection, and trend computation.
type HeuristicAnalyzer struct {
	graph      *graph.Graph
	agentNames map[string]bool // lowercase → true
	skillNames map[string]bool // lowercase → true
	corpus     *TFIDFCorpus
}

// NewHeuristicAnalyzer creates an analyzer and loads known entity names from the graph.
func NewHeuristicAnalyzer(g *graph.Graph) *HeuristicAnalyzer {
	ha := &HeuristicAnalyzer{
		graph:      g,
		agentNames: make(map[string]bool),
		skillNames: make(map[string]bool),
		corpus:     NewTFIDFCorpus(),
	}
	ha.RefreshKnowledge()
	return ha
}

// RefreshKnowledge reloads known agent and skill names from the graph.
// Call at the start of each reflection cycle.
func (ha *HeuristicAnalyzer) RefreshKnowledge() {
	ha.agentNames = make(map[string]bool)
	ha.skillNames = make(map[string]bool)

	agents, err := ha.graph.NodesByLabel("Agent")
	if err == nil {
		for _, n := range agents {
			if name, ok := n.Properties["name"].(string); ok && name != "" {
				ha.agentNames[strings.ToLower(name)] = true
			}
		}
	}

	skills, err := ha.graph.NodesByLabel("Skill")
	if err == nil {
		for _, n := range skills {
			if name, ok := n.Properties["name"].(string); ok && name != "" {
				ha.skillNames[strings.ToLower(name)] = true
			}
		}
	}
}

// SetCorpus sets the TF-IDF corpus for keyword extraction.
// CycleRunner builds the corpus from all window nodes and injects it here.
func (ha *HeuristicAnalyzer) SetCorpus(c *TFIDFCorpus) {
	ha.corpus = c
}

// AnalyzeText extracts entities, keywords, and tags from text.
func (ha *HeuristicAnalyzer) AnalyzeText(text string) AnalysisResult {
	if text == "" {
		return AnalysisResult{}
	}

	entities := ha.recognizeEntities(text)
	keywords := ha.corpus.TopKeywords(text, 10)
	tags := ha.autoTag(entities, keywords)

	return AnalysisResult{
		Entities: entities,
		Keywords: keywords,
		Tags:     tags,
	}
}

// SummarizeSignals produces a structured summary grouped by label.
func (ha *HeuristicAnalyzer) SummarizeSignals(signals []ScoredSignal) string {
	if len(signals) == 0 {
		return "No activity in window."
	}

	// Group by label.
	groups := make(map[string]int)
	for _, sig := range signals {
		groups[sig.Label]++
	}

	// Build group description.
	var parts []string
	for label, count := range groups {
		parts = append(parts, fmt.Sprintf("%d %ss", count, strings.ToLower(label)))
	}
	sort.Strings(parts)

	// Collect all entities and keywords across signals.
	entityCounts := make(map[string]int)
	keywordCounts := make(map[string]int)
	for _, sig := range signals {
		for _, e := range sig.Entities {
			entityCounts[e.Name]++
		}
		for _, k := range sig.Keywords {
			keywordCounts[k.Term]++
		}
	}

	topEntities := topN(entityCounts, 3)
	topKeywords := topN(keywordCounts, 3)

	summary := fmt.Sprintf("Analyzed %d signals (%s)", len(signals), strings.Join(parts, ", "))

	if len(topKeywords) > 0 {
		summary += fmt.Sprintf(": key themes are %s", strings.Join(topKeywords, ", "))
	}
	if len(topEntities) > 0 {
		summary += fmt.Sprintf(". Top entities: %s", strings.Join(topEntities, ", "))
	}
	summary += "."

	return summary
}

// DetectPatterns finds co-occurring entities and recurring keyword themes.
func (ha *HeuristicAnalyzer) DetectPatterns(signals []ScoredSignal) []Pattern {
	if len(signals) == 0 {
		return nil
	}

	// Build entity co-occurrence: pairs of entities appearing in same signal.
	type entityPair struct{ a, b string }
	pairNodes := make(map[entityPair]map[string]bool)
	// Track entity occurrences per signal.
	entitySignals := make(map[string][]string) // entity → signal node IDs

	for _, sig := range signals {
		nodeID := string(sig.NodeID)
		names := make([]string, 0, len(sig.Entities))
		for _, e := range sig.Entities {
			names = append(names, e.Name)
			entitySignals[e.Name] = append(entitySignals[e.Name], nodeID)
		}
		// Record co-occurrences.
		for i := 0; i < len(names); i++ {
			for j := i + 1; j < len(names); j++ {
				a, b := names[i], names[j]
				if a > b {
					a, b = b, a
				}
				pair := entityPair{a, b}
				if pairNodes[pair] == nil {
					pairNodes[pair] = make(map[string]bool)
				}
				pairNodes[pair][nodeID] = true
			}
		}
	}

	var patterns []Pattern

	// Patterns from entity co-occurrence (2+ co-occurrences).
	for pair, nodes := range pairNodes {
		if len(nodes) < 2 {
			continue
		}
		var nodeList []string
		for n := range nodes {
			nodeList = append(nodeList, n)
		}
		patterns = append(patterns, Pattern{
			Theme:        pair.a + " + " + pair.b,
			Occurrences:  len(nodes),
			RelatedNodes: nodeList,
			Entities:     []string{pair.a, pair.b},
		})
	}

	// Patterns from shared top keywords across signals.
	keywordSignals := make(map[string][]string) // keyword → signal node IDs
	for _, sig := range signals {
		nodeID := string(sig.NodeID)
		for _, k := range sig.Keywords {
			if k.Score > 0 {
				keywordSignals[k.Term] = append(keywordSignals[k.Term], nodeID)
			}
		}
	}
	for kw, nodeIDs := range keywordSignals {
		if len(nodeIDs) < 2 {
			continue
		}
		// Deduplicate.
		unique := make(map[string]bool)
		for _, id := range nodeIDs {
			unique[id] = true
		}
		if len(unique) < 2 {
			continue
		}
		var nodeList []string
		for id := range unique {
			nodeList = append(nodeList, id)
		}
		patterns = append(patterns, Pattern{
			Theme:        kw,
			Occurrences:  len(unique),
			RelatedNodes: nodeList,
			Keywords:     []string{kw},
		})
	}

	// Sort by occurrences descending.
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Occurrences > patterns[j].Occurrences
	})

	return patterns
}

// ComputeTrend compares current and prior pattern sets using Jaccard on themes.
func (ha *HeuristicAnalyzer) ComputeTrend(current, prior []Pattern) string {
	if len(prior) == 0 && len(current) == 0 {
		return "stable"
	}
	if len(prior) == 0 {
		return "rising"
	}
	if len(current) == 0 {
		return "declining"
	}

	currentThemes := make(map[string]bool)
	for _, p := range current {
		currentThemes[p.Theme] = true
	}
	priorThemes := make(map[string]bool)
	for _, p := range prior {
		priorThemes[p.Theme] = true
	}

	j := jaccardSimilarity(currentThemes, priorThemes)
	switch {
	case j > 0.5:
		return "stable"
	case j >= 0.2:
		return "shifting"
	default:
		// Check if themes appeared or disappeared.
		newCount := 0
		for t := range currentThemes {
			if !priorThemes[t] {
				newCount++
			}
		}
		goneCount := 0
		for t := range priorThemes {
			if !currentThemes[t] {
				goneCount++
			}
		}
		if newCount > goneCount {
			return "rising"
		}
		return "declining"
	}
}

// recognizeEntities scans text for known agent/skill names and high-TF-IDF bigrams.
func (ha *HeuristicAnalyzer) recognizeEntities(text string) []Entity {
	lower := strings.ToLower(text)
	var entities []Entity
	seen := make(map[string]bool)

	// Exact match against known agents.
	for name := range ha.agentNames {
		if strings.Contains(lower, name) && !seen[name] {
			seen[name] = true
			entities = append(entities, Entity{
				Name:       name,
				Type:       EntityAgent,
				Confidence: 1.0,
			})
		}
	}

	// Exact match against known skills.
	for name := range ha.skillNames {
		if strings.Contains(lower, name) && !seen[name] {
			seen[name] = true
			entities = append(entities, Entity{
				Name:       name,
				Type:       EntitySkill,
				Confidence: 1.0,
			})
		}
	}

	// High-TF-IDF bigrams as topic entities.
	for _, bg := range extractBigrams(text) {
		if seen[bg] {
			continue
		}
		// A bigram is a topic if both words are meaningful (not stopwords)
		// and the bigram itself has some presence in the corpus.
		words := strings.Fields(bg)
		if len(words) == 2 && !isStopword(words[0]) && !isStopword(words[1]) {
			seen[bg] = true
			entities = append(entities, Entity{
				Name:       bg,
				Type:       EntityTopic,
				Confidence: 0.5,
			})
		}
	}

	return entities
}

// autoTag combines top keywords and entity names into a tag set (max 8).
func (ha *HeuristicAnalyzer) autoTag(entities []Entity, keywords []Keyword) []string {
	seen := make(map[string]bool)
	var tags []string

	// Add entity names (agent/skill first, then topic).
	for _, e := range entities {
		if e.Type == EntityAgent || e.Type == EntitySkill {
			tag := strings.ToLower(e.Name)
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	for _, e := range entities {
		if e.Type == EntityTopic && len(tags) < 8 {
			tag := strings.ToLower(e.Name)
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}

	// Fill remaining with top keywords.
	for _, k := range keywords {
		if len(tags) >= 8 {
			break
		}
		tag := strings.ToLower(k.Term)
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}

	return tags
}

// AutoTag implements brain.Enricher. It analyzes the content text and
// returns auto-generated tags (max 8). Used by brain.Capture when the
// user provides fewer than 3 tags.
func (ha *HeuristicAnalyzer) AutoTag(content string) []string {
	result := ha.AnalyzeText(content)
	return result.Tags
}

// topN returns the top N keys from a count map, sorted by frequency.
func topN(counts map[string]int, n int) []string {
	type entry struct {
		key   string
		count int
	}
	var entries []entry
	for k, v := range counts {
		entries = append(entries, entry{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].key < entries[j].key
	})
	if n > len(entries) {
		n = len(entries)
	}
	result := make([]string, n)
	for i := 0; i < n; i++ {
		result[i] = entries[i].key
	}
	return result
}
