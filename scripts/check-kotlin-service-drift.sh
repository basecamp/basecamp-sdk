#!/bin/bash
# check-kotlin-service-drift.sh
#
# Compares generated Kotlin service operation IDs against OpenAPI spec operations.
# Detects drift when the generator needs to be re-run.
#
# Run after: make kt-generate-services
# Exit codes:
#   0 = No drift detected
#   1 = Drift detected (operation set mismatch)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

OPENAPI_FILE="$ROOT_DIR/openapi.json"
SERVICE_DIR="$ROOT_DIR/kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated/services"

SPEC_TMP=$(mktemp)
GEN_TMP=$(mktemp)
trap 'rm -f "$SPEC_TMP" "$GEN_TMP"' EXIT

# Extract operationIds from OpenAPI spec (sorted, deduplicated)
jq -r '[.paths | to_entries[] | .value | to_entries[] | select(.value.operationId) | .value.operationId] | .[]' "$OPENAPI_FILE" | LC_ALL=C sort -u > "$SPEC_TMP"

# Extract operationIds from generated service code (sorted, deduplicated)
for f in "$SERVICE_DIR"/*.kt; do
  case "$(basename "$f")" in
    Types.kt) continue ;;
  esac
  grep 'operation = "' "$f" 2>/dev/null | sed 's/.*operation = "//;s/".*//' || true
done | LC_ALL=C sort -u > "$GEN_TMP"

SPEC_COUNT=$(wc -l < "$SPEC_TMP" | tr -d ' ')
GEN_COUNT=$(wc -l < "$GEN_TMP" | tr -d ' ')

echo "OpenAPI spec operations: $SPEC_COUNT"
echo "Generated Kotlin operations: $GEN_COUNT"

# Compare operation ID sets
MISSING=$(LC_ALL=C comm -23 "$SPEC_TMP" "$GEN_TMP")
EXTRA=$(LC_ALL=C comm -13 "$SPEC_TMP" "$GEN_TMP")

DRIFT=0

if [ -n "$MISSING" ]; then
  echo ""
  echo "MISSING from generated code (in spec but not generated):"
  echo "$MISSING" | sed 's/^/  /'
  DRIFT=1
fi

if [ -n "$EXTRA" ]; then
  echo ""
  echo "EXTRA in generated code (not in spec):"
  echo "$EXTRA" | sed 's/^/  /'
  DRIFT=1
fi

if [ "$DRIFT" -eq 1 ]; then
  echo ""
  echo "DRIFT DETECTED. Run 'make kt-generate-services' to regenerate."
  exit 1
fi

echo ""
echo "No drift detected."
exit 0
