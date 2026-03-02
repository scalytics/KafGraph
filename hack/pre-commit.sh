#!/usr/bin/env bash
# Copyright 2026 Scalytics, Inc.
# Copyright 2026 Mirko Kämpf
#
# Pre-commit hook: runs formatting and lint checks.
# Install: ln -sf ../../hack/pre-commit.sh .git/hooks/pre-commit

set -euo pipefail

echo "Running pre-commit checks..."

# Check formatting
if ! make fmt-check 2>/dev/null; then
    echo "FAIL: Code is not formatted. Run 'make fmt' to fix."
    exit 1
fi

# Run linter
if ! make lint 2>/dev/null; then
    echo "FAIL: Lint errors found. Run 'make lint-fix' to fix."
    exit 1
fi

# Run vet
if ! make vet 2>/dev/null; then
    echo "FAIL: go vet found issues."
    exit 1
fi

echo "Pre-commit checks passed."
