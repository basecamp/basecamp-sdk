#!/usr/bin/env bash
# Bucket↔flat parity lint.
#
# For every GET /{accountId}/buckets/{bucketId}/<resource>(/{filter})?.json operation
# whose response is a list, check that a flat counterpart at
# /{accountId}/<resource>(/{filter})?.json exists. If not, the path must be
# entered in spec/bucket-scoped-allowlist.txt with a justification.
#
# Cross-project SDK consumers shouldn't have to walk every project to query
# resources that already exist account-wide. Catching this early at the spec
# layer prevents bucket-only collections from shipping by accident.
#
# Allowlist format: one bucket path-pattern per line. Comments (lines starting
# with `#`) and blank lines are ignored. Each entry should have a justification
# comment on the line immediately above it.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

SPEC="${1:-$PROJECT_ROOT/openapi.json}"
ALLOWLIST="${2:-$PROJECT_ROOT/spec/bucket-scoped-allowlist.txt}"

if [ ! -f "$SPEC" ]; then
  echo "ERROR: openapi spec not found at $SPEC" >&2
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required" >&2
  exit 2
fi

# Build the set of allowlisted patterns.
ALLOWED=""
if [ -f "$ALLOWLIST" ]; then
  ALLOWED=$(grep -Ev '^\s*(#|$)' "$ALLOWLIST" || true)
fi

# Find candidate bucket-scoped GET list operations and their expected flat paths.
# A "list" response is a top-level array (recursively resolving $ref one level).
# jq exits non-zero for invalid OpenAPI / broken filter — let it fail loud rather
# than silently turning bad input into "0 operations found".
CANDIDATES=$(jq -r '
  . as $s
  | $s.paths
  | to_entries[]
  | select(.key | test("^/\\{accountId\\}/buckets/\\{bucketId\\}/[^/]+(/\\{[^}]+\\})?\\.json$"))
  | select(.value.get != null)
  | . as $entry
  | (
      $entry.value.get.responses["200"].content["application/json"].schema as $rs
      | if $rs == null then null
        elif $rs["$ref"] then
          ($rs["$ref"] | sub("^#/components/schemas/"; "")) as $name
          | $s.components.schemas[$name]
        else $rs end
    ) as $resolved
  | select($resolved.type == "array")
  | $entry.key
' "$SPEC")

if [ -z "$CANDIDATES" ]; then
  echo "==> Bucket↔flat parity: 0 bucket-scoped list operations found"
  exit 0
fi

VIOLATIONS=""
COUNT=0
while IFS= read -r BUCKET_PATH; do
  [ -z "$BUCKET_PATH" ] && continue
  COUNT=$((COUNT + 1))

  # Compute the flat path: strip /buckets/{bucketId} from the bucket path.
  FLAT_PATH=$(echo "$BUCKET_PATH" | sed 's|/buckets/{bucketId}||')

  # Already covered by a flat sibling?
  if jq -e --arg p "$FLAT_PATH" '.paths[$p].get != null' "$SPEC" >/dev/null 2>&1; then
    continue
  fi

  # Allowlisted?
  if echo "$ALLOWED" | grep -Fxq "$BUCKET_PATH"; then
    continue
  fi

  VIOLATIONS="${VIOLATIONS}${BUCKET_PATH}|${FLAT_PATH}
"
done <<< "$CANDIDATES"

echo "==> Bucket↔flat parity: scanned $COUNT bucket-scoped list operation(s)"

if [ -n "$VIOLATIONS" ]; then
  echo ""
  echo "ERROR: bucket-scoped list operations without a flat counterpart:" >&2
  while IFS='|' read -r BP FP; do
    [ -z "$BP" ] && continue
    echo "  $BP" >&2
    echo "    expected flat path: $FP" >&2
  done <<< "$VIOLATIONS"
  echo "" >&2
  echo "Either add the flat path to the spec, or add the bucket path to" >&2
  echo "$ALLOWLIST with a justification comment immediately above the entry." >&2
  exit 1
fi

echo "Bucket↔flat parity is clean"
