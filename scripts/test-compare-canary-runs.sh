#!/usr/bin/env bash
# Regression tests for scripts/compare-canary-runs.sh.
#
# Network-free: builds synthetic wire snapshots in a temp dir and asserts the
# compare script's exit code + key output for each scenario. Guards the bug
# classes fixed for PR #308:
#
#   - P0  snapshot filename must match the TS live runner's scheme exactly
#         (testName.replace(/[^a-z0-9_-]+/gi, "_"), case-preserved). The old
#         lowercase/hyphen/operation-first form found nothing and silently
#         passed.
#   - P1  a declared pairwise test with a missing snapshot is a hard error,
#         not a silent skip (which would let check-bc5-compat false-green).
#   - the `memories` pairwiseDeltaAllowed waiver scopes to `memories` only;
#         an unrelated regression still fails.
#   - pairwiseEqual compares semantically (object key order is irrelevant).
#   - an empty `paths` array is rejected at runtime (defense in depth behind
#         the schema's minItems:1).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPARE="$SCRIPT_DIR/compare-canary-runs.sh"
REAL_TESTS="$SCRIPT_DIR/../conformance/tests/live-my-surface.json"

# The exact filename the TS runner writes for the real GetMyNotifications test.
# Hardcoded (not re-derived from the sed in the compare script) so a regression
# in the sanitizer is actually caught: this is the golden value from
# conformance/runner/typescript/live-runner.test.ts persistSnapshot().
GMN_TEST_NAME="GetMyNotifications decodes unreads/reads/memories/bubble_ups"
GMN_SAFE_NAME="GetMyNotifications_decodes_unreads_reads_memories_bubble_ups"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILURES=0
pass() { printf '  ok    %s\n' "$1"; }
fail() { printf '  FAIL  %s\n' "$1" >&2; FAILURES=$((FAILURES + 1)); }

# write_snapshot <file> <operation> <body-json>
write_snapshot() {
  local file="$1" operation="$2" body="$3"
  mkdir -p "$(dirname "$file")"
  jq -n --arg op "$operation" --argjson body "$body" \
    '{operation: $op, pages: [{status: 200, headers: {}, body: $body, bodyText: ($body | tostring), url: "https://example.test"}], pages_count: 1}' \
    >"$file"
}

# fresh_dirs <scenario> -> echoes "<bc4dir> <bc5dir>" (created empty)
fresh_dirs() {
  local s="$1"
  local bc4="$TMP/$s/bc4/wire" bc5="$TMP/$s/bc5/wire"
  mkdir -p "$bc4" "$bc5"
  echo "$bc4 $bc5"
}

# run the compare script, capturing exit code + combined output
# usage: run_compare <bc4dir> <bc5dir> [tests-file]
RUN_OUT=""
RUN_RC=0
run_compare() {
  set +e
  RUN_OUT="$("$COMPARE" "$@" 2>&1)"
  RUN_RC=$?
  set -e
}

echo "==> test-compare-canary-runs"

# ---------------------------------------------------------------------------
# Test A: TS filename scheme is found + evaluated, and the memories waiver
# allows BC5 memories[] to be shorter than BC4's. Uses the REAL fixture.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs A)"
write_snapshot "$BC4/$GMN_SAFE_NAME.json" GetMyNotifications \
  '{"unreads":[1,2],"reads":[3],"memories":[10,11,12],"bubble_ups":[]}'
write_snapshot "$BC5/$GMN_SAFE_NAME.json" GetMyNotifications \
  '{"unreads":[1,2],"reads":[3],"memories":[],"bubble_ups":[],"scheduled_bubble_ups":[]}'
run_compare "$BC4" "$BC5" "$REAL_TESTS"
if [ "$RUN_RC" -eq 0 ] && grep -q "compared 1 operation" <<<"$RUN_OUT"; then
  pass "A: correct-named snapshot found + memories waiver applied (exit 0)"
else
  fail "A: expected exit 0 with 'compared 1 operation'; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test B: the OLD filename form (lowercase + hyphen + operation-first) is NOT
