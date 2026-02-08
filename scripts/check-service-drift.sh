#!/bin/bash
# check-service-drift.sh
#
# Compares generated client operations against service layer usage.
# Detects drift between the OpenAPI spec and the service layer wrapper.
#
# Run after: make generate
# Exit codes:
#   0 = No drift detected
#   1 = Drift detected (new generated ops not wrapped, or calls to missing ops)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK_DIR="$(dirname "$SCRIPT_DIR")/go"

GENERATED_FILE="$SDK_DIR/pkg/generated/client.gen.go"
SERVICE_DIR="$SDK_DIR/pkg/basecamp"

# Temporary files
GEN_OPS=$(mktemp)
SVC_OPS=$(mktemp)
TEST_OPS=$(mktemp)
trap "rm -f $GEN_OPS $SVC_OPS $TEST_OPS" EXIT

# Extract generated operations, normalizing WithBodyWithResponse to base operation name
# e.g., CreateAttachmentWithBodyWithResponse -> CreateAttachment
#       ListProjectsWithResponse -> ListProjects
grep "^func (c \*ClientWithResponses)" "$GENERATED_FILE" 2>/dev/null \
  | sed 's/.*) \([A-Za-z]*\)WithResponse.*/\1/' \
  | sed 's/WithBody$//' \
  | sort -u > "$GEN_OPS"

# Extract service layer calls to gen.*WithResponse (excluding test files)
# Normalize WithBodyWithResponse calls to base operation name
for f in "$SERVICE_DIR"/*.go; do
  case "$f" in
    *_test.go) continue ;;
  esac
  grep "\.gen\.[A-Za-z]*WithResponse" "$f" 2>/dev/null || true
done | sed 's/.*\.gen\.\([A-Za-z]*\)WithResponse.*/\1/' \
  | sed 's/WithBody$//' \
  | sort -u > "$SVC_OPS"

# Extract test file calls to gen.*WithResponse (test coverage check)
for f in "$SERVICE_DIR"/*_test.go; do
  [ -f "$f" ] || continue
  grep "\.gen\.[A-Za-z]*WithResponse" "$f" 2>/dev/null || true
done | sed 's/.*\.gen\.\([A-Za-z]*\)WithResponse.*/\1/' \
  | sed 's/WithBody$//' \
  | sort -u > "$TEST_OPS"

# Count operations
GEN_COUNT=$(wc -l < "$GEN_OPS" | tr -d ' ')
SVC_COUNT=$(wc -l < "$SVC_OPS" | tr -d ' ')

echo "Generated client operations: $GEN_COUNT"
echo "Service layer wrapped operations: $SVC_COUNT"
echo ""

# Find operations in generated but not wrapped by services
UNWRAPPED=$(comm -23 "$GEN_OPS" "$SVC_OPS")
UNWRAPPED_COUNT=$(echo "$UNWRAPPED" | grep -c . || true)

# Find service calls to non-existent operations
MISSING=$(comm -13 "$GEN_OPS" "$SVC_OPS")
MISSING_COUNT=$(echo "$MISSING" | grep -c . || true)

# Find wrapped operations without test coverage
UNTESTED=$(comm -23 "$SVC_OPS" "$TEST_OPS")
UNTESTED_COUNT=$(echo "$UNTESTED" | grep -c . || true)

HAS_DRIFT=0

if [ "$UNWRAPPED_COUNT" -gt 0 ]; then
  echo "=== Generated operations NOT YET wrapped by service layer ($UNWRAPPED_COUNT) ==="
  echo "$UNWRAPPED"
  echo ""
  # Note: This is informational, not a failure - new ops may be intentionally unwrapped
fi

if [ "$MISSING_COUNT" -gt 0 ]; then
  echo "=== ERROR: Service calls to NON-EXISTENT generated operations ($MISSING_COUNT) ==="
  echo "$MISSING"
  echo ""
  echo "These service methods call generated operations that don't exist."
  echo "Either the spec is missing these operations, or there's a typo in the service layer."
  HAS_DRIFT=1
fi

if [ "$UNTESTED_COUNT" -gt 0 ]; then
  echo "=== WARNING: Wrapped operations WITHOUT test coverage ($UNTESTED_COUNT) ==="
  echo "$UNTESTED"
  echo ""
  echo "These operations have service wrappers but no tests calling the generated client."
  echo "Add tests to verify the service layer works end-to-end."
fi

# Summary
if [ "$GEN_COUNT" -eq 0 ]; then
  echo "ERROR: No generated operations found. Check GENERATED_FILE path or parsing."
  exit 1
fi
COVERAGE_PCT=$((SVC_COUNT * 100 / GEN_COUNT))
echo "Coverage: $SVC_COUNT / $GEN_COUNT operations ($COVERAGE_PCT%)"

if [ "$HAS_DRIFT" -eq 1 ]; then
  echo ""
  echo "DRIFT DETECTED - Fix the issues above before proceeding."
  exit 1
fi

echo ""
echo "No critical drift detected."
exit 0
