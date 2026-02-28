#!/usr/bin/env bash
# Syncs API_VERSION constants across all SDKs from openapi.json info.version.
# Usage: scripts/sync-api-version.sh [openapi.json]
set -euo pipefail

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
  sed "$expr" "$file" > "$tmp" && mv "$tmp" "$file"
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

echo "Done."