# found, proving the scheme matters (this is the P0 regression made concrete).
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs B)"
OLD_NAME="getmynotifications-decodes-unreads-reads-memories-bubble_ups"
write_snapshot "$BC4/$OLD_NAME.json" GetMyNotifications '{"memories":[1]}'
write_snapshot "$BC5/$OLD_NAME.json" GetMyNotifications '{"memories":[]}'
run_compare "$BC4" "$BC5" "$REAL_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "missing .* snapshot" <<<"$RUN_OUT"; then
  pass "B: old lowercase/hyphen filename is not found (exit 2)"
else
  fail "B: expected exit 2 (missing) for old-scheme name; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test C (P1): a declared pairwise test missing a snapshot on one backend is a
# hard error, not a silent skip.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs C)"
write_snapshot "$BC4/$GMN_SAFE_NAME.json" GetMyNotifications '{"memories":[1,2,3]}'
# (no BC5 snapshot written)
run_compare "$BC4" "$BC5" "$REAL_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "missing BC5 snapshot" <<<"$RUN_OUT"; then
  pass "C: missing BC5 snapshot hard-fails (exit 2), no silent skip"
else
  fail "C: expected exit 2 with 'missing BC5 snapshot'; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test D: the memories waiver scopes to `memories` only. A real regression on a
# different path (a dropped top-level key) still fails via pairwiseSupersetKeys.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs D)"
write_snapshot "$BC4/$GMN_SAFE_NAME.json" GetMyNotifications \
  '{"unreads":[1],"reads":[2],"memories":[10],"bubble_ups":[]}'
# BC5 drops the top-level "unreads" key entirely (a genuine regression).
write_snapshot "$BC5/$GMN_SAFE_NAME.json" GetMyNotifications \
  '{"reads":[2],"memories":[],"bubble_ups":[]}'
run_compare "$BC4" "$BC5" "$REAL_TESTS"
if [ "$RUN_RC" -eq 1 ] && grep -q "missing keys present in BC4: unreads" <<<"$RUN_OUT"; then
  pass "D: dropped top-level key still fails (exit 1); waiver scoped to memories"
else
  fail "D: expected exit 1 with dropped-key violation; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test E: pairwiseEqual compares semantically — object key order is irrelevant.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs E)"
EQ_TESTS="$TMP/E/eq-tests.json"
cat >"$EQ_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "Eq order test",
    "operation": "EqOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseEqual", "paths": ["obj"], "reason": "discriminator shape must match" }
    ]
  }
]
JSON
write_snapshot "$BC4/Eq_order_test.json" EqOp '{"obj":{"a":1,"b":2,"c":3}}'
write_snapshot "$BC5/Eq_order_test.json" EqOp '{"obj":{"c":3,"b":2,"a":1}}'
run_compare "$BC4" "$BC5" "$EQ_TESTS"
if [ "$RUN_RC" -eq 0 ]; then
  pass "E: pairwiseEqual ignores object key order (exit 0)"
else
  fail "E: expected exit 0 for key-order-only difference; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test F: an empty `paths` array is rejected at runtime (not run against the
# body root). Both snapshots present so missing-snapshot can't mask this.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs F)"
EMPTY_TESTS="$TMP/F/empty-tests.json"
cat >"$EMPTY_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "Empty paths test",
    "operation": "EpOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseSupersetKeys", "paths": [] }
    ]
  }
]
JSON
write_snapshot "$BC4/Empty_paths_test.json" EpOp '{"k":1}'
write_snapshot "$BC5/Empty_paths_test.json" EpOp '{"k":1}'
run_compare "$BC4" "$BC5" "$EMPTY_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "empty 'paths' array" <<<"$RUN_OUT"; then
  pass "F: empty 'paths' array rejected at runtime (exit 2)"
else
  fail "F: expected exit 2 with empty-paths error; got rc=$RUN_RC: $RUN_OUT"
fi

echo ""
if [ "$FAILURES" -ne 0 ]; then
  echo "FAILED: $FAILURES test(s)" >&2
  exit 1
fi
echo "All compare-canary-runs regression tests passed"
