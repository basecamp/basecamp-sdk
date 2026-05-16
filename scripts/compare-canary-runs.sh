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
#                          Catches "memories went to []".
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
# 2  operator error: missing directory, missing test fixture, jq unavailable, etc.
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

# Read a JSON value at a normalized jq path from a snapshot file.
# Streams from pages[*] are wrapped into an array so the caller can treat
# them as a single aggregated value.
read_value() {
  local snapshot="$1"
  local user_path="$2"
  local jq_path
  jq_path="$(to_jq_path "$user_path")"

  if [[ "$user_path" == *"[*]"* ]]; then
    jq -c "[ $jq_path ]" "$snapshot"
  else
    jq -c "$jq_path" "$snapshot"
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
mapfile -t TEST_ENTRIES < <(
  jq -c '.[] | select((.pairwiseAssertions // []) | length > 0)' "$TESTS_FILE"
)

if [ "${#TEST_ENTRIES[@]}" -eq 0 ]; then
  echo "==> Pairwise canary: no tests carry pairwiseAssertions; nothing to compare"
  exit 0
fi

COMPARED=0
SKIPPED_MISSING=0
FILE_MISSING_DETAIL=""

for entry in "${TEST_ENTRIES[@]}"; do
  NAME="$(jq -r '.name' <<<"$entry")"
  OPERATION="$(jq -r '.operation' <<<"$entry")"

  # Snapshot filenames in the TS live runner are the operation name (per
  # conformance/runner/typescript/wire-capture.ts safeTestName). Look for
  # both `<operation>.json` and the historical `<name>.json` form so callers
  # don't have to pre-rename anything if conventions shift.
  BC4_SNAPSHOT=""
  BC5_SNAPSHOT=""
  for candidate in "$OPERATION" "$NAME"; do
    safe="$(echo "$candidate" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9._-]/-/g')"
    [ -f "$BC4_DIR/$safe.json" ] && BC4_SNAPSHOT="$BC4_DIR/$safe.json"
    [ -f "$BC5_DIR/$safe.json" ] && BC5_SNAPSHOT="$BC5_DIR/$safe.json"
  done

  if [ -z "$BC4_SNAPSHOT" ] || [ -z "$BC5_SNAPSHOT" ]; then
    SKIPPED_MISSING=$((SKIPPED_MISSING + 1))
    FILE_MISSING_DETAIL="${FILE_MISSING_DETAIL}  $OPERATION
"
    continue
  fi

  COMPARED=$((COMPARED + 1))

  # Allowlisted paths for this operation (skipped by the other rule types).
  mapfile -t ALLOW_PATHS < <(
    jq -r '
      (.pairwiseAssertions // [])
      | map(select(.type == "pairwiseDeltaAllowed"))
      | map(.paths // [])
      | flatten
      | .[]
    ' <<<"$entry"
  )
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
  mapfile -t ENFORCED_RULES < <(
    jq -c '
      (.pairwiseAssertions // [])
      | map(select(.type != "pairwiseDeltaAllowed"))
      | .[]
    ' <<<"$entry"
  )

  for rule in "${ENFORCED_RULES[@]}"; do
    RULE_TYPE="$(jq -r '.type' <<<"$rule")"
    mapfile -t RULE_PATHS < <(jq -r '.paths[]' <<<"$rule")

    for upath in "${RULE_PATHS[@]:-}"; do
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
          BC4_LEN="$(jq -r 'if type == "array" then length else (if . == null then 0 else "INVALID" end) end' <<<"$BC4_VAL")"
          BC5_LEN="$(jq -r 'if type == "array" then length else (if . == null then 0 else "INVALID" end) end' <<<"$BC5_VAL")"

          if [ "$BC4_LEN" = "INVALID" ] || [ "$BC5_LEN" = "INVALID" ]; then
            violation "$OPERATION  pairwiseSupersetArray($DISPLAY): expected arrays on both sides; BC4=$BC4_VAL BC5=$BC5_VAL"
          elif [ "$BC5_LEN" -lt "$BC4_LEN" ]; then
            violation "$OPERATION  pairwiseSupersetArray($DISPLAY): BC5 length $BC5_LEN < BC4 length $BC4_LEN"
          fi
          ;;

        pairwiseSupersetKeys)
          # Missing on either side counts as empty set.
          BC4_TYPE="$(jq -r 'type' <<<"$BC4_VAL")"
          BC5_TYPE="$(jq -r 'type' <<<"$BC5_VAL")"

          if [ "$BC4_TYPE" = "object" ] && [ "$BC5_TYPE" != "object" ]; then
            violation "$OPERATION  pairwiseSupersetKeys($DISPLAY): BC4 is object but BC5 is $BC5_TYPE"
          elif [ "$BC4_TYPE" = "object" ] && [ "$BC5_TYPE" = "object" ]; then
            MISSING="$(jq -r --argjson bc5 "$BC5_VAL" '
              keys
              | map(select(. as $k | ($bc5 | has($k)) | not))
              | join(",")
            ' <<<"$BC4_VAL")"
            if [ -n "$MISSING" ]; then
              violation "$OPERATION  pairwiseSupersetKeys($DISPLAY): BC5 missing keys present in BC4: $MISSING"
            fi
          fi
          ;;

        pairwiseEqual)
          if [ "$BC4_VAL" != "$BC5_VAL" ]; then
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
if [ "$SKIPPED_MISSING" -gt 0 ]; then
  echo "    Skipped $SKIPPED_MISSING operation(s) due to missing snapshot files:"
  printf '%s' "$FILE_MISSING_DETAIL"
  echo "    (the TS live runner skips tests that can't resolve a fixture ID — confirm the corresponding snapshot exists for both backends)"
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
