# KafGraph — Solution Design

*Version: 0.1-draft — 2026-03-02*

---

## 1. Design Goals

1. **Kafka-native**: every subsystem communicates through Kafka topics; no RPC fabric is needed for the data plane.
2. **Bolt-compatible**: clients interact with the graph using the standard Neo4j Go driver without modification.
3. **Co-located with KafScale**: Processors sit next to S3 data; brokers are never overloaded by reflection workloads.
4. **Per-agent + distributed**: the same binary runs as a lightweight embedded instance or as a cluster shard.
5. **Learning-first**: reflection and feedback cycles are first-class, not afterthoughts.

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
| `lovoo/goka` | Kafka partition processing | Partition-aware, state management, fault tolerant |
| `segmentio/kafka-go` | Low-level Kafka I/O | Offset management, topic admin |
| `antlr4-go/antlr` | Cypher grammar parsing | ANTLR4 runtime for Go; OpenCypher grammar available |
| `prometheus/client_golang` | Metrics | Standard Prometheus instrumentation |
| `open-telemetry/opentelemetry-go` | Distributed tracing | OTLP export |
| `hashicorp/memberlist` | Cluster membership (gossip) | Lightweight; no ZooKeeper dependency |
| `spf13/viper` | Configuration | YAML + env-var override |

---

## 3. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        KafScale Processor Node                              │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         KafGraph Process                            │   │
│  │                                                                     │   │
│  │  ┌───────────────┐   ┌─────────────────┐   ┌──────────────────┐   │   │
│  │  │  Kafka Ingest │   │  Graph Engine   │   │   Bolt Server    │   │   │
│  │  │  (goka-based) │──▶│  (BadgerDB +    │◀──│  (port 7687)     │   │   │
│  │  │               │   │   index layer)  │──▶│                  │   │   │
│  │  └───────┬───────┘   └────────┬────────┘   └──────────────────┘   │   │
│  │          │                    │                                     │   │
│  │  ┌───────▼───────┐   ┌────────▼────────┐   ┌──────────────────┐   │   │
│  │  │  S3 Segment   │   │  Reflection     │   │  HTTP API        │   │   │
│  │  │  Reader       │   │  Scheduler      │   │  /healthz        │   │   │
│  │  │  (KafScale    │   │  (daily/weekly/ │   │  /readyz         │   │   │
│  │  │   Processor   │   │   monthly)      │   │  /metrics        │   │   │
│  │  │   API)        │   └────────┬────────┘   └──────────────────┘   │   │
│  │  └───────────────┘            │                                     │   │
│  │                       ┌───────▼───────┐                            │   │
│  │                       │  Feedback     │                            │   │
│  │                       │  Request      │                            │   │
│  │                       │  Emitter      │                            │   │
│  │                       └───────────────┘                            │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘

         ▲ Kafka Topics                              ▲ S3 (tiered storage)
         │                                           │
┌────────┴───────────────────────────────────────────┴────────────────────────┐
│                        Apache Kafka Cluster (KRaft)                         │
│                                                                             │
│  kafclaw.conversations.<team>   kafclaw.audits.<team>                      │
│  kafgraph.feedback-requests     kafgraph.human-feedback                    │
│  kafgraph.reflection-signals    kafgraph.cluster-coord                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Component Design

### 4.1 Kafka Ingest Layer

The ingest layer is built on **goka**. Each KafGraph instance joins a goka processor
group keyed on `agentID`. This gives us automatic partition assignment and replay.

```
KafClaw Topic: kafclaw.conversations.<team>
  Message key   : agentID
  Message value : ConversationEvent (protobuf or JSON)

goka Processor:
  - Consumes ConversationEvent
  - Upserts Agent, Conversation, Message nodes in local BadgerDB shard
  - Emits derived events to kafgraph.reflection-signals
```

**S3 Direct Read**: When a reflection cycle needs historical data beyond Kafka's
retention window, the `S3SegmentReader` calls the KafScale Processor API to fetch
archived segments directly. The decoded records are fed through the same ingest pipeline
— no broker round-trip.

### 4.2 Graph Engine

The graph engine wraps BadgerDB with three logical layers:

