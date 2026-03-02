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
| `internal/storage` | Storage backends (BadgerDB) |
| `internal/server` | Bolt v4 protocol, HTTP API |

## Data Model

KafGraph uses a labeled property graph:

**Node Types**: Agent, Conversation, Message, HumanFeedback, ReflectionCycle,
LearningSignal, Skill, SharedMemory, AuditEvent

**Edge Types**: AUTHORED, REPLIED_TO, BELONGS_TO, HAS_FEEDBACK,
TRIGGERED_REFLECTION, LINKS_TO, CONTRIBUTED_VALUE, USES_SKILL,
SHARED_BY, REFERENCES, DELEGATES_TO, REPORTS_TO

Every node and edge carries:
- `createdAt` timestamp
- `sourceOffset` (Kafka topic + partition + offset)

Edges support three weight dimensions:
- `impact` (float 0-1)
- `relevance` (float 0-1)
- `valueContribution` (float 0-1)

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
