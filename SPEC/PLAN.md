# KafGraph — Phase Plan

*Living document — updated after each milestone.*

## Phases

| Phase | Name | Status | Description |
|-------|------|--------|-------------|
| 0 | Foundation | **Complete** | BadgerDB storage, graph API, Bolt handshake, config, Makefile, docs, CI/CD |
| 1 | Running Server | **Complete** | HTTP REST API, Bolt accept loop, graceful shutdown, 84.9% coverage |
| 2 | Processor | **Complete** | KafScale 5-layer processor, SegmentReader interface, GroupEnvelope decoder, ingest pipeline |
| 3 | Query | Planned | OpenCypher parser, vector index (ANN), full-text search |
| 4 | Agent Brain | Planned | Brain Tool API (7 tools), KafClaw skill registration, embedding pipeline |
| 5 | Reflection | Planned | Reflection scheduler, historic iterator, daily/weekly/monthly cycles, scoring |
| 6 | Feedback | Planned | Human feedback loop, impact tracking, feedback-request events |
| 7 | Distribution | Planned | Multi-node cluster, gossip, cross-partition queries, partition strategies |
| 8 | Hardening | Planned | TLS, encryption at rest, OpenTelemetry, Helm chart, load tests |

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

### Next Phase
Phase 3 begins after Phase 2 verification passes.

---

*Last updated: 2026-03-02*
