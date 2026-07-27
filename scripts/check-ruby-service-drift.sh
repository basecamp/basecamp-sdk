#!/bin/bash
# check-ruby-service-drift.sh
#
# Verifies that ALL generated Ruby artifacts are current by regenerating
# to a temp directory and diffing against the committed files:
#   1. metadata.json (from openapi.json + behavior-model.json)
#   2. types.rb       (from openapi.json)
#   3. Generated service files (from openapi.json)
#
# Non-mutative: regeneration lands in a temp dir; the working tree is never
# touched. Detects BOTH missing (in spec, not committed) and extra (committed,
# not in spec) drift.
#
# NOTE on timestamps: generate-metadata.rb and generate-types.rb embed a
# wall-clock `generated` stamp (Time.now.utc.iso8601). That single line changes
# on every run and is NOT drift, so both sides are canonicalized through
# normalize_stamp() before the metadata/types diff. Every other line is
# compared verbatim. (The service generator embeds no timestamp.)
#
# Exit codes:
#   0 = No drift detected
#   1 = Drift detected

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

GENERATED_DIR="$ROOT_DIR/ruby/lib/basecamp/generated"
TMPDIR_BASE=$(mktemp -d)
trap 'rm -rf "$TMPDIR_BASE"' EXIT

DRIFT=0

# Canonicalize the embedded generation timestamp so it never registers as drift.
# Targets exactly the two known stamp lines and nothing else.
normalize_stamp() {
  sed -E \
    -e 's/("generated": )"[^"]*"/\1"<NORMALIZED>"/' \
    -e 's/(# Generated: ).*/\1<NORMALIZED>/' \
    "$1"
}

# ---------------------------------------------------------------------------
# 1. Check metadata.json freshness
# ---------------------------------------------------------------------------
echo "==> Checking metadata.json freshness..."

META_TMP="$TMPDIR_BASE/metadata.json"
(cd "$ROOT_DIR/ruby" && ruby scripts/generate-metadata.rb) > "$META_TMP"

if [ ! -f "$GENERATED_DIR/metadata.json" ]; then
  echo "ERROR: committed metadata.json is missing. Run 'make rb-generate'"
  DRIFT=1
elif ! diff -q <(normalize_stamp "$GENERATED_DIR/metadata.json") <(normalize_stamp "$META_TMP") > /dev/null; then
  echo "ERROR: metadata.json is out of date. Run 'make rb-generate'"
  diff <(normalize_stamp "$GENERATED_DIR/metadata.json") <(normalize_stamp "$META_TMP") || true
  DRIFT=1
else
  echo "metadata.json is up to date"
fi

# ---------------------------------------------------------------------------
# 2. Check types.rb freshness
# ---------------------------------------------------------------------------
echo ""
echo "==> Checking types.rb freshness..."

TYPES_TMP="$TMPDIR_BASE/types.rb"
(cd "$ROOT_DIR/ruby" && ruby scripts/generate-types.rb) > "$TYPES_TMP"

if [ ! -f "$GENERATED_DIR/types.rb" ]; then
  echo "ERROR: committed types.rb is missing. Run 'make rb-generate'"
  DRIFT=1
elif ! diff -q <(normalize_stamp "$GENERATED_DIR/types.rb") <(normalize_stamp "$TYPES_TMP") > /dev/null; then
  echo "ERROR: types.rb is out of date. Run 'make rb-generate'"
  diff <(normalize_stamp "$GENERATED_DIR/types.rb") <(normalize_stamp "$TYPES_TMP") || true
  DRIFT=1
else
  echo "types.rb is up to date"
fi

# ---------------------------------------------------------------------------
# 3. Check generated services freshness
# ---------------------------------------------------------------------------
echo ""
echo "==> Checking generated services freshness..."

SERVICES_TMP="$TMPDIR_BASE/services"
mkdir -p "$SERVICES_TMP"
(cd "$ROOT_DIR/ruby" && ruby scripts/generate-services.rb --output "$SERVICES_TMP") > /dev/null

SERVICES_COMMITTED="$TMPDIR_BASE/services_committed"
mkdir -p "$SERVICES_COMMITTED"
if [ ! -d "$GENERATED_DIR/services" ]; then
  echo "ERROR: committed services/ directory is missing. Run 'make rb-generate-services'"
  DRIFT=1
else
  # Copy the WHOLE committed services/ tree (not just *.rb) so any stray extra
  # artifact — a non-.rb file or a nested directory the generator never emits —
  # surfaces as drift against the generated set. The generator writes only
  # *_service.rb into an empty temp dir, so anything else here is extra.
  cp -R "$GENERATED_DIR/services/." "$SERVICES_COMMITTED/"
  # Exclude the hand-written base class (lives alongside generated services but
  # is not produced by the generator; it carries no @generated marker).
  rm -f "$SERVICES_COMMITTED/base_service.rb"

  if ! diff -rq "$SERVICES_COMMITTED" "$SERVICES_TMP" > /dev/null; then
    echo "ERROR: Generated services are out of date. Run 'make rb-generate-services'"
    diff -rq "$SERVICES_COMMITTED" "$SERVICES_TMP" || true
    DRIFT=1
  else
    echo "Generated services are up to date"
  fi
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
if [ "$DRIFT" -eq 1 ]; then
  echo "DRIFT DETECTED. Run 'make rb-generate rb-generate-services' to regenerate."
  exit 1
fi

echo "No drift detected."
exit 0
