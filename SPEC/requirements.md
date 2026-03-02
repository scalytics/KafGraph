# KafGraph — Requirements

*Version: 0.1-draft — 2026-03-02*

---

## 1. Purpose and Scope

KafGraph is a distributed graph database and reflection engine for AI agent teams.
It ingests agent conversation data from Apache Kafka Topics, structures that data as
a property graph, and provides temporal reflection services (daily / weekly / monthly)
that allow individual agents and teams to learn from their past interactions.

This document defines functional requirements (FR), non-functional requirements (NFR),
and integration requirements (IR) for the first production-grade iteration of KafGraph.

---

## 2. Stakeholders

| Role | Concern |
|------|---------|
| Agent Runtime | Writes conversation events to Kafka; reads reflection results |
| Agent Operator / Owner | Reviews learning signals; provides human feedback on demand |
| KafClaw Platform | Produces group topics (`group.<name>.*`) with `GroupEnvelope` JSON wire format — see `kafclaw-topic-reference.md` |
| KafScale Infrastructure | KafGraph runs as a KafScale Processor (5-layer stack); co-located with S3 segments; uses LFS Proxy for large payloads |
| Graph Consumers (tools, dashboards) | Query the graph via Bolt / Cypher or REST API |

---

## 3. Functional Requirements

### 3.1 Data Ingestion

| ID | Requirement |
|----|-------------|
| FR-ING-01 | The system MUST consume conversation events from KafClaw group topics using the `group.<group_name>.*` naming convention. Primary topics: `announce`, `requests`, `responses`, `tasks.status`, `skill.*.requests`, `skill.*.responses`, `memory.shared`. See `kafclaw-topic-reference.md`. |
| FR-ING-02 | The system MUST support KafClaw's `GroupEnvelope` JSON wire format without requiring schema changes in KafClaw. |
| FR-ING-03 | The system MUST consume enrichment events from KafClaw topics: `traces`, `observe.audit`, `control.roster`, `orchestrator`. |
| FR-ING-04 | The system MUST be implemented as a **KafScale Processor**, consuming S3 segments directly using the 5-layer processor stack (Discovery, Decoder, Checkpoint, Sink, Locking) without routing data through Kafka brokers. |
| FR-ING-05 | The system MUST handle out-of-order and late-arriving messages with configurable tolerance windows. |
| FR-ING-06 | The system MUST be idempotent: replaying the same offset range MUST NOT produce duplicate graph nodes or edges. |
| FR-ING-07 | The system MUST support at-least-once delivery semantics with deduplication at the graph layer. |
| FR-ING-08 | The system MUST dynamically subscribe to new KafClaw skill topics by consuming the `group.<group_name>.control.roster` topic and tracking `TopicManifest` updates. |
| FR-ING-09 | The system MUST resolve KafScale LFS envelopes (Claim Check pattern) during ingestion to retrieve large payloads from S3. |
| FR-ING-10 | The Reflection Scheduler MUST use an **Isolated Historic Iterator** with a separate checkpoint namespace, reading S3 segments in a defined time window without interfering with real-time ingestion. |

### 3.2 Graph Model

| ID | Requirement |
|----|-------------|
| FR-GM-01 | The graph MUST be a **labeled property graph** (nodes and directed edges each carry a label and an arbitrary set of key-value properties). |
| FR-GM-02 | The following built-in node types MUST exist: `Agent`, `Conversation`, `Message`, `HumanFeedback`, `ReflectionCycle`, `LearningSignal`, `Skill`, `SharedMemory`, `AuditEvent`. |
| FR-GM-03 | The following built-in edge types MUST exist: `AUTHORED`, `REPLIED_TO`, `BELONGS_TO`, `HAS_FEEDBACK`, `TRIGGERED_REFLECTION`, `LINKS_TO`, `CONTRIBUTED_VALUE`, `USES_SKILL`, `SHARED_BY`, `REFERENCES`, `DELEGATES_TO`, `REPORTS_TO`. |
| FR-GM-04 | Every node and edge MUST carry a `createdAt` timestamp and a `sourceOffset` (Kafka topic + partition + offset) for lineage tracing. |
| FR-GM-05 | Edges MUST support three qualitative weight dimensions: `impact` (float 0–1), `relevance` (float 0–1), `valueContribution` (float 0–1). |
| FR-GM-06 | The graph schema MUST be extensible: consumers MUST be able to add custom node/edge labels and properties without system restart. |
| FR-GM-07 | The system MUST support full ACID transactions for multi-node/multi-edge writes within a single Processor instance. |