```
┌─────────────────────────────────────┐
│         Graph API (internal)        │  <- CreateNode, CreateEdge, Match, Traverse
├─────────────────────────────────────┤
│         Index Layer                 │  <- Label index, property index, edge index
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
```

All writes within a single ingest event use a single BadgerDB transaction, guaranteeing
atomicity.

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
┌─────────────────────────────────────────────────────────────────────┐
│                      Reflection Scheduler                           │
│                                                                     │
│  cron("@daily")  ─────┐                                            │
│  cron("@weekly") ─────┼──▶  CycleRunner                           │
│  cron("@monthly")─────┘        │                                   │
│                                 │  1. Determine window              │
│  REST POST /reflect/{agentID}──▶│  2. Collect Message nodes        │
│                                 │  3. Score (impact/rel/value)     │
│                                 │  4. Emit LearningSignal nodes    │
│                                 │  5. Write ReflectionCycle node   │
│                                 │  6. Publish to reflection-signals│
│                                 │  7. Check feedback status        │
│                                 │  8. Emit feedback-request if     │
│                                 │     humanFeedbackStatus=PENDING  │
└─────────────────────────────────────────────────────────────────────┘
```

**Heuristic scoring (v1):**

- `impact` = normalised reply-chain depth + downstream conversation count
- `relevance` = TF-IDF cosine similarity between message content and team goal vector
  (goal vector seeded from conversation topic metadata)
- `valueContribution` = ratio of messages in the thread that were acted upon (follow-up
  actions detected by keyword heuristics) to total messages in thread

v2 will replace these heuristics with LLM-based scoring via an async Kafka pipeline.

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

---

## 5. Topic Schema

### 5.1 Topics Consumed

| Topic | Producer | Schema | Purpose |
|-------|----------|--------|---------|
| `kafclaw.conversations.<team>` | KafClaw | `ConversationEvent` | Agent messages |
| `kafclaw.audits.<team>` | KafClaw | `AuditEvent` | Long-term audit records |
| `kafgraph.human-feedback` | Owner tooling | `HumanFeedbackEvent` | Human scores and overrides |

### 5.2 Topics Produced

| Topic | Consumer | Schema | Purpose |
|-------|----------|--------|---------|
| `kafgraph.feedback-requests` | KafClaw notifier / owner | `FeedbackRequestEvent` | Request human review |
| `kafgraph.reflection-signals` | Cluster peers, dashboards | `ReflectionSignalEvent` | Published learning signals |
| `kafgraph.cluster-coord` | KafGraph instances | `ClusterCoordEvent` | Membership, checkpoints |

### 5.3 Event Schemas (protobuf identifiers)

```protobuf
// ConversationEvent (consumed — defined by KafClaw)
message ConversationEvent {
  string conversation_id = 1;
  string agent_id        = 2;
  string team_id         = 3;
  string message_id      = 4;
  string content         = 5;
  int64  timestamp_ms    = 6;
  string parent_msg_id   = 7;  // empty if root message
  map<string,string> metadata = 8;
}

// FeedbackRequestEvent (produced by KafGraph)
message FeedbackRequestEvent {
  string agent_id        = 1;
  string cycle_id        = 2;
  string cycle_type      = 3;  // DAILY | WEEKLY | MONTHLY
  int64  window_start_ms = 4;
  int64  window_end_ms   = 5;
  string owner_ref       = 6;
  repeated LearningSignalSummary top_signals = 7;
}

// HumanFeedbackEvent (consumed by KafGraph)
message HumanFeedbackEvent {
  string cycle_id            = 1;
  string reviewer_id         = 2;
  int64  submitted_ms        = 3;
  repeated SignalFeedback signals = 4;
  string status              = 5;  // RECEIVED | WAIVED
}
```

---

## 6. Deployment

### 6.1 Configuration File (`kafgraph.yaml`)

```yaml
mode: distributed          # embedded | distributed

kafka:
  brokers: ["kafka-1:9092", "kafka-2:9092"]
  consumerGroup: kafgraph-prod
  topics:
    conversations: "kafclaw.conversations.team-alpha"
    audits: "kafclaw.audits.team-alpha"
    humanFeedback: "kafgraph.human-feedback"
  sasl:
    mechanism: SCRAM-SHA-512
    username: ${KAFGRAPH_KAFKA_USER}
    password: ${KAFGRAPH_KAFKA_PASS}

