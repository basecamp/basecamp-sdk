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
#   - pairwiseSupersetKeys flags non-object values instead of silently
#         skipping; null (absent) counts as the empty key set on either side.
set -euo pipefail

# Fail fast with one clear message rather than sixteen confusing scenario
# failures: compare-canary-runs.sh requires bash >= 4.4 (mapfile -d), and on
# macOS `/usr/bin/env bash` may resolve to the system 3.2.
if [ -z "${BASH_VERSINFO:-}" ] || [ "${BASH_VERSINFO[0]}" -lt 4 ] \
  || { [ "${BASH_VERSINFO[0]}" -eq 4 ] && [ "${BASH_VERSINFO[1]}" -lt 4 ]; }; then
  echo "ERROR: bash >= 4.4 is required (found ${BASH_VERSION:-unknown}). On macOS: brew install bash" >&2
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPARE="$SCRIPT_DIR/compare-canary-runs.sh"
REAL_TESTS="$SCRIPT_DIR/../conformance/tests/live-my-surface.json"

# The exact filename the TS runner writes for the real GetMyNotifications test.
# Hardcoded (not re-derived from the sed in the compare script) so a regression
# in the sanitizer is actually caught: this is the golden value from
# conformance/runner/typescript/live-runner.test.ts persistSnapshot().
GMN_TEST_NAME="GetMyNotifications decodes unreads/reads/memories/bubble_ups"
GMN_SAFE_NAME="GetMyNotifications_decodes_unreads_reads_memories_bubble_ups"

# Template form for portability: current macOS mktemp accepts a bare -d,
# but older BSD variants insist on a template — and the template costs nothing.
TMP="$(mktemp -d "${TMPDIR:-/tmp}/compare-canary-tests.XXXXXX")"
trap 'rm -rf -- "$TMP"' EXIT

# Scenarios A–D exercise the real GetMyNotifications entry (golden filename,
# memories waiver, missing-snapshot hard error). Filter that one entry out of
# the real fixture so adding more pairwise-bearing tests to live-my-surface.json
# doesn't break these scenarios with missing-snapshot errors for tests this
# suite never writes snapshots for. The entry itself stays verbatim-real.
GMN_TESTS="$TMP/gmn-tests.json"
jq --arg name "$GMN_TEST_NAME" 'map(select(.name == $name))' "$REAL_TESTS" >"$GMN_TESTS"
if [ "$(jq 'length' "$GMN_TESTS")" -ne 1 ]; then
  echo "FATAL: expected exactly one '$GMN_TEST_NAME' entry in $REAL_TESTS" >&2
  exit 1
fi

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

# write_snapshot_2p <file> <operation> <page1-body-json> <page2-body-json>
write_snapshot_2p() {
  local file="$1" operation="$2" body1="$3" body2="$4"
  mkdir -p "$(dirname "$file")"
  jq -n --arg op "$operation" --argjson b1 "$body1" --argjson b2 "$body2" \
    '{operation: $op, pages: [
       {status: 200, headers: {}, body: $b1, bodyText: ($b1 | tostring), url: "https://example.test?page=1"},
       {status: 200, headers: {}, body: $b2, bodyText: ($b2 | tostring), url: "https://example.test?page=2"}
     ], pages_count: 2}' \
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
run_compare "$BC4" "$BC5" "$GMN_TESTS"
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
run_compare "$BC4" "$BC5" "$GMN_TESTS"
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
run_compare "$BC4" "$BC5" "$GMN_TESTS"
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
run_compare "$BC4" "$BC5" "$GMN_TESTS"
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

# ---------------------------------------------------------------------------
# Test G: pairwiseSupersetKeys flags a non-object value instead of silently
# skipping — a mis-specified path or an object → scalar shape change must not
# hide behind a no-op rule.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs G)"
SK_TESTS="$TMP/G/sk-tests.json"
cat >"$SK_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "Keys type test",
    "operation": "SkOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseSupersetKeys", "paths": ["obj"], "reason": "keys rule must reject non-object shapes" }
    ]
  }
]
JSON
write_snapshot "$BC4/Keys_type_test.json" SkOp '{"obj":{"a":1}}'
write_snapshot "$BC5/Keys_type_test.json" SkOp '{"obj":"not-an-object"}'
run_compare "$BC4" "$BC5" "$SK_TESTS"
if [ "$RUN_RC" -eq 1 ] && grep -q "expected objects" <<<"$RUN_OUT"; then
  pass "G: non-object at a supersetKeys path fails (exit 1), no silent skip"
else
  fail "G: expected exit 1 with type violation; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test H: null (absent) counts as the empty key set — BC5 null where BC4 has
