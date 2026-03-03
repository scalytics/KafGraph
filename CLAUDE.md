# KafGraph

KafGraph is the distributed shared brain of collaborating agents — a graph database
and reflection engine written in Go. It ingests agent conversation data from Apache
Kafka topics, structures it as a property graph, and provides temporal reflection
services that allow agents and teams to learn from past interactions.

## Build Commands

```bash
make build           # Build the kafgraph binary
make build-linux     # Cross-compile for linux/amd64
make install         # Install to $GOPATH/bin
make clean           # Remove build artifacts
```

## Test Commands

```bash
make test            # Run all unit + E2E tests
make test-unit       # Unit tests only
make test-e2e        # End-to-end tests (in-process, uses temp BadgerDB)
make test-integration # Integration tests (requires docker-compose)
make test-fuzz       # Fuzz tests (Go native fuzzing)
make test-race       # Unit + E2E with -race detector
```

## Lint & Format

```bash
make lint            # Run golangci-lint
make lint-fix        # Auto-fix lint issues
make vet             # go vet
make fmt             # gofmt -w
make fmt-check       # Check formatting (CI gate)
```

## Coverage

```bash
make cover           # Generate coverage report
make cover-html      # Open HTML coverage report
make cover-check     # Fail if coverage < 80%
```

## Docker

```bash
make docker-build    # Build multi-stage Docker image
make docker-push     # Push to registry
make docker-up       # Start dev environment (MinIO + Kafka + KafGraph)
make docker-down     # Stop dev environment
make docker-logs     # Tail dev environment logs
```

## Docs

```bash
make docs-serve      # Serve Jekyll docs locally (http://localhost:4000)
make docs-build      # Build static Jekyll site
make docs-sync-check # Verify docs match code (CI gate)
```

## Requirements & Specs

```bash
make req-index       # Regenerate SPEC/FR/INDEX.md
make spec-check      # Validate requirement files
```

## Release & Quality Gates

```bash
make commit-check    # Pre-push gate: fmt + vet + race + coverage
make release-check   # Full pre-release gate: lint + test + cover + docs + fmt
make ci              # Simulate CI pipeline locally
```

## Code Maintenance Procedures

**Never commit directly to main after the initial scaffold.** Configure GitHub branch
protection rules: require PR with 0 approvals minimum, merge only after all CI checks
pass. A single broken commit can block the workflow for hours — clean code in main is
worth the discipline.

### Pre-push checklist (`make commit-check`)

Every push must pass: `go fmt`, `go vet`, `go test -race`, 80% coverage minimum, and
`govulncheck`. Run `make commit-check` before every push. The pre-commit hook
(`hack/pre-commit.sh`) enforces formatting and linting automatically.

### CI pipeline (runs on every PR)

1. `golangci-lint` — static analysis with strict config
2. `go vet` — correctness checks
3. `go test -race` — race condition detection on all tests
4. Coverage gate — 80% minimum, 100% on critical paths (storage, graph, server)
5. `gofmt` check — consistent formatting
6. License header check — Apache 2.0 on all `.go` files
7. `gosec` — security-focused static analysis (blocks merge on findings)
8. `govulncheck` — known vulnerability scanning in dependencies
9. Docs sync check — documentation matches code

### Security scanning (free, no GitHub Advanced Security needed)

Instead of CodeQL ($30/month), we use two free tools that cover the same ground:
- **gosec** — Go-specific security linter (SQL injection, command injection, hardcoded
  credentials, weak crypto, etc.). Runs via golangci-lint on every PR.
- **govulncheck** — Go's official vulnerability checker. Scans dependencies against the
  Go vulnerability database. Runs on every PR and locally via `make vulncheck`.

Both block merge on findings. Run `make sec` locally to check both.

### Branch protection rules (configure in GitHub Settings > Rules)

- Require pull request before merging (0 reviews minimum)
- Require status checks to pass: `ci`, `security`
- No direct pushes to main after initial setup

## Conventions

- **Go version**: 1.25+ with `CGO_ENABLED=0` (static linking)
- **Linter**: golangci-lint with config from `.golangci.yml`
- **Coverage**: 80% minimum enforced by `hack/coverage.sh`; 100% on critical packages
- **Race detection**: all tests run with `-race` in CI
- **Security**: gosec + govulncheck on all PRs (free, blocks merge on findings)
- **License**: Apache 2.0 header on all `.go` files (enforced by `hack/license-header.sh`)
- **Sync gate**: every public API change must update `docs/` and tests (release gate)
- **Requirements**: tracked as `SPEC/FR/req-NNN.md` with monotonic numbering
- **Phases**: tracked in `SPEC/PLAN.md`
- **Commit messages**: imperative mood, reference `req-NNN` where applicable
- **No direct commits to main**: always use PRs after initial scaffold

## Architecture

- **SPEC/initial-idea.md** — project vision and motivation
- **SPEC/requirements.md** — full functional, non-functional, and integration requirements
- **SPEC/solution-design.md** — technology selection, layer architecture, data model
- **SPEC/about-agent-brains.md** — agent brain concept and design rationale
- **SPEC/kafclaw-topic-reference.md** — KafClaw topic hierarchy and wire format
- **SPEC/PLAN.md** — phase tracker (0: Foundation → 8: Hardening)

## Key Packages

| Package | Purpose |
|---------|---------|
| `cmd/kafgraph` | Entry point, CLI setup |
| `internal/config` | Viper-based configuration loader |
| `internal/graph` | Core graph API (CRUD nodes/edges, property graph model) |
| `internal/storage` | Storage engines (BadgerDB default) |
| `internal/reflect` | Reflection Engine (scheduler, cycle runner, scorer, feedback checker) |
| `internal/server` | Bolt v4 protocol, HTTP API, Brain Tool API |

## Reference Repos

- **KafScale** (`github.com/scalytics/platform`) — Go infrastructure patterns, Makefile, Docker, CI/CD
- **KafClaw** (`github.com/kamir/KafClaw`) — Agent skills system, SKILL.md manifests, Jekyll docs

## Skills

Each brain tool is defined as a skill in `skills/brain_*/SKILL.md`:
- `brain_search` — Semantic search across the knowledge graph
- `brain_recall` — Load accumulated agent context
- `brain_capture` — Write insights/decisions into the brain
- `brain_recent` — Browse recent activity
- `brain_patterns` — Surface recurring themes and connections
- `brain_reflect` — Trigger on-demand reflection cycle
- `brain_feedback` — Submit human feedback on reflection cycles
