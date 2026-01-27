#!/usr/bin/env bash
#
# Generate behavior-model.json from Smithy AST JSON.
#
# Extracts operation semantics (readonly, idempotent, pagination, retry policies)
# and redaction rules (sensitive fields) from the Smithy model.
#
# Prerequisites: Run `cd spec && smithy build` first to generate the AST.
#
# Usage: ./generate-behavior-model.sh [model.json] [output.json]
#        ./generate-behavior-model.sh  # uses defaults

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

MODEL_FILE="${1:-$PROJECT_ROOT/spec/build/smithy/source/model/model.json}"
OUTPUT_FILE="${2:-$PROJECT_ROOT/behavior-model.json}"

if [[ ! -f "$MODEL_FILE" ]]; then
    echo "Error: Model file not found: $MODEL_FILE" >&2
    echo "Run 'cd spec && smithy build' first to generate the Smithy AST." >&2
    exit 1
fi

# Generate behavior model using jq
jq '
# Helper to extract operation name from qualified name (e.g., "basecamp#ListProjects" -> "ListProjects")
def op_name: split("#") | .[-1];

# Helper to check if a trait exists
def has_trait($name): .traits[$name] != null;

# Helper to get trait value
def get_trait($name): .traits[$name];

# Get all sensitive type names
def sensitive_types:
  [.shapes | to_entries[] | select(.value.traits["smithy.api#sensitive"]?) | .key | op_name];

# Build redaction rules by finding structures with sensitive field targets
def build_redaction($sensitive):
  [.shapes | to_entries[] |
    select(.value.type == "structure") |
    select(.value.members != null) |
    .key as $struct_name |
    .value.members | to_entries |
    map(select(.value.target | op_name | IN($sensitive[]))) |
    select(length > 0) |
    {
      key: ($struct_name | op_name),
      value: map("$." + .key)
    }
  ] | from_entries;

# Process a single operation into behavior model format
def process_operation:
  . as $op |
  {
    readonly: (if has_trait("smithy.api#readonly") then true else null end),
    idempotent: (if has_trait("smithy.api#idempotent") or has_trait("basecamp.traits#basecampIdempotent") then true else null end),
    pagination: (
      if has_trait("basecamp.traits#basecampPagination") then
        get_trait("basecamp.traits#basecampPagination") |
        {style: .style} +
        (if .maxPageSize then {maxPageSize: .maxPageSize} else {} end)
      else null end
    ),
    retry: (
      if has_trait("basecamp.traits#basecampRetry") then
        get_trait("basecamp.traits#basecampRetry") |
        {
          max: .maxAttempts,
          base_delay_ms: .baseDelayMs,
          backoff: .backoff,
          retry_on: .retryOn
        }
      elif has_trait("smithy.api#readonly") then
        {max: 3, base_delay_seconds: 1, backoff: "exp+jitter"}
      else
        {max: 0}
      end
    )
  } |
  # Remove null values
  with_entries(select(.value != null));

# Main transformation
sensitive_types as $sensitive |
{
  "$schema": "https://basecamp.com/schemas/behavior-model.json",
  version: "1.0.0",
  generated: true,
  operations: (
    [.shapes | to_entries[] |
      select(.value.type == "operation") |
      {
        key: (.key | op_name),
        value: (.value | process_operation)
      }
    ] | sort_by(.key) | from_entries
  ),
  redaction: build_redaction($sensitive),
  sensitiveTypes: ($sensitive | sort)
}
' "$MODEL_FILE" > "${OUTPUT_FILE}.tmp"

mv "${OUTPUT_FILE}.tmp" "$OUTPUT_FILE"

# Summary output
op_count=$(jq '.operations | length' "$OUTPUT_FILE")
readonly_count=$(jq '[.operations | to_entries[] | select(.value.readonly == true)] | length' "$OUTPUT_FILE")
paginated_count=$(jq '[.operations | to_entries[] | select(.value.pagination != null)] | length' "$OUTPUT_FILE")
redaction_count=$(jq '.redaction | length' "$OUTPUT_FILE")
sensitive_count=$(jq '.sensitiveTypes | length' "$OUTPUT_FILE")

echo "Generated $OUTPUT_FILE"
echo "  Operations: $op_count ($readonly_count readonly, $paginated_count paginated)"
echo "  Redaction rules: $redaction_count structures"
echo "  Sensitive types: $sensitive_count"
