---
layout: default
title: Reflection Engine
nav_order: 7
---

# Reflection Engine

The reflection engine is KafGraph's intelligence layer. During each reflection
cycle it analyzes signals (messages, conversations, learning signals) to
recognize entities, extract keywords, generate tags, synthesize summaries,
detect patterns, and track trends over time.

## What Reflection Aims For

During a reflection cycle, KafGraph performs six analysis steps:

1. **Recognize** — identify agents, skills, and topics mentioned in message text
2. **Extract** — pull out the most important keywords using TF-IDF
3. **Tag** — auto-generate tags so `brain_patterns` has data to work with
4. **Summarize** — synthesize a coherent summary across multiple signals
5. **Correlate** — detect co-occurring entities and recurring themes
6. **Trend** — compare current patterns against prior cycles to detect change

## Analyzer Interface

All analysis is performed through a pluggable `Analyzer` interface
(`internal/reflect/analyzer.go`):

```go
type Analyzer interface {
    AnalyzeText(text string) AnalysisResult
    SummarizeSignals(signals []ScoredSignal) string
    DetectPatterns(signals []ScoredSignal) []Pattern
    ComputeTrend(current, prior []Pattern) string
}
```

The default implementation is `HeuristicAnalyzer` — a pure-Go analyzer that
requires no external services, LLMs, or embedding models. An LLM or embedding
backend can be swapped in by implementing this interface.

## Analysis Topics

### Entity Recognition

Scans message text for known graph entities (agent names, skill names) using
dictionary matching against live graph data loaded via `RefreshKnowledge()`.
Remaining high-TF-IDF bigrams are classified as topic entities.

Each entity has:
- **Name**: the matched string
- **Type**: `agent`, `skill`, or `topic`
- **Confidence**: `1.0` for exact dictionary match, `0.5` for heuristic/partial

### TF-IDF Keyword Extraction

Builds a per-window document corpus from all nodes in the reflection window.
Computes term frequency / inverse document frequency to surface terms that
are important in a specific signal but not common across all signals.

- Filters ~175 English stopwords and tokens shorter than 3 characters
- Returns top-N keywords ranked by TF-IDF score
- Smoothed IDF formula: `log((N+1) / df)` ensures single-document corpora
  still produce positive scores

### Auto-Tagging

Combines top TF-IDF keywords and recognized entity names into a tag set
(max 8 tags per signal). Tags are stored on LearningSignal nodes in the
`"tags"` property as a comma-separated string.

This feeds `brain_patterns` which previously found zero patterns because
nobody was generating tags.

### Summary Synthesis

Replaces the previous `"Found N signals: sig1; sig2; ..."` concatenation
with a structured summary that:

- Groups signals by label and counts per group
- Extracts top 3 keywords and top entities across all signals
- Produces a sentence like: `"Analyzed 12 signals (8 messages,
  4 conversations): key themes are distributed-systems,
  agent-coordination. Top entities: researcher, editor."`

### Cross-Signal Pattern Detection

Builds an entity co-occurrence matrix across all signals in a cycle.
Signals sharing 2+ entities form a pattern. Also groups signals sharing
top keywords. Each pattern gets a theme string from its most frequent
entity/keyword pair.

### Trend Detection

Compares the current cycle's pattern set against prior cycles using Jaccard
similarity on theme strings:

| Jaccard | Trend |
|---------|-------|
| > 0.5 | `stable` — themes are consistent |
| 0.2 – 0.5 | `shifting` — moderate change |
| < 0.2 (more new) | `rising` — new patterns appearing |
| < 0.2 (more gone) | `declining` — old patterns disappearing |

## Enriched Data Flow

```
Window Nodes
    │
    ▼
TF-IDF Corpus ──► Keyword Extraction
    │                    │
    ▼                    ▼
Entity Recognition   Auto-Tagging
    │                    │
    ▼                    ▼
LearningSignal nodes stored with:
  - tags (comma-separated)
  - keywords (comma-separated)
  - entities (comma-separated)
    │
    ▼
brain_patterns reads tags/entities
brain_search uses full-text + (future) vector
```

## Plugging In an LLM Backend

To replace the heuristic analyzer with an LLM-backed implementation:

1. Implement the `Analyzer` interface
2. Implement the `Embedder` interface for vector search:
   ```go
   type Embedder interface {
       Embed(text string) ([]float32, error)
   }
   ```
3. In `main.go`, replace `NewHeuristicAnalyzer` with your implementation
4. Call `bs.SetEmbedder(yourEmbedder)` to enable vector search

No other code changes are needed — the analyzer and embedder interfaces
decouple the intelligence layer from the pipeline.

## Configuration

The reflection engine is configured via `kafgraph.yaml`:

```yaml
reflect:
  enabled: true
  check_interval: "1m"
  daily_time: "02:00"
  weekly_time: "03:00"
  weekly_day: "monday"
  monthly_time: "04:00"
  monthly_day: 1
  feedback_grace_period: "48h"
  feedback_request_topic: "kafgraph.feedback.requests"
  feedback_top_n: 5
```

The analyzer is automatically wired when the reflection scheduler starts.
No additional configuration is needed for the heuristic analyzer.
