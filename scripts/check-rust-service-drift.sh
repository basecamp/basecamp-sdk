#!/bin/bash
# check-rust-service-drift.sh
#
# Verifies that the committed generated Rust artifacts are current by
# regenerating the whole rust/basecamp-sdk/src/generated/ tree into a temp
# directory and diffing. Non-mutating: the committed tree is never touched
# (the Swift/Python shape — see release-python.yml for why an in-place
# `git status` check cannot see a stale module the sweep would have removed).
#
# Exit codes:
#   0 = No drift detected
#   1 = Drift detected
#   2 = cargo toolchain not available

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

if ! command -v cargo >/dev/null 2>&1; then
  echo "ERROR: cargo is required for check-rust-service-drift.sh but was not found" >&2
  exit 2
fi

GENERATED_DIR="$ROOT_DIR/rust/basecamp-sdk/src/generated"
# Explicit template so this works identically under GNU and BSD/macOS mktemp.
TMP_OUT="$(mktemp -d "${TMPDIR:-/tmp}/rust-service-drift.XXXXXX")"
trap 'rm -rf "$TMP_OUT"' EXIT

echo "==> Regenerating Rust SDK into a temp directory..."
# The generator is a workspace member of rust/; run it from there (like
# `make rs-generate-services`). It reads openapi.json, behavior-model.json and its
# names.toml relative to --root, and --output sends the tree to the temp path
# so the committed one is untouched. --locked: drift detection must not be the
# thing that rewrites the lockfile.
(cd "$ROOT_DIR/rust" && \
  cargo run -q --locked -p basecamp-sdk-generator -- \
    --root "$ROOT_DIR" \
    --output "$TMP_OUT") > /dev/null

echo "==> Diffing against committed rust/basecamp-sdk/src/generated/ ..."
if ! diff -rq "$GENERATED_DIR" "$TMP_OUT" > /dev/null; then
  echo "ERROR: Generated Rust is out of date. Run 'make rs-generate-services'"
  diff -rq "$GENERATED_DIR" "$TMP_OUT" || true
  exit 1
fi

echo "No drift detected."
exit 0