storage:
  dataDir: /var/kafgraph/data
  encryptionKey: ${KAFGRAPH_STORAGE_KEY}   # empty = no encryption

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

cluster:
  gossipPort: 7946
  peers: []                               # auto-discovered via Kafka coord topic
  replicationFactor: 2

kafscale:
  processorAPI: "http://localhost:8080"   # KafScale Processor API base URL
  s3Bucket: "kafscale-tiered-storage"
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
Agent Runtime
    │
    │ conversation message
    ▼
KafClaw Topic (kafclaw.conversations.team)
    │
    │ goka consumer (agentID partition key)
    ▼
KafGraph Ingest
    │ upsert Agent, Conversation, Message nodes
    ▼
BadgerDB shard (local)
    │
    │ (reflection cycle trigger)
    ▼
Reflection Scheduler
    │ traverse Message nodes in window
    │ compute impact / relevance / valueContribution
    │ create ReflectionCycle + LearningSignal nodes
    │ link via TRIGGERED_REFLECTION, LINKS_TO edges
    ▼
kafgraph.reflection-signals topic
    │
    │ (feedback check — 24 h grace)
    ▼
kafgraph.feedback-requests topic → Owner notification
    │
    │ (owner responds)
    ▼
kafgraph.human-feedback topic
    │
    ▼
KafGraph attaches HumanFeedback node
Overrides scores if provided
Sets ReflectionCycle.humanFeedbackStatus = RECEIVED
    │
    ▼
Bolt client (Neo4j driver) queries
MATCH (a:Agent {id: "agent-7"})-[:TRIGGERED_REFLECTION]->(r:ReflectionCycle)
WHERE r.type = "WEEKLY"
RETURN r, r.learningSignals ORDER BY r.createdAt DESC LIMIT 5
```

---

## 8. Phased Delivery Plan

| Phase | Milestone | Scope |
|-------|-----------|-------|
| **0 — Foundation** | Runnable binary | BadgerDB integration, graph API, Bolt handshake (no Cypher yet), config loading |
| **1 — Ingest** | Kafka → Graph | goka processor, ConversationEvent parsing, Agent/Conversation/Message nodes |
| **2 — Query** | Cypher v1 | OpenCypher parser, MATCH/RETURN/WHERE/CREATE/MERGE, Bolt streaming results |
| **3 — Reflection** | Reflection cycles | Scheduler, heuristic scoring, ReflectionCycle + LearningSignal nodes |
| **4 — Feedback** | Human feedback loop | FeedbackRequestEvent producer, HumanFeedbackEvent consumer, status tracking |
| **5 — Distribution** | Cluster mode | Gossip membership, cross-partition routing, replication factor, S3 direct read |
| **6 — Hardening** | Production-ready | TLS everywhere, encryption at rest, OTel tracing, Helm chart, load tests |

---

## 9. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| OpenCypher parser complexity exceeds v1 scope | High | Medium | Start with a hand-written recursive descent parser for the 10 most-used patterns; full ANTLR grammar in v2 |
| BadgerDB write amplification under high ingestion | Medium | High | Tune value-log GC interval; benchmark early; add compaction metrics to dashboard |
| Heuristic scoring produces low-quality learning signals | High | Medium | Treat v1 as a baseline; human feedback override is the primary signal; LLM scoring in v2 |
| Gossip-based cluster has split-brain under network partition | Low | High | Use Kafka coordination topic as ground truth for partition ownership; gossip is discovery-only |
| KafScale Processor API changes break S3 reader | Medium | Medium | Version-pin the API; add integration test against KafScale in CI |
| Human feedback never arrives (owner inattentive) | High | Medium | Configurable WAIVED auto-timeout; escaltation chain in feedback-request event |

---

*References:*
- *Neo4j Bolt Protocol Specification v4.4: https://neo4j.com/docs/bolt/current/*
- *OpenCypher Grammar: https://opencypher.org/resources/*
- *BadgerDB Documentation: https://dgraph.io/docs/badger/*
- *Goka: https://github.com/lovoo/goka*
- *Memgraph Bolt Compatibility: https://memgraph.com/docs/client-libraries/go*
- *hashicorp/memberlist: https://github.com/hashicorp/memberlist*
