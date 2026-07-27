#!/bin/bash
# check-typescript-service-drift.sh
#
# Verifies that ALL generated TypeScript artifacts under src/generated/ are
# current by regenerating and diffing against the committed tree:
#   1. openapi-stripped.json (stripped OpenAPI)
#   2. schema.d.ts           (openapi-typescript, patched)
#   3. metadata.ts           (x-basecamp-* extension metadata)
#   4. path-mapping.ts       (operation -> path map)
#   5. Generated service files (+ services/index.ts)
#
# Non-mutating: the `npm run generate` pipeline writes to paths resolved from the
# generator scripts' own location (import.meta __dirname), so it cannot be
# redirected by changing the working directory. Instead this script assembles a
# throwaway project OUTSIDE the repo — the real generator scripts, a symlinked
# node_modules, and a copy of openapi.json placed where the pipeline's relative
# `../openapi.json` resolves — and runs the committed `npm run generate` +
# generate-services verbatim there. The committed working tree is never touched,
# so this is safe to run concurrently with other `make check` targets (including
# under `make -j`). Regenerating into an empty tree also means the diff detects
# BOTH missing and extra files (stale artifacts survive only in the committed
# copy).
#
# NOTE on timestamps: extract-metadata.ts embeds a wall-clock `generated` stamp
# in metadata.ts. That single line changes on every run and is NOT drift, so
# both sides are canonicalized through normalize_stamp() before the diff. Every
# other line is compared verbatim. (No other artifact embeds a timestamp.)
#
# Exit codes:
#   0 = No drift detected
#   1 = Drift detected

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

TS_DIR="$ROOT_DIR/typescript"
GENERATED_DIR="$TS_DIR/src/generated"

if [ ! -d "$TS_DIR/node_modules" ]; then
  echo "ERROR: $TS_DIR/node_modules is missing. Run 'make ts-install' first." >&2
  exit 1
fi

TMPBASE=$(mktemp -d)
trap 'rm -rf "$TMPBASE"' EXIT

# openapi.json copied to the temp parent so the pipeline's relative
# `../openapi.json` (from the project dir) resolves to it.
cp "$ROOT_DIR/openapi.json" "$TMPBASE/openapi.json"

PROJ="$TMPBASE/proj"
mkdir -p "$PROJ/src/generated"
cp -R "$TS_DIR/scripts" "$PROJ/scripts"
cp "$TS_DIR/package.json" "$PROJ/package.json"
cp "$TS_DIR"/tsconfig*.json "$PROJ/" 2>/dev/null || true
ln -s "$TS_DIR/node_modules" "$PROJ/node_modules"

GEN_LOG="$TMPBASE/generate.log"

echo "==> Regenerating TypeScript artifacts into a temp project (non-mutating)..."
# Verbatim `npm run generate` (robust to pipeline changes) + generate-services.
if ! (cd "$PROJ" && npm run generate && npx tsx scripts/generate-services.ts) > "$GEN_LOG" 2>&1; then
  echo "ERROR: TypeScript generation failed:" >&2
  cat "$GEN_LOG" >&2
  exit 1
fi

REGEN="$PROJ/src/generated"

# Snapshot the committed tree (never mutate it) so the timestamp line can be
# canonicalized on both sides before diffing.
CMT="$TMPBASE/committed"
mkdir -p "$CMT"
cp -R "$GENERATED_DIR/." "$CMT/"

normalize_stamp() {
  if [ -f "$1/metadata.ts" ]; then
    sed -E -i.bak 's/("generated": )"[^"]*"/\1"<NORMALIZED>"/' "$1/metadata.ts"
    rm -f "$1/metadata.ts.bak"
  fi
}
normalize_stamp "$CMT"
normalize_stamp "$REGEN"

echo ""
if ! diff -rq "$CMT" "$REGEN" > /dev/null; then
  echo "ERROR: Generated TypeScript code is out of date."
  echo "Run 'make ts-generate ts-generate-services' and commit."
  diff -rq "$CMT" "$REGEN" || true
  exit 1
fi

echo "No drift detected."
exit 0
