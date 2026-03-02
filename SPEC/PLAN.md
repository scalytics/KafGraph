# KafGraph — Phase Plan

*Living document — updated after each milestone.*

## Phases

| Phase | Name | Status | Description |
|-------|------|--------|-------------|
| 0 | Foundation | **Active** | BadgerDB storage, graph API, Bolt handshake, config, Makefile, docs, CI/CD |
| 1 | Processor | Planned | KafScale 5-layer processor, S3 segment ingestion, GroupEnvelope decoder |
| 2 | Query | Planned | OpenCypher parser, vector index (ANN), full-text search |
| 3 | Agent Brain | Planned | Brain Tool API (7 tools), KafClaw skill registration, embedding pipeline |
| 4 | Reflection | Planned | Reflection scheduler, historic iterator, daily/weekly/monthly cycles, scoring |
| 5 | Feedback | Planned | Human feedback loop, impact tracking, feedback-request events |
| 6 | Distribution | Planned | Multi-node cluster, gossip, cross-partition queries, partition strategies |
| 7 | Hardening | Planned | TLS, encryption at rest, OpenTelemetry, Helm chart, load tests |

## Phase 0 — Foundation (Current)

### Goals
- Project scaffold with all build, test, and CI infrastructure
- Core property graph data model with BadgerDB backend
- Bolt v4.4 protocol handshake
- Viper-based configuration
- Jekyll documentation site
- Requirements tracking framework

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
- [ ] Pass `make ci` locally
- [ ] Pass `make release-check`

### Next Phase
Phase 1 begins after `make release-check` passes and Phase 0 deliverables are verified.

---

*Last updated: 2026-03-02*
