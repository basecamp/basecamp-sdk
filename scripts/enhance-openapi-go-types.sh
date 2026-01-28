#!/usr/bin/env bash
#
# Enhance OpenAPI spec with Go-specific type extensions for oapi-codegen.
#
# Adds x-go-type: "time.Time" to all timestamp fields (those ending in _at or _on)
# so oapi-codegen generates time.Time instead of string.
#
# Usage: ./enhance-openapi-go-types.sh [input.json] [output.json]
#        ./enhance-openapi-go-types.sh               # defaults to openapi.json in-place

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

INPUT_FILE="${1:-$PROJECT_ROOT/openapi.json}"
OUTPUT_FILE="${2:-$INPUT_FILE}"

if [[ ! -f "$INPUT_FILE" ]]; then
    echo "Error: Input file not found: $INPUT_FILE" >&2
    exit 1
fi

jq '
walk(
  if type == "object" then
    to_entries | map(
      if (.key | test("_(at|on)$")) and (.value | type == "object") and (.value.type == "string") then
        .value += {
          "x-go-type": "time.Time",
          "x-go-type-import": {"path": "time"},
          "x-go-type-skip-optional-pointer": true
        }
      else
        .
      end
    ) | from_entries
  else
    .
  end
)
' "$INPUT_FILE" > "${OUTPUT_FILE}.tmp"

mv "${OUTPUT_FILE}.tmp" "$OUTPUT_FILE"

# Count enhancements
timestamp_count=$(jq '[.. | objects | select(.["x-go-type"] == "time.Time")] | length' "$OUTPUT_FILE")

echo "Added x-go-type extensions for timestamp fields"
echo "  Timestamp fields enhanced: $timestamp_count"
