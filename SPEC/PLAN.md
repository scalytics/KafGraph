# KafGraph — Phase Plan

*Living document — updated after each milestone.*

## Phases

| Phase | Name | Status | Description |
|-------|------|--------|-------------|
| 0 | Foundation | **Complete** | BadgerDB storage, graph API, Bolt handshake, config, Makefile, docs, CI/CD |
| 1 | Running Server | **Complete** | HTTP REST API, Bolt accept loop, graceful shutdown, 84.9% coverage |
| 2 | Processor | **Complete** | KafScale 5-layer processor, SegmentReader interface, GroupEnvelope decoder, ingest pipeline |
| 3 | Query | **Complete** | OpenCypher parser, secondary indexes, full-text search, vector search, Bolt message loop, HTTP query endpoint |
| 4 | Agent Brain | **Complete** | Brain Tool API (7 tools), HTTP tool execution endpoint, full-text search integration |
| 5 | Reflection | **Complete** | Reflection scheduler, historic iterator, daily/weekly/monthly cycles, scoring |
| 6 | Feedback | **Complete** | Human feedback loop, impact tracking, feedback-request events |
| 7 | Distribution | **Complete** | Multi-node cluster, gossip, cross-partition queries, partition strategies |
| 8 | Management UI | **Complete** | Embedded management UI, Management API, graph browser, reflection dashboard |
| 9 | Hardening | Planned | TLS, encryption at rest, OpenTelemetry, Helm chart, load tests |

## Phase 0 — Foundation (Complete)

### Deliverables
- [x] CLAUDE.md with conventions
- [x] Makefile with ~40 targets
- [x] Go module with initial packages
- [x] BadgerDB storage engine scaffold
- [x] Graph CRUD API scaffold
- [x] Bolt v4 handshake scaffold
- [x] Unit tests for all packages
- [x] E2E and integration test framework
- [x] Jekyll docs site
- [x] Docker configs (Dockerfile, 3x docker-compose)
- [x] GitHub Actions CI/CD
- [x] SPEC/FR/ requirements tracking
- [x] Skills directory with SKILL.md manifests

## Phase 1 — Running Server (Complete)

### Deliverables
- [x] HTTP REST API (nodes, edges, tools, health)
- [x] Bolt v4 accept loop with handshake
- [x] Graceful shutdown (SIGINT/SIGTERM)
- [x] 84.9% test coverage

## Phase 2 — Processor (Complete)

### Goals
- Ingest pipeline: SegmentReader → GroupEnvelope parser → Router → Handlers → Graph
- 11 envelope types (announce, request, response, task_status, skill_request/response, memory, trace, audit, roster, orchestrator)
- Deterministic node/edge IDs for idempotent replay
- Checkpoint store for at-least-once delivery
- MemoryReader for testing; real S3 reader plugs in later

### Deliverables
- [x] GroupEnvelope types + ParseEnvelope + 9 payload types
- [x] Deterministic node ID functions (Agent, Conversation, Message, Skill, SharedMemory, AuditEvent)
- [x] Deterministic edge ID function (sha256-short)
- [x] UpsertNode and UpsertEdge graph methods (idempotent, no endpoint check)
- [x] Router dispatching 11 envelope types to handlers
- [x] 11 handler functions (announce→Agent, request→Conv+Msg, etc.)
- [x] SegmentReader interface + MemoryReader implementation
- [x] CheckpointStore (offset tracking persisted to BadgerDB)
- [x] Processor poll loop (discover → read → parse → route → checkpoint)
- [x] IngestConfig (enabled, poll_interval, batch_size, namespace, group_name)
- [x] Main wiring (gated by cfg.Ingest.Enabled)
- [x] E2E ingest pipeline test
- [x] 50+ new tests with fuzz coverage

## Phase 3 — Query Engine (Complete)

### Goals
- OpenCypher subset parser (MATCH, WHERE, RETURN, CREATE, MERGE, SET, DELETE, CALL)
- Secondary indexes in BadgerDB (label, outgoing, incoming, edge label)
- Full-text search via bleve
- Brute-force vector similarity search (cosine)
- Bolt v4 message framing (PackStream encoding/decoding, chunked transport)
- HTTP query endpoint (POST /api/v1/query)

