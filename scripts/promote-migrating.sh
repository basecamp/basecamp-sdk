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
# Captured ONCE and searched with herestrings: piping the prose pass into
# `grep -q` under pipefail turns a successful early match into a failure —
# grep closes the pipe, awk dies on SIGPIPE writing the rest of a large
# guide, and the pipeline reports the producer's 141.
PROSE=$(awk '/^```/ { fenced = !fenced; next } !fenced' "$FILE")

FIRST=$(grep -m1 -E '^# (Unreleased|v[0-9]+\.[0-9]+\.[0-9]+)$' <<< "$PROSE" || true)

if ! printf '%s' "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "ERROR: target version '$VERSION' is not X.Y.Z." >&2
  exit 1
fi

# The ordering authority is the REMOTE TAG LIST, fetched once — neither the
# guide's headings (a no-notes release advances without a heading) nor the
# version constants (an unshipped bump moves them, and a hand edit can move
# them backward) can answer what actually shipped. The override serves the
# self-test; empty models "cannot establish", which fails closed.
if [ -n "${PROMOTE_MIGRATING_RELEASED+x}" ]; then
  SHIPPED="$PROMOTE_MIGRATING_RELEASED"
  if [ -z "$SHIPPED" ]; then
    echo "ERROR: the shipped releases cannot be established. Check the remote connection and retry." >&2
    exit 1
  fi
else
  if ! TAGLIST=$(git -C "$(dirname "$0")/.." ls-remote --tags origin 'refs/tags/v[0-9]*' 2>/dev/null); then
    echo "ERROR: the remote cannot be reached to establish what has shipped. Check the connection and retry." >&2
    exit 1
  fi
  SHIPPED=$(sed -n 's|.*refs/tags/v\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)$|\1|p' <<< "$TAGLIST" | sort -u | tr '\n' ' ')
  if [ -z "${SHIPPED// /}" ]; then
    echo "ERROR: the remote lists no release tags — refusing to judge ordering with no shipped history." >&2
    exit 1
  fi
fi

if [ -z "$FIRST" ]; then
  echo "ERROR: $FILE has no release heading ('# Unreleased' or '# vX.Y.Z')." >&2
  exit 1
fi

# released W: true when vW is in the shipped list. Local tags are consulted
# for nothing: their absence proves nothing (partial clones), their presence
# proves nothing either (a failed tag push leaves one behind with nothing
# published).
released() {
  local r
  for r in $SHIPPED; do
    [ "$r" = "$1" ] && return 0
  done
  return 1
}

newest_shipped() {
  local max="" r
  for r in $SHIPPED; do
    if [ -z "$max" ] || newer "$r" "$max"; then
      max="$r"
    fi
  done
  printf '%s' "$max"
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

# The one ordering gate: the target must be strictly newer than everything
# that shipped. This subsumes the rollback, re-release, and hand-edited
# constant cases in one judgment against the one authority — and it is
# deliberately indifferent to UNSHIPPED state, so a mistaken 0.17.0 bump can
# be corrected downward to a still-forward 0.16.0 before anything ships.
MAX_SHIPPED=$(newest_shipped)
if ! newer "$VERSION" "$MAX_SHIPPED"; then
  echo "ERROR: $VERSION is not newer than the newest shipped release (v$MAX_SHIPPED) — refusing." >&2
  exit 1
fi

case "$FIRST" in
  "# Unreleased")
    if [ "$(grep -cxF "# Unreleased" <<< "$PROSE" || true)" -gt 1 ]; then
      echo "ERROR: $FILE has more than one '# Unreleased' heading — promotion would rename only the first and leave the rest to fail the release late." >&2
      exit 1
    fi
    if grep -qxF "# v$VERSION" <<< "$PROSE"; then
      echo "ERROR: $FILE already has a '# v$VERSION' section below '# Unreleased' — releasing $VERSION again would be a version rollback." >&2
      exit 1
    fi
    # Any UNSHIPPED version heading below is an abandoned promotion whose
    # notes would be orphaned forever the moment a fresh Unreleased promotes
    # past it — resolve it (fold or re-title) before promoting.
    while IFS= read -r h; do
      hv="${h#\# v}"
      if ! released "$hv"; then
        echo "ERROR: $FILE carries an unshipped section '# v$hv' below '# Unreleased' — an abandoned promotion; fold or re-title it before promoting new notes past it." >&2
        exit 1
      fi
    done < <(grep -E '^# v[0-9]+\.[0-9]+\.[0-9]+$' <<< "$PROSE" || true)
    if [ "$MODE" = check ]; then
      echo "ERROR: $FILE still has an '# Unreleased' section — its notes would miss the release. Run 'make bump VERSION=$VERSION' first." >&2
      exit 1
    fi
    awk -v v="# v$VERSION" 'BEGIN { done = 0 }
      /^```/ { fenced = !fenced }
      !fenced && !done && $0 == "# Unreleased" { print v; done = 1; next }
      { print }' "$FILE" > "$FILE.tmp" && mv "$FILE.tmp" "$FILE"
    echo "Promoted '# Unreleased' -> '# v$VERSION' in $FILE"
    ;;
  "# v$VERSION")
    if grep -qxF "# Unreleased" <<< "$PROSE"; then
      echo "ERROR: $FILE has a misplaced '# Unreleased' section below '# v$VERSION' — its notes would silently miss the release." >&2
      exit 1
    fi
    echo "$FILE: '# v$VERSION' is the newest section — promoted."
    ;;
  *)
    TOP="${FIRST#\# v}"
    if grep -qxF "# Unreleased" <<< "$PROSE"; then
      echo "ERROR: $FILE has a misplaced '# Unreleased' section below '$FIRST' — its notes would silently miss the release." >&2
      exit 1
    fi
    if released "$TOP"; then
      # Legitimate: a release with nothing migration-worthy has no section,
      # per the guide's one-section-per-release-that-breaks-something rule.
      # The shipped gate already proved VERSION newer than every shipped
      # release, TOP included.
      echo "$FILE: no '# Unreleased' section — v$VERSION ships without migration notes (newest documented: v$TOP)."
    else
      # "# v$TOP" was promoted but never released — a bump corrected before
      # committing, in EITHER direction: the shipped gate holds for the new
      # target, so its notes carry to it whether it is above or below the
      # mistaken one.
      if [ "$MODE" = check ]; then
        echo "ERROR: $FILE's newest section '# v$TOP' is promoted but unreleased — run 'make bump VERSION=$VERSION' to carry its notes forward." >&2
        exit 1
      fi
      awk -v old="# v$TOP" -v v="# v$VERSION" 'BEGIN { done = 0 }
        /^```/ { fenced = !fenced }
        !fenced && !done && $0 == old { print v; done = 1; next }
        { print }' "$FILE" > "$FILE.tmp" && mv "$FILE.tmp" "$FILE"
      echo "Re-promoted pending '# v$TOP' -> '# v$VERSION' in $FILE (v$TOP was never released)."
      exit 0
    fi
    ;;
esac
