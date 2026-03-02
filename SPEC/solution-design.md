# KafGraph — Solution Design

*Version: 0.1-draft — 2026-03-02*

---

## 1. Design Goals

> **KafGraph becomes the distributed shared brain of collaborating agents.**

1. **Agent Brain**: KafGraph is the self-owned, persistent, agent-accessible knowledge system for every agent in a team. No agent ever starts from zero. No platform lock-in. The brain compounds with every interaction.
2. **Tool-native**: the brain is accessed through **KafGraph tool calls** — callable functions that agents invoke directly via the KafGraph API or via KafClaw skill routing. No protocol middleware. The tool schemas are LLM-friendly so any agent runtime can register and call them.
3. **Kafka-native**: every subsystem communicates through Kafka topics; no RPC fabric is needed for the data plane.
4. **Co-located with KafScale**: Processors sit next to S3 data; brokers are never overloaded by reflection workloads.
5. **Per-agent + distributed**: the same binary runs as a lightweight embedded instance (the agent's local brain) or as a cluster shard (the team's shared brain).
6. **Learning-first**: reflection and feedback cycles are first-class, not afterthoughts. The brain learns what had positive and negative impact through human feedback.

---

## 2. Technology Selection

### 2.1 Existing Go Graph Systems — Research Summary

| Project | Type | Neo4j Bolt | Kafka | Maturity | Notes |
|---------|------|-----------|-------|----------|-------|
| `neo4j/neo4j-go-driver` | Client driver | Yes (v4+) | No | Production | Official; connects to external Neo4j |
| `mindstand/go-bolt` | Bolt client library | Yes (v1–4) | No | Stable | Community; implements Bolt wire protocol |
| `dgraph-io/dgraph` | Distributed graph DB | No (GraphQL+gRPC) | No | Production | Go-native, horizontally scalable; not Bolt |
| `krotik/eliasdb` | Embedded graph DB | No | No | Stable | GraphQL API; custom query language (EQL) |
| `kuzudb/kuzu` | Embedded analytical graph | Partial (Cypher) | No | Active dev | Embedded; excellent for analytics; C++ core |
| `dominikbraun/graph` | Graph library (in-memory) | No | No | Active | Data structures only; not a database |
| `lovoo/goka` | Kafka stream processor | No | Yes | Production | Has graph.go; partition-aware state |
| `segmentio/kafka-go` | Kafka client | No | Yes | Production | Low + high-level Kafka API |
| Memgraph | Graph DB | Yes (Bolt v4) | Yes (streams) | Production | C++ core; Go driver available; Kafka native |

### 2.2 Decision: Build a Bolt-compatible server in Go backed by BadgerDB

**Rationale:**

- **Neo4j itself** is a JVM application — embedding it in a Go binary is not feasible.
- **Memgraph** is the closest to what we need (Bolt v4 + Kafka native) but is C++ and
  not embeddable. It is retained as a **reference implementation** and **compatibility
  target**: KafGraph will pass Memgraph's Bolt integration tests.
- **Dgraph** is production-grade Go but uses GraphQL/gRPC, not Bolt/Cypher. Adopting it
  would break the Neo4j driver compatibility requirement.
- **EliasDB** and **Kuzu** lack Bolt protocol support.
- **KafGraph's approach**: implement an OpenCypher-over-Bolt server in Go using:
  - `BadgerDB` as the embedded key-value storage engine (production-grade, LSM-tree, pure Go)
  - `goka` for Kafka partition-aware stream processing
  - A purpose-built Bolt v4 server (implementing the Bolt handshake and message framing)
  - A subset of the openCypher grammar (using ANTLR4-generated Go parser or hand-written recursive descent)

This gives us a **single statically-linked Go binary** that speaks Bolt, consumes Kafka,
and co-locates with KafScale.

### 2.3 Key Libraries

| Library | Role | Why |
|---------|------|-----|
| `dgraph-io/badger/v4` | Local storage engine | Pure Go, LSM-tree, embeddable, ACID |
| KafScale Processor SDK | S3 segment processing | Provides Discovery, Decoder, Checkpoint interfaces; shared with Iceberg/SQL processors |
| `segmentio/kafka-go` | Low-level Kafka I/O | Topic admin, human-feedback consumer, reflection-signals producer |
| `minio/minio-go/v7` | MinIO / S3 access | Direct segment reads for the Discovery and Decoder layers; MinIO is the target object store |
| `blevesearch/bleve/v2` | Full-text search | Pure Go, embeddable, supports BM25 scoring, integrates with BadgerDB as KV store |
| HNSW library (e.g., `coder/hnsw` or custom) | Vector similarity index | Approximate nearest-neighbour for embedding-based queries |
| `antlr4-go/antlr` | Cypher grammar parsing | ANTLR4 runtime for Go; OpenCypher grammar available |
| `prometheus/client_golang` | Metrics | Standard Prometheus instrumentation |
| `open-telemetry/opentelemetry-go` | Distributed tracing | OTLP export |
| `hashicorp/memberlist` | Cluster membership (gossip) | Lightweight; no ZooKeeper dependency |
| `spf13/viper` | Configuration | YAML + env-var override |

> **Note**: The original design used `lovoo/goka` for Kafka partition processing.
> This has been replaced by the KafScale Processor pattern, which reads directly
> from S3 segments rather than consuming from Kafka brokers. The `segmentio/kafka-go`
> library is retained for producing to KafGraph's own topics and consuming the
> human-feedback topic.

---

## 3. Architecture Overview

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                           KafScale Processor Node                                │
│                                                                                  │
│  ┌────────────────────────────────────────────────────────────────────────────┐  │
│  │                          KafGraph Process                                  │  │
│  │                                                                            │  │
│  │  ┌────────────────────────────────────┐   ┌────────────────────────────┐  │  │
│  │  │    KafScale Processor Stack        │   │       Bolt Server          │  │  │
│  │  │                                    │   │       (port 7687)          │  │  │
│  │  │  ┌────────────┐                    │   │                            │  │  │
│  │  │  │ Discovery   │ S3 segment list   │   │  Cypher queries ──────┐   │  │  │
│  │  │  └─────┬──────┘                    │   │                       │   │  │  │
│  │  │        ▼                           │   └───────────────────────┼───┘  │  │
│  │  │  ┌────────────┐                    │                           │      │  │
│  │  │  │ Decoder     │ .kfs parsing      │                           ▼      │  │
│  │  │  └─────┬──────┘                    │   ┌────────────────────────────┐  │  │
│  │  │        ▼                           │   │       Graph Engine         │  │  │
│  │  │  ┌────────────┐                    │   │   (BadgerDB + indexes)     │  │  │
│  │  │  │ Checkpoint  │ lease + offsets   │   │                            │  │  │
│  │  │  └─────┬──────┘                    │   └──────────┬─────────────────┘  │  │
│  │  │        ▼                           │              │                    │  │
│  │  │  ┌────────────┐  GroupEnvelope     │              │                    │  │
│  │  │  │ Graph Sink  │─ routing ─────────┼──────────────┘                    │  │
│  │  │  │ (Writer)    │                   │                                   │  │
│  │  │  └─────┬──────┘                    │   ┌────────────────────────────┐  │  │
│  │  │        ▼                           │   │  Reflection Scheduler      │  │  │
│  │  │  ┌────────────┐                    │   │  (daily / weekly / monthly)│  │  │
│  │  │  │ TopicLocker │ per-topic mutex   │   │                            │  │  │
│  │  │  └────────────┘                    │   │  Uses Isolated Iterator    │  │  │
│  │  │                                    │   │  (separate checkpoint NS)  │  │  │
│  │  └────────────────────────────────────┘   └──────────┬─────────────────┘  │  │
│  │                                                      │                    │  │
│  │  ┌──────────────────────┐   ┌────────────────────────▼─────────────────┐  │  │
│  │  │  HTTP API            │   │  Feedback Request Emitter                │  │  │
│  │  │  /healthz /readyz    │   │  (grace period → kafgraph.feedback-      │  │  │
│  │  │  /metrics            │   │   requests topic)                        │  │  │
│  │  └──────────────────────┘   └──────────────────────────────────────────┘  │  │
│  │                                                                            │  │
│  │  ┌──────────────────────────────────────────────────────────────────────┐  │  │
│  │  │  Brain Tool API (Agent Brain Interface)                              │  │  │
│  │  │  brain_search | brain_recall | brain_capture | brain_recent          │  │  │
│  │  │  brain_patterns | brain_reflect | brain_feedback                     │  │  │
│  │  │  Access: HTTP Tool API (/api/v1/tools) + KafClaw skill routing      │  │  │
│  │  └──────────────────────────────────────────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────────┘

         ▲ S3 segments                                ▲ Kafka topics
         │ (.kfs + .index)                            │
