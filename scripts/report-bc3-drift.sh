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
if [ "$(jq 'length' <<< "$FILES_JSON")" -ge 300 ]; then
  DIFF="$(gh api -H "Accept: application/vnd.github.diff" "$COMPARE")" || {
    echo "ERROR: $SHORT_REV..$BRANCH exceeds GitHub's 300-file compare cap and the raw-diff fallback failed; refusing to report a partial file list" >&2
    exit 1
  }
  # Reduce the raw diff to the same [{status, filename}] shape: one block per
  # `diff --git` header, status from the block's mode/rename lines. The
  # filename baseline comes from the header itself — its ` b/` path is the
  # JSON endpoint's `filename` in every case (post-image path; the old path
  # for a delete, where both header paths are the same; the new name for a
  # rename) — because a block is not guaranteed any other line naming the
  # file: mode-only changes and empty-file adds/deletes carry no ---/+++,
  # rename, or Binary lines at all. `rename to`/`+++ b/` then refine it,
  # which also covers headers the naive parse cannot split (git quotes paths
  # with unusual characters). A block that still has no filename aborts the
  # whole run — this fallback exists to never under-report.
  FILES_JSON="$(awk '
    function flush() {
      if (inblock) {
        if (fn == "") { bad = 1; print "ERROR: diff block with no derivable filename; refusing to under-report" > "/dev/stderr"; exit 1 }
        printf "%s\t%s\n", st, fn
      }
      inblock = 1; st = "modified"; fn = ""
    }
    /^diff --git /  {
      flush()
      if (match($0, / b\//)) fn = substr($0, RSTART + 3)
    }
    /^new file mode /   { st = "added" }
    /^deleted file mode / { st = "removed" }
    /^rename from /     { st = "renamed" }
    /^rename to /       { fn = substr($0, 11) }
    /^\+\+\+ b\//       { fn = substr($0, 7) }
    END { if (!bad) flush() }
  ' <<< "$DIFF" | jq -Rn '[inputs | split("\t") | {status: .[0], filename: .[1]}]')"
fi

report() {
  jq -r --arg section "$1" "map(select(.filename | test(\$section))) | if length == 0 then \"  (no changes in $2)\" else .[] | \"  \" + .status[:1] + \" \" + .filename end" <<< "$FILES_JSON"
}

echo "==> $HEADER API docs changes since last sync ($SHORT_REV..$BRANCH):"
report '^doc/api/' 'doc/api/'

echo ""
echo "==> $HEADER API implementation changes since last sync ($SHORT_REV..$BRANCH):"
report '^app/(controllers|views/api)/' 'app/controllers/ or app/views/api/'
