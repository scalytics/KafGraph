---
layout: default
title: Demo Data
nav_order: 6
---

# Demo Data

KafGraph ships with a built-in demo scenario that populates the graph with
realistic multi-agent conversation data, reflection cycles, and human feedback
-- no Kafka broker or S3 required.

## The Blog-Team Scenario

Three specialist agents collaboratively write a blog post, orchestrated by a
coordinator:

| Agent | Role | Skills |
|-------|------|--------|
| **Coordinator** | Orchestrates the pipeline, delegates tasks | -- |
| **Researcher** | Researches and enriches the draft | `web_search`, `summarize` |
| **Editor** | Sharpens content for clarity and tone | `rewrite`, `tone_check` |
| **Formatter** | Polishes formatting and finalizes | `format_markdown`, `proofread` |

### Pipeline Flow

```
Coordinator ──delegate──▶ Researcher ──delegate──▶ Editor ──delegate──▶ Formatter
     │                        │                       │                      │
     │ request                │ skill_request          │ skill_request        │ skill_request
     │                        │ skill_response         │ skill_response       │ skill_response
     │                        │ memory                 │ memory               │ memory
     │                        │ response               │ response             │ response
     │                        │ audit                  │ audit                │ audit
     ◀────────────────────────┘                        │                      │
     ◀────────────────────────────────────────────────┘                      │
     ◀──────────────────────────────────────────────────────────────────────┘
     │
     │ report (pipeline_completed)
     │ audit
```

### Envelope Types Exercised

The scenario produces **42 envelopes** covering every envelope type:

| Type | Count | Description |
|------|-------|-------------|
| `announce` | 4 | Agent joins (coordinator, researcher, editor, formatter) |
| `roster` | 3 | Skill manifests for researcher, editor, formatter |
| `orchestrator` | 4 | 3 delegate + 1 report |
| `request` | 3 | Task requests from coordinator to each specialist |
| `response` | 3 | Task completions from each specialist |
| `task_status` | 6 | in_progress + completed for each specialist |
| `skill_request` | 6 | 2 per specialist (each uses both skills) |
| `skill_response` | 6 | Results from each skill invocation |
| `memory` | 3 | Shared memory: research-findings, editorial-notes, final-blog |
| `audit` | 4 | 3 task_completed + 1 pipeline_completed |

### Graph Produced

After ingestion, the graph contains:

| Node Type | Count | Notes |
|-----------|-------|-------|
| Agent | 4 | coordinator, researcher, editor, formatter |
| Conversation | 1 | All messages share correlation ID `blog-draft-2026` |
| Message | 18 | Requests, responses, skill invocations |
| Skill | 6 | web_search, summarize, rewrite, tone_check, format_markdown, proofread |
| SharedMemory | 3 | Research findings, editorial notes, final blog |
| AuditEvent | 4 | Task and pipeline completion audit trail |

Plus **30+ edges** connecting them: `AUTHORED`, `BELONGS_TO`, `USES_SKILL`,
`HAS_SKILL`, `SHARED_BY`, `REFERENCES`, `DELEGATES_TO`, `REPORTS_TO`,
`AUDITED_BY`.

## When Is Reflection Triggered?

KafGraph has **three** paths that trigger reflection cycles:

| Trigger | When | How |
|---------|------|-----|
| **Scheduler** (automatic) | Production server with `reflect.enabled=true` | Ticker loop checks daily/weekly/monthly schedules and fires cycles for every discovered Agent node. Configured via `DailyTime`, `WeeklyDay`, `MonthlyDay` in `kafgraph.yaml`. |
| **`brain_reflect` tool** (on-demand) | Agent or human calls `POST /api/v1/tools/brain_reflect` | Accepts `{agentId, scope, windowHours}` and delegates to `CycleRunner.ExecuteForBrain()`. |
| **Direct** (programmatic) | Demo seed, tests, custom tooling | Calls `CycleRunner.Execute()` with an explicit `CycleRequest` specifying type, agent, and time window. |

After a cycle completes, the **FeedbackChecker** monitors grace periods and
transitions cycles through the feedback state machine:

```
PENDING ──(grace period expires)──▶ NEEDS_FEEDBACK ──(request sent)──▶ REQUESTED ──▶ RECEIVED
                                                                                  ──▶ WAIVED
```

The **demo-seed** CLI uses the direct path: it calls `CycleRunner.Execute()`
for each of the 4 agents immediately after ingesting conversation data, then
submits a human feedback envelope to exercise the full feedback pipeline. The
scheduler and `brain_reflect` tool are not involved since there is no
long-running server during seeding.

## Reflection Cycles

