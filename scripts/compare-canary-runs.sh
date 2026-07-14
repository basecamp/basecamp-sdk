#!/usr/bin/env bash
# Pairwise BC4↔BC5 wire-snapshot comparison.
#
# Reads two snapshot directories (one per backend) written by the TS live
# canary runner — `<LIVE_RECORD_DIR>/<backend>/wire/<test>.json` — and applies
# the `pairwiseAssertions` rules from each test's entry in
# `conformance/tests/live-my-surface.json`. Reports violations of the
# additive-only invariant: BC5 must not drop arrays/keys/values that BC4
# emitted, except where `pairwiseDeltaAllowed` explicitly accepts the drift.
#
# Path semantics
# --------------
# Each rule's `paths` entries are dotted identifiers relative to a page body.
#
# - Empty string `""` addresses the body root.
# - `foo.bar` defaults to `pages[0].body.foo.bar` for single-page snapshots.
# - Paths starting with `pages[` are taken absolute, useful when a test
#   captures multiple pages or wants to address a specific page index.
#
# Rule types
# ----------
# - pairwiseSupersetArray: BC5 array length at each path must be ≥ BC4's.
#                          Catches "memories went to []". For aggregated
#                          `pages[*]` paths the comparison is the TOTAL item
#                          count across pages (a page missing the field
#                          contributes 0; a non-null non-array page value is
#                          invalid) — never the page count itself, which
#                          would false-green a field dropped on every page.
# - pairwiseSupersetKeys:  BC5 object's keys at each path must be ⊇ BC4's.
#                          Catches "field disappeared from BC5".
# - pairwiseEqual:         BC5 value at each path must equal BC4's. Use sparingly.
# - pairwiseDeltaAllowed:  paths where BC5↔BC4 divergence is explicitly
#                          accepted; the listed paths are skipped by the
#                          other three rules for this operation. `reason`
#                          is required.
#
# Exit codes
# ----------
# 0  clean: every rule held, or violations were covered by pairwiseDeltaAllowed.
# 1  one or more pairwise violations.
# 2  operator error: missing directory, missing test fixture, a missing or
#    malformed wire snapshot for a pairwise test, jq unavailable, etc.
#
# Usage
# -----
#   compare-canary-runs.sh <bc4-snapshot-dir> <bc5-snapshot-dir> [tests-file]
#
#   <bc4-snapshot-dir>  Path to the BC4 wire/ directory, e.g.
#                       tmp/live-canary/bc4/wire
#   <bc5-snapshot-dir>  Path to the BC5 wire/ directory.
#   [tests-file]        Optional path to live-my-surface.json. Defaults to
#                       conformance/tests/live-my-surface.json relative to
#                       the script's project root.
#
# Comparison requires identical account state across the two runs. The
# CONTRIBUTING.md "Live canary" section documents this — without it,
# `unreads` and similar collections drift naturally and rules will false-fail.
set -euo pipefail

# mapfile with NUL delimiters (bash 4.4+) is used below; macOS ships bash 3.2
# at /bin/bash, where the failure mode is an opaque "mapfile: command not found".
if [ -z "${BASH_VERSINFO:-}" ] || [ "${BASH_VERSINFO[0]}" -lt 4 ] \
  || { [ "${BASH_VERSINFO[0]}" -eq 4 ] && [ "${BASH_VERSINFO[1]}" -lt 4 ]; }; then
  echo "ERROR: bash >= 4.4 is required (found ${BASH_VERSION:-unknown}). On macOS: brew install bash" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
  echo "Usage: $0 <bc4-snapshot-dir> <bc5-snapshot-dir> [tests-file]" >&2
  exit 2
fi

BC4_DIR="$1"
BC5_DIR="$2"
TESTS_FILE="${3:-$PROJECT_ROOT/conformance/tests/live-my-surface.json}"

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required" >&2
  exit 2
fi

for arg in "$BC4_DIR" "$BC5_DIR"; do
  if [ ! -d "$arg" ]; then
    echo "ERROR: snapshot directory not found: $arg" >&2
    exit 2
  fi
done

if [ ! -f "$TESTS_FILE" ]; then
  echo "ERROR: tests file not found: $TESTS_FILE" >&2
  exit 2
