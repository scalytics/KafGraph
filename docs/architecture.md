---
layout: default
title: Architecture
nav_order: 2
---

# Architecture

KafGraph is built as a KafScale Processor implementing the 5-layer processor stack.

## System Overview

```
  KafClaw Agents          KafScale Broker
       │                       │
       ▼                       ▼
  Kafka Topics ──────▶ S3 Segments
                           │
                           ▼
                   ┌───────────────┐
                   │   Discovery   │  Layer 1: Find new segments
                   ├───────────────┤
                   │    Decoder    │  Layer 2: Parse GroupEnvelope
                   ├───────────────┤
                   │  Checkpoint   │  Layer 3: Track offsets
                   ├───────────────┤
                   │     Sink      │  Layer 4: Write to graph
                   ├───────────────┤
                   │   Locking     │  Layer 5: Coordination
                   └───────────────┘
                           │
                           ▼
                   ┌───────────────┐
                   │   BadgerDB    │  Embedded storage
                   │  Graph Store  │
                   └───────────────┘
                           │
                    ┌──────┼──────┐
                    ▼      ▼      ▼
                  Bolt   HTTP   Brain
                  v4.4   API    Tool API
```

## Key Packages

| Package | Responsibility |
|---------|---------------|
| `cmd/kafgraph` | Entry point, CLI, signal handling |
| `internal/config` | Configuration loading (Viper) |
| `internal/graph` | Core property graph model and CRUD |
| `internal/storage` | Storage backends (BadgerDB) with secondary indexes |
| `internal/index` | BadgerDB secondary indexes (label, edge out/in/label) |
| `internal/search` | Full-text search (bleve) and vector similarity search |
| `internal/query` | OpenCypher subset parser, planner, and executor |
| `internal/ingest` | KafScale 5-layer processor and envelope handlers |
| `internal/brain` | Brain Tool API (7 tools: search, recall, capture, recent, patterns, reflect, feedback) |
| `internal/reflect` | Reflection Engine: scheduler, cycle runner, scorer, iterator, feedback checker |
| `internal/server` | Bolt v4 protocol, HTTP API, query endpoint, brain tool endpoint |

## Data Model

KafGraph uses a labeled property graph:

**Node Types**: Agent, Conversation, Message, HumanFeedback, ReflectionCycle,
LearningSignal, Skill, SharedMemory, AuditEvent

**Edge Types**: AUTHORED, REPLIED_TO, BELONGS_TO, HAS_FEEDBACK,
TRIGGERED_REFLECTION, LINKS_TO, CONTRIBUTED_VALUE, ROLLUP_OF,
USES_SKILL, SHARED_BY, REFERENCES, DELEGATES_TO, REPORTS_TO

Every node and edge carries:
- `createdAt` timestamp
- `sourceOffset` (Kafka topic + partition + offset)

Edges support three weight dimensions:
- `impact` (float 0-1)
- `relevance` (float 0-1)
- `valueContribution` (float 0-1)

## Query Engine

KafGraph includes an OpenCypher subset query engine with the following pipeline:

```
Cypher string → Lexer → Parser → AST → Planner → Plan → Executor → ResultSet
```

**Supported Cypher**: MATCH, WHERE, RETURN, CREATE, MERGE, SET, DELETE, CALL/YIELD,
ORDER BY, LIMIT, SKIP. WHERE supports =, <>, <, >, <=, >=, AND, OR, NOT, CONTAINS, IN.

**Secondary Indexes**: BadgerDB key prefixes for label, outgoing edge, incoming edge,
and edge label lookups. Maintained transactionally alongside writes.

**Full-Text Search**: bleve-powered text indexing on configurable label/property pairs.
Accessible via `CALL kafgraph.fullTextSearch(label, property, query) YIELD node, score`.

**Vector Search**: Brute-force cosine similarity over float32 vectors stored in BadgerDB.
Accessible via `CALL kafgraph.vectorSearch(label, property, vector, k) YIELD node, score`.

**Bolt v4 Protocol**: Full message loop with PackStream encoding, chunked transport,
and HELLO/RUN/PULL/RECORD/SUCCESS/FAILURE/RESET message types.

**HTTP Query Endpoint**: `POST /api/v1/query` accepts `{"cypher": "...", "params": {}}`.

## Brain Tool API

KafGraph exposes seven brain tools via `POST /api/v1/tools/{toolName}`:

| Tool | Purpose |
|------|---------|
| `brain_search` | Full-text search across Message, SharedMemory, LearningSignal, Conversation nodes |
| `brain_recall` | Load agent context via edge traversal (conversations, decisions, feedback, threads) |
| `brain_capture` | Create LearningSignal nodes with auto-linking to agents and referenced nodes |
| `brain_recent` | Time-windowed activity browsing with type and agent filtering |
| `brain_patterns` | Tag aggregation from LearningSignal nodes to surface recurring themes |
| `brain_reflect` | Trigger reflection cycle with deterministic IDs, heuristic scoring, and signal aggregation |
| `brain_feedback` | Submit HumanFeedback on ReflectionCycles with impact scoring |

Tool schemas are listed at `GET /api/v1/tools`. See [Brain Tool API](brain-tool-api.md)
for full input/output schemas.

## Reflection Engine

The reflection engine (`internal/reflect/`) automates periodic reflection cycles
that score agent activity and create structured learning signals.

### Cycle Types

| Cadence | Window | Rollup |
|---------|--------|--------|
| Daily | Midnight UTC to now | Scores individual messages/conversations |
| Weekly | Most recent Monday to now | Aggregates daily cycles via ROLLUP_OF edges |
| Monthly | 1st of month to now | Aggregates weekly cycles via ROLLUP_OF edges |

### Heuristic Scoring

Three dimensions scored per signal (no LLM, pure heuristics):

- **Impact**: Edge count from node, normalized by cap of 10
- **Relevance**: Jaccard word-set similarity between message text and conversation description
- **ValueContribution**: Ratio of replied messages to total messages in conversation

### Idempotency

All cycle and signal IDs are deterministic (`CycleNodeID`, `SignalNodeID`).
Re-running the same cycle request produces the same IDs, and `UpsertNode`
merges properties — no duplicates.

### Feedback Grace Period

Completed cycles get `humanFeedbackStatus: "PENDING"`. After a configurable
grace period (default 24h), the `FeedbackChecker` transitions them to
`"NEEDS_FEEDBACK"`.

### Scheduler

When `reflect.enabled=true`, the scheduler runs a ticker loop that:
1. Checks daily/weekly/monthly schedules against current time
2. Discovers all Agent nodes in the graph
3. Runs due cycles for each agent
4. Checks feedback grace periods

### Brain Integration

`brain_reflect` delegates to the reflection engine's `CycleRunner` via the
`ReflectionRunner` interface, providing deterministic IDs and heuristic
scoring for on-demand reflection. Falls back to original behavior when
the runner is not configured.

## Deployment Modes

### Per-Agent Mode
Lightweight embedded instance co-located with a single agent runtime.
Local BadgerDB storage, all brain tools available via localhost HTTP.

### Distributed Mode
Multi-node cluster with graph partitions distributed across KafScale Processor nodes.
Partitioned by agent ID, coordinated via Kafka topics.

## References

- [SPEC/solution-design.md](https://github.com/scalytics/kafgraph/blob/main/SPEC/solution-design.md) — full design document
- [SPEC/requirements.md](https://github.com/scalytics/kafgraph/blob/main/SPEC/requirements.md) — functional and non-functional requirements
