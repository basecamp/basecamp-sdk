#!/bin/bash
# check-swift-service-drift.sh
#
# Verifies that ALL generated Swift artifacts are current by regenerating to a
# temp directory and diffing against the committed Generated/ tree.
#
# Everything under swift/Sources/Basecamp/Generated/ is emitted by
# BasecampGenerator (Models/ and Services/ are wiped+rewritten; the root files
# AccountClient+Services.swift, Metadata.swift, and QueryString.swift are each
# written directly). None of it is hand-written, and none of it embeds a
# timestamp, so a plain regen-and-diff is exact.
#
# Exit codes:
#   0 = No drift detected
#   1 = Drift detected

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "==> Checking generated Swift code freshness..."

# Regenerate into the temp dir directly. We invoke the generator ourselves
# rather than `make swift-generate`, which hardcodes the committed output path.
(cd "$ROOT_DIR/swift" && swift run BasecampGenerator \
  --openapi ../openapi.json \
  --behavior ../behavior-model.json \
  --output "$TMP") > /dev/null

# Guard the diff so `set -e` can't swallow the diagnostic: a bare `diff`
# returning non-zero would abort before the echo.
if ! diff -rq "$ROOT_DIR/swift/Sources/Basecamp/Generated/" "$TMP/" > /dev/null; then
  echo "ERROR: Generated Swift code is out of date. Run 'make swift-generate'"
  diff -rq "$ROOT_DIR/swift/Sources/Basecamp/Generated/" "$TMP/" || true
  exit 1
fi

echo "Generated Swift code is up to date"