### 3.3 Reflection Service

| ID | Requirement |
|----|-------------|
| FR-REF-01 | The system MUST execute a **daily reflection cycle** for each active agent at a configurable time-of-day. |
| FR-REF-02 | The system MUST execute a **weekly reflection cycle** for each active agent, rolling up daily signals. |
| FR-REF-03 | The system MUST execute a **monthly reflection cycle** for each active agent, rolling up weekly signals. |
| FR-REF-04 | Each reflection cycle MUST produce a `ReflectionCycle` node linked to all `Message` and `Conversation` nodes in scope. |
| FR-REF-05 | Reflection MUST be **self-directed**: each agent reflects on its own activity within the cycle window. |
| FR-REF-06 | Reflection MUST be **cross-directed**: each agent reflects on the activity and contributions of every other agent in its team within the cycle window. |
| FR-REF-07 | Reflection results MUST be materialised as `LearningSignal` nodes attached to relevant conversation sub-graphs. |
| FR-REF-08 | The reflection engine MUST score each examined conversation segment on `impact`, `relevance`, and `valueContribution` and persist these scores as edge weights on `LINKS_TO` / `CONTRIBUTED_VALUE` edges. |
| FR-REF-09 | All reflection cycles MUST be idempotent: re-running a cycle for the same window MUST update existing `ReflectionCycle` nodes rather than creating duplicates. |
| FR-REF-10 | Reflection execution MUST be triggered by both schedule (cron) and explicit API call. |

### 3.4 Human Feedback Loop

| ID | Requirement |
|----|-------------|
| FR-HF-01 | Each `ReflectionCycle` node MUST carry a `humanFeedbackStatus` property: one of `PENDING`, `REQUESTED`, `RECEIVED`, `WAIVED`. |
| FR-HF-02 | If a reflection cycle completes without any `HumanFeedback` node attached, the system MUST emit a feedback-request event to a dedicated Kafka topic within a configurable grace period (default: 24 h). |
| FR-HF-03 | The feedback-request event MUST include: agent ID, cycle ID, cycle window, a summary of the top-N learning signals that require validation, and the contact references for the **team leader and designated experts** (configurable per group). |
| FR-HF-04 | The system MUST accept inbound `HumanFeedback` events from a dedicated Kafka topic and attach them to the appropriate `ReflectionCycle` and `LearningSignal` nodes. |
| FR-HF-05 | Human feedback MUST track both **positive impact** and **negative impact** of agent actions. Feedback MUST be able to **override** or **confirm** any automatically computed `impact`, `relevance`, or `valueContribution` score. |
| FR-HF-06 | The system MUST NOT mark a `ReflectionCycle` as `COMPLETE` until either feedback is received or the owner explicitly waives it by emitting a `WAIVED` event. |
| FR-HF-07 | The feedback owner MUST be configurable per group: the **team leader** is the default recipient, with the ability to designate additional **expert reviewers** per agent group. |

### 3.5 Query Interface

