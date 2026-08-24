#!/usr/bin/env bash
# Promotes MIGRATING.md's "# Unreleased" heading to "# v<version>".
# Called by bump-version.sh; `make release` guards the result, so a bump that
# skips this (or a hand-rolled bump) cannot tag with notes still unpromoted.
set -euo pipefail

VERSION="${1:?usage: $0 <version> [file]}"
FILE="${2:-MIGRATING.md}"

if grep -qxF "# v$VERSION" "$FILE"; then
  if grep -qxF "# Unreleased" "$FILE"; then
    echo "ERROR: $FILE carries both '# v$VERSION' and '# Unreleased' — resolve by hand." >&2
    exit 1
  fi
  echo "$FILE already promoted to v$VERSION"
  exit 0
fi

if ! grep -qxF "# Unreleased" "$FILE"; then
  echo "ERROR: $FILE has neither '# Unreleased' nor '# v$VERSION' — nothing to promote." >&2
  exit 1
fi

awk -v v="# v$VERSION" 'BEGIN { done = 0 }
  !done && $0 == "# Unreleased" { print v; done = 1; next }
  { print }' "$FILE" > "$FILE.tmp" && mv "$FILE.tmp" "$FILE"
echo "Promoted '# Unreleased' -> '# v$VERSION' in $FILE"
