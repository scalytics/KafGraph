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
| KafClaw Platform | Produces conversation and audit topics consumed by KafGraph |
| KafScale Infrastructure | Hosts Processors collocated with KafGraph; manages S3 storage |
| Graph Consumers (tools, dashboards) | Query the graph via Bolt / Cypher or REST API |

---

## 3. Functional Requirements

### 3.1 Data Ingestion

| ID | Requirement |
|----|-------------|
| FR-ING-01 | The system MUST consume conversation events from one or more named Kafka topics. |
| FR-ING-02 | The system MUST support KafClaw's existing topic schema for agent conversations without requiring schema changes in KafClaw. |
| FR-ING-03 | The system MUST consume audit events from KafClaw long-term audit topics. |
| FR-ING-04 | The system MUST support direct consumption from S3 (via KafScale Processor API) without routing data through Kafka brokers again. |
| FR-ING-05 | The system MUST handle out-of-order and late-arriving messages with configurable tolerance windows. |
| FR-ING-06 | The system MUST be idempotent: replaying the same Kafka offset range MUST NOT produce duplicate graph nodes or edges. |
| FR-ING-07 | The system MUST support at-least-once delivery semantics with deduplication at the graph layer. |

### 3.2 Graph Model

| ID | Requirement |
|----|-------------|
| FR-GM-01 | The graph MUST be a **labeled property graph** (nodes and directed edges each carry a label and an arbitrary set of key-value properties). |
| FR-GM-02 | The following built-in node types MUST exist: `Agent`, `Conversation`, `Message`, `HumanFeedback`, `ReflectionCycle`, `LearningSignal`. |
| FR-GM-03 | The following built-in edge types MUST exist: `AUTHORED`, `REPLIED_TO`, `BELONGS_TO`, `HAS_FEEDBACK`, `TRIGGERED_REFLECTION`, `LINKS_TO`, `CONTRIBUTED_VALUE`. |
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
| FR-HF-03 | The feedback-request event MUST include: agent ID, cycle ID, cycle window, a summary of the top-N learning signals that require validation, and the owner's contact reference. |
| FR-HF-04 | The system MUST accept inbound `HumanFeedback` events from a dedicated Kafka topic and attach them to the appropriate `ReflectionCycle` and `LearningSignal` nodes. |
| FR-HF-05 | Human feedback MUST be able to **override** or **confirm** any automatically computed `impact`, `relevance`, or `valueContribution` score. |
| FR-HF-06 | The system MUST NOT mark a `ReflectionCycle` as `COMPLETE` until either feedback is received or the owner explicitly waives it by emitting a `WAIVED` event. |

### 3.5 Query Interface

| ID | Requirement |
|----|-------------|
| FR-QI-01 | The system MUST expose a **Neo4j Bolt-compatible** query interface (port 7687 by default) accepting Cypher queries. |
| FR-QI-02 | The system MUST support at minimum Cypher clauses: `MATCH`, `WHERE`, `RETURN`, `CREATE`, `MERGE`, `SET`, `DELETE`, `WITH`, `UNWIND`, `ORDER BY`, `LIMIT`, `SKIP`. |
| FR-QI-03 | The system MUST expose an HTTP REST API for health, metrics, and basic CRUD on nodes/edges. |
| FR-QI-04 | The Bolt interface MUST support authenticated sessions (username + password; token-based auth is optional in v1). |
| FR-QI-05 | The system SHOULD expose a GraphQL endpoint for structured reflection queries (optional for v1, required for v2). |

### 3.6 Per-Agent Mode

| ID | Requirement |
|----|-------------|
| FR-PA-01 | The system MUST be runnable as a **lightweight embedded process** co-located with a single agent runtime (per-agent mode). |
| FR-PA-02 | In per-agent mode, the graph MUST be stored on local disk with optional in-memory overlay for hot data. |
| FR-PA-03 | Per-agent mode MUST support all reflection and feedback requirements (FR-REF-*, FR-HF-*). |
| FR-PA-04 | Per-agent instances MUST be able to **sync selectively** to the distributed cluster: publishing reflection results and learning signals without exposing raw conversation data unless explicitly configured. |
| FR-PA-05 | Per-agent mode MUST have a memory footprint below 512 MB under normal conversation volumes (< 10k messages/day). |

### 3.7 Distributed / Collaborative Mode

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
| NFR-PERF-01 | Ingestion latency from Kafka commit to graph node availability MUST be < 500 ms at p99 under normal load. |
| NFR-PERF-02 | A daily reflection cycle over 100k messages MUST complete within 10 minutes on a single Processor node (4 vCPU, 8 GB RAM). |
| NFR-PERF-03 | Simple Cypher point-lookups (single node by ID) MUST return in < 10 ms at p99. |
| NFR-PERF-04 | Cross-agent graph traversals (depth ≤ 5) MUST complete in < 2 s at p99 in distributed mode. |

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
| IR-01 | KafGraph MUST integrate with KafClaw's Kafka topic naming convention without requiring KafClaw source changes. |
| IR-02 | KafGraph MUST integrate with KafScale Processor API for direct S3 segment reads. |
| IR-03 | KafGraph MUST be deployable alongside a KafScale broker node on the same host without port conflicts (configurable port bindings). |
| IR-04 | KafGraph MUST emit feedback-request events in a format consumable by the KafClaw notification subsystem. |
| IR-05 | KafGraph MUST support the official Neo4j Go driver as a client without modification. |
| IR-06 | KafGraph SHOULD provide a Helm chart for Kubernetes deployment collocated with KafScale StatefulSets. |

---

## 6. Constraints

- **Implementation language**: Go (1.22+)
- **Minimum Go version**: 1.22 (for range-over-func and improved structured concurrency)
- **Kafka protocol**: Apache Kafka 3.x (KRaft mode supported; ZooKeeper not required)
- **Storage backend**: pluggable, with BadgerDB as the default embedded engine
- **Graph wire protocol**: Bolt v4+ (Neo4j compatible)
- **Graph query language**: OpenCypher (subset sufficient for v1)
- **License**: to be decided by project owner before first public release

---

## 7. Out of Scope (v1)

- LLM-based automatic reflection scoring (v1 uses heuristic scoring only; LLM integration is a v2 feature)
- Multi-tenancy (v1 assumes a single organisational tenant per cluster)
- Graph visualisation UI (external tools such as Neo4j Browser or Bloom can connect via Bolt)
- Real-time streaming Cypher subscriptions (planned for v2)
- Fine-grained RBAC (v1 supports single shared credentials only)

---

*Next: see `solution-design.md` for the architectural approach.*