| ID | Requirement |
|----|-------------|
| FR-QI-01 | The system MUST expose its **own graph query endpoint** using a well-accepted graph query language (OpenCypher). This is KafGraph's primary query surface — autonomous, no external database dependency. |
| FR-QI-02 | The system MUST support at minimum Cypher clauses: `MATCH`, `WHERE`, `RETURN`, `CREATE`, `MERGE`, `SET`, `DELETE`, `WITH`, `UNWIND`, `ORDER BY`, `LIMIT`, `SKIP`. |
| FR-QI-03 | The system MUST expose an HTTP REST API for health, metrics, and basic CRUD on nodes/edges. |
| FR-QI-04 | The query interface MUST support authenticated sessions (username + password; token-based auth is optional in v1). |
| FR-QI-05 | The system SHOULD expose a GraphQL endpoint for structured reflection queries (optional for v1, required for v2). |
| FR-QI-06 | The system MUST support **embedding-based queries** (vector similarity search): given an embedding vector, return the top-K most similar nodes. Embeddings MUST be storable as node properties and indexed for efficient approximate nearest-neighbour (ANN) search. |
| FR-QI-07 | The system MUST support **full-text search** on text properties of graph nodes (e.g., `Message.content`, `SharedMemory.title`, `LearningSignal.summary`). Full-text queries MUST be invocable from within Cypher (e.g., via a `CALL` procedure or custom function). |
| FR-QI-08 | The system SHOULD support a **Bolt v4-compatible** wire protocol (port 7687) so that standard Neo4j drivers can connect. This enables optional use of Neo4j Browser or other Bolt-compatible tooling. |
| FR-QI-09 | External graph databases (Neo4j, TigerGraph) SHOULD be supported as **optional export/sync targets** for organisations that require them. This is NOT a dependency — KafGraph MUST be fully functional without any external graph database. |

### 3.6 Agent Brain (Tool API)

| ID | Requirement |
|----|-------------|
| FR-AB-01 | The system MUST expose brain capabilities as **tool definitions** (JSON-schema-described functions) callable via HTTP (`POST /api/v1/tools/{toolName}`) and via **KafClaw skill routing** (skill name: `kafgraph_brain`). No protocol middleware (MCP or otherwise) is required. |
| FR-AB-02 | The system MUST expose a `brain_search` tool that performs **semantic search** (vector similarity) across the agent's knowledge graph. Agents MUST be able to find nodes by meaning, not just keywords. |
| FR-AB-03 | The system MUST expose a `brain_recall` tool that loads **accumulated context** for a specific agent: active conversations, recent decisions, pending feedback, team context, and unresolved threads. This is the "no more starting from zero" capability. |
| FR-AB-04 | The system MUST expose a `brain_capture` tool that allows agents to **write insights, decisions, and observations** directly into the brain. Captured items MUST be auto-embedded, auto-classified, and auto-linked to related graph nodes via vector similarity. |
| FR-AB-05 | The system MUST expose a `brain_recent` tool for browsing recent activity within a configurable time window. |
| FR-AB-06 | The system MUST expose a `brain_patterns` tool that surfaces **recurring themes, connections, and patterns** from the knowledge graph using reflection cycle results and cross-agent links. |
| FR-AB-07 | The system MUST expose a `brain_reflect` tool that triggers an **on-demand reflection cycle** and returns the results inline. |
| FR-AB-08 | The system MUST expose a `brain_feedback` tool for submitting human feedback on reflection cycles directly through the tool API. |
| FR-AB-09 | The tool schemas MUST be served at `GET /api/v1/tools` in standard LLM tool-call format (OpenAI/Anthropic compatible) so any agent runtime can fetch and register them dynamically. |
| FR-AB-10 | The system MUST **register `kafgraph_brain` as a KafClaw skill** on startup, creating `group.<name>.skill.kafgraph_brain.requests/responses` topics and publishing to the roster. Any agent in the group MUST be able to discover and call brain tools via existing KafClaw skill routing. |
| FR-AB-11 | The brain MUST be **self-owned infrastructure** with no SaaS dependency. Switching AI providers (Claude, GPT, Gemini, local models) MUST NOT lose any brain context — any agent that can make tool calls can access the same brain. |
| FR-AB-12 | Every text node ingested or captured MUST be automatically embedded (vector representation) and indexed for semantic search. The embedding endpoint MUST be configurable (local Ollama, OpenAI-compatible API, etc.). |
| FR-AB-13 | The `brain_capture` tool MUST publish captured insights to a `kafgraph.brain-captures` Kafka topic for cluster-wide synchronization in distributed mode. |

### 3.7 Per-Agent Mode

