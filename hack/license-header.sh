#!/usr/bin/env bash
# Copyright 2026 Scalytics, Inc.
# Copyright 2026 Mirko Kämpf
#
# Check that all .go files have the Apache 2.0 license header.
# Usage: hack/license-header.sh [--fix]

set -euo pipefail

FIX="${1:-}"
MISSING=()

EXPECTED="// Copyright 2026 Scalytics, Inc.
// Copyright 2026 Mirko Kämpf
//
// Licensed under the Apache License, Version 2.0"

while IFS= read -r -d '' file; do
    if ! head -4 "$file" | grep -q "Licensed under the Apache License"; then
        MISSING+=("$file")
    fi
done < <(find . -name '*.go' -not -path './vendor/*' -print0)

if [ ${#MISSING[@]} -eq 0 ]; then
    echo "PASS: All Go files have license headers."
    exit 0
fi

echo "Files missing Apache 2.0 license header:"
for f in "${MISSING[@]}"; do
    echo "  $f"
done

if [ "$FIX" = "--fix" ]; then
    HEADER="// Copyright 2026 Scalytics, Inc.
// Copyright 2026 Mirko Kämpf
//
// Licensed under the Apache License, Version 2.0 (the \"License\");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an \"AS IS\" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

"
    for f in "${MISSING[@]}"; do
        TMPFILE=$(mktemp)
        echo "$HEADER" > "$TMPFILE"
        cat "$f" >> "$TMPFILE"
        mv "$TMPFILE" "$f"
        echo "  Fixed: $f"
    done
    echo "Done. Re-run without --fix to verify."
else
    echo ""
    echo "FAIL: ${#MISSING[@]} file(s) missing license headers."
    echo "Run 'hack/license-header.sh --fix' to add them automatically."
    exit 1
fi