# keys reports the missing keys rather than passing or erroring on type.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs H)"
NK_TESTS="$TMP/H/nk-tests.json"
cat >"$NK_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "Keys null test",
    "operation": "NkOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseSupersetKeys", "paths": ["obj"], "reason": "absent object on BC5 is a dropped-keys regression" }
    ]
  }
]
JSON
write_snapshot "$BC4/Keys_null_test.json" NkOp '{"obj":{"a":1,"b":2}}'
write_snapshot "$BC5/Keys_null_test.json" NkOp '{}'
run_compare "$BC4" "$BC5" "$NK_TESTS"
if [ "$RUN_RC" -eq 1 ] && grep -q "missing keys present in BC4: a,b" <<<"$RUN_OUT"; then
  pass "H: BC5 null where BC4 has keys reports the missing keys (exit 1)"
else
  fail "H: expected exit 1 with missing-keys a,b; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test I: a malformed tests file is an operator error (exit 2), not a silent
# "nothing to compare" pass — jq's failure must not vanish into the mapfile
# process substitution.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs I)"
BAD_TESTS="$TMP/I/bad-tests.json"
printf '{ this is not json' >"$BAD_TESTS"
run_compare "$BC4" "$BC5" "$BAD_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "failed to parse tests file" <<<"$RUN_OUT"; then
  pass "I: malformed tests file fails with exit 2, not a silent pass"
else
  fail "I: expected exit 2 with parse error; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test J: a tests file whose top level is an object (not an array) is also an
# operator error — jq's `.[]` would happily iterate an object's values, so the
# type is checked explicitly.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs J)"
OBJ_TESTS="$TMP/J/obj-tests.json"
printf '{"tests": []}' >"$OBJ_TESTS"
run_compare "$BC4" "$BC5" "$OBJ_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "failed to parse tests file" <<<"$RUN_OUT"; then
  pass "J: top-level-object tests file fails with exit 2, not a silent pass"
else
  fail "J: expected exit 2 for non-array top level; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test K: pages[*] superset arrays compare TOTAL items across pages, not the
# page count — BC4 with items on every page vs BC5 with the field dropped on
# every page must fail even though pages_count matches.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs K)"
AGG_TESTS="$TMP/K/agg-tests.json"
cat >"$AGG_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "Agg pages test",
    "operation": "AggOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseSupersetArray", "paths": ["pages[*].body.items"], "reason": "items must not shrink across the full paginated collection" }
    ]
  }
]
JSON
write_snapshot_2p "$BC4/Agg_pages_test.json" AggOp '{"items":[1,2]}' '{"items":[3]}'
write_snapshot_2p "$BC5/Agg_pages_test.json" AggOp '{"other":true}' '{"other":true}'
run_compare "$BC4" "$BC5" "$AGG_TESTS"
if [ "$RUN_RC" -eq 1 ] && grep -q "BC5 length 0 < BC4 length 3" <<<"$RUN_OUT"; then
  pass "K: pages[*] field dropped on every page fails on totals (exit 1)"
else
  fail "K: expected exit 1 comparing totals 0 < 3; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test L: pages[*] totals tolerate redistribution — the same items split
# differently across pages is not a regression.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs L)"
write_snapshot_2p "$BC4/Agg_pages_test.json" AggOp '{"items":[1,2]}' '{"items":[3]}'
write_snapshot_2p "$BC5/Agg_pages_test.json" AggOp '{"items":[1]}' '{"items":[2,3]}'
run_compare "$BC4" "$BC5" "$AGG_TESTS"
if [ "$RUN_RC" -eq 0 ]; then
  pass "L: pages[*] redistribution with total preserved passes (exit 0)"
else
  fail "L: expected exit 0 for preserved total; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test M: a non-array `paths` (string/object) is an operator error (exit 2)
# with an explicit message — not jq's raw exit status and not a silent
# misinterpretation (an object's values would otherwise iterate as paths).
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs M)"
STR_TESTS="$TMP/M/str-tests.json"
cat >"$STR_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "String paths test",
    "operation": "SpOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseEqual", "paths": "obj" }
    ]
  }
]
JSON
write_snapshot "$BC4/String_paths_test.json" SpOp '{"obj":1}'
write_snapshot "$BC5/String_paths_test.json" SpOp '{"obj":1}'
run_compare "$BC4" "$BC5" "$STR_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "must be an array of strings" <<<"$RUN_OUT"; then
  pass "M: non-array 'paths' fails with exit 2 and explicit message"
else
  fail "M: expected exit 2 with type error; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test N: a structurally invalid snapshot (valid JSON, wrong shape) is an
