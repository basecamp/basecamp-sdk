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

echo "==> $HEADER API docs changes since last sync ($SHORT_REV..$BRANCH):"
gh api "repos/$BC3_REPO/compare/$REV...$BRANCH" \
  --jq '[.files[] | select(.filename | startswith("doc/api/"))] | if length == 0 then "  (no changes in doc/api/)" else .[] | "  " + .status[:1] + " " + .filename end'

echo ""
echo "==> $HEADER API implementation changes since last sync ($SHORT_REV..$BRANCH):"
gh api "repos/$BC3_REPO/compare/$REV...$BRANCH" \
  --jq '[.files[] | select(.filename | startswith("app/controllers/") or startswith("app/views/api/"))] | if length == 0 then "  (no changes in app/controllers/ or app/views/api/)" else .[] | "  " + .status[:1] + " " + .filename end'