After ingesting conversation data, `demo-seed` runs **daily reflection cycles**
for each of the four agents. The reflection engine uses the `HeuristicAnalyzer`
to enrich each cycle with entity recognition, TF-IDF keywords, auto-tagging,
and structured summaries:

1. Gathers all Message and Conversation nodes connected to each agent
2. Builds a TF-IDF corpus from all window nodes for keyword extraction
3. Refreshes the analyzer's knowledge of known agents and skills
4. Scores each node with three heuristic dimensions:
   - **Impact** -- edge count normalized by cap of 10
   - **Relevance** -- Jaccard word-set similarity to conversation context
   - **Value Contribution** -- ratio of replied messages to total messages
5. Enriches each signal via the Analyzer:
   - **Entities** -- recognized agents, skills, and topic bigrams
   - **Keywords** -- top TF-IDF terms specific to each signal
   - **Tags** -- auto-generated from entities and keywords (max 8)
6. Creates `LearningSignal` nodes with enriched properties (`tags`,
   `keywords`, `entities`) linked to `ReflectionCycle` nodes via
   `LINKS_TO` edges carrying the score dimensions
7. Synthesizes a structured summary (grouped by label, top themes, top entities)
8. Links each cycle to its agent via `TRIGGERED_REFLECTION` edges

The enriched tags feed `brain_patterns`, which can now detect recurring themes
across signals. See [Reflection Engine](reflection-engine.md) for details.

### Human Feedback

The demo also submits a positive human feedback entry on the coordinator's
reflection cycle, exercising the full feedback pipeline:

| Field | Value |
|-------|-------|
| Feedback Type | positive |
| Impact | 0.85 |
| Relevance | 0.90 |
| Value Contribution | 0.80 |
| Comment | "Good coordination across the blog writing pipeline..." |

This creates a `HumanFeedback` node, links it to the cycle via `HAS_FEEDBACK`,
and updates the cycle's `humanFeedbackStatus` to `RECEIVED`.

### Reflection Nodes Created

| Node Type | Count | Enriched Properties |
|-----------|-------|---------------------|
| ReflectionCycle | 4 (one per agent) | status, completedAt |
| LearningSignal | Varies (depends on connected nodes) | tags, keywords, entities, impact, relevance, valueContribution |
| HumanFeedback | 1 | scores, comment |

## Usage

### Seed and Browse

```bash
make demo-seed
```

Seeds the graph into a temporary directory and starts the HTTP server on
`http://localhost:7474`. Open the management UI to explore the graph,
view the reflection dashboard, and inspect the feedback pipeline.

### Export as JSON

```bash
make demo-generate
```

Exports all 42 envelopes as numbered JSON files:

```
demo/data/
  group.blog-team/
    partition-0/
      000000.json    # Coordinator announce
      000001.json    # Researcher announce
      ...
      000041.json    # Final audit
```

Each file is pretty-printed for human readability and can be loaded by a
future file-based `SegmentReader`.

### Custom Data Directory

```bash
go run ./cmd/demo-seed --data-dir /path/to/data --serve
```

Persists the seeded graph to a specific directory (useful for repeated
exploration without re-seeding).

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--data-dir` | (temp) | BadgerDB data directory |
| `--serve` | false | Start HTTP server after seeding |
| `--addr` | `0.0.0.0:7474` | HTTP listen address |
| `--export` | (none) | Export envelopes as JSON to this directory |

## Exploring in the UI

After running `make demo-seed`, navigate to `http://localhost:7474`:

- **Dashboard** -- overview of node/edge counts and system health
- **Graph Browser** -- visual exploration of the agent collaboration graph
- **Reflection** -- reflection cycle summary, learning signal scores,
  feedback pipeline status (PENDING / NEEDS_FEEDBACK / REQUESTED / RECEIVED)
- **Data Stats** -- detailed breakdowns by node type and edge type

### Example Queries

Use the query endpoint or Bolt protocol to explore the seeded data:

```cypher
-- All agents and their skills
MATCH (a:Agent)-[:HAS_SKILL]->(s:Skill) RETURN a, s

-- Delegation chain
MATCH (from:Agent)-[:DELEGATES_TO]->(to:Agent) RETURN from, to

-- Messages in the conversation
MATCH (m:Message)-[:BELONGS_TO]->(c:Conversation) RETURN m, c

-- Reflection cycles with signals
MATCH (s:LearningSignal)-[:LINKS_TO]->(c:ReflectionCycle) RETURN s, c

-- Feedback on cycles
MATCH (c:ReflectionCycle)-[:HAS_FEEDBACK]->(f:HumanFeedback) RETURN c, f
```
