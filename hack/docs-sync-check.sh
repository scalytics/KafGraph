#!/usr/bin/env bash
# Copyright 2026 Scalytics, Inc.
# Copyright 2026 Mirko Kämpf
#
# Verify that docs are in sync with code.
# Checks that key API types and endpoints are documented.
# Usage: hack/docs-sync-check.sh

set -euo pipefail

ERRORS=0

# Check that docs directory exists
if [ ! -d "docs" ]; then
    echo "ERROR: docs/ directory not found"
    exit 1
fi

# Check required doc pages exist
REQUIRED_PAGES=(
    "docs/index.md"
    "docs/architecture.md"
    "docs/brain-tool-api.md"
    "docs/getting-started.md"
    "docs/configuration.md"
)

for page in "${REQUIRED_PAGES[@]}"; do
    if [ ! -f "$page" ]; then
        echo "MISSING: $page"
        ERRORS=$((ERRORS + 1))
    fi
done

# Check that brain tool skills are documented
SKILLS=(brain_search brain_recall brain_capture brain_recent brain_patterns brain_reflect brain_feedback)
for skill in "${SKILLS[@]}"; do
    if ! grep -q "$skill" docs/brain-tool-api.md 2>/dev/null; then
        echo "WARNING: $skill not mentioned in docs/brain-tool-api.md"
    fi
done

if [ $ERRORS -gt 0 ]; then
    echo ""
    echo "FAIL: $ERRORS documentation issue(s) found."
    exit 1
fi

echo "PASS: Documentation sync check passed."
