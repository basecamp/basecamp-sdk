#!/usr/bin/env bash
#
# Enhance OpenAPI spec with Go-specific type extensions for oapi-codegen.
#
# Type mappings:
#   - _at fields (created_at, updated_at, etc.) → time.Time (full timestamps)
#   - _on fields (due_on, starts_on, etc.) → types.Date (date-only)
#   - id fields → keep as pointers to distinguish nil from zero
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
      # Timestamp fields (_at): use time.Time
      if (.key | test("_at$")) and (.value | type == "object") and (.value.type == "string") then
        .value += {
          "x-go-type": "time.Time",
          "x-go-type-import": {"path": "time"},
          "x-go-type-skip-optional-pointer": true
        }
      # Date-only fields (_on): use types.Date
      elif (.key | test("_on$")) and (.value | type == "object") and (.value.type == "string") then
        .value += {
          "x-go-type": "types.Date",
          "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
          "x-go-type-skip-optional-pointer": true
        }
      # Id fields: keep as pointers (to distinguish nil from zero)
      # Matches "id", "*_id" (e.g., recording_id, category_id, todolist_id)
      elif (.key | test("^id$|_id$")) and (.value | type == "object") and (.value.type == "integer") then
        .value += {
          "x-go-type-skip-optional-pointer": false
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
date_count=$(jq '[.. | objects | select(.["x-go-type"] == "types.Date")] | length' "$OUTPUT_FILE")
id_count=$(jq '[.. | objects | select(.["x-go-type-skip-optional-pointer"] == false)] | length' "$OUTPUT_FILE")

echo "Enhanced OpenAPI spec with Go type extensions:"
echo "  Timestamp fields (time.Time): $timestamp_count"
echo "  Date fields (basecamp.Date): $date_count"
echo "  Id fields (keeping pointers): $id_count"
