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

# Heading judgments read the guide's PROSE only: the guide legitimately
# quotes old headings inside fenced code blocks (its own derivation recipes
# do), and a fence containing "# Unreleased" is an example, not a section.
guide_prose() {
  awk '/^```/ { fenced = !fenced; next } !fenced' "$FILE"
}

FIRST=$(guide_prose | grep -m1 -E '^# (Unreleased|v[0-9]+\.[0-9]+\.[0-9]+)$' || true)

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
# Fail closed on an absent OR malformed authority: an empty CURRENT would
# make current_blocks vacuously permissive, and a non-numeric one (say
# "dev") errors every arithmetic test in `newer`, which an `if` reads as
# false — the rollback authority bypassed either way.
if ! printf '%s' "$CURRENT" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "ERROR: current SDK version '$CURRENT' (from go/pkg/basecamp/version.go) is not X.Y.Z — refusing to run without a usable rollback authority." >&2
  exit 1
fi
if ! printf '%s' "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "ERROR: target version '$VERSION' is not X.Y.Z." >&2
  exit 1
fi

if [ -z "$FIRST" ]; then
  echo "ERROR: $FILE has no release heading ('# Unreleased' or '# vX.Y.Z')." >&2
  exit 1
fi

# current_blocks TARGET: refuse a target strictly older than the SDK's own
# current version constant.
current_blocks() {
  newer "$CURRENT" "$1"
}

# released W: true when the release tag vW exists ON THE REMOTE — the only
# authority on whether a version actually shipped. Local tags are consulted
# for nothing: their absence proves nothing (shallow and filtered clones
# carry partial inventories), and their presence proves nothing either — a
# release whose tag push failed leaves the local tag behind with nothing
# published. When the remote cannot answer — offline, no origin — the
# question fails closed rather than guessing at history.
# PROMOTE_MIGRATING_RELEASED (a space-separated version list — the complete
# authority; may be set empty to model "cannot establish") overrides for the
# self-test's scratch-file cases; its git-fixture cases run this path for
# real against a file:// remote.
released() {
  if [ -n "${PROMOTE_MIGRATING_RELEASED+x}" ]; then
    local r
    for r in $PROMOTE_MIGRATING_RELEASED; do
      [ "$r" = "$1" ] && return 0
    done
    if [ -z "$PROMOTE_MIGRATING_RELEASED" ]; then
      echo "ERROR: whether '# v$1' ever shipped cannot be established. Check the remote connection and retry." >&2
      exit 1
    fi
    return 1
  fi
  local remote
  if ! remote=$(git -C "$(dirname "$0")/.." ls-remote --tags origin "refs/tags/v$1" 2>/dev/null); then
    echo "ERROR: the remote cannot be reached to establish whether v$1 ever shipped. Check the connection and retry." >&2
    exit 1
  fi
  [ -n "$remote" ]
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

# A release target that is already tagged cannot be released again; catching
# it here keeps `make release` from pushing main before dying on the tag.
if [ "$MODE" = check ] && released "$VERSION"; then
  echo "ERROR: v$VERSION is already tagged — it cannot be released again." >&2
  exit 1
fi

case "$FIRST" in
  "# Unreleased")
    if [ "$(guide_prose | grep -cxF "# Unreleased")" -gt 1 ]; then
      echo "ERROR: $FILE has more than one '# Unreleased' heading — promotion would rename only the first and leave the rest to fail the release late." >&2
      exit 1
    fi
    if guide_prose | grep -qxF "# v$VERSION"; then
      echo "ERROR: $FILE already has a '# v$VERSION' section below '# Unreleased' — releasing $VERSION again would be a version rollback." >&2
      exit 1
    fi
    # The target must also be newer than the newest RELEASED section, or the
    # promotion itself would file today's notes behind history (bump 0.9.0
    # with 0.15.0 released would otherwise happily mint "# v0.9.0").
    NEWEST_RELEASED=$(guide_prose | grep -m1 -E '^# v[0-9]+\.[0-9]+\.[0-9]+$' || true)
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
    # New notes belong to a release NEWER than what already shipped: with the
    # SDK at 0.16.0 (released without notes), promoting fresh notes to
    # "# v0.16.0" would file them under a tag that already exists, and the
    # release would push main before failing on that tag. Equality is legal
    # only for the no-mutation paths.
    if ! newer "$VERSION" "$CURRENT"; then
      echo "ERROR: $VERSION is not newer than the SDK's current version ($CURRENT) — these notes belong to a later release." >&2
      exit 1
    fi
    awk -v v="# v$VERSION" 'BEGIN { done = 0 }
      !done && $0 == "# Unreleased" { print v; done = 1; next }
      { print }' "$FILE" > "$FILE.tmp" && mv "$FILE.tmp" "$FILE"
    echo "Promoted '# Unreleased' -> '# v$VERSION' in $FILE"
    ;;
  "# v$VERSION")
    if guide_prose | grep -qxF "# Unreleased"; then
      echo "ERROR: $FILE has a misplaced '# Unreleased' section below '# v$VERSION' — its notes would silently miss the release." >&2
      exit 1
    fi
    echo "$FILE: '# v$VERSION' is the newest section — promoted."
    ;;
  *)
    TOP="${FIRST#\# v}"
    if guide_prose | grep -qxF "# Unreleased"; then
      echo "ERROR: $FILE has a misplaced '# Unreleased' section below '$FIRST' — its notes would silently miss the release." >&2
      exit 1
    fi
    if newer "$VERSION" "$TOP"; then
      if ! released "$TOP"; then
        # "# v$TOP" was promoted but never released — a bump corrected to a
        # higher version before committing. Its notes belong to THIS release,
        # not to a version no tag will ever name.
        if [ "$MODE" = check ]; then
          echo "ERROR: $FILE's newest section '# v$TOP' is promoted but unreleased — run 'make bump VERSION=$VERSION' to carry its notes forward." >&2
          exit 1
        fi
        awk -v old="# v$TOP" -v v="# v$VERSION" 'BEGIN { done = 0 }
          !done && $0 == old { print v; done = 1; next }
          { print }' "$FILE" > "$FILE.tmp" && mv "$FILE.tmp" "$FILE"
        echo "Re-promoted pending '# v$TOP' -> '# v$VERSION' in $FILE (v$TOP was never released)."
        exit 0
      fi
      # Legitimate: a release with nothing migration-worthy has no section,
      # per the guide's one-section-per-release-that-breaks-something rule.
      echo "$FILE: no '# Unreleased' section — v$VERSION ships without migration notes (newest documented: v$TOP)."
    else
      echo "ERROR: $FILE's newest section is '# v$TOP'; $VERSION would release backward past it." >&2
      exit 1
    fi
    ;;
esac