| ID | Requirement |
|----|-------------|
| FR-PA-01 | The system MUST be runnable as a **lightweight embedded process** co-located with a single agent runtime (per-agent mode). |
| FR-PA-02 | In per-agent mode, the graph MUST be stored on local disk with optional in-memory overlay for hot data. |
| FR-PA-03 | Per-agent mode MUST support all reflection, feedback, and agent brain requirements (FR-REF-*, FR-HF-*, FR-AB-*). |
| FR-PA-04 | Per-agent instances MUST be able to **sync selectively** to the distributed cluster: publishing reflection results, learning signals, and brain captures without exposing raw conversation data unless explicitly configured. |
| FR-PA-05 | Per-agent mode MUST have a memory footprint below 512 MB under normal conversation volumes (~10 events/minute/agent, ~14.4k events/day). |
| FR-PA-06 | In per-agent mode, the Brain Tool API MUST be the agent's **local brain** — accessible only to the co-located agent runtime via direct HTTP calls. The brain grows with every interaction and persists across sessions. |

### 3.8 Distributed / Collaborative Mode

| ID | Requirement |
|----|-------------|
| FR-DM-01 | The system MUST support a **multi-node cluster** where graph partitions are distributed across KafScale Processor nodes. |
| FR-DM-02 | Graph partitioning MUST be based on agent ID by default, with pluggable partition strategies. |
| FR-DM-03 | The cluster MUST use Kafka topics for inter-node coordination (leader election, partition assignment, checkpoint broadcasting). |
| FR-DM-04 | The cluster MUST remain available for reads during a minority node failure (i.e., reads are served from replicas). |
| FR-DM-05 | Writes MUST be synchronously replicated to at least one additional node before acknowledgement (configurable replication factor ≥ 2). |
| FR-DM-06 | The cluster MUST support **cross-agent reflection queries**: a single Cypher query spanning multiple agent partitions. |
| FR-DM-07 | The system MUST expose a cluster-wide Bolt endpoint that routes queries to the correct partition(s) transparently. |

---

## 4. Non-Functional Requirements

### 4.1 Performance

| ID | Requirement |
|----|-------------|
| NFR-PERF-01 | Ingestion latency from S3 segment availability to graph node creation MUST be < 10 s at p99 (bounded by the 5 s processor poll interval). |
| NFR-PERF-02 | A daily reflection cycle over 14.4k events/agent MUST complete within 10 minutes on a single Processor node (4 vCPU, 8 GB RAM). At expected volume (~10 events/min/agent), this is well within budget. |
| NFR-PERF-03 | Simple Cypher point-lookups (single node by ID) MUST return in < 10 ms at p99. |
| NFR-PERF-04 | Cross-agent graph traversals (depth ≤ 5) MUST complete in < 2 s at p99 in distributed mode. |
| NFR-PERF-05 | Reflection results MUST be available for querying within **1 day** after the cycle window ends. |
| NFR-PERF-06 | Embedding-based ANN queries (top-K similarity) MUST return in < 100 ms at p99 for graphs with up to 1M vector-indexed nodes. |
| NFR-PERF-07 | Full-text search queries MUST return in < 200 ms at p99. |

### 4.2 Reliability

| ID | Requirement |
|----|-------------|
| NFR-REL-01 | The system MUST guarantee no data loss for messages consumed from Kafka (at-least-once with deduplication). |
| NFR-REL-02 | In per-agent mode, the system MUST recover to a consistent state after a crash without manual intervention, using WAL or equivalent. |
| NFR-REL-03 | In distributed mode, the system MUST tolerate loss of up to (N-1)/2 nodes without data loss (quorum-based). |

### 4.3 Scalability

| ID | Requirement |
|----|-------------|
| NFR-SCALE-01 | The distributed cluster MUST scale horizontally by adding Processor nodes without downtime. |
| NFR-SCALE-02 | Each Processor node MUST be capable of handling at least 50k message-ingestion events per second. |
| NFR-SCALE-03 | The graph MUST support at least 1 billion nodes and 10 billion edges per cluster without schema changes. |

### 4.4 Observability

| ID | Requirement |
|----|-------------|
| NFR-OBS-01 | The system MUST expose Prometheus-compatible metrics at `/metrics`. |
| NFR-OBS-02 | Metrics MUST include: ingestion lag, graph write throughput, reflection cycle duration, query latency percentiles, human feedback outstanding count. |
| NFR-OBS-03 | Structured JSON logs MUST be emitted to stdout, compatible with common log aggregators (Loki, OpenSearch). |
| NFR-OBS-04 | Distributed traces MUST be emitted using OpenTelemetry (OTLP) for reflection cycle execution and cross-partition queries. |

