#!/usr/bin/env bash
# D-invariant guard: every optional generated Kotlin array is `List<T>? = null`,
# every required array stays `List<T>`, and none default to `= emptyList()`.
# See scripts/check-kotlin-optional-arrays.rb.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v ruby >/dev/null 2>&1; then
  echo "ERROR: ruby is required for check-kotlin-optional-arrays" >&2
  exit 2
fi

exec ruby "$SCRIPT_DIR/check-kotlin-optional-arrays.rb"
