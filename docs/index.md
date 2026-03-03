---
layout: default
title: Home
nav_order: 1
---

# KafGraph

**The distributed shared brain of collaborating agents.**

KafGraph is a graph database and reflection engine that ingests agent conversation data
from Apache Kafka topics, structures it as a property graph, and provides temporal
reflection services (daily / weekly / monthly) so agents and teams learn from their
past interactions.

## Key Features

- **Agent Brain** — persistent, self-owned knowledge system accessible via tool calls
- **Kafka-native** — all data flows through Kafka topics; co-located with KafScale
- **Property Graph** — labeled nodes and directed edges with arbitrary properties
- **Query Engine** — OpenCypher subset with full-text search and vector similarity
- **Reflection Engine** — daily/weekly/monthly reflection cycles with learning signals
- **Human Feedback** — feedback loops for positive/negative impact scoring
- **Brain Tool API** — seven brain tools callable via HTTP or KafClaw skill routing
- **Bolt v4 Protocol** — Neo4j-compatible binary protocol for driver connectivity
- **Per-agent + Distributed** — same binary runs embedded or as a cluster shard

## Quick Start

```bash
# Clone and setup
git clone https://github.com/scalytics/kafgraph.git
cd kafgraph
make dev-setup

# Build and test
make build
make test

# Run locally
make dev-run
```

See [Getting Started](getting-started.md) for detailed setup instructions.

## Architecture

KafGraph follows the KafScale Processor 5-layer architecture:

1. **Discovery** — S3 segment discovery
2. **Decoder** — GroupEnvelope / KafClaw wire format parsing
3. **Checkpoint** — offset tracking and deduplication
4. **Sink** — BadgerDB graph storage engine
5. **Locking** — distributed coordination via Kafka

See [Architecture](architecture.md) for the full design.

## Brain Tools

| Tool | Description |
|------|-------------|
| `brain_search` | Semantic search across the knowledge graph |
| `brain_recall` | Load accumulated agent context |
| `brain_capture` | Write insights and decisions into the brain |
| `brain_recent` | Browse recent activity |
| `brain_patterns` | Surface recurring themes and connections |
| `brain_reflect` | Trigger reflection cycle with deterministic IDs and heuristic scoring |
| `brain_feedback` | Submit human feedback on reflection cycles |

See [Brain Tool API](brain-tool-api.md) for schemas and usage.
