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

// Analyzer is the pluggable intelligence interface for the reflection engine.
// The default implementation is HeuristicAnalyzer (pure-Go, no external deps).
// An LLM/embedding backend can be swapped in by implementing this interface.
type Analyzer interface {
	// AnalyzeText extracts entities, keywords, and tags from a text fragment.
	AnalyzeText(text string) AnalysisResult

	// SummarizeSignals produces a structured summary across multiple scored signals.
	SummarizeSignals(signals []ScoredSignal) string

	// DetectPatterns finds co-occurring entities and recurring themes across signals.
	DetectPatterns(signals []ScoredSignal) []Pattern

	// ComputeTrend compares current and prior pattern sets to detect change.
	ComputeTrend(current, prior []Pattern) string
}

// AnalysisResult holds the output of AnalyzeText.
type AnalysisResult struct {
	Entities []Entity
	Keywords []Keyword
	Tags     []string
}

// EntityType classifies a recognized entity.
type EntityType string

const (
	EntityAgent EntityType = "agent"
	EntitySkill EntityType = "skill"
	EntityTopic EntityType = "topic"
)

// Entity is a recognized mention in text.
type Entity struct {
	Name       string
	Type       EntityType
	Confidence float64 // 1.0 = exact dictionary match, 0.5 = heuristic/partial
}

// Keyword is a term with its TF-IDF score.
type Keyword struct {
	Term  string
	Score float64
}

// Pattern is a recurring theme detected across signals.
type Pattern struct {
	Theme        string
	Occurrences  int
	RelatedNodes []string
	Entities     []string
	Keywords     []string
}
