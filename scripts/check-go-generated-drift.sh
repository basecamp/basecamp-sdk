#!/bin/bash
# check-go-generated-drift.sh
#
# Verifies that the committed generated Go client is current by regenerating
# and diffing against the committed file:
#   go/pkg/generated/client.gen.go  (oapi-codegen output, AFTER the
#                                    normalize-go-deprecation-godoc.sh pass)
#
# This is a freshness gate: it re-runs the generation pipeline (oapi-codegen +
# normalization) and asserts the committed output matches. It is distinct from
# check-service-drift.sh, which is an operation-level coverage check of the
# service wrappers.
#
# Non-mutating: oapi-codegen honors the `output:` path in its config and ignores
# -o, so generation is redirected by writing a throwaway config whose `output:`
# points at a temp file. The committed working tree is never touched, so this is
# safe to run concurrently with other `make check` targets (including under
# `make -j`). The generator embeds no timestamp; the comparison is verbatim.
#
# Exit codes:
#   0 = No drift detected
#   1 = Drift detected

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

GO_DIR="$ROOT_DIR/go"
GENERATED_FILE="$GO_DIR/pkg/generated/client.gen.go"

TMPDIR_BASE=$(mktemp -d)
trap 'rm -rf "$TMPDIR_BASE"' EXIT

REGEN="$TMPDIR_BASE/client.gen.go"
TMP_CONFIG="$TMPDIR_BASE/oapi-codegen.yaml"
# Separate log per phase so a later failure never clobbers earlier diagnostics.
GEN_LOG="$TMPDIR_BASE/oapi-codegen.log"
NORM_LOG="$TMPDIR_BASE/normalize.log"

# Copy the committed config, overriding only the `output:` line to an absolute
# temp path. `output-options:` (a different key) is left untouched; the
# user-template paths stay relative and resolve against the go/ working dir.
sed "s#^output: .*#output: $REGEN#" "$GO_DIR/oapi-codegen.yaml" > "$TMP_CONFIG"

echo "==> Regenerating Go client into a temp directory (non-mutating)..."
# Run from go/ so the relative user-template paths in the config resolve.
if ! (cd "$GO_DIR" && go tool oapi-codegen -config "$TMP_CONFIG" ../openapi.json) > "$GEN_LOG" 2>&1; then
  echo "ERROR: oapi-codegen failed:" >&2
  cat "$GEN_LOG" >&2
  exit 1
fi

# Apply the same normalization the committed file receives (see generate.go).
if ! "$SCRIPT_DIR/normalize-go-deprecation-godoc.sh" "$REGEN" > "$NORM_LOG" 2>&1; then
  echo "ERROR: normalization failed:" >&2
  cat "$NORM_LOG" >&2
  exit 1
fi

echo ""
if ! diff -q "$GENERATED_FILE" "$REGEN" > /dev/null; then
  echo "ERROR: Generated Go client is out of date."
  echo "Run 'make -C go generate' and commit."
  diff "$GENERATED_FILE" "$REGEN" | head -40 || true
  exit 1
fi

echo "No drift detected."
exit 0
