#!/usr/bin/env bash
# Syncs the Smithy service version from spec/api-provenance.json.
# Usage: scripts/sync-spec-version.sh [api-provenance.json] [basecamp.smithy]
set -euo pipefail

PROVENANCE_FILE="${1:-spec/api-provenance.json}"
SMITHY_FILE="${2:-spec/basecamp.smithy}"

if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required" >&2
  exit 1
fi

BC3_DATE=$(jq -r '.bc3.date' "$PROVENANCE_FILE")

if [ -z "$BC3_DATE" ] || [ "$BC3_DATE" = "null" ]; then
  echo "ERROR: Could not read bc3.date from $PROVENANCE_FILE" >&2
  exit 1
fi

if ! printf '%s' "$BC3_DATE" | grep -Eq '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'; then
  echo "ERROR: Invalid provenance date format: $BC3_DATE" >&2
  exit 1
fi

# Portable in-place sed: use temp file instead of -i flag
sedi() {
  local expr="$1" file="$2"
  local tmp
  tmp=$(mktemp)
  sed "$expr" "$file" > "$tmp" && cat "$tmp" > "$file" && rm "$tmp"
}

echo "Syncing Smithy service version: $BC3_DATE"

sedi "s/^  version: \".*\"/  version: \"$BC3_DATE\"/" "$SMITHY_FILE"

if ! grep -q "  version: \"$BC3_DATE\"" "$SMITHY_FILE"; then
  echo "ERROR: Failed to update version line in $SMITHY_FILE" >&2
  exit 1
fi

echo "Done."
