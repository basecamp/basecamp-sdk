#!/bin/bash
# check-python-service-drift.sh
#
# Verifies that ALL generated Python artifacts are current by regenerating
# to a temp directory and diffing against committed files:
#   1. Generated service files (from openapi.json)
#   2. metadata.json (from openapi.json + behavior-model.json)
#   3. types.py (from openapi.json)
#
# Exit codes:
#   0 = No drift detected
#   1 = Drift detected

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

GENERATED_DIR="$ROOT_DIR/python/src/basecamp/generated"
TMPDIR_BASE=$(mktemp -d)
trap 'rm -rf "$TMPDIR_BASE"' EXIT

DRIFT=0

# ---------------------------------------------------------------------------
# 1. Check generated services freshness
# ---------------------------------------------------------------------------
echo "==> Checking generated services freshness..."

SERVICES_TMP="$TMPDIR_BASE/services"
mkdir -p "$SERVICES_TMP"

(cd "$ROOT_DIR/python" && uv run python scripts/generate_services.py --output "$SERVICES_TMP") > /dev/null
(cd "$ROOT_DIR/python" && uv run ruff format "$SERVICES_TMP/") > /dev/null 2>&1

SERVICES_COMMITTED="$TMPDIR_BASE/services_committed"
cp -R "$GENERATED_DIR/services/" "$SERVICES_COMMITTED/"
rm -f "$SERVICES_COMMITTED/_base.py" "$SERVICES_COMMITTED/_async_base.py"
rm -rf "$SERVICES_COMMITTED/__pycache__"
if ! diff -rq "$SERVICES_COMMITTED/" "$SERVICES_TMP/" > /dev/null; then
  echo "ERROR: Generated services are out of date. Run 'make py-generate-services'"
  diff -rq "$SERVICES_COMMITTED/" "$SERVICES_TMP/" || true
  DRIFT=1
else
  echo "Generated services are up to date"
fi

# ---------------------------------------------------------------------------
# 2. Check metadata.json freshness
# ---------------------------------------------------------------------------
echo ""
echo "==> Checking metadata.json freshness..."

META_TMP="$TMPDIR_BASE/metadata.json"
(cd "$ROOT_DIR/python" && uv run python scripts/generate_metadata.py --output "$META_TMP") > /dev/null

if ! diff -q "$GENERATED_DIR/metadata.json" "$META_TMP" > /dev/null; then
  echo "ERROR: metadata.json is out of date. Run 'make py-generate'"
  diff "$GENERATED_DIR/metadata.json" "$META_TMP" || true
  DRIFT=1
else
  echo "metadata.json is up to date"
fi

# ---------------------------------------------------------------------------
# 3. Check types.py freshness
# ---------------------------------------------------------------------------
echo ""
echo "==> Checking types.py freshness..."

TYPES_TMP="$TMPDIR_BASE/types.py"
(cd "$ROOT_DIR/python" && uv run python scripts/generate_types.py --output "$TYPES_TMP") > /dev/null
(cd "$ROOT_DIR/python" && uv run ruff format "$TYPES_TMP") > /dev/null 2>&1

if ! diff -q "$GENERATED_DIR/types.py" "$TYPES_TMP" > /dev/null; then
  echo "ERROR: types.py is out of date. Run 'make py-generate'"
  diff "$GENERATED_DIR/types.py" "$TYPES_TMP" || true
  DRIFT=1
else
  echo "types.py is up to date"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
if [ "$DRIFT" -eq 1 ]; then
  echo "DRIFT DETECTED. Run 'make py-generate py-generate-services' to regenerate."
  exit 1
fi

echo "No drift detected."
exit 0
