#!/usr/bin/env bash
# Copyright 2026 Scalytics, Inc.
# Copyright 2026 Mirko Kämpf
#
# Fail if total Go coverage drops below the configured threshold.
# Usage: hack/coverage.sh <coverage.out> <min-percent>

set -euo pipefail

if [ "$#" -ne 2 ]; then
    echo "usage: $0 <coverage-file> <min-percent>" >&2
    exit 2
fi

COVERAGE_FILE="$1"
MIN_COVERAGE="$2"

if [ ! -f "$COVERAGE_FILE" ]; then
    echo "FAIL: coverage file not found: $COVERAGE_FILE" >&2
    exit 1
fi

TOTAL_COVERAGE="$(
    go tool cover -func="$COVERAGE_FILE" \
        | awk '/^total:/ { sub(/%/, "", $3); print $3 }'
)"

if [ -z "$TOTAL_COVERAGE" ]; then
    echo "FAIL: could not read total coverage from $COVERAGE_FILE" >&2
    exit 1
fi

if awk -v total="$TOTAL_COVERAGE" -v min="$MIN_COVERAGE" 'BEGIN { exit !(total + 0 >= min + 0) }'; then
    echo "PASS: coverage ${TOTAL_COVERAGE}% >= ${MIN_COVERAGE}%"
    exit 0
fi

echo "FAIL: coverage ${TOTAL_COVERAGE}% < ${MIN_COVERAGE}%"
exit 1
