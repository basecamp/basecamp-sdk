#!/usr/bin/env bash
# Promotes MIGRATING.md's "# Unreleased" heading to "# v<version>", or verifies
# the promoted state (--check, used by `make release`).
#
# The current release's heading is always the FIRST release heading in the
# file — the guide is newest-first, and its top stays prose (code blocks that
# quote old headings all sit below the newest section). Matching "# vX"
# anywhere in the file is the v0.15.0 regression this replaces: a historical
# section satisfied the "already promoted" check, so a bump rolled backward
# to an old version reported success and `make release` would have pushed the
# rollback before failing on the existing tag.
set -euo pipefail

MODE=promote
if [ "${1:-}" = "--check" ]; then MODE=check; shift; fi
VERSION="${1:?usage: $0 [--check] <version> [file]}"
FILE="${2:-MIGRATING.md}"

FIRST=$(grep -m1 -E '^# (Unreleased|v[0-9]+\.[0-9]+\.[0-9]+)$' "$FILE" || true)

if [ -z "$FIRST" ]; then
  echo "ERROR: $FILE has no release heading ('# Unreleased' or '# vX.Y.Z')." >&2
  exit 1
fi

# newer A B: true when A sorts strictly after B by version order
newer() {
  [ "$1" != "$2" ] && [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -1)" = "$1" ]
}

case "$FIRST" in
  "# Unreleased")
    if grep -qxF "# v$VERSION" "$FILE"; then
      echo "ERROR: $FILE already has a '# v$VERSION' section below '# Unreleased' — releasing $VERSION again would be a version rollback." >&2
      exit 1
    fi
    if [ "$MODE" = check ]; then
      echo "ERROR: $FILE still has an '# Unreleased' section — its notes would miss the release. Run 'make bump VERSION=$VERSION' first." >&2
      exit 1
    fi
    awk -v v="# v$VERSION" 'BEGIN { done = 0 }
      !done && $0 == "# Unreleased" { print v; done = 1; next }
      { print }' "$FILE" > "$FILE.tmp" && mv "$FILE.tmp" "$FILE"
    echo "Promoted '# Unreleased' -> '# v$VERSION' in $FILE"
    ;;
  "# v$VERSION")
    echo "$FILE: '# v$VERSION' is the newest section — promoted."
    ;;
  *)
    TOP="${FIRST#\# v}"
    if newer "$VERSION" "$TOP"; then
      # Legitimate: a release with nothing migration-worthy has no section,
      # per the guide's one-section-per-release-that-breaks-something rule.
      echo "$FILE: no '# Unreleased' section — v$VERSION ships without migration notes (newest documented: v$TOP)."
    else
      echo "ERROR: $FILE's newest section is '# v$TOP'; $VERSION would release backward past it." >&2
      exit 1
    fi
    ;;
esac