### Deliverables
- [x] Secondary index package (internal/index/) with BadgerIndex Manager
- [x] IndexedStorage interface (NodeIDsByLabel, OutgoingEdgeIDs, IncomingEdgeIDs, EdgeIDsByLabel)
- [x] BadgerStorage index integration (write-time index maintenance)
- [x] Full-text search via bleve (internal/search/fulltext.go)
- [x] Brute-force vector search with cosine similarity (internal/search/vector.go)
- [x] Hand-written Cypher lexer with 28+ keyword types
- [x] Recursive descent parser → AST (MATCH, WHERE, RETURN, CREATE, MERGE, SET, DELETE, CALL, YIELD)
- [x] Expression precedence: OR → AND → NOT → comparison → atom
- [x] Query planner: AST → execution plan (ScanByLabel, ExpandOut/In, Filter, Project, Sort, LimitOffset)
- [x] Query executor: plan tree walker → ResultSet
- [x] Procedure registry with built-in fullTextSearch and vectorSearch
- [x] PackStream encoding/decoding (ints, strings, lists, maps, bools, nil, structs)
- [x] Chunked Bolt transport
- [x] Full Bolt message loop (HELLO/RUN/PULL/RECORD/SUCCESS/FAILURE/RESET)
- [x] HTTP query endpoint (POST /api/v1/query with Cypher + params)
- [x] StorageBackend() accessor on Graph
- [x] DB() accessor on BadgerStorage
- [x] Main wiring (index, search, executor, updated server constructors)
- [x] E2E query pipeline test
- [x] ~124 new tests

## Phase 4 — Agent Brain (Complete)

### Goals
- Brain Tool API: 7 tools callable via HTTP (search, recall, capture, recent, patterns, reflect, feedback)
- Full-text search integration for brain_search
- Agent context recall via graph traversal
- Reflection cycle creation with learning signal aggregation
- Human feedback loop on reflection cycles

### Deliverables
- [x] Brain tool service (internal/brain/) with Service struct
- [x] brain_search: full-text search across Message, SharedMemory, LearningSignal, Conversation nodes
- [x] brain_recall: agent context loading via edge traversal (conversations, decisions, feedback, threads)
- [x] brain_capture: create LearningSignal nodes with auto-linking to agent and referenced nodes
- [x] brain_recent: time-windowed activity browsing with type/agent filtering
- [x] brain_patterns: tag aggregation from LearningSignal nodes with occurrence thresholds
- [x] brain_reflect: on-demand ReflectionCycle creation with LearningSignal gathering
- [x] brain_feedback: HumanFeedback node creation linked to ReflectionCycle
- [x] HTTP tool execution endpoint (POST /api/v1/tools/{toolName})
- [x] Tool schema listing endpoint (GET /api/v1/tools)
- [x] ServerOption pattern for backward-compatible dependency injection
- [x] Main wiring (brain.NewService → server.WithBrain)
- [x] 25+ brain unit tests (88.8% coverage)
- [x] 12 HTTP brain tool integration tests
- [x] All CI gates passing (82.2% total coverage)

### Next Phase
Phase 5 begins after Phase 4 verification passes.

## Phase 5 — Reflection Engine (Complete)

### Goals
- Automated daily/weekly/monthly reflection cycles
- Deterministic cycle/signal/edge IDs for idempotent execution
- Historic time-windowed graph traversal
- Heuristic scoring (impact, relevance, valueContribution)
- Weekly/monthly rollup of prior cycle signals
- Feedback grace period monitoring
- Brain tool delegation (brain_reflect → CycleRunner)

### Deliverables
- [x] Reflection types (CycleType, CycleRequest, ScoredSignal, CycleResult)
- [x] Deterministic ID functions (CycleNodeID, SignalNodeID, ScoreEdgeID)
- [x] Window truncation (DailyWindowStart, WeeklyWindowStart, MonthlyWindowStart)
- [x] HistoricIterator (NodesInWindow, AgentNodesInWindow)
- [x] Heuristic Scorer (impact, relevance, valueContribution, Jaccard similarity)
- [x] CycleRunner (Execute, ExecuteForBrain) with idempotent upsert semantics
- [x] Weekly/monthly rollup aggregation (ROLLUP_OF edges)
- [x] Schedule type with IsDue logic and ParseSchedule parser
- [x] FeedbackChecker (grace period → NEEDS_FEEDBACK status transition)
- [x] Scheduler Run loop (ticker + context, discovers agents, runs due cycles)
- [x] BrainAdapter (reflect.CycleRunner → brain.ReflectionRunner interface)
- [x] ReflectionRunner interface in brain package (avoids import cycle)
- [x] brain.Reflect() delegation with fallback to original behavior
- [x] ReflectConfig struct with defaults in config.Load()
- [x] Main wiring (CycleRunner always, Scheduler gated by config)
- [x] 83 new reflect package tests
- [x] 3 new brain delegation tests
- [x] 3 E2E reflection tests
- [x] All tests pass with -race detector
- [x] All existing tests continue to pass

### Next Phase
Phase 6 begins after Phase 5 verification passes.

## Phase 6 — Human Feedback Loop (Complete)

### Goals
- Typed FeedbackStatus constants with state machine enforcement
- Publisher interface for outbound feedback request events
- NEEDS_FEEDBACK → REQUESTED transition with event emission
- Inbound human feedback handler (TypeHumanFeedback)
- Score overrides from human feedback applied to learning signals
- HTTP endpoints for cycle listing and waiving
- Brain feedback method updates cycle status and applies overrides

