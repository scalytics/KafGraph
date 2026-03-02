# KafGraph — Makefile
# Distributed graph database and agent brain
#
# Usage: make help

# ─── Variables ───────────────────────────────────────────────────────────────

GO          ?= go
GOFLAGS     ?=
CGO_ENABLED ?= 0
GOOS        ?= $(shell $(GO) env GOOS)
GOARCH      ?= $(shell $(GO) env GOARCH)

BINARY      := kafgraph
MODULE      := github.com/scalytics/kafgraph
CMD_DIR     := ./cmd/kafgraph

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE  := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS     := -s -w \
	-X $(MODULE)/internal/config.Version=$(VERSION) \
	-X $(MODULE)/internal/config.GitCommit=$(GIT_COMMIT) \
	-X $(MODULE)/internal/config.BuildDate=$(BUILD_DATE)

BIN_DIR     := bin
STAMP_DIR   := .build

DOCKER_REGISTRY ?= ghcr.io/scalytics
DOCKER_IMAGE    ?= $(DOCKER_REGISTRY)/kafgraph
DOCKER_TAG      ?= $(VERSION)

COVERAGE_MIN   ?= 80
COVERAGE_FILE  := coverage.out

GOLANGCI_LINT  ?= golangci-lint
GOLANGCI_FLAGS ?= --config .golangci.yml

JEKYLL         ?= bundle exec jekyll
DOCS_DIR       := docs

# ─── Help ────────────────────────────────────────────────────────────────────

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@echo "KafGraph — make targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""

# ─── Build ───────────────────────────────────────────────────────────────────

.PHONY: build
build: ## Build the kafgraph binary
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) $(CMD_DIR)

.PHONY: build-linux
build-linux: ## Cross-compile for linux/amd64
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY)-linux-amd64 $(CMD_DIR)

.PHONY: install
install: ## Install to $GOPATH/bin
	CGO_ENABLED=$(CGO_ENABLED) $(GO) install $(GOFLAGS) -ldflags '$(LDFLAGS)' $(CMD_DIR)

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) $(STAMP_DIR) $(COVERAGE_FILE) coverage.html
	$(GO) clean -cache -testcache

# ─── Test ────────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run all unit and E2E tests
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) ./...

.PHONY: test-unit
test-unit: ## Run unit tests only (exclude e2e and integration)
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) $(shell $(GO) list ./... | grep -v '/test/')

.PHONY: test-e2e
test-e2e: ## Run E2E tests (in-process, uses temp BadgerDB)
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) -tags=e2e -v ./test/e2e/...

.PHONY: test-integration
test-integration: ## Run integration tests (requires docker-compose)
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) -tags=integration -v -timeout 300s ./test/integration/...

.PHONY: test-fuzz
test-fuzz: ## Run fuzz tests (30s default)
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) -fuzz=. -fuzztime=30s ./...

.PHONY: test-race
test-race: ## Run unit + E2E tests with race detector
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) -race ./...

# ─── Coverage ────────────────────────────────────────────────────────────────

.PHONY: cover
cover: ## Generate coverage report
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	$(GO) tool cover -func=$(COVERAGE_FILE)

.PHONY: cover-html
cover-html: cover ## Open HTML coverage report
	$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: cover-check
cover-check: cover ## Fail if coverage < $(COVERAGE_MIN)%
	@bash hack/coverage.sh $(COVERAGE_FILE) $(COVERAGE_MIN)

# ─── Lint & Format ──────────────────────────────────────────────────────────

.PHONY: lint
lint: ## Run golangci-lint
	$(GOLANGCI_LINT) run $(GOLANGCI_FLAGS) ./...

.PHONY: lint-fix
lint-fix: ## Auto-fix lint issues
	$(GOLANGCI_LINT) run $(GOLANGCI_FLAGS) --fix ./...

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: fmt
fmt: ## Format Go source files
	gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## Check formatting (CI gate)
	@test -z "$$(gofmt -l .)" || (echo "Files not formatted:"; gofmt -l .; exit 1)

# ─── Docker ──────────────────────────────────────────────────────────────────

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f deploy/Dockerfile .

.PHONY: docker-push
docker-push: ## Push Docker image to registry
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-up
docker-up: ## Start dev environment (MinIO + Kafka + KafGraph)
	docker compose -f deploy/docker-compose.yml up -d

.PHONY: docker-down
docker-down: ## Stop dev environment
	docker compose -f deploy/docker-compose.yml down

.PHONY: docker-logs
docker-logs: ## Tail dev environment logs
	docker compose -f deploy/docker-compose.yml logs -f

# ─── Docs ────────────────────────────────────────────────────────────────────

.PHONY: docs-serve
docs-serve: ## Serve Jekyll docs locally
	cd $(DOCS_DIR) && $(JEKYLL) serve --livereload

.PHONY: docs-build
docs-build: ## Build static Jekyll site
	cd $(DOCS_DIR) && $(JEKYLL) build

.PHONY: docs-sync-check
docs-sync-check: ## Verify docs match code (CI gate)
	@bash hack/docs-sync-check.sh

# ─── Code Generation ────────────────────────────────────────────────────────

.PHONY: generate
generate: ## Run code generation (reserved for future use)
	$(GO) generate ./...

# ─── SPEC & Requirements ────────────────────────────────────────────────────

.PHONY: req-index
req-index: ## Regenerate SPEC/FR/INDEX.md
	@bash hack/req-index.sh

.PHONY: spec-check
spec-check: ## Validate requirement files
	@bash hack/req-index.sh --check

# ─── Release Gate ────────────────────────────────────────────────────────────

.PHONY: release-check
release-check: lint test-race cover-check fmt-check docs-sync-check ## Full pre-release gate
	@echo "All release checks passed."

# ─── Dev ─────────────────────────────────────────────────────────────────────

.PHONY: dev-setup
dev-setup: ## One-command dev environment setup
	@bash scripts/dev-setup.sh

.PHONY: dev-run
dev-run: build ## Build and run locally
	./$(BIN_DIR)/$(BINARY)

.PHONY: dev-clean
dev-clean: clean docker-down ## Clean everything including Docker

# ─── Commit Check (pre-push quality gate) ────────────────────────────────────

.PHONY: commit-check
commit-check: fmt vet test-race cover-check ## Pre-commit quality gate (fmt, vet, race, coverage)
	@echo "commit-check passed — safe to push."

.PHONY: commit-and-push
commit-and-push: commit-check ## Run quality gates, commit all changes, and push
	@if [ -z "$$(git status --porcelain)" ]; then \
		echo "Nothing to commit."; \
	else \
		git add -A && \
		read -p "Commit message: " MSG && \
		git commit -m "$$MSG" && \
		git push; \
	fi

# ─── CI ──────────────────────────────────────────────────────────────────────

.PHONY: ci
ci: lint test-race cover-check fmt-check ## Simulate CI pipeline locally
	@echo "CI checks passed."