┌────────┴────────────────────────────────────────────┴──────────────────────────────┐
│                    Apache Kafka / KafScale Cluster (KRaft)                          │
│                                                                                    │
│  KafClaw group topics (per agent group):                                           │
│    group.<name>.announce        group.<name>.requests                              │
│    group.<name>.responses       group.<name>.tasks.status                          │
│    group.<name>.traces          group.<name>.control.roster                        │
│    group.<name>.observe.audit   group.<name>.memory.shared                         │
│    group.<name>.orchestrator    group.<name>.skill.<skill>.requests/responses       │
│                                                                                    │
│  KafGraph topics:                                                                  │
│    kafgraph.feedback-requests   kafgraph.human-feedback                            │
│    kafgraph.reflection-signals  kafgraph.cluster-coord                             │
│                                                                                    │
│  S3 tiered storage:                                                                │
│    s3://<bucket>/<ns>/<topic>/<partition>/segment-<offset>.kfs                      │
│    s3://<bucket>/<ns>/<topic>/<partition>/segment-<offset>.index                    │
└────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Component Design

### 4.1 Kafka Ingest Layer — KafScale Processor Architecture

KafGraph's ingest layer is a **KafScale Processor** — the same abstraction used by
KafScale's Iceberg and SQL processors. This is not merely an API client: KafGraph
implements the full 5-layer KafScale processor stack, with the Graph Engine as its Sink.

#### 4.1.1 Processor Stack (5 Layers)

KafGraph implements the standard KafScale processor interfaces (as defined in
`platform/addons/processors/skeleton/`):

```
┌─────────────────────────────────────────────────────────┐
│                  KafGraph Processor                      │
│                                                          │
│  ┌──────────────┐                                       │
│  │  Discovery    │  Lists completed S3 segments          │
│  │  (Lister)     │  per topic / partition                │
│  └──────┬───────┘                                       │
│         ▼                                                │
│  ┌──────────────┐                                       │
│  │  Decoder      │  Parses KafScale binary segment       │
│  │  (Decoder)    │  format (.kfs + .index)              │
│  └──────┬───────┘                                       │
│         ▼                                                │
│  ┌──────────────┐                                       │
│  │  Checkpoint   │  Lease-based ownership per partition   │
│  │  (Store)      │  + offset persistence (etcd or local) │
│  └──────┬───────┘                                       │
│         ▼                                                │
│  ┌──────────────┐                                       │
│  │  Graph Sink   │  Deserializes GroupEnvelope JSON,      │
│  │  (Writer)     │  upserts nodes/edges in BadgerDB      │
│  └──────┬───────┘                                       │
│         ▼                                                │
│  ┌──────────────┐                                       │
│  │  TopicLocker  │  Per-topic mutex for ordered writes    │
│  │  (Locking)    │                                       │
│  └──────────────┘                                       │
└─────────────────────────────────────────────────────────┘
```

