#!/usr/bin/env bash
# Verify prose doc constants (API_VERSION, the bc3 provenance pin, SPEC §19's
# assertion-type table) still match their machine-readable sources.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v ruby >/dev/null 2>&1; then
  echo "ERROR: ruby is required for doc-constants-check" >&2
  exit 2
fi

exec ruby "$SCRIPT_DIR/sync-doc-constants.rb" --check
