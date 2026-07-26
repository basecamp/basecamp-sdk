#!/usr/bin/env bash
# Fixture-completeness guard: validate spec/fixtures/manifest.yaml — every
# manifest target validates against its schema and every covered schema keeps a
# concrete active representative. See scripts/check-fixture-coverage.rb.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v ruby >/dev/null 2>&1; then
  echo "ERROR: ruby is required for check-fixture-coverage" >&2
  exit 2
fi

exec ruby "$SCRIPT_DIR/check-fixture-coverage.rb"
