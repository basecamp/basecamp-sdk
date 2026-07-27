#!/usr/bin/env bash
# Presence guard for generated Kotlin ARRAY and PRIMITIVE SCALAR properties:
# optional -> `T? = null`, required -> `T`, required-and-nullable -> `T?` with
# no default, and no zero-value sentinel defaults anywhere.
#
# Object/$ref/enum-typed properties are deliberately OUT of scope — see the
# Scope note in scripts/check-kotlin-optional-arrays-and-scalars.rb.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v ruby >/dev/null 2>&1; then
  echo "ERROR: ruby is required for check-kotlin-optional-arrays-and-scalars" >&2
  exit 2
fi

exec ruby "$SCRIPT_DIR/check-kotlin-optional-arrays-and-scalars.rb"
