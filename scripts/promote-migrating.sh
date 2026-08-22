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

# The guide's headings are not the authority on the released order: a
# no-notes release advances the SDK version without adding a heading, so the
# newest heading can legitimately lag. The version constant is the authority,
# read from a bump-written source; at bump time (step 0) it still holds the
# PRE-bump version, so a target older than it is a rollback however the
# headings read. At release time the version guards have already pinned the
# constants to the target, so the comparison is trivially equal — harmless.
# PROMOTE_MIGRATING_CURRENT overrides the source for the self-test only.
if [ -n "${PROMOTE_MIGRATING_CURRENT:-}" ]; then
  CURRENT="$PROMOTE_MIGRATING_CURRENT"
else
  VERSION_GO="$(dirname "$0")/../go/pkg/basecamp/version.go"
  CURRENT=$(sed -n 's/^const Version = "\(.*\)"$/\1/p' "$VERSION_GO" 2>/dev/null || true)
fi

if [ -z "$FIRST" ]; then
  echo "ERROR: $FILE has no release heading ('# Unreleased' or '# vX.Y.Z')." >&2
  exit 1
fi

# current_blocks TARGET: refuse a target strictly older than the SDK's own
# current version constant.
current_blocks() {
  [ -n "$CURRENT" ] && newer "$CURRENT" "$1"
}

# newer A B: true when A is strictly newer than B, compared component-wise.
# Pure arithmetic on the three semver components — no reliance on sort -V,
# whose availability varies across BSD userlands (Apple's current sort has
# it; the portable spelling retires the question).
newer() {
  local a1 a2 a3 b1 b2 b3
  IFS=. read -r a1 a2 a3 <<< "$1"
  IFS=. read -r b1 b2 b3 <<< "$2"
  [ "$a1" -ne "$b1" ] && { [ "$a1" -gt "$b1" ]; return; }
  [ "$a2" -ne "$b2" ] && { [ "$a2" -gt "$b2" ]; return; }
  [ "$a3" -gt "$b3" ]
}

if current_blocks "$VERSION"; then
  echo "ERROR: $VERSION is older than the SDK's current version ($CURRENT in go/pkg/basecamp/version.go) — refusing the rollback." >&2
  exit 1
fi

case "$FIRST" in
  "# Unreleased")
    if [ "$(grep -cxF "# Unreleased" "$FILE")" -gt 1 ]; then
      echo "ERROR: $FILE has more than one '# Unreleased' heading — promotion would rename only the first and leave the rest to fail the release late." >&2
      exit 1
    fi
    if grep -qxF "# v$VERSION" "$FILE"; then
      echo "ERROR: $FILE already has a '# v$VERSION' section below '# Unreleased' — releasing $VERSION again would be a version rollback." >&2
      exit 1
    fi
    # The target must also be newer than the newest RELEASED section, or the
    # promotion itself would file today's notes behind history (bump 0.9.0
    # with 0.15.0 released would otherwise happily mint "# v0.9.0").
    NEWEST_RELEASED=$(grep -m1 -E '^# v[0-9]+\.[0-9]+\.[0-9]+$' "$FILE" || true)
    if [ -n "$NEWEST_RELEASED" ]; then
      REL="${NEWEST_RELEASED#\# v}"
      if ! newer "$VERSION" "$REL"; then
        echo "ERROR: $VERSION is not newer than the newest released section in $FILE ('# v$REL') — refusing to promote backward." >&2
        exit 1
      fi
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
    if grep -qxF "# Unreleased" "$FILE"; then
      echo "ERROR: $FILE has a misplaced '# Unreleased' section below '# v$VERSION' — its notes would silently miss the release." >&2
      exit 1
    fi
    echo "$FILE: '# v$VERSION' is the newest section — promoted."
    ;;
  *)
    TOP="${FIRST#\# v}"
    if grep -qxF "# Unreleased" "$FILE"; then
      echo "ERROR: $FILE has a misplaced '# Unreleased' section below '$FIRST' — its notes would silently miss the release." >&2
      exit 1
    fi
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