# operator error — '{}' would otherwise make every read return null and
# false-green superset rules.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs N)"
write_snapshot "$BC4/Eq_order_test.json" EqOp '{"obj":{"a":1}}'
printf '{}' >"$BC5/Eq_order_test.json"
run_compare "$BC4" "$BC5" "$EQ_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "structurally invalid wire snapshot" <<<"$RUN_OUT"; then
  pass "N: structurally invalid snapshot fails with exit 2, not a false-green"
else
  fail "N: expected exit 2 with structural error; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test O: a trailing empty-string path ("" = body root) in a paths list is
# preserved and evaluated — newline splitting would have dropped it and
# exit-2'd on a count mismatch for a perfectly valid rule.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs O)"
BR_TESTS="$TMP/O/br-tests.json"
cat >"$BR_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "Body root trailing test",
    "operation": "BrOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseEqual", "paths": ["obj", ""], "reason": "body root listed last must still be compared" }
    ]
  }
]
JSON
write_snapshot "$BC4/Body_root_trailing_test.json" BrOp '{"obj":1,"k":2}'
write_snapshot "$BC5/Body_root_trailing_test.json" BrOp '{"obj":1,"k":2}'
run_compare "$BC4" "$BC5" "$BR_TESTS"
if [ "$RUN_RC" -eq 0 ] && grep -q "compared 1 operation" <<<"$RUN_OUT"; then
  pass "O: trailing body-root path is preserved and evaluated (exit 0)"
else
  fail "O: expected exit 0 with comparison run; got rc=$RUN_RC: $RUN_OUT"
fi

# O2: same rule must still FAIL when the body root actually differs,
# proving the trailing "" path is evaluated rather than merely tolerated.
read -r BC4 BC5 <<<"$(fresh_dirs O2)"
write_snapshot "$BC4/Body_root_trailing_test.json" BrOp '{"obj":1,"k":2}'
write_snapshot "$BC5/Body_root_trailing_test.json" BrOp '{"obj":1,"k":3}'
run_compare "$BC4" "$BC5" "$BR_TESTS"
if [ "$RUN_RC" -eq 1 ] && grep -q "pairwiseEqual(<body>)" <<<"$RUN_OUT"; then
  pass "O2: trailing body-root path violation still fails (exit 1)"
else
  fail "O2: expected exit 1 with body-root violation; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test P: a waiver whose `paths` is a bare string (not an array) is an
# operator error — flatten would otherwise accept the string as one allowed
# path and silently suppress the enforcing rule (false-green, not a typo).
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs P)"
WV_TESTS="$TMP/P/wv-tests.json"
cat >"$WV_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "Waiver type test",
    "operation": "WvOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseSupersetArray", "paths": ["memories"], "reason": "no shrink" },
      { "type": "pairwiseDeltaAllowed", "paths": "memories", "reason": "typo: bare string, must be rejected" }
    ]
  }
]
JSON
write_snapshot "$BC4/Waiver_type_test.json" WvOp '{"memories":[1,2,3]}'
write_snapshot "$BC5/Waiver_type_test.json" WvOp '{"memories":[]}'
run_compare "$BC4" "$BC5" "$WV_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "array of strings" <<<"$RUN_OUT"; then
  pass "P: bare-string waiver paths fails with exit 2, no silent suppression"
else
  fail "P: expected exit 2 with waiver type error; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test Q: a page object missing the documented keys (e.g. no body) is a
# structurally invalid snapshot — reads would return null and false-green.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs Q)"
write_snapshot "$BC4/Eq_order_test.json" EqOp '{"obj":{"a":1}}'
printf '{"operation":"EqOp","pages":[{"status":200}],"pages_count":1}' >"$BC5/Eq_order_test.json"
run_compare "$BC4" "$BC5" "$EQ_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "structurally invalid wire snapshot" <<<"$RUN_OUT"; then
  pass "Q: page missing documented keys fails with exit 2"
else
  fail "Q: expected exit 2 for body-less page; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test R: a waiver without a reason is rejected at runtime — schema.json
# requires reasons for accepted divergences, and this script also runs
# standalone (check-bc5-compat, scheduled workflow) with no schema step,
# so an unaudited waiver must not suppress enforcement.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs R)"
NR_TESTS="$TMP/R/nr-tests.json"
cat >"$NR_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "Waiver reason test",
    "operation": "WrOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseSupersetArray", "paths": ["memories"], "reason": "no shrink" },
      { "type": "pairwiseDeltaAllowed", "paths": ["memories"] }
    ]
  }
]
JSON
write_snapshot "$BC4/Waiver_reason_test.json" WrOp '{"memories":[1,2,3]}'
write_snapshot "$BC5/Waiver_reason_test.json" WrOp '{"memories":[]}'
run_compare "$BC4" "$BC5" "$NR_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "non-empty 'reason'" <<<"$RUN_OUT"; then
  pass "R: reason-less waiver fails with exit 2, no unaudited suppression"