**Interface contracts** (matching KafScale's processor skeleton):

```go
// Discovery — lists completed S3 segments
type Lister interface {
    ListCompleted(ctx context.Context) ([]SegmentRef, error)
}

// Decoding — parses KafScale binary segment format
type Decoder interface {
    Decode(ctx context.Context, segmentKey, indexKey string) ([]Record, error)
}

// Checkpointing — lease + offset persistence
type Store interface {
    ClaimLease(ctx, topic, partition, ownerID) (Lease, error)
    RenewLease(ctx, lease) error
    ReleaseLease(ctx, lease) error
    LoadOffset(ctx, topic, partition) (OffsetState, error)
    CommitOffset(ctx, state) error
}

// Sink — KafGraph's custom implementation writes to the Graph Engine
type Writer interface {
    Write(ctx context.Context, records []Record) error
    Close(ctx context.Context) error
}
```

#### 4.1.2 Processor Run Loop

The KafGraph processor follows the standard KafScale run loop (5-second poll):

```
1. List completed segments from S3 (Discovery)
2. For each partition without a lease, claim one (Checkpoint — TTL-based)
3. Renew lease in background goroutine
4. Decode segments and filter by last committed offset (Decoder + Checkpoint)
5. Deserialize GroupEnvelope JSON from each Record
6. Route envelope by Type to the appropriate graph write handler (Graph Sink)
7. Commit offset only AFTER successful graph write (Checkpoint)
8. Handle lease expiration gracefully (release + re-claim)
```

This guarantees **at-least-once delivery with deduplication at the graph layer**:
offsets are only committed after the graph write succeeds, and the graph engine
deduplicates by `sourceOffset` (topic + partition + offset).

#### 4.1.3 Graph Sink — Envelope Routing

The Graph Sink deserializes each KafScale `Record` value as a KafClaw
`GroupEnvelope` (JSON) and routes by envelope type:

| Envelope Type | Graph Action |
|---------------|-------------|
| `announce` (join) | Upsert `Agent` node with identity properties |
| `announce` (leave) | Set `Agent.status = inactive` |
| `announce` (heartbeat) | Update `Agent.lastSeenAt` |
| `request` | Create `Conversation` + `Message` nodes, `AUTHORED` + `BELONGS_TO` edges |
| `response` | Create `Message` node, `REPLIED_TO` + `AUTHORED` + `BELONGS_TO` edges |
| `task_status` | Update `Conversation.status` property |
| `skill_request` | Create `Conversation` + `Message` + `Skill` nodes, `USES_SKILL` edge |
| `skill_response` | Create `Message` node, `REPLIED_TO` edge |
| `memory` | Create `SharedMemory` node, resolve LFS envelope, `SHARED_BY` edge |
| `trace` | Annotate `Message` / `Conversation` nodes with timing properties |
| `audit` | Create `AuditEvent` node, link to `Agent` |
| `roster` | Update local topic manifest — auto-subscribe to new skill topics |
| `orchestrator` | Create `DELEGATES_TO` / `REPORTS_TO` edges between `Agent` nodes |

#### 4.1.4 KafClaw Topic Subscription

KafGraph consumes from the full set of KafClaw group topics (see
`kafclaw-topic-reference.md` for the complete topic model):

**Primary topics** (consumed by default):
```
group.<group_name>.announce
group.<group_name>.requests
group.<group_name>.responses
group.<group_name>.tasks.status
group.<group_name>.skill.*.requests
group.<group_name>.skill.*.responses
group.<group_name>.memory.shared
```

**Enrichment topics** (consumed when `enrichment.enabled: true`):
```
group.<group_name>.traces
group.<group_name>.observe.audit
group.<group_name>.control.roster
group.<group_name>.orchestrator
```

**Dynamic skill topics**: KafGraph subscribes to the roster topic and automatically
adds subscriptions for newly registered skill topics without restart.

#### 4.1.5 Historic Conversation Replay (Isolated Iteration)

A key advantage of the KafScale processor model is **offset-based replay for
isolated historic iteration**. During reflection cycles, KafGraph needs to
re-examine past conversations without interfering with real-time ingestion.

```
┌────────────────────────────────────────────────────────────────┐
│                  Isolated Historic Iterator                     │
│                                                                │
│  1. Create a separate Processor instance with its own          │
│     checkpoint namespace (e.g., "kafgraph-reflect-daily")      │
│                                                                │
│  2. Load offset state for the reflection window:               │
│     - windowStart → find segment containing start offset       │
│     - windowEnd   → find segment containing end offset         │
│                                                                │
│  3. Iterate S3 segments in [windowStart, windowEnd]:           │
│     - Discovery: list segments in offset range                 │
│     - Decoder:   parse each segment                            │
│     - Filter:    only records within time window               │
│     - NO lease required (read-only, no competing consumers)    │
│                                                                │
│  4. Feed records through the Reflection Scorer                 │
│     (NOT through the Graph Sink — graph already has them)      │
│                                                                │
│  5. Scorer produces ReflectionCycle + LearningSignal nodes     │
│     and writes them to the graph via the Graph Engine API      │
└────────────────────────────────────────────────────────────────┘
```

This isolation means:
- **No interference** with real-time ingestion (separate checkpoint namespace)
- **No broker load** (reads directly from S3 segments)
- **Repeatable** (same window always produces same records)
- **Efficient** (only reads segments within the time window, skips the rest)

The real-time ingest processor and the reflection iterator share the same Discovery
and Decoder implementations but use **separate checkpoint namespaces** so they
never interfere with each other's offset tracking.

#### 4.1.6 LFS Envelope Resolution

When the Graph Sink encounters a `memory` envelope or any payload with an LFS
reference, it resolves the content from S3 using the KafScale LFS API:

```
LFSEnvelope { bucket, key, size, checksum }
    │
    ▼
S3 fetch (via KafScale LFS Proxy or direct S3 client)
    │
    ▼
Verify SHA-256 checksum
    │
    ▼
Store content as property on SharedMemory node
(or as an external reference if content exceeds graph inline limit)
```

### 4.2 Graph Engine

The graph engine wraps BadgerDB with five logical layers:

```
┌─────────────────────────────────────┐
│         Graph API (internal)        │  <- CreateNode, CreateEdge, Match, Traverse,
│                                     │     VectorSearch, FullTextSearch
├─────────────────────────────────────┤
│         Index Layer                 │  <- Label index, property index, edge index
├─────────────────────────────────────┤
│         Vector Index (HNSW)         │  <- Embedding-based ANN search
├─────────────────────────────────────┤
│         Full-Text Index (bleve)     │  <- Text search on node properties
├─────────────────────────────────────┤
│         BadgerDB (LSM-tree)         │  <- Physical key-value storage, WAL, ACID txns
└─────────────────────────────────────┘
```

**Key encoding scheme** (BadgerDB keys):

```
n:<nodeID>              → NodeRecord (label, properties, createdAt, sourceOffset)
e:<edgeID>              → EdgeRecord (fromID, toID, label, weights, properties)
i:lbl:<label>:<nodeID>  → (presence index — label → nodes)
i:prop:<key>:<val>:<id> → (property value index)
i:out:<nodeID>:<edgeID> → (outgoing edge index per node)
i:in:<nodeID>:<edgeID>  → (incoming edge index per node)
v:<nodeID>              → embedding vector (float32 array, indexed by HNSW)
t:<nodeID>              → (full-text index managed by bleve, stored on disk)
```

All writes within a single ingest event use a single BadgerDB transaction, guaranteeing
atomicity.

#### 4.2.1 Embedding-Based Queries (Vector Index)

KafGraph supports storing embedding vectors as node properties and querying them via
approximate nearest-neighbour (ANN) search. The vector index uses **HNSW**
(Hierarchical Navigable Small World) — a proven ANN algorithm with sub-linear query
time.

**Use cases:**
- Find conversations semantically similar to a given query
- Cluster learning signals by topic embedding
- Link related messages across agents by content similarity

**Cypher extension:**
```cypher
CALL kafgraph.vectorSearch('Message', 'embedding', $queryVector, 10)
YIELD node, score
RETURN node.content, score ORDER BY score DESC
```

Embeddings are computed externally (by the agent runtime or a dedicated embedding
service) and attached to nodes during ingestion or via Cypher `SET`. KafGraph
indexes them but does not generate them (embedding model choice remains with the
agent stack).

#### 4.2.2 Full-Text Search

KafGraph indexes text properties of graph nodes using **bleve** (pure Go full-text
search library). Indexed properties are configurable per label.

**Default indexed properties:**
- `Message.content`
- `SharedMemory.title`
- `LearningSignal.summary`
- `Conversation.description`

**Cypher extension:**
```cypher
CALL kafgraph.fullTextSearch('Message', 'content', 'quarterly revenue analysis')
YIELD node, score
MATCH (node)-[:AUTHORED]->(a:Agent)
RETURN a.agentName, node.content, score
```

The full-text index is updated transactionally alongside graph writes to ensure
consistency.

#### 4.2.3 Optional External Graph Database Export

For organisations that require Neo4j or TigerGraph, KafGraph supports **one-way
export** of graph data to these external systems. This is an optional add-on — not
a dependency.

| Target | Mechanism | Scope |
|--------|-----------|-------|
| Neo4j | Bolt driver push (batch export) | ReflectionCycle + LearningSignal subgraph |
| TigerGraph | REST API push (GSQL loader) | Full graph or filtered subgraph |

Export runs as an optional background job, configurable per reflection cycle or on
demand via REST API. KafGraph remains fully functional without any external graph
database.

### 4.3 Bolt Server

KafGraph implements the **Bolt v4.4 protocol** (the last version before Neo4j 5's
incremental changes) to maximise client compatibility.

```
Bolt handshake → version negotiation → HELLO (auth) → RUN (Cypher) → PULL
```

The Bolt server is a net.Listener on port 7687. Each connection spawns a goroutine
that parses Bolt message frames and dispatches to the Cypher executor.

**Cypher Executor (v1 scope):**

```
Cypher text
    │
    ▼
OpenCypher Parser (ANTLR4 grammar)
    │
    ▼
Logical Plan (AST → plan tree)
    │
    ▼
Physical Plan (index selection, join order)
    │
    ▼
Iterator Engine → BadgerDB reads → result stream
    │
    ▼
Bolt RECORD messages → client
```

Supported clauses in v1: `MATCH`, `WHERE`, `RETURN`, `CREATE`, `MERGE`, `SET`,
`DELETE`, `WITH`, `UNWIND`, `ORDER BY`, `LIMIT`, `SKIP`.

### 4.4 Reflection Scheduler

The scheduler runs as an in-process goroutine loop. Cycle triggers are defined in
configuration but can also be fired via the REST API.

```
┌───────────────────────────────────────────────────────────────────────┐
│                      Reflection Scheduler                             │
│                                                                       │
│  cron("@daily")  ─────┐                                              │
│  cron("@weekly") ─────┼──▶  CycleRunner                             │
│  cron("@monthly")─────┘        │                                     │
│                                 │  1. Determine time window           │
│  REST POST /reflect/{agentID}──▶│  2. Create Isolated Iterator       │
│                                 │     (separate checkpoint NS)       │
│                                 │  3. Iterate S3 segments in window  │
│                                 │     (Discovery → Decoder → filter) │
│                                 │  4. Score (impact/rel/value)       │
│                                 │  5. Emit LearningSignal nodes      │
│                                 │  6. Write ReflectionCycle node     │
│                                 │  7. Publish to reflection-signals  │
│                                 │  8. Check feedback status          │
│                                 │  9. Emit feedback-request if       │
│                                 │     humanFeedbackStatus=PENDING    │
└───────────────────────────────────────────────────────────────────────┘
```

The Isolated Historic Iterator (see Section 4.1.5) ensures that reflection cycles
read directly from S3 segments without broker load and without interfering with
real-time ingestion. Each reflection type (daily, weekly, monthly) uses its own
checkpoint namespace to enable independent, repeatable iteration.

**Scoring model (v1):**

Impact is primarily measured through **human feedback** (both positive and negative).
Heuristic pre-scoring provides initial estimates that humans then confirm or override:

- `impact` = **human feedback is the authoritative signal** (positive or negative).
  Pre-score heuristic: normalised reply-chain depth + downstream conversation count.
  Human feedback overrides the heuristic with a signed value (-1.0 to +1.0) capturing
  both positive impact (beneficial actions) and negative impact (harmful/wasteful actions).
- `relevance` = TF-IDF cosine similarity between message content and team goal vector
  (goal vector seeded from conversation topic metadata). Can also use embedding-based
  similarity when vector index is enabled.
- `valueContribution` = ratio of messages in the thread that were acted upon (follow-up
  actions detected by keyword heuristics) to total messages in thread.

v2 will augment heuristics with LLM-based scoring via an async Kafka pipeline, but
human feedback remains the authoritative override at all versions.

### 4.5 Human Feedback Loop

```
Reflection completes
        │
        ▼
humanFeedbackStatus = PENDING

Wait configurable grace period (default 24 h)
        │
        ▼
Still PENDING?
   YES ──▶ Emit FeedbackRequestEvent to kafgraph.feedback-requests
           {agentID, cycleID, window, topN signals, ownerRef}
           Set status = REQUESTED
        │
        ▼
Consume kafgraph.human-feedback
   Match cycleID ──▶ Attach HumanFeedback node
                     Override scores if provided
                     Set status = RECEIVED
```

### 4.6 Distributed Mode — Cluster Coordination

In distributed mode, multiple KafGraph instances form a cluster. Coordination uses
**two mechanisms**:

1. **Kafka partition assignment** (via goka): each instance owns a set of agentID
   partitions. Partition rebalance is handled by Kafka consumer group protocol.
2. **Gossip membership** (via hashicorp/memberlist): instances discover each other
   for cross-partition query routing. No ZooKeeper or etcd dependency.

**Cross-partition query routing:**

```
Client Bolt query (cross-agent MATCH)
        │
        ▼
Cluster-aware Bolt listener (router)
        │
   ┌────▼─────────────────────────────┐
   │  Partition map (agentID → node)  │
   └────────────────┬─────────────────┘
        │           │           │
   Shard A      Shard B      Shard C
   (local)      (remote)     (remote)
        │           │           │
   Partial     Partial      Partial
   results ────▶ Merge ◀───── results
                  │
                  ▼
              Final result → client
```

Remote shard calls use a lightweight internal RPC over TCP (length-prefixed protobuf),
**not** Bolt (to avoid recursive parsing overhead).

### 4.7 Per-Agent Embedded Mode

In per-agent mode (`mode: embedded` in config), KafGraph:

- Opens a BadgerDB instance in the agent's local data directory.
- Starts the Bolt listener on `127.0.0.1:7687` only (not exposed externally).
- Runs reflection scheduler with reduced resource limits.
- Optionally syncs `ReflectionCycle` and `LearningSignal` nodes to a central cluster
  (sync is one-way push via `kafgraph.reflection-signals` topic).

Per-agent instances never expose raw `Message` or `HumanFeedback` nodes to the
cluster unless `shareConversations: true` is set in config.

### 4.8 Agent Brain — Self-Owned, Agent-Accessible Knowledge System

KafGraph is not just a graph database or a reflection engine. It is the **brain**
of every agent in the team — a persistent, searchable, self-owned knowledge system
that no agent ever starts from zero with.

#### 4.8.1 The Memory Problem KafGraph Solves

The core problem: every time an agent starts a new session, it begins without
context. Every time an agent switches tasks or scales to a new instance, accumulated
knowledge is lost. Platform-provided memories (Claude memory, ChatGPT memory) are
walled gardens — they don't follow the agent across tools, they aren't accessible to
other agents in the team, and they create lock-in.

KafGraph solves this by being the **single brain** that every agent in a KafClaw
group can read from and write to, regardless of the underlying LLM:

```
┌─────────────────────────────────────────────────────────────────────┐
│                      The Agent Brain (KafGraph)                      │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  Graph DB (BadgerDB)                                         │   │
│  │  ├── Conversations, Messages, Tasks (what happened)         │   │
│  │  ├── ReflectionCycles, LearningSignals (what was learned)   │   │
│  │  ├── HumanFeedback (what had positive/negative impact)      │   │
│  │  ├── SharedMemory (knowledge artifacts)                     │   │
│  │  ├── Agent identity, skills, hierarchy (who does what)      │   │
│  │  └── Vector embeddings on all text (searchable by meaning)  │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                           │                                          │
│  ┌────────────────────────┼──────────────────────────────────────┐  │
│  │     Access Layer       │                                      │  │
│  │                        │                                      │  │
│  │  ┌──────────────┐  ┌──┴────────────┐  ┌────────┐ ┌────────┐ │  │
│  │  │ Brain Tool   │  │ Cypher/Bolt   │  │  REST  │ │ Kafka  │ │  │
│  │  │ API          │  │ (port 7687)   │  │  API   │ │ Topics │ │  │
│  │  │ /api/v1/tools│  │ (compatible)  │  │/healthz│ │(produce│ │  │
│  │  │              │  │               │  │/metrics│ │        │ │  │
│  │  └──────┬───────┘  └───────────────┘  └────────┘ └────────┘ │  │
│  │         │                                                     │  │
│  │  Two access paths:                                            │  │
│  │   1. Direct HTTP call (agent → KafGraph API)                  │  │
│  │   2. KafClaw skill routing (agent → skill topic → KafGraph)   │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
        │                   │                │               │
   Agent tool           Neo4j            Health/          Cluster
   calls               Browser            Metrics          Peers
   (any LLM)           tooling
```

#### 4.8.2 Brain Tool API — The Brain's Primary Interface

Instead of introducing a separate protocol layer (MCP), KafGraph exposes its brain
capabilities as **tool definitions** — JSON-schema-described functions that any LLM
agent can call. This is the native language of agent tool use.

**Two access paths, same tools:**

```
Path 1: Direct HTTP Tool API
  Agent runtime → POST /api/v1/tools/{toolName} → KafGraph → response
  (fastest path; used in per-agent embedded mode and co-located setups)

Path 2: KafClaw Skill Routing
  Agent runtime → group.<name>.skill.kafgraph_brain.requests → KafGraph
  KafGraph      → group.<name>.skill.kafgraph_brain.responses → Agent
  (used in distributed mode; KafGraph registers as a KafClaw skill provider)
```

Path 2 is critical: KafGraph **registers itself as a KafClaw skill** named
`kafgraph_brain`. This means any agent in the group can discover and call the brain
tools through the existing KafClaw skill routing mechanism — no new infrastructure,
no new protocol. The `TopicManifest` advertises the brain skill, and agents
auto-discover it via the roster topic.

**Brain tools (LLM tool-call format):**

```json
[
  {
    "name": "brain_search",
    "description": "Semantic search across the agent's knowledge graph. Finds nodes by meaning using vector embeddings, not just keywords.",
    "parameters": {
      "type": "object",
      "properties": {
        "query":    { "type": "string", "description": "Natural language search query" },
        "agentId":  { "type": "string", "description": "Scope to a specific agent (optional, defaults to caller)" },
        "labels":   { "type": "array", "items": { "type": "string" }, "description": "Filter by node labels: Message, LearningSignal, SharedMemory, etc." },
        "timeRange": { "type": "object", "properties": { "from": { "type": "string" }, "to": { "type": "string" } } },
        "topK":     { "type": "integer", "default": 10 }
      },
      "required": ["query"]
    }
  },
  {
    "name": "brain_recall",
    "description": "Load accumulated context for an agent. Returns identity, active conversations, recent decisions, pending feedback, team context, and unresolved threads. Call this at session start so you never begin from zero.",
    "parameters": {
      "type": "object",
      "properties": {
        "agentId":    { "type": "string" },
        "scope":      { "type": "string", "enum": ["self", "team"], "default": "self" },
        "recentDays": { "type": "integer", "default": 7 }
      },
      "required": ["agentId"]
    }
  },
  {
    "name": "brain_capture",
    "description": "Write a thought, insight, decision, or observation into the brain. Auto-embeds, classifies, and links to related graph nodes.",
    "parameters": {
      "type": "object",
      "properties": {
        "agentId": { "type": "string" },
        "type":    { "type": "string", "enum": ["insight", "decision", "observation", "person_note", "meeting_debrief"] },
        "content": { "type": "string", "description": "The thought to capture" },
        "tags":    { "type": "array", "items": { "type": "string" } },
        "relatedTo": { "type": "string", "description": "Optional: conversationId or nodeId to link to" }
      },
      "required": ["agentId", "content"]
    }
  },
  {
    "name": "brain_recent",
    "description": "Browse recent activity: conversations, reflections, feedback within a time window.",
    "parameters": {
      "type": "object",
      "properties": {
        "agentId":    { "type": "string" },
        "scope":      { "type": "string", "enum": ["self", "team"], "default": "self" },
        "recentDays": { "type": "integer", "default": 7 },
        "labels":     { "type": "array", "items": { "type": "string" } }
      },
      "required": ["agentId"]
    }
  },
  {
    "name": "brain_patterns",
    "description": "Surface recurring themes, connections, and patterns from the knowledge graph. Uses reflection cycle results and cross-agent links.",
    "parameters": {
      "type": "object",
      "properties": {
        "agentId": { "type": "string" },
        "scope":   { "type": "string", "enum": ["self", "team"], "default": "self" },
        "window":  { "type": "string", "enum": ["daily", "weekly", "monthly"], "default": "weekly" }
      },
      "required": ["agentId"]
    }
  },
  {
    "name": "brain_reflect",
    "description": "Trigger an on-demand reflection cycle and return the results inline.",
    "parameters": {
      "type": "object",
      "properties": {
        "agentId": { "type": "string" },
        "window":  { "type": "string", "enum": ["daily", "weekly", "monthly"], "default": "daily" }
      },
      "required": ["agentId"]
    }
  },
  {
    "name": "brain_feedback",
    "description": "Submit human feedback on a reflection cycle or learning signal. Tracks both positive and negative impact.",
    "parameters": {
      "type": "object",
      "properties": {
        "cycleId":   { "type": "string" },
        "signalId":  { "type": "string" },
        "impactType": { "type": "string", "enum": ["positive", "negative"] },
        "score":     { "type": "number", "minimum": -1.0, "maximum": 1.0 },
        "comment":   { "type": "string" }
      },
      "required": ["cycleId", "impactType", "score"]
    }
  }
]
```

These tool schemas are served by KafGraph at `GET /api/v1/tools` so that any agent
runtime can fetch and register them dynamically. The schemas follow the standard
OpenAI/Anthropic tool-call format — any LLM that supports function calling can use
them directly.

#### 4.8.3 KafClaw Skill Registration

When KafGraph starts in a KafClaw group, it **registers `kafgraph_brain` as a
skill** via the KafClaw skill system:

```
KafGraph startup
    │
    ▼
KafClaw Manager.RegisterSkill("kafgraph_brain", handler)
    │
    ├─ Creates topics:
    │    group.<name>.skill.kafgraph_brain.requests
    │    group.<name>.skill.kafgraph_brain.responses
    │
    ├─ Publishes updated TopicManifest to roster
    │
    └─ Subscribes to skill.kafgraph_brain.requests
    │
    ▼
Any agent in the group can now call:
    Manager.SubmitSkillTask("kafgraph_brain", taskID, {
      "tool":   "brain_search",
      "params": { "query": "API rate limiting", "topK": 5 }
    })
    │
    ▼
KafGraph receives via skill topic → routes to brain_search handler
    → executes vector search → returns results via skill response topic
```

This means:
- **Zero new infrastructure** — uses existing KafClaw skill routing
- **Auto-discovery** — agents find the brain via the roster topic manifest
- **Any agent in the group** — not just the co-located one — can query the brain
- **The brain is a team resource** — every agent can search, recall, and capture

#### 4.8.4 Context Loading Protocol — No More Starting from Zero

When a KafClaw agent starts a new session, it calls `brain_recall` as its first
tool invocation:

```
Agent session starts
    │
    ▼
Tool call: brain_recall({ agentId: "agent-researcher", scope: "self" })
    │
    ▼
KafGraph builds context summary:
    ├─ Agent identity and capabilities (from Agent node)
    ├─ Active conversations (open Conversation nodes, last 7 days)
    ├─ Recent decisions (LearningSignal nodes with high impact)
    ├─ Pending feedback requests (ReflectionCycles with status=PENDING)
    ├─ Team context (other agents' recent contributions, cross-agent signals)
    └─ Unresolved threads (conversations with no response or follow-up)
    │
    ▼
Tool response: structured JSON context summary
    │
    ▼
Agent LLM receives context → session is productive immediately
```

This replaces the "explain everything from scratch" pattern. The agent's system
prompt can simply include: "At session start, call brain_recall to load your context."

In per-agent embedded mode, this is a local function call (sub-millisecond). In
distributed mode via KafClaw skill routing, it's a Kafka round-trip (~50-200ms).

#### 4.8.5 Brain Capture — Every Interaction Compounds

Every conversation the agent has is automatically ingested into the brain via the
KafScale Processor (Section 4.1). But agents can also **write directly** to the
brain via the `brain_capture` tool for insights that emerge during work:

```
Tool call: brain_capture({
  "agentId": "agent-coder",
  "type":    "decision",
  "content": "Switching from REST to gRPC for inter-service communication
              because latency requirements dropped below 5ms",
  "tags":    ["architecture", "grpc", "performance"]
})
    │
    ▼
KafGraph:
  1. Generate embedding for content (via configured embedding endpoint)
  2. Create Insight node with properties + embedding
  3. Full-text index the content
  4. Auto-link to related nodes via vector similarity (find nearest)
  5. Publish to kafgraph.brain-captures topic (for cluster sync)
    │
    ▼
Tool response: { nodeId: "insight-42", linkedTo: ["conv-17", "signal-8"] }
```

This creates a **compounding advantage**: every thought captured makes the next
search smarter, the next connection more likely to surface. The brain grows with
every interaction.

#### 4.8.6 The Compounding Effect

The brain's value compounds through three feedback loops:

```
Loop 1: Automatic Ingestion
  Agent conversations → KafScale Processor → Graph nodes + embeddings
  (runs continuously — every message is brain food)

Loop 2: Reflection Cycles
  Graph nodes → Reflection Scheduler → LearningSignals
  → Impact/relevance scoring → Pattern discovery
  (runs daily/weekly/monthly — the brain learns what mattered)

Loop 3: Human Feedback
  LearningSignals → Feedback request → Human review
  → Confirmed/overridden scores → Authoritative quality signal
  (runs on demand — humans teach the brain what was truly valuable)
```

Each loop enriches the graph, making subsequent queries more informative, subsequent
reflections more accurate, and subsequent agent sessions more productive.

#### 4.8.7 No Platform Lock-In

The brain is self-owned infrastructure. Any agent that can make tool calls can use it:

- Switch the LLM from Claude to GPT to Gemini → same brain, same context
- Add a new agent to the team → it discovers the brain skill via roster, gets
  full team history on first `brain_recall`
- Replace the agent framework → the brain tools are standard HTTP + JSON, callable
  from any runtime
- Scale agents up or down → the brain persists independently

The brain data lives in BadgerDB on infrastructure the team controls (KafScale
node or local disk in embedded mode). No SaaS dependency. No vendor lock-in.
No protocol middleware. Just tool calls to your own graph.

---

## 5. Topic Schema

> **See also**: `kafclaw-topic-reference.md` for the full KafClaw topic model,
> envelope types, and payload structures.

### 5.1 Topics Consumed — KafClaw Group Topics

KafGraph consumes from the actual KafClaw group topic hierarchy. The group name
is configurable (e.g., `workshop`, `team-alpha`).

**Primary (conversation data):**

| Topic | Envelope Type | Schema | Purpose |
|-------|--------------|--------|---------|
| `group.<name>.announce` | `announce` | `GroupEnvelope<AnnouncePayload>` | Agent join / leave / heartbeat |
| `group.<name>.requests` | `request` | `GroupEnvelope<TaskRequestPayload>` | Task delegation (conversation starts) |
| `group.<name>.responses` | `response` | `GroupEnvelope<TaskResponsePayload>` | Task completions (conversation replies) |
| `group.<name>.tasks.status` | `task_status` | `GroupEnvelope<TaskStatusPayload>` | Progress updates |
| `group.<name>.skill.*.requests` | `skill_request` | `GroupEnvelope<TaskRequestPayload>` | Skill-specific task requests |
| `group.<name>.skill.*.responses` | `skill_response` | `GroupEnvelope<TaskResponsePayload>` | Skill-specific results |
| `group.<name>.memory.shared` | `memory` | `GroupEnvelope<MemoryItem>` | Shared knowledge artifacts |

**Enrichment (supplementary context):**

| Topic | Envelope Type | Schema | Purpose |
|-------|--------------|--------|---------|
| `group.<name>.traces` | `trace` | `GroupEnvelope<TracePayload>` | Distributed trace spans |
| `group.<name>.observe.audit` | `audit` | `GroupEnvelope<Map>` | Admin audit trail |
| `group.<name>.control.roster` | `roster` | `GroupEnvelope<TopicManifest>` | Topic registry + skill discovery |
| `group.<name>.orchestrator` | `orchestrator` | `GroupEnvelope<DiscoveryPayload>` | Hierarchy + zone coordination |

**KafGraph's own feedback topic:**

| Topic | Schema | Purpose |
|-------|--------|---------|
| `kafgraph.human-feedback` | `HumanFeedbackEvent` (JSON) | Human scores and overrides |

### 5.2 Topics Produced

| Topic | Consumer | Schema | Purpose |
|-------|----------|--------|---------|
| `kafgraph.feedback-requests` | KafClaw notifier / owner | `FeedbackRequestEvent` (JSON) | Request human review |
| `kafgraph.reflection-signals` | Cluster peers, dashboards | `ReflectionSignalEvent` (JSON) | Published learning signals |
| `kafgraph.brain-captures` | KafGraph cluster peers | `BrainCaptureEvent` (JSON) | Insights written via MCP `brain_capture` — synced to cluster |
| `kafgraph.cluster-coord` | KafGraph instances | `ClusterCoordEvent` (JSON) | Membership, checkpoints |

### 5.3 Wire Format — KafClaw GroupEnvelope (JSON)

KafClaw uses **JSON** (not protobuf) for all group messages. Every message shares
this common envelope:

```json
{
  "Type":          "request",
  "CorrelationID": "550e8400-e29b-41d4-a716-446655440000",
  "SenderID":      "agent-researcher",
  "Timestamp":     "2026-03-02T14:30:00Z",
  "Payload": {
    "taskID":      "task-42",
    "description": "Summarize the quarterly report",
    "content":     "Please analyze Q1 revenue trends...",
    "requesterID": "agent-orchestrator",
    "delegationDepth": 1
  }
}
```

The `Payload` field is polymorphic — its structure depends on the `Type` field.
See `kafclaw-topic-reference.md` Section 4 for all payload types.

### 5.4 KafGraph-Owned Event Schemas (JSON)

```json
// FeedbackRequestEvent (produced by KafGraph)
{
  "agentId":       "agent-researcher",
  "cycleId":       "cycle-2026-03-02-daily",
  "cycleType":     "DAILY",
  "windowStartMs": 1740873600000,
  "windowEndMs":   1740960000000,
  "ownerRef":      "owner@example.com",
  "topSignals": [
    {
      "signalId":  "sig-001",
      "summary":   "High-impact code review discussion with 3 follow-up actions",
      "impact":    0.87,
      "relevance": 0.92
    }
  ]
}

// HumanFeedbackEvent (consumed by KafGraph)
{
  "cycleId":     "cycle-2026-03-02-daily",
  "reviewerId":  "team-lead@example.com",
  "reviewerRole": "teamLeader",
  "submittedMs": 1740960000000,
  "signals": [
    {
      "signalId":          "sig-001",
      "impactType":        "positive",
      "confirmedImpact":   0.90,
      "confirmedRelevance": 0.85,
      "comment":           "Good analysis, led to actionable Q1 insights"
    },
    {
      "signalId":          "sig-002",
      "impactType":        "negative",
      "confirmedImpact":   -0.40,
      "confirmedRelevance": 0.70,
      "comment":           "Incorrect data interpretation caused downstream rework"
    }
  ],
  "status": "RECEIVED"
}
```

---

## 6. Deployment

### 6.1 Configuration File (`kafgraph.yaml`)

```yaml
mode: distributed          # embedded | distributed

# --- KafScale 2.7.0 Processor Configuration ---
# KafGraph is a KafScale Processor — it uses the same S3 discovery,
# binary decoding, and checkpoint model as other KafScale processors.
kafscale:
  version: "2.7.0"
  s3:
    endpoint: "http://minio:9000"          # MinIO endpoint
    bucket: kafscale-tiered-storage
    namespace: production
    accessKey: ${KAFGRAPH_MINIO_ACCESS_KEY}
    secretKey: ${KAFGRAPH_MINIO_SECRET_KEY}
  processorAPI: "http://localhost:8080"    # KafScale Processor API (co-located)
  lfsProxy: "http://localhost:8080/lfs"   # LFS Proxy for large artifact resolution
  checkpoint:
    backend: etcd                          # etcd | local (BadgerDB-backed)
    etcdEndpoints: ["etcd-1:2379"]
  pollInterval: 5s                         # S3 segment poll interval

# --- KafClaw Group Topics ---
# KafGraph subscribes to the actual KafClaw topic hierarchy.
kafclaw:
  groupName: team-alpha                    # KafClaw group name → topic prefix
  # Primary topics (always consumed):
  #   group.team-alpha.announce
  #   group.team-alpha.requests
  #   group.team-alpha.responses
  #   group.team-alpha.tasks.status
  #   group.team-alpha.memory.shared
  #   group.team-alpha.skill.*.requests / responses (auto-discovered via roster)
  enrichment:
    enabled: true                          # Also consume traces, audit, roster, orchestrator
  autoDiscoverSkills: true                 # Subscribe to new skill topics via roster

kafka:
  brokers: ["kafka-1:9092", "kafka-2:9092"]
  consumerGroup: kafgraph-prod
  topics:
    humanFeedback: "kafgraph.human-feedback"
  sasl:
    mechanism: SCRAM-SHA-512
    username: ${KAFGRAPH_KAFKA_USER}
    password: ${KAFGRAPH_KAFKA_PASS}

storage:
  dataDir: /var/kafgraph/data
  encryptionKey: ${KAFGRAPH_STORAGE_KEY}   # empty = no encryption

# --- Brain Tool API ---
# Primary interface for agents to interact with their brain.
# Tools are served at /api/v1/tools and registered as KafClaw skill "kafgraph_brain".
brain:
  enabled: true
  kafclawSkill:
    register: true                         # Register as KafClaw skill for group-wide access
    skillName: kafgraph_brain              # Skill name in topic manifest
  embedding:
    endpoint: "http://localhost:11434/api/embeddings"  # Ollama, or any OpenAI-compatible endpoint
    model: "nomic-embed-text"              # Embedding model (runs locally or remote)
    dimensions: 768                        # Must match search.vector.dimensions
  contextWindow:
    recentDays: 7                          # How far back brain_recall looks by default
    maxNodes: 200                          # Max nodes in a context summary

bolt:
  listen: "0.0.0.0:7687"
  tls:
    certFile: /etc/kafgraph/tls/cert.pem
    keyFile:  /etc/kafgraph/tls/key.pem
  auth:
    username: ${KAFGRAPH_BOLT_USER}
    password: ${KAFGRAPH_BOLT_PASS}

http:
  listen: "0.0.0.0:7474"

reflection:
  daily:   { cron: "0 2 * * *" }         # 02:00 UTC
  weekly:  { cron: "0 3 * * 1" }         # 03:00 UTC Monday
  monthly: { cron: "0 4 1 * *" }         # 04:00 UTC 1st of month
  feedbackGracePeriod: 24h
  checkpointNamespace: kafgraph-reflect   # Isolated from real-time ingest offsets
  feedbackOwner:
    teamLeader: "team-lead@example.com"   # Primary feedback recipient
    experts:                               # Additional expert reviewers
      - "expert-1@example.com"
      - "expert-2@example.com"

# --- Search Indexes ---
search:
  fullText:
    enabled: true
    indexedProperties:                     # Label.property pairs to index
      - "Message.content"
      - "SharedMemory.title"
      - "LearningSignal.summary"
      - "Conversation.description"
  vector:
    enabled: true
    dimensions: 768                        # Must match mcp.embedding.dimensions
    similarity: cosine                     # cosine | euclidean | dot

# --- Optional External Graph Export ---
export:
  neo4j:
    enabled: false                         # Optional: push subgraph to Neo4j
    # uri: "bolt://neo4j:7687"
    # username: ${NEO4J_USER}
    # password: ${NEO4J_PASS}
  tigergraph:
    enabled: false                         # Optional: push subgraph to TigerGraph
    # url: "http://tigergraph:9000"

cluster:
  gossipPort: 7946
  peers: []                               # auto-discovered via Kafka coord topic
  replicationFactor: 2
```

### 6.2 Kubernetes / KafScale Deployment (Helm sketch)

```yaml
# KafScale StatefulSet (existing)
# KafGraph runs as a sidecar container in the same Pod

containers:
  - name: kafscale-broker
    image: kafscale:latest
    ports: [{containerPort: 9092}]

  - name: kafgraph
    image: kafgraph:latest
    ports:
      - {containerPort: 7687, name: bolt}
      - {containerPort: 7474, name: http}
      - {containerPort: 7946, name: gossip}
    env:
      - {name: KAFGRAPH_KAFKA_USER, valueFrom: {secretKeyRef: ...}}
      - {name: KAFGRAPH_KAFKA_PASS, valueFrom: {secretKeyRef: ...}}
    volumeMounts:
      - {name: kafgraph-data, mountPath: /var/kafgraph/data}
```

Placing KafGraph as a **sidecar in the KafScale Pod** satisfies the co-location
requirement: the KafScale Processor API is reachable at `localhost:8080` without any
network hop, and the two containers share a Pod-local volume for S3 segment caching.

---

## 7. Data Flow — End to End

```
Agent Runtime (KafClaw agent)
    │
    │  GroupEnvelope (JSON) — e.g., type="request"
    ▼
KafClaw Group Topics
    group.team-alpha.requests      ← task delegation messages
    group.team-alpha.responses     ← task completions
    group.team-alpha.announce      ← agent join/leave/heartbeat
    group.team-alpha.skill.*.{requests,responses}  ← skill conversations
    group.team-alpha.memory.shared ← shared knowledge artifacts
    │
    │  Messages flow to KafScale S3 tiered storage
    ▼
S3 Segments
    s3://bucket/ns/group.team-alpha.requests/0/segment-42.kfs
    │
    │  KafScale Processor Stack (inside KafGraph)
    │
    ├─ Discovery: list completed segments from S3
    ├─ Decoder:   parse .kfs binary → Record[]
    ├─ Checkpoint: filter by committed offset (skip already-processed)
    ├─ Graph Sink: deserialize GroupEnvelope → route by Type
    │    ├─ "announce" (join) → upsert Agent node
    │    ├─ "request"         → create Conversation + Message, AUTHORED edge
    │    ├─ "response"        → create Message, REPLIED_TO edge
    │    ├─ "skill_request"   → create Conversation + Message + Skill, USES_SKILL edge
    │    ├─ "memory"          → resolve LFS envelope, create SharedMemory node
    │    └─ ...
    └─ Checkpoint: commit offset after successful graph write
    │
    ▼
BadgerDB Graph (local shard)
    │
    │  (reflection cycle trigger — cron or REST API)
    ▼
Reflection Scheduler
    │  Creates Isolated Iterator (separate checkpoint namespace)
    │  Reads S3 segments for time window [start, end]
    │  Scores conversations: impact / relevance / valueContribution
    │  Creates ReflectionCycle + LearningSignal nodes
    │  Links via TRIGGERED_REFLECTION, LINKS_TO edges
    ▼
kafgraph.reflection-signals topic → cluster peers, dashboards
    │
    │  (feedback check — 24 h grace period)
    ▼
kafgraph.feedback-requests topic → Owner notification
    │
    │  (owner responds)
    ▼
kafgraph.human-feedback topic
    │
    ▼
KafGraph attaches HumanFeedback node
Overrides scores if provided
Sets ReflectionCycle.humanFeedbackStatus = RECEIVED
    │
    ▼
Agent Brain (Tool Calls) — the primary access path for agents
    │
    │  Two paths to the same tools:
    │  ├─ Direct: POST /api/v1/tools/brain_recall (co-located / embedded)
    │  └─ KafClaw: SubmitSkillTask("kafgraph_brain", ...) (distributed)
    │
    ├─ brain_recall: agent loads accumulated context at session start
    │   → no more starting from zero
    │
    ├─ brain_search: semantic search across all conversations and learnings
    │   → "find discussions about API rate limiting" → vector similarity match
    │
    ├─ brain_capture: agent writes insights directly into the brain
    │   → auto-embedded, auto-linked, synced to cluster
    │
    ├─ brain_patterns: surface recurring themes and connections
    │   → "code review discussions keep raising the same auth concern"
    │
    └─ brain_reflect: on-demand reflection
        → "what had the most impact this week?"

Also available via:
    ├─ Bolt/Cypher (port 7687) — for tooling and dashboards
    └─ REST API (port 7474) — for health, metrics, CRUD
```

---

## 8. Phased Delivery Plan

| Phase | Milestone | Scope |
|-------|-----------|-------|
| **0 — Foundation** | Runnable binary | BadgerDB integration, graph API, Bolt handshake (no Cypher yet), config loading |
| **1 — Processor** | KafScale Processor | Implement 5-layer processor stack (Discovery, Decoder, Checkpoint, Graph Sink, TopicLocker); consume KafClaw GroupEnvelope from S3 segments; create Agent / Conversation / Message nodes |
| **2 — Query** | Cypher v1 + Search | OpenCypher parser, MATCH/RETURN/WHERE/CREATE/MERGE, Bolt streaming; vector index (HNSW) for embedding queries; full-text index (bleve) for text search |
| **3 — Agent Brain** | Brain Tool API | HTTP tool endpoints at `/api/v1/tools/*`; KafClaw skill registration (`kafgraph_brain`); brain_search, brain_recall, brain_capture, brain_recent, brain_patterns, brain_reflect, brain_feedback tools; context loading protocol; embedding integration; brain_capture → kafgraph.brain-captures topic sync |
| **4 — Reflection** | Reflection cycles | Scheduler, Isolated Historic Iterator (separate checkpoint NS), heuristic scoring, ReflectionCycle + LearningSignal nodes, brain compounding loops |
| **5 — Feedback** | Human feedback loop | FeedbackRequestEvent producer, HumanFeedbackEvent consumer, positive/negative impact tracking, team leader + expert routing |
| **6 — Distribution** | Cluster mode | Gossip membership, cross-partition routing, replication factor, brain-captures sync |
| **7 — Hardening** | Production-ready | TLS everywhere, encryption at rest, OTel tracing, Helm chart, load tests, auto-discover skill topics via roster |

---

## 9. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| OpenCypher parser complexity exceeds v1 scope | High | Medium | Start with a hand-written recursive descent parser for the 10 most-used patterns; full ANTLR grammar in v2 |
| BadgerDB write amplification under high ingestion | Medium | High | Tune value-log GC interval; benchmark early; add compaction metrics to dashboard |
| Heuristic scoring produces low-quality learning signals | High | Medium | Treat v1 as a baseline; human feedback override is the primary signal; LLM scoring in v2 |
| Gossip-based cluster has split-brain under network partition | Low | High | Use Kafka coordination topic as ground truth for partition ownership; gossip is discovery-only |
| KafScale Processor API changes break S3 reader | Medium | Medium | Version-pin the API; add integration test against KafScale in CI |
| Human feedback never arrives (owner inattentive) | High | Medium | Configurable WAIVED auto-timeout; escalation chain in feedback-request event |
| KafClaw skill routing adds latency to brain tool calls | Low | Medium | Direct HTTP path available for co-located/embedded mode; skill routing only for distributed cross-group access; Kafka round-trip is ~50-200ms which is acceptable for brain queries |
| Embedding endpoint unavailable degrades brain capture | Medium | High | Queue unembedded captures in BadgerDB; retry embedding in background; brain_search falls back to full-text when vectors unavailable |
| Brain context summaries grow too large for agent context windows | Medium | Medium | Configurable maxNodes limit; use reflection signals (compressed learnings) instead of raw messages; progressive summarization |
| Agents write low-quality captures that pollute the brain | Medium | Low | Human feedback loop naturally scores and filters; weekly reflection surfaces noise; configurable capture validation rules |

---

*References:*
- *Neo4j Bolt Protocol Specification v4.4: https://neo4j.com/docs/bolt/current/*
- *OpenCypher Grammar: https://opencypher.org/resources/*
- *BadgerDB Documentation: https://dgraph.io/docs/badger/*
- *Memgraph Bolt Compatibility: https://memgraph.com/docs/client-libraries/go*
- *hashicorp/memberlist: https://github.com/hashicorp/memberlist*

*Internal references:*
- *KafClaw agent stack: `/Users/kamir/GITHUB.kamir/KafClaw` — topic definitions in `internal/group/topics.go`, wire format in `internal/group/types.go`*
- *KafScale platform: `/Users/kamir/GITHUB.scalytics/platform` — processor skeleton in `addons/processors/skeleton/`, LFS in `pkg/lfs/`*
- *KafClaw topic reference: `kafclaw-topic-reference.md` (this SPEC folder)*