### Deliverables
- [x] FeedbackStatus typed constants (PENDING, NEEDS_FEEDBACK, REQUESTED, RECEIVED, WAIVED)
- [x] Publisher interface + MemoryPublisher (internal/ingest/publisher.go)
- [x] FeedbackRequestEvent and SignalSummary types
- [x] FeedbackChecker NEEDS_FEEDBACK → REQUESTED transition with publisher
- [x] Top-N signal gathering sorted by impact
- [x] SchedulerConfig extended with Publisher, RequestTopic, TopN
- [x] TypeHumanFeedback envelope type + HumanFeedbackPayload
- [x] HumanFeedbackNodeID deterministic ID function
- [x] HandleHumanFeedback handler with score overrides
- [x] Router registration for human_feedback type
- [x] Brain Feedback() updates cycle status to RECEIVED + applies score overrides
- [x] GET /api/v1/cycles with status and agentId filters
- [x] POST /api/v1/cycles/{id}/waive endpoint
- [x] ReflectConfig extended with feedback_request_topic and feedback_top_n
- [x] Main wiring with MemoryPublisher
- [x] ~37 new tests across all packages
- [x] 3 E2E feedback loop tests

### Next Phase
Phase 7 begins after Phase 6 verification passes.

## Phase 7 — Distribution (Complete)

### Goals
- Multi-node cluster with gossip-based membership
- Agent-ID-based graph partitioning (FNV-1a hash)
- Internal RPC for cross-node query forwarding
- Query router with fan-out and result merging
- Backward-compatible QueryExecutor interface

### Deliverables
- [x] Core types: NodeInfo, PartitionStrategy, QueryExecutor interface (cluster/types.go)
- [x] AgentIDPartitioner with FNV-1a hashing (cluster/partition.go)
- [x] PartitionMap with deterministic round-robin rebalancing
- [x] Memberlist wrapper with gossip-based discovery (cluster/membership.go)
- [x] Node metadata encoding/decoding (RPC, Bolt, HTTP ports)
- [x] Async event processing to avoid memberlist lock deadlocks
- [x] Idempotent Leave() for safe multi-call cleanup
- [x] Length-prefixed JSON over TCP RPC (cluster/rpc.go)
- [x] RPCServer and RPCClient with concurrent query support
- [x] QueryRouter with fan-out to all shards and result merging (cluster/router.go)
- [x] Partial failure tolerance (returns available results)
- [x] QueryExecutor interface in server/bolt.go, server/http.go, server/routes.go
- [x] ClusterConfig struct with defaults in config.go
- [x] Main wiring gated by cluster.enabled flag
- [x] 14 partition tests, 8 membership tests, 8 RPC tests, 10 router tests
- [x] 3 E2E cluster tests (3-node join, cross-node query, node leave)
- [x] Distribution architecture documentation (docs/distribution.md)

### Next Phase
Phase 8 begins after Phase 7 verification passes.

## Phase 8 — Management UI (Complete)

### Goals
- Embedded management UI served on the HTTP port (`:7474`)
- Management API at `/api/v2/mgmt/` with 10 endpoints
- Interactive graph browser with Cytoscape.js
- Data visualization with Apache ECharts
- Configuration viewer with secret redaction
- Reflection dashboard with cycle filtering and score charts
- Cluster topology view (read-only)
- Air-gapped deployment via Go `embed.FS`

### Deliverables
- [x] Web embed package (`web/embed.go`) with `//go:embed all:static`
- [x] SPA shell with hash-based routing (`web/static/index.html`, `js/app.js`)
- [x] Custom design system (CSS tokens, layout, components, graph browser, charts)
- [x] Sidebar navigation with live status (nodes, uptime, version)
- [x] Dashboard view with metric cards, activity timeline, node distribution chart
- [x] Graph Browser with Cytoscape.js (search, label filter, depth selection, layout options)
- [x] Node detail panel with property inspection
- [x] Data Stats view with ECharts bar charts and storage metrics
- [x] Configuration view (tabbed: node config + cluster) with S3 secret redaction
- [x] Reflection view with summary cards, feedback pipeline pie chart, filterable cycle table
- [x] Vendored Cytoscape.js and Apache ECharts (air-gapped compatible)
- [x] Management API: 10 endpoints (info, storage, stats, explore, search, config, cluster, reflect/summary, reflect/cycles, activity)
- [x] CORS middleware for cross-origin requests
- [x] Static file serving via `http.FS` with API route precedence
- [x] New ServerOptions: `WithConfig`, `WithMembership`, `WithPartitionMap`
- [x] Main wiring: cfg, membership, partitionMap passed to HTTPServer
- [x] 22 management unit tests
- [x] 5 E2E management tests
- [x] Management UI documentation (`docs/management-ui.md`)
- [x] All existing tests continue to pass

### Next Phase
Phase 9 (Hardening) begins after Phase 8 verification passes.

---

*Last updated: 2026-03-03*
