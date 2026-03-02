#!/usr/bin/env bash
# Copyright 2026 Scalytics, Inc.
# Copyright 2026 Mirko Kämpf
#
# One-command development environment setup.
# Usage: make dev-setup  OR  bash scripts/dev-setup.sh

set -euo pipefail

echo "=== KafGraph Dev Setup ==="
echo ""

# Check Go version
if ! command -v go &>/dev/null; then
    echo "ERROR: Go is not installed. Install Go 1.25+ from https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "Go version: $GO_VERSION"

# Download Go dependencies
echo "Downloading Go dependencies..."
go mod download
echo "  Done."

# Install golangci-lint
if ! command -v golangci-lint &>/dev/null; then
    echo "Installing golangci-lint..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    echo "  Done."
else
    echo "golangci-lint: $(golangci-lint --version 2>&1 | head -1)"
fi

# Create .env from example if it doesn't exist
if [ ! -f .env ]; then
    echo "Creating .env from .env.example..."
    cp .env.example .env
    echo "  Done. Edit .env for your local setup."
else
    echo ".env already exists, skipping."
fi

# Create data directory
mkdir -p data
echo "Data directory: ./data"

# Install pre-commit hook
if [ -d .git ]; then
    echo "Installing pre-commit hook..."
    ln -sf ../../hack/pre-commit.sh .git/hooks/pre-commit
    echo "  Done."
fi

# Build
echo ""
echo "Building kafgraph..."
make build
echo "  Done."

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "  make test          Run tests"
echo "  make dev-run       Build and run locally"
echo "  make docker-up     Start dev environment with Docker"
echo "  make help          Show all available targets"
