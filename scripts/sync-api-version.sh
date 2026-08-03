#!/usr/bin/env bash
# Syncs API_VERSION constants across all SDKs from openapi.json info.version,
# then syncs the doc constants restated in prose (marked spans only) from the
# same sources — see scripts/sync-doc-constants.rb.
# Usage: scripts/sync-api-version.sh [openapi.json]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

OPENAPI="${1:-openapi.json}"

if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required" >&2
  exit 1
fi

API_VERSION=$(jq -r '.info.version' "$OPENAPI")
if [ -z "$API_VERSION" ] || [ "$API_VERSION" = "null" ]; then
  echo "ERROR: Could not read info.version from $OPENAPI" >&2
  exit 1
fi

# Portable in-place sed: use temp file instead of -i flag
sedi() {
  local expr="$1" file="$2"
  local tmp
  tmp=$(mktemp)
  sed "$expr" "$file" > "$tmp" && cat "$tmp" > "$file" && rm "$tmp"
}

echo "Syncing API version: $API_VERSION"

# Go
sedi "s/^const APIVersion = \".*\"/const APIVersion = \"$API_VERSION\"/" \
  go/pkg/basecamp/version.go

# TypeScript
sedi "s/^export const API_VERSION = \".*\"/export const API_VERSION = \"$API_VERSION\"/" \
  typescript/src/client.ts

# Ruby
sedi "s/^  API_VERSION = \".*\"/  API_VERSION = \"$API_VERSION\"/" \
  ruby/lib/basecamp/version.rb

# Kotlin
sedi "s/const val API_VERSION = \".*\"/const val API_VERSION = \"$API_VERSION\"/" \
  kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/BasecampConfig.kt

# Swift
sedi "s/public static let apiVersion = \".*\"/public static let apiVersion = \"$API_VERSION\"/" \
  swift/Sources/Basecamp/BasecampConfig.swift

# Python
sedi "s/^API_VERSION = \".*\"/API_VERSION = \"$API_VERSION\"/" \
  python/src/basecamp/_version.py

# Prose: the same constants restated in SPEC.md / COORDINATION.md / api-gaps.
# Only HTML-comment-marked spans are touched, so the ~20 historical bc3 SHAs
# cited in spec/api-gaps/ narrative are left alone. Assertion-type table drift
# is reported by `make doc-constants-check`, not fixed here — a new row needs a
# human-written description, and failing here would break every `make generate`.
# $OPENAPI is forwarded, not re-defaulted: a caller who passes their own spec
# file must get the SDK constants AND the prose from that same file, or one
# sync leaves the two disagreeing.
if command -v ruby >/dev/null 2>&1; then
  ruby "$SCRIPT_DIR/sync-doc-constants.rb" --write --openapi "$OPENAPI"
else
  echo "WARNING: ruby not found; skipped prose doc-constant sync (make doc-constants-check will fail)" >&2
fi

echo "Done."
