#!/bin/bash
# check-swift-service-drift.sh
#
# Verifies that the committed generated Swift artifacts are current by
# regenerating the whole Generated/ tree into a temp directory and diffing.
# Generation needs only the Swift toolchain (not macOS), so this runs on any
# platform with `swift`. It is non-mutating: the committed tree is never
# touched.
#
# Exit codes:
#   0 = No drift detected
#   1 = Drift detected
#   2 = swift toolchain not available

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

if ! command -v swift >/dev/null 2>&1; then
  echo "ERROR: swift is required for check-swift-service-drift.sh but was not found" >&2
  exit 2
fi

GENERATED_DIR="$ROOT_DIR/swift/Sources/Basecamp/Generated"
# Explicit template so this works identically under GNU and BSD/macOS mktemp.
TMP_OUT="$(mktemp -d "${TMPDIR:-/tmp}/swift-service-drift.XXXXXX")"
trap 'rm -rf "$TMP_OUT"' EXIT

echo "==> Regenerating Swift SDK into a temp directory..."
# The generator resolves --openapi/--behavior relative to its working
# directory; run from swift/ (like `make swift-generate`) but emit to an
# absolute temp path so the committed tree is untouched.
(cd "$ROOT_DIR/swift" && \
  swift run BasecampGenerator \
    --openapi "$ROOT_DIR/openapi.json" \
    --behavior "$ROOT_DIR/behavior-model.json" \
    --output "$TMP_OUT") > /dev/null

echo "==> Diffing against committed swift/Sources/Basecamp/Generated/ ..."
if ! diff -rq "$GENERATED_DIR" "$TMP_OUT" > /dev/null; then
  echo "ERROR: Generated Swift is out of date. Run 'make swift-generate'"
  diff -rq "$GENERATED_DIR" "$TMP_OUT" || true
  exit 1
fi

echo "No drift detected."
exit 0
