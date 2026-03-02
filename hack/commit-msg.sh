#!/usr/bin/env bash
# Copyright 2026 Scalytics, Inc.
# Copyright 2026 Mirko Kämpf
#
# Auto-generate a commit message from staged/unstaged changes.
# Usage: bash hack/commit-msg.sh

set -euo pipefail

# Collect change summary from git
ADDED=$(git diff --cached --diff-filter=A --name-only 2>/dev/null | wc -l | tr -d ' ')
MODIFIED=$(git diff --cached --diff-filter=M --name-only 2>/dev/null | wc -l | tr -d ' ')
DELETED=$(git diff --cached --diff-filter=D --name-only 2>/dev/null | wc -l | tr -d ' ')

# If nothing is staged yet, count unstaged + untracked
if [ "$((ADDED + MODIFIED + DELETED))" -eq 0 ]; then
    ADDED=$(git ls-files --others --exclude-standard 2>/dev/null | wc -l | tr -d ' ')
    MODIFIED=$(git diff --name-only 2>/dev/null | wc -l | tr -d ' ')
    DELETED=0
fi

# Identify which areas changed
AREAS=()
has_changes_in() { git status --porcelain | grep -q "$1"; }

has_changes_in "cmd/"          2>/dev/null && AREAS+=("cmd")
has_changes_in "internal/"     2>/dev/null && AREAS+=("internal")
has_changes_in "test/"         2>/dev/null && AREAS+=("test")
has_changes_in "docs/"         2>/dev/null && AREAS+=("docs")
has_changes_in "deploy/"       2>/dev/null && AREAS+=("deploy")
has_changes_in "hack/"         2>/dev/null && AREAS+=("hack")
has_changes_in "scripts/"      2>/dev/null && AREAS+=("scripts")
has_changes_in "skills/"       2>/dev/null && AREAS+=("skills")
has_changes_in "SPEC/"         2>/dev/null && AREAS+=("spec")
has_changes_in ".github/"      2>/dev/null && AREAS+=("ci")
has_changes_in "Makefile"      2>/dev/null && AREAS+=("build")
has_changes_in ".golangci"     2>/dev/null && AREAS+=("lint")
has_changes_in "CLAUDE.md"     2>/dev/null && AREAS+=("conventions")
has_changes_in "go.mod"        2>/dev/null && AREAS+=("deps")
has_changes_in "Dockerfile"    2>/dev/null && AREAS+=("docker")
has_changes_in "_config.yml"   2>/dev/null && AREAS+=("docs")

# Deduplicate areas
AREAS=($(printf '%s\n' "${AREAS[@]}" | sort -u))

# Build scope prefix
if [ ${#AREAS[@]} -eq 0 ]; then
    SCOPE=""
elif [ ${#AREAS[@]} -le 3 ]; then
    SCOPE="$(IFS=','; echo "${AREAS[*]}"): "
else
    SCOPE="$(IFS=','; echo "${AREAS[*]:0:3}"),+$((${#AREAS[@]}-3)): "
fi

# Determine verb
if [ "$ADDED" -gt 0 ] && [ "$MODIFIED" -eq 0 ] && [ "$DELETED" -eq 0 ]; then
    VERB="add"
elif [ "$MODIFIED" -gt 0 ] && [ "$ADDED" -eq 0 ] && [ "$DELETED" -eq 0 ]; then
    VERB="update"
elif [ "$DELETED" -gt 0 ] && [ "$ADDED" -eq 0 ] && [ "$MODIFIED" -eq 0 ]; then
    VERB="remove"
else
    VERB="update"
fi

# Build description
PARTS=()
[ "$ADDED" -gt 0 ]    && PARTS+=("${ADDED} added")
[ "$MODIFIED" -gt 0 ] && PARTS+=("${MODIFIED} modified")
[ "$DELETED" -gt 0 ]  && PARTS+=("${DELETED} deleted")
DETAIL=$(IFS=', '; echo "${PARTS[*]}")

echo "${SCOPE}${VERB} files (${DETAIL})"