fi

# Normalize a user-supplied path to a jq path expression.
#
# Inputs                  → jq path
#   ""                    → .pages[0].body
#   "foo.bar"             → .pages[0].body.foo.bar
#   "pages[0].body.foo"   → .pages[0].body.foo
#   "pages[*].body.foo"   → [.pages[].body.foo]   (jq stream collapsed to array)
to_jq_path() {
  local raw="$1"
  if [ -z "$raw" ]; then
    echo ".pages[0].body"
  elif [[ "$raw" == pages\[* ]]; then
    # Absolute path. Convert pages[*] to .pages[] (stream); we'll collect
    # downstream by wrapping into [...] if a `*` is present.
    local jq_path=".${raw//\[\*\]/[]}"
    echo "$jq_path"
  else
    echo ".pages[0].body.$raw"
  fi
}

# Snapshot reads that fail (malformed JSON, unreadable file) are operator
# errors, not pairwise violations. Remap jq's exit status (often 4 or 5) to
# the documented exit code 2 so the 0/1/2 contract holds under `set -e`.
snapshot_read_error() {
  echo "ERROR: failed to read snapshot '$1' at path '$2' (malformed JSON?)" >&2
  exit 2
}

# Read a JSON value at a normalized jq path from a snapshot file.
# Streams from pages[*] are wrapped into an array so the caller can treat
# them as a single aggregated value.
read_value() {
  local snapshot="$1"
  local user_path="$2"
  local jq_path
  jq_path="$(to_jq_path "$user_path")"

  if [[ "$user_path" == *"[*]"* ]]; then
    jq -c "[ $jq_path ]" "$snapshot" || snapshot_read_error "$snapshot" "$user_path"
  else
    jq -c "$jq_path" "$snapshot" || snapshot_read_error "$snapshot" "$user_path"
  fi
}

# Structural validation of a wire snapshot: valid JSON alone isn't enough —
# `{}` or `{"pages": {}}` would make every read_value return null, silently
# false-greening superset rules (null counts as absent/empty). Enforce the
# TS live runner's contract: object with a non-empty pages array whose
# length matches pages_count.
validate_snapshot() {
  local snap="$1"
  if ! jq -e '
      type == "object"
      and ((.pages | type) == "array")
      and ((.pages | length) > 0)
      and (.pages_count == (.pages | length))
      and all(.pages[];
            type == "object"
            and has("status") and has("headers") and has("body")
            and has("bodyText") and has("url"))
    ' "$snap" >/dev/null 2>&1; then
    echo "ERROR: structurally invalid wire snapshot '$snap' (expected an object with a non-empty pages array, matching pages_count, and {status, headers, body, bodyText, url} on every page)" >&2
    exit 2
  fi
}

# Path → display string for error messages.
display_path() {
  local raw="$1"
  if [ -z "$raw" ]; then
    echo "<body>"
  else
    echo "$raw"
  fi
}

VIOLATIONS=""
violation() {
  VIOLATIONS="${VIOLATIONS}$1
"
}

# Collect comparable operations: tests that have pairwiseAssertions.
# Materialize the jq output before mapfile: a jq failure inside a process
# substitution isn't seen by set -e, so a malformed tests file would leave
# TEST_ENTRIES empty and the script would exit 0 ("nothing to compare"),
# silently masking an operator error.
if ! ENTRIES_RAW="$(jq -c 'if type != "array" then error("tests file must be a top-level array") else . end | .[] | select((.pairwiseAssertions // []) | length > 0)' "$TESTS_FILE")"; then
  echo "ERROR: failed to parse tests file '$TESTS_FILE' (malformed JSON or not a top-level array)" >&2
  exit 2
fi
mapfile -t TEST_ENTRIES <<<"$ENTRIES_RAW"
if [ "${#TEST_ENTRIES[@]}" -eq 1 ] && [ -z "${TEST_ENTRIES[0]}" ]; then
  TEST_ENTRIES=()
fi

if [ "${#TEST_ENTRIES[@]}" -eq 0 ]; then
  echo "==> Pairwise canary: no tests carry pairwiseAssertions; nothing to compare"
  exit 0
fi

COMPARED=0
MISSING_SNAPSHOTS=0

for entry in "${TEST_ENTRIES[@]}"; do
  NAME="$(jq -r '.name' <<<"$entry")"
  OPERATION="$(jq -r '.operation' <<<"$entry")"

  # Snapshot filenames must match exactly what the TS live runner writes.
  # conformance/runner/typescript/live-runner.test.ts persistSnapshot() uses
  #   safeName = testName.replace(/[^a-z0-9_-]+/gi, "_")
  # i.e. the TEST NAME (not the operation), case preserved, with each run of
  # characters outside [A-Za-z0-9_-] collapsed to a single "_". There is no
  # operation-based or lowercased filename — the earlier candidate forms never
  # matched a real file, so every comparison was silently skipped.
  safe="$(printf '%s' "$NAME" | sed -E 's/[^a-zA-Z0-9_-]+/_/g')"
  BC4_SNAPSHOT="$BC4_DIR/$safe.json"
  BC5_SNAPSHOT="$BC5_DIR/$safe.json"

  # TEST_ENTRIES contains only tests that declare pairwiseAssertions, so a
  # missing snapshot on either backend is a hard error (exit 2): skipping it
  # would let check-bc5-compat pass without ever evaluating the declared rule.
  if [ ! -f "$BC4_SNAPSHOT" ] || [ ! -f "$BC5_SNAPSHOT" ]; then
    MISSING_SNAPSHOTS=$((MISSING_SNAPSHOTS + 1))
    [ ! -f "$BC4_SNAPSHOT" ] && echo "ERROR: missing BC4 snapshot for test '$NAME' ($OPERATION): $BC4_SNAPSHOT" >&2
    [ ! -f "$BC5_SNAPSHOT" ] && echo "ERROR: missing BC5 snapshot for test '$NAME' ($OPERATION): $BC5_SNAPSHOT" >&2
    continue
  fi

  validate_snapshot "$BC4_SNAPSHOT"
  validate_snapshot "$BC5_SNAPSHOT"

  COMPARED=$((COMPARED + 1))

  # Allowlisted paths for this operation (skipped by the other rule types).
  # Materialize jq output before mapfile (a process substitution's failure is
  # invisible to set -e), and drive emptiness off jq's own length so a lone
  # empty-string path ("" = body root) isn't confused with no paths at all.
  if ! ALLOW_JSON="$(jq -c '
      (.pairwiseAssertions // [])
      | map(select(.type == "pairwiseDeltaAllowed"))
      | if all(.[]; (.reason | type == "string" and length > 0)) then .
        else error("pairwiseDeltaAllowed requires a non-empty reason")
        end
      | map(.paths // [])
      | if all(.[]; type == "array" and all(.[]; type == "string"))
        then flatten
        else error("pairwiseDeltaAllowed paths must be arrays of strings")
        end
    ' <<<"$entry")"; then
    echo "ERROR: invalid pairwiseDeltaAllowed waiver on $OPERATION — each waiver needs a non-empty 'reason' (accepted divergences must be audited) and 'paths' as an array of strings (a bare-string paths would silently suppress enforcement)" >&2
    exit 2
  fi
  ALLOW_COUNT="$(jq -r 'length' <<<"$ALLOW_JSON")"
  ALLOW_PATHS=()
  if [ "$ALLOW_COUNT" -gt 0 ]; then
    # NUL-delimited extraction: newline splitting via command substitution
    # strips trailing blank lines, silently dropping a trailing empty-string
    # path ("" = body root). The count check backstops jq failures inside
    # the process substitution (invisible to set -e) and non-string elements.
    mapfile -d '' -t ALLOW_PATHS < <(jq -j '.[] + "\u0000"' <<<"$ALLOW_JSON")
    if [ "${#ALLOW_PATHS[@]}" -ne "$ALLOW_COUNT" ]; then
      echo "ERROR: pairwiseDeltaAllowed path extraction mismatch for $OPERATION (expected $ALLOW_COUNT, got ${#ALLOW_PATHS[@]}; paths must be strings)" >&2
      exit 2
    fi
  fi
  is_allowed() {
    local p="$1"
    local ap
    # Use ${#var[@]} guard rather than ${var[@]:-} — the latter substitutes a
    # single empty string for an empty array, which would erroneously match
    # an empty-string `""` rule path (the "body root" sentinel).
    if [ "${#ALLOW_PATHS[@]}" -eq 0 ]; then
      return 1
    fi
    for ap in "${ALLOW_PATHS[@]}"; do
      [ "$p" = "$ap" ] && return 0
    done
    return 1
  }

  # Iterate over the enforcing rules (everything except pairwiseDeltaAllowed).
  # Same materialize-before-mapfile treatment as ALLOW_PATHS above.
  if ! RULES_RAW="$(jq -c '
      (.pairwiseAssertions // [])
      | map(select(.type != "pairwiseDeltaAllowed"))
      | .[]
    ' <<<"$entry")"; then
    echo "ERROR: failed to extract pairwise rules for $OPERATION" >&2
    exit 2
  fi
  mapfile -t ENFORCED_RULES <<<"$RULES_RAW"
  if [ "${#ENFORCED_RULES[@]}" -eq 1 ] && [ -z "${ENFORCED_RULES[0]}" ]; then
    ENFORCED_RULES=()
  fi

  for rule in "${ENFORCED_RULES[@]}"; do
    RULE_TYPE="$(jq -r '.type' <<<"$rule")"

    # Guard the empty-array case off jq's own length, BEFORE splitting into
    # lines: an empty `paths` must not run the rule against the body root
    # (schema.json enforces minItems:1; this is defense in depth), and a lone
    # empty-string path ("" = body root) must not be mistaken for empty.
    RP_COUNT="$(jq -r '(.paths // []) | if type == "array" then length else "INVALID" end' <<<"$rule")"
    if [ "$RP_COUNT" = "INVALID" ]; then
      echo "ERROR: 'paths' for $RULE_TYPE rule on $OPERATION must be an array of strings" >&2
      exit 2
    fi
    if [ "$RP_COUNT" -eq 0 ]; then
      echo "ERROR: $RULE_TYPE rule on $OPERATION has an empty 'paths' array" >&2
      exit 2
    fi
    # NUL-delimited extraction preserves a trailing empty-string path ("" =
    # body root), which newline splitting would strip. The count check
    # backstops jq failures inside the process substitution (invisible to
    # set -e) and non-string elements.
    mapfile -d '' -t RULE_PATHS < <(jq -j '.paths[] + "\u0000"' <<<"$rule")
    if [ "${#RULE_PATHS[@]}" -ne "$RP_COUNT" ]; then
      echo "ERROR: 'paths' extraction mismatch for $RULE_TYPE rule on $OPERATION (expected $RP_COUNT, got ${#RULE_PATHS[@]}; paths must be strings)" >&2
      exit 2
    fi

    for upath in "${RULE_PATHS[@]}"; do
      if is_allowed "$upath"; then
        continue
      fi

      DISPLAY="$(display_path "$upath")"
      BC4_VAL="$(read_value "$BC4_SNAPSHOT" "$upath")"
      BC5_VAL="$(read_value "$BC5_SNAPSHOT" "$upath")"

      case "$RULE_TYPE" in
        pairwiseSupersetArray)
          # null at a path means "field absent on this backend". A BC4 array
          # of N items vs BC5 null is a regression; treat null as length 0
          # only when there's nothing on either side.
          #
          # Aggregated `pages[*]` paths wrap one value per page, so comparing
          # the outer length would just compare page counts — a field dropped
          # on every page would false-green. Compare the total item count
          # across pages instead: an absent (null) page value contributes 0,
          # any non-null non-array page value poisons the total as INVALID.
          if [[ "$upath" == *"[*]"* ]]; then
            AGG_LEN='[ .[] | if . == null then 0 elif type == "array" then length else "INVALID" end ]
                     | if any(.[]; . == "INVALID") then "INVALID" else (add // 0) end'
            BC4_LEN="$(jq -r "$AGG_LEN" <<<"$BC4_VAL")"
            BC5_LEN="$(jq -r "$AGG_LEN" <<<"$BC5_VAL")"
          else
            BC4_LEN="$(jq -r 'if type == "array" then length else (if . == null then 0 else "INVALID" end) end' <<<"$BC4_VAL")"
            BC5_LEN="$(jq -r 'if type == "array" then length else (if . == null then 0 else "INVALID" end) end' <<<"$BC5_VAL")"
          fi

          if [ "$BC4_LEN" = "INVALID" ] || [ "$BC5_LEN" = "INVALID" ]; then
            violation "$OPERATION  pairwiseSupersetArray($DISPLAY): expected arrays on both sides; BC4=$BC4_VAL BC5=$BC5_VAL"
          elif [ "$BC5_LEN" -lt "$BC4_LEN" ]; then
            violation "$OPERATION  pairwiseSupersetArray($DISPLAY): BC5 length $BC5_LEN < BC4 length $BC4_LEN"
          fi
          ;;

        pairwiseSupersetKeys)
          # Missing (null) on either side counts as the empty key set. Any
          # other non-object is an invalid target for a keys rule — flag it
          # instead of silently skipping, so a mis-specified path or a real
          # shape change (object → array/scalar) can't hide.
          BC4_KIND="$(jq -r 'if type == "object" then "object" elif . == null then "null" else "INVALID" end' <<<"$BC4_VAL")"
          BC5_KIND="$(jq -r 'if type == "object" then "object" elif . == null then "null" else "INVALID" end' <<<"$BC5_VAL")"

          if [ "$BC4_KIND" = "INVALID" ] || [ "$BC5_KIND" = "INVALID" ]; then
            violation "$OPERATION  pairwiseSupersetKeys($DISPLAY): expected objects (or null for absent) on both sides; BC4=$BC4_VAL BC5=$BC5_VAL"
          else
            BC4_OBJ="$BC4_VAL"
            BC5_OBJ="$BC5_VAL"
            if [ "$BC4_KIND" = "null" ]; then BC4_OBJ="{}"; fi
            if [ "$BC5_KIND" = "null" ]; then BC5_OBJ="{}"; fi
            MISSING="$(jq -r --argjson bc5 "$BC5_OBJ" '
              keys
              | map(select(. as $k | ($bc5 | has($k)) | not))
              | join(",")
            ' <<<"$BC4_OBJ")"
            if [ -n "$MISSING" ]; then
              violation "$OPERATION  pairwiseSupersetKeys($DISPLAY): BC5 missing keys present in BC4: $MISSING"
            fi
          fi
          ;;

        pairwiseEqual)
          # Compare semantically: jq deep-equality is independent of object
          # key order, so two snapshots that serialize the same object with
          # different key order don't false-fail.
          if [ "$(jq -n --argjson a "$BC4_VAL" --argjson b "$BC5_VAL" '$a == $b')" != "true" ]; then
            violation "$OPERATION  pairwiseEqual($DISPLAY): BC4=$BC4_VAL BC5=$BC5_VAL"
          fi
          ;;

        *)
          echo "ERROR: unknown pairwise rule type '$RULE_TYPE' on $OPERATION — schema validation should have caught this" >&2
          exit 2
          ;;
      esac
    done
  done
done

echo "==> Pairwise canary: compared $COMPARED operation(s)"

if [ "$MISSING_SNAPSHOTS" -gt 0 ]; then
  echo "" >&2
  echo "ERROR: $MISSING_SNAPSHOTS pairwise test(s) were missing a wire snapshot on" >&2
  echo "one or both backends (listed above). The TS live runner must capture every" >&2
  echo "pairwise-bearing test for both BC4 and BC5 before comparison; a missing" >&2
  echo "snapshot is a hard error so check-bc5-compat can't report a false green" >&2
  echo "without evaluating the declared rule." >&2
  if [ -n "$VIOLATIONS" ]; then
    echo "" >&2
    echo "Pairwise violations were also found in the snapshots that were present:" >&2
    printf '%s' "$VIOLATIONS" >&2
  fi
  exit 2
fi

if [ -n "$VIOLATIONS" ]; then
  echo "" >&2
  echo "Additive-only invariant violated:" >&2
  printf '%s' "$VIOLATIONS" >&2
  echo "" >&2
  echo "If a divergence is intentional, add a pairwiseDeltaAllowed entry on" >&2
  echo "the operation in live-my-surface.json with a reason." >&2
  exit 1
fi

echo "Pairwise canary clean"
