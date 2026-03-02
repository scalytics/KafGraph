---
layout: default
title: Getting Started
nav_order: 4
---

# Getting Started

## Prerequisites

- Go 1.25 or later
- Docker and Docker Compose (for integration tests)
- Ruby and Bundler (for docs, optional)

## Quick Setup

```bash
# Clone the repository
git clone https://github.com/scalytics/kafgraph.git
cd kafgraph

# Run the automated dev setup
make dev-setup

# Build the binary
make build

# Run tests
make test

# Run locally
make dev-run
```

## Manual Setup

### 1. Install Go Dependencies

```bash
go mod download
```

### 2. Install golangci-lint

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### 3. Build

```bash
make build
```

The binary is output to `bin/kafgraph`.

### 4. Configure

Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` to match your local setup. See [Configuration](configuration.md) for details.

### 5. Run

```bash
./bin/kafgraph
```

## Development Workflow

```bash
# Format code
make fmt

# Run linter
make lint

# Run tests with race detector
make test-race

# Check coverage
make cover-check

# Full CI check (what CI runs)
make ci

# Serve docs locally
cd docs && bundle install && cd ..
make docs-serve
```

## Docker

```bash
# Build Docker image
make docker-build

# Start dev environment (MinIO + Kafka + KafGraph)
make docker-up

# View logs
make docker-logs

# Stop
make docker-down
```
