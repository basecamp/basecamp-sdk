#!/usr/bin/env bash
# Report drift between a baseline bc3 revision and the head of a branch.
# Usage: report-bc3-drift.sh <revision> <branch> <label>
#
# label is one of:
#   primary  — block we sync against (printed as "bc3 (<branch>) ...")
#   compat   — additional compatibility tracking block
set -euo pipefail

REV="${1:-}"
BRANCH="${2:-master}"
LABEL="${3:-primary}"
BC3_REPO="${BC3_REPO:-basecamp/bc3}"

case "$LABEL" in
  primary) HEADER="bc3 (active branch: $BRANCH)" ;;
  compat)  HEADER="bc3 compatibility (branch: $BRANCH)" ;;
  *)       HEADER="bc3 ($BRANCH)" ;;
esac

if [ -z "$REV" ]; then
  echo "==> $HEADER API docs: no baseline revision set"
  echo ""
  echo "==> $HEADER API implementation: no baseline revision set"
  exit 0
fi

SHORT_REV="${REV:0:7}"
COMPARE="repos/$BC3_REPO/compare/$REV...$BRANCH"

# One fetch for both sections below, as [{status, filename}].
FILES_JSON="$(gh api "$COMPARE" --jq '[.files[] | {status, filename}]')"

# GitHub's compare endpoint hard-caps .files at 300 per comparison and offers
# no pagination for them: ?page= paginates the COMMIT list only, so page 2 of
# a 213-commit range answers zero commits and zero files. At the cap the list
# is silently incomplete — and because it arrives sorted, the casualties are
# whatever sorts last, which is exactly doc/api/ behind hundreds of app/*
# files. This script once answered "(no changes in doc/api/)" over a 553-file
# range in which two doc/api sections had changed. Past the cap, recover the
# full file list from the diff media type, which is not subject to it; if
# that fetch fails too (GitHub also bounds raw-diff rendering, at sizes far
# beyond the cap), refuse to answer rather than under-report.
JSON_COUNT="$(jq 'length' <<< "$FILES_JSON")"
if [ "$JSON_COUNT" -ge 300 ]; then
  DIFF="$(gh api -H "Accept: application/vnd.github.diff" "$COMPARE")" || {
    echo "ERROR: $SHORT_REV..$BRANCH exceeds GitHub's 300-file compare cap and the raw-diff fallback failed; refusing to report a partial file list" >&2
    exit 1
  }
  # Reduce the raw diff to the same [{status, filename}] shape: one block per
  # `diff --git` header, status from the block's mode/rename lines. The
  # filename baseline comes from the header itself, because a block is not
  # guaranteed any other line naming the file: mode-only changes and
  # empty-file adds/deletes carry no ---/+++, rename, or Binary lines at
  # all. Those degenerate blocks are always symmetric — `a/X b/X`, same X on
  # both sides — so X is recovered by length, validating that the two halves
  # match; searching for a ` b/` delimiter instead would mis-split any X that
  # itself contains ` b/`, first match and last match alike, and hand back a
  # wrong-but-nonempty name the abort guard cannot see. Asymmetric headers
  # are renames or copies, which git always names on their own `rename to` /
  # `copy to` lines; `+++ b/` refines content-bearing blocks.
  #
  # The coverage claim is total, and closed by construction rather than by
  # enumerating path shapes: git path output is exactly two grammars. An
  # UNQUOTED path is parsed exactly by the rules above; a QUOTED path — git
  # wraps the whole name in double quotes when it holds specials — is
  # refused loudly, never decoded. Every extraction funnels through one
  # fail-closed boundary at flush(): a filename that is empty (unsplittable
  # header, no refinement line matched) or begins with a double quote (a
  # quoted path reached fn through any rule) aborts the whole run with exit
  # 1. Any future "what about X in a path" lands on one side or the other:
  # unquoted parses, quoted refuses. There is no third case.
  FILES_JSON="$(awk '
    function flush() {
      if (inblock) {
        if (fn == "" || fn ~ /^"/) { bad = 1; print "ERROR: diff block with no derivable unquoted filename; refusing to under-report" > "/dev/stderr"; exit 1 }
        printf "%s\t%s\n", st, fn
      }
      inblock = 1; st = "modified"; fn = ""
    }
    /^diff --git /  {
      flush()
      s = substr($0, 12)
      n = (length(s) - 5) / 2
      if (s ~ /^a\// && n == int(n) && substr(s, n + 3, 3) == " b/" && substr(s, 3, n) == substr(s, n + 6)) fn = substr(s, 3, n)
    }
    /^new file mode /   { st = "added" }
    /^deleted file mode / { st = "removed" }
    /^rename from /     { st = "renamed" }
    /^rename to /       { fn = substr($0, 11) }
    /^copy from /       { st = "copied" }
    /^copy to /         { fn = substr($0, 9) }
    /^\+\+\+ b\//       { fn = substr($0, 7) }
    END { if (!bad) flush() }
  ' <<< "$DIFF" | jq -Rn '[inputs | split("\t") | {status: .[0], filename: .[1]}]')"

  # Response-integrity invariant, closing this class the way quote-refusal
  # closed the path grammar: this branch only executes because the JSON
  # endpoint just proved the comparison holds at least JSON_COUNT (>= 300)
  # files, so a fallback that recovers fewer — an empty 200, a truncated
  # body, a blockless response — is by definition incomplete. Require the
  # recovery to be at least as large as what was already proven, or answer
  # nothing at all.
  RECOVERED="$(jq 'length' <<< "$FILES_JSON")"
  if [ "$RECOVERED" -lt "$JSON_COUNT" ]; then
    echo "ERROR: raw-diff fallback recovered only $RECOVERED files where the compare endpoint already proved $JSON_COUNT; refusing to under-report" >&2
    exit 1
  fi
fi

report() {
  jq -r --arg section "$1" "map(select(.filename | test(\$section))) | if length == 0 then \"  (no changes in $2)\" else .[] | \"  \" + .status[:1] + \" \" + .filename end" <<< "$FILES_JSON"
}

echo "==> $HEADER API docs changes since last sync ($SHORT_REV..$BRANCH):"
report '^doc/api/' 'doc/api/'

echo ""
echo "==> $HEADER API implementation changes since last sync ($SHORT_REV..$BRANCH):"
report '^app/(controllers|views/api)/' 'app/controllers/ or app/views/api/'