else
  fail "R: expected exit 2 with reason requirement; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test S: a snapshot whose recorded operation doesn't match the test's is an
# operator error — a stale/overwritten file with the right name must not be
# compared as this test's capture.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs S)"
write_snapshot "$BC4/Eq_order_test.json" EqOp '{"obj":{"a":1}}'
write_snapshot "$BC5/Eq_order_test.json" SomeOtherOp '{"obj":{"a":1}}'
run_compare "$BC4" "$BC5" "$EQ_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "stale or overwritten snapshot" <<<"$RUN_OUT"; then
  pass "S: snapshot/test operation mismatch fails with exit 2"
else
  fail "S: expected exit 2 with operation mismatch; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test T: a waiver with an empty (or missing) `paths` array is an operator
# error at runtime — schema.json's minItems:1 doesn't apply when the compare
# script runs standalone, and an empty waiver is always a fixture mistake.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs T)"
EW_TESTS="$TMP/T/ew-tests.json"
cat >"$EW_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "Waiver empty paths test",
    "operation": "EwOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseEqual", "paths": ["obj"], "reason": "must match" },
      { "type": "pairwiseDeltaAllowed", "paths": [], "reason": "empty waiver is a fixture mistake" }
    ]
  }
]
JSON
write_snapshot "$BC4/Waiver_empty_paths_test.json" EwOp '{"obj":1}'
write_snapshot "$BC5/Waiver_empty_paths_test.json" EwOp '{"obj":1}'
run_compare "$BC4" "$BC5" "$EW_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "non-empty array" <<<"$RUN_OUT"; then
  pass "T: empty waiver paths fails with exit 2 at runtime"
else
  fail "T: expected exit 2 with non-empty-paths error; got rc=$RUN_RC: $RUN_OUT"
fi

# ---------------------------------------------------------------------------
# Test U: '[*]' anywhere other than the leading 'pages[*]' segment is an
# unsupported path — reject as a fixture mistake rather than streaming
# through jq with undefined comparison semantics.
# ---------------------------------------------------------------------------
read -r BC4 BC5 <<<"$(fresh_dirs U)"
UP_TESTS="$TMP/U/up-tests.json"
cat >"$UP_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "Unsupported star test",
    "operation": "UsOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseSupersetArray", "paths": ["items[*].foo"], "reason": "undocumented star form" }
    ]
  }
]
JSON
write_snapshot "$BC4/Unsupported_star_test.json" UsOp '{"items":[{"foo":[1]}]}'
write_snapshot "$BC5/Unsupported_star_test.json" UsOp '{"items":[{"foo":[1]}]}'
run_compare "$BC4" "$BC5" "$UP_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "only supported as the leading" <<<"$RUN_OUT"; then
  pass "U: non-pages '[*]' path fails with exit 2 as unsupported"
else
  fail "U: expected exit 2 with unsupported-path error; got rc=$RUN_RC: $RUN_OUT"
fi

# U2: a SECOND '[*]' after a valid leading 'pages[*]' is equally unsupported.
read -r BC4 BC5 <<<"$(fresh_dirs U2)"
UP2_TESTS="$TMP/U2/up2-tests.json"
cat >"$UP2_TESTS" <<'JSON'
[
  {
    "mode": "live",
    "name": "Double star test",
    "operation": "DsOp",
    "method": "GET",
    "path": "/x",
    "liveAssertions": [{ "type": "liveCallSucceeds" }],
    "pairwiseAssertions": [
      { "type": "pairwiseSupersetArray", "paths": ["pages[*].body.items[*]"], "reason": "second star is unsupported" }
    ]
  }
]
JSON
write_snapshot "$BC4/Double_star_test.json" DsOp '{"items":[[1]]}'
write_snapshot "$BC5/Double_star_test.json" DsOp '{"items":[[1]]}'
run_compare "$BC4" "$BC5" "$UP2_TESTS"
if [ "$RUN_RC" -eq 2 ] && grep -q "only supported as the leading" <<<"$RUN_OUT"; then
  pass "U2: second '[*]' after pages[*] fails with exit 2 as unsupported"
else
  fail "U2: expected exit 2 with unsupported-path error; got rc=$RUN_RC: $RUN_OUT"
fi

echo ""
if [ "$FAILURES" -ne 0 ]; then
  echo "FAILED: $FAILURES test(s)" >&2
  exit 1
fi
echo "All compare-canary-runs regression tests passed"
