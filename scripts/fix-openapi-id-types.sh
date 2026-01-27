#!/usr/bin/env bash
#
# Fix OpenAPI ID types: change "number" to "integer" with format "int64"
# for properties named *Id, *_id, or id.
#
# Usage: ./fix-openapi-id-types.sh [input.json] [output.json]
#        ./fix-openapi-id-types.sh               # defaults to openapi.json in-place

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

INPUT_FILE="${1:-$PROJECT_ROOT/openapi.json}"
OUTPUT_FILE="${2:-$INPUT_FILE}"

if [[ ! -f "$INPUT_FILE" ]]; then
    echo "Error: Input file not found: $INPUT_FILE" >&2
    exit 1
fi

# Count matches before fixing
before_count=$(jq '[paths(type == "object" and .type == "number")] | length' "$INPUT_FILE")

# Fix ID types using jq walk function
# This recursively walks the JSON and fixes:
# 1. Properties with key names matching *Id, *_id, or id with type: "number"
# 2. OpenAPI parameters with name matching ID patterns and schema.type: "number"
jq '
def fix_id_types:
  walk(
    if type == "object" then
      # Fix property keys that match ID patterns
      to_entries | map(
        if (.key | test("(?i)(Id|_id|^id)$")) and (.value | type == "object") and (.value.type == "number") then
          .value = {type: "integer", format: "int64"}
        # Fix OpenAPI parameters with name matching ID patterns
        elif .key == "parameters" and (.value | type == "array") then
          .value = (.value | map(
            if (.name | test("(?i)(Id|_id|^id)$")) and (.schema.type == "number") then
              .schema = {type: "integer", format: "int64"}
            else
              .
            end
          ))
        else
          .
        end
      ) | from_entries
    else
      .
    end
  );

fix_id_types
' "$INPUT_FILE" > "${OUTPUT_FILE}.tmp"

mv "${OUTPUT_FILE}.tmp" "$OUTPUT_FILE"

# Count matches after fixing
after_count=$(jq '[paths(type == "object" and .type == "number")] | length' "$OUTPUT_FILE")
fixed_count=$((before_count - after_count))

echo "Fixed $fixed_count ID type(s) in $OUTPUT_FILE"
echo "  Before: $before_count 'number' types"
echo "  After:  $after_count 'number' types"
