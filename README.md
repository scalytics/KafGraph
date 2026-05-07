![alt text](docs/assets/images/image-3.png)

# KafGraph

**The distributed shared brain of collaborating agents.**

A self-owned, persistent, agent-accessible knowledge graph written in Go.
Apache 2.0. Open beta.

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8.svg)](go.mod)
[![Status](https://img.shields.io/badge/status-open%20beta-1D9E75.svg)](#project-status)

---

## What KafGraph is

KafGraph is the memory layer for AI agent teams. It ingests every conversation,
decision, and artifact that flows through your agent group, structures it as a
queryable property graph, and exposes it back to agents through tool calls. No
agent ever starts from zero.

```
Agent conversations  →  Kafka topics  →  KafScale processor  →  KafGraph (BadgerDB)
                                                                       │
                                                  ┌────────────────────┤
                                                  │                    │
                                            Brain Tool API      Cypher / Bolt v4
                                            (agent access)      (tooling access)
```

Two deployment modes from one binary:

* **Embedded.** A local brain for one agent. BadgerDB on disk, Brain Tool API on
  localhost. Useful for per-agent personalization, coding agents on a developer
  laptop, or air-gapped single-agent workloads.
* **Distributed.** A shared brain for an agent team. Gossip-based cluster
  membership via `hashicorp/memberlist`, agentID partitioning via FNV-1a,
  cross-partition fan-out RPC. The mode no other agent memory system ships.

## Why "shared brain"

Every other agent memory system on the market (Mem0, Zep / Graphiti, Cognee,
Letta, LangMem, Claude memory, ChatGPT memory) is built around a single user or
a single agent. Once you scale to a team of collaborating agents (researcher,
coder, reviewer, orchestrator), the memory layer becomes the bottleneck:

* Different agents operate on different versions of reality.
* Downstream agents act on assumptions upstream peers have already invalidated.
* Each session starts from zero. Yesterday's lessons stay locked inside one
  agent's context window.

KafGraph is a single graph that all agents in the team read and write through
the same seven brain tools. A researcher's findings enrich the coder's context.
A reviewer's feedback flows back into the team's reflection scores. The brain
compounds.

## Features

| Capability | Status |
|------------|--------|
| Property graph storage on BadgerDB (LSM, ACID) | Shipping |
| OpenCypher subset (MATCH, WHERE, RETURN, CREATE, MERGE, SET, DELETE, CALL, YIELD) | Shipping |
| Bolt v4 wire protocol (PackStream, chunked transport) | Shipping |
| HTTP REST API for graph CRUD and tool execution | Shipping |
| Brute-force vector search (cosine similarity) | Shipping |
| HNSW vector index | Planned (hardening phase) |
| `bleve` full-text search | Shipping |
| Seven brain tools (`brain_search`, `brain_recall`, `brain_capture`, `brain_recent`, `brain_patterns`, `brain_reflect`, `brain_feedback`) | Shipping |
| Reflection engine (daily, weekly, monthly cycles with scoring) | Shipping |
| Human feedback loop (state machine, score overrides) | Shipping |
| Distributed cluster (gossip, agentID partitioning, fan-out RPC) | Shipping |
| Embedded management UI (graph browser, reflection dashboard) | Shipping |
| TLS, encryption at rest, OpenTelemetry, load tests | In progress |

## The brain tool API

Agents talk to KafGraph through standard JSON-schema tool calls. Two transports:
direct HTTP for embedded mode, or KafClaw skill routing for distributed mode
(auto-discovered via the group roster).

| Tool | Purpose |
|------|---------|
| `brain_search` | Semantic search across the graph. Vector similarity over Message, SharedMemory, LearningSignal, Conversation nodes. |
| `brain_recall` | Load accumulated context at session start. Edge traversal across conversations, decisions, feedback. |
| `brain_capture` | Write insights into the brain. Creates `LearningSignal` nodes auto-linked to the agent. |
| `brain_recent` | Time-windowed activity browsing with type and agent filtering. |
| `brain_patterns` | Recurring themes via tag aggregation across `LearningSignal` nodes. |
| `brain_reflect` | Trigger a reflection cycle on demand. |
| `brain_feedback` | Submit human feedback on a reflection cycle. Score overrides flow back to `LearningSignal` nodes. |

Each tool ships with its own `SKILL.md` manifest under `skills/`. The HTTP
contract is in `docs/api.md`.

## Quick start

### Prerequisites

* Go 1.25 or later
* Docker and Docker Compose (for integration tests and the dev environment)

### Build and run

```bash
git clone https://github.com/scalytics/KafGraph.git
cd KafGraph

# Set up dev environment (formats, linters, hooks)
make dev-setup

# Build
make build

# Run tests
make test

# Start MinIO + Kafka + KafGraph
make docker-up

# Run KafGraph locally against the dev environment
make dev-run
```

### Try it with demo data

The demo seeds a realistic multi-agent conversation, including reflection
cycles and human feedback, so you can browse a populated graph immediately:

```bash
make demo-seed
# → Management UI at http://localhost:7474
# → Bolt endpoint at bolt://localhost:7687
```

### Configuration

KafGraph reads YAML and env-var overrides via `viper`. The defaults in
`.env.example`:

```env
KAFGRAPH_HOST=0.0.0.0
KAFGRAPH_PORT=7474
KAFGRAPH_BOLT_PORT=7687
KAFGRAPH_DATA_DIR=./data
KAFGRAPH_KAFKA_BROKERS=localhost:9092
KAFGRAPH_S3_ENDPOINT=localhost:9000
KAFGRAPH_EMBEDDING_ENDPOINT=http://localhost:11434
KAFGRAPH_EMBEDDING_MODEL=nomic-embed-text
```

Full reference in `docs/configuration.md`.

## Architecture

KafGraph runs as a KafScale processor, co-located with KafScale broker nodes.
It consumes KafClaw group topics by reading S3 segments directly. No broker
round-trip, no replay storms.

| Component | Implementation |
|-----------|----------------|
| Storage engine | BadgerDB v4 (pure Go, LSM, ACID) |
| Segment processing | KafScale processor SDK |
| Object storage | MinIO or any S3-compatible target |
| Vector search | Brute-force cosine similarity (HNSW planned) |
| Full-text index | `bleve` |
| Query language | OpenCypher subset |
| Wire protocol | Bolt v4 (PackStream) |
| Cluster membership | `hashicorp/memberlist` (gossip) |
| Partitioning | FNV-1a hashing on agentID |
| Configuration | YAML + env-var overrides via `viper` |

### KafClaw integration

KafGraph consumes the full KafClaw group topic hierarchy:

```
group.<group_name>.announce                     →  Agent nodes
group.<group_name>.requests / responses         →  Conversation + Message nodes
group.<group_name>.skill.*.requests / responses →  Skill-specific conversations
group.<group_name>.memory.shared                →  SharedMemory nodes
group.<group_name>.observe.audit                →  AuditEvent nodes
group.<group_name>.control.roster               →  Dynamic skill topic discovery
group.<group_name>.orchestrator                 →  Agent hierarchy edges
```

Wire format and topic model: `SPEC/kafclaw-topic-reference.md`.

## What KafGraph is not

* **Not a general-purpose graph database.** KafGraph implements a deliberate
  OpenCypher subset focused on agent memory access patterns. If you need full
  Cypher with stored procedures and a query planner that competes with Neo4j
  Enterprise, use Neo4j or Memgraph.
* **Not a pure vector database.** Vector search exists in KafGraph because
  agents need it for semantic recall, not because we compete with Pinecone or
  Weaviate. If your workload is "millions of embeddings, no graph," use a real
  vector DB.
* **Not a drop-in Neo4j replacement.** We speak Bolt v4 and a Cypher subset so
  existing Bolt clients and dashboards work for inspection. We are not a 1-to-1
  swap for an existing Neo4j deployment with custom procedures, plugins, or
  APOC dependencies.
* **Not GA software, today.** Open beta means feature-complete on the core
  scope but enterprise hardening still in flight. Use it for evaluations,
  design partnerships, and non-production agent work. Talk to us before you
  commit a regulated production workload.

## Where KafGraph fits in the stack

KafGraph is one of four open-source layers from Scalytics that compose into a
complete agent platform. Each is independently useful.

| Layer | Project | Role |
|-------|---------|------|
| Transport | [KafScale](https://github.com/scalytics/kafscale) | S3-native, Kafka-compatible streaming. Stateless brokers, infinite retention. |
| Runtime | [KafClaw](https://github.com/scalytics/KafClaw) | Agent groups, skill routing, policy mesh, tool registry, shared memory topics. |
| **Brain** | **KafGraph** | **The shared brain. Property graph, reflection engine, Brain Tool API.** |
| Observability | [kafSIEM](https://github.com/scalytics/kafSIEM) | Streaming threat detection and audit on the same Kafka backbone. |

## Project status

Open beta. All nine layers run end-to-end with extensive test coverage. The
remaining work is enterprise hardening.

| Phase | Name | Status |
|-------|------|--------|
| 0 | Foundation (BadgerDB, graph API, Bolt handshake, config) | Complete |
| 1 | Running Server (HTTP REST, Bolt accept loop, 84.9% coverage) | Complete |
| 2 | Processor (KafScale 5-layer, GroupEnvelope decoder, 11 envelope types) | Complete |
| 3 | Query Engine (OpenCypher parser, indexes, full-text, vector, Bolt message loop) | Complete |
| 4 | Agent Brain (7 brain tools, 88.8% coverage) | Complete |
| 5 | Reflection (daily / weekly / monthly cycles, scoring, `LearningSignal` nodes) | Complete |
| 6 | Feedback (human feedback state machine, score overrides) | Complete |
| 7 | Distribution (gossip, FNV-1a partitioning, cross-partition fan-out) | Complete |
| 8 | Management UI (Cytoscape.js, ECharts, embedded, air-gapped) | Complete |
| 9 | Hardening (TLS, encryption at rest, OTel, Helm, load tests) | In progress |

Living plan: `SPEC/PLAN.md`.

## Specifications and design notes

The original spec documents are kept in the repository for traceability:

| Document | Description |
|----------|-------------|
| `SPEC/PLAN.md` | Living phase plan, updated after each milestone |
| `SPEC/initial-idea.md` | Original vision and resolved open questions |
| `SPEC/requirements.md` | Functional, non-functional, and integration requirements |
| `SPEC/solution-design.md` | Architecture and component design |
| `SPEC/kafclaw-topic-reference.md` | KafClaw topic naming, wire format, and KafGraph mapping |
| `SPEC/about-agent-brains.md` | Foundational thinking on agent-readable memory systems |

## Development

```bash
make help            # List all targets
make test-unit       # Unit tests only
make test-e2e        # End-to-end tests (in-process, temp BadgerDB)
make test-integration # Integration tests (docker-compose)
make test-race       # Unit + E2E with race detector
make cover-check     # Fail if coverage drops below the gate
make lint            # golangci-lint
make sec             # gosec + govulncheck
make docker-up       # Start MinIO + Kafka + KafGraph dev environment
make demo-seed       # Seed demo graph and open the UI
```

Coverage gates run on CI. Race detector runs on every PR. The `Makefile` has
the full target list.

## Contributing

Issues and pull requests welcome. No CLA. The code style is enforced via
`golangci-lint` (`.golangci.yml`) and `gofmt`. Conventional commits preferred.

For substantial changes, open an issue first so we can align on direction.
KafGraph follows the design discipline in `CLAUDE.md`.

## Built by

KafGraph is built and maintained by [Scalytics](https://www.scalytics.io),
founded by the original creators of [Apache Wayang](https://wayang.apache.org).
The same engineering team maintains [KafScale](https://github.com/scalytics/kafscale)
and [KafClaw](https://github.com/scalytics/KafClaw). The release discipline that
ships Wayang code through the Apache Software Foundation process applies here:
race detection, coverage gates, security scanning, and reproducible builds.

## License

Apache 2.0. See [LICENSE](LICENSE).

## Links

* Product page: <https://www.scalytics.io/en-gb/kafgraph>
* Documentation: `docs/`
* Issues: <https://github.com/scalytics/KafGraph/issues>
* Scalytics: <https://www.scalytics.io>