### 4.5 Security

| ID | Requirement |
|----|-------------|
| NFR-SEC-01 | All inter-node communication MUST be encrypted (TLS 1.3 minimum). |
| NFR-SEC-02 | Kafka consumer credentials MUST be managed via environment variables or a secret store; they MUST NOT appear in configuration files in plaintext. |
| NFR-SEC-03 | The Bolt interface MUST support TLS-encrypted connections. |
| NFR-SEC-04 | Data at rest on disk MUST support optional encryption (AES-256). |
| NFR-SEC-05 | Access to human feedback topics MUST be restricted to authorised identities only. |

### 4.6 Operability

| ID | Requirement |
|----|-------------|
| NFR-OPS-01 | The system MUST be distributable as a single statically-linked binary with no external runtime dependencies. |
| NFR-OPS-02 | Configuration MUST be driven by a single YAML file with environment-variable overrides. |
| NFR-OPS-03 | The system MUST expose a `/healthz` (liveness) and `/readyz` (readiness) HTTP endpoint. |
| NFR-OPS-04 | Cluster membership changes (node add / remove) MUST be achievable with zero planned downtime. |

---

## 5. Integration Requirements

| ID | Requirement |
|----|-------------|
| IR-01 | KafGraph MUST integrate with KafClaw's `group.<group_name>.*` topic hierarchy and `GroupEnvelope` JSON wire format without requiring KafClaw source changes. |
| IR-02 | KafGraph MUST implement the KafScale Processor 5-layer stack (Discovery, Decoder, Checkpoint, Sink, Locking) for direct S3 segment consumption, sharing interfaces with the processor skeleton at `platform/addons/processors/skeleton/`. |
| IR-03 | KafGraph MUST be deployable alongside a KafScale broker node on the same host without port conflicts (configurable port bindings). |
| IR-04 | KafGraph MUST emit feedback-request events in a format consumable by the KafClaw notification subsystem. |
| IR-05 | KafGraph MUST support the official Neo4j Go driver as a client without modification. |
| IR-06 | KafGraph SHOULD provide a Helm chart for Kubernetes deployment collocated with KafScale StatefulSets. |
| IR-07 | KafGraph MUST resolve KafScale LFS envelopes using the KafScale LFS Proxy API for large payloads stored in S3. |
| IR-08 | KafGraph MUST consume KafClaw `TopicManifest` messages from the roster topic to auto-discover skill topics registered by agents. |

---

## 6. Constraints

- **Implementation language**: Go (1.22+)
- **Minimum Go version**: 1.22 (for range-over-func and improved structured concurrency)
- **KafScale version**: 2.7.0
- **Object storage**: MinIO (S3-compatible)
- **Kafka protocol**: Apache Kafka 3.x (KRaft mode supported; ZooKeeper not required)
- **Storage backend**: pluggable, with BadgerDB as the default embedded engine
- **Graph wire protocol**: Bolt v4+ (Neo4j compatible) — optional, not a hard dependency
- **Graph query language**: OpenCypher (subset for v1) with extensions for embedding-based queries and full-text search
- **Expected volume**: ~10 events/minute/agent (~14.4k events/day/agent)
- **License**: to be decided by project owner before first public release

---

## 7. Out of Scope (v1)

- LLM-based automatic reflection scoring (v1 uses heuristic scoring only; LLM integration is a v2 feature)
- Multi-tenancy (v1 assumes a single organisational tenant per cluster)
- Graph visualisation UI (external tools such as Neo4j Browser or Bloom can connect via Bolt)
- Real-time streaming Cypher subscriptions (planned for v2)
- Fine-grained RBAC (v1 supports single shared credentials only)
- Neo4j / TigerGraph as mandatory dependencies (they are **optional export targets** only — KafGraph is fully autonomous)

---

*Next: see `solution-design.md` for the architectural approach.*
