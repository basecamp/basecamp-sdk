#!/bin/bash
# check-kotlin-generated-drift.sh
#
# Verifies that the committed generated Kotlin artifacts are current by
# regenerating the whole generated/ tree into a temp directory and diffing.
# Non-mutating: the committed tree is never touched, so this detects both
# missing AND extra files and is safe to run concurrently under `make -j`.
#
# This is the regenerate-and-diff sibling of the fast, coverage-only
# check-kotlin-service-drift.sh (operationId-set compare). The whole generated
# tree carries @generated; the hand-written base class lives OUTSIDE it, so a
# plain `diff -rq` needs no exclusions and no timestamp normalization (nothing
# generated embeds a wall-clock stamp).
#
# Exit codes:
#   0 = No drift detected
#   1 = Drift detected
#   2 = Could not run the check: required toolchain missing, or `diff` itself
#       failed (e.g. an unreadable path)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

# The Gradle generator runs on the JVM; fail with a distinct code (like the
# Swift sibling) when the toolchain is absent, rather than surfacing a raw
# gradlew 127. Mirror gradlew's own JAVA_HOME-first resolution (kotlin/gradlew)
# so we don't reject a valid environment where java is reachable only via
# JAVA_HOME and not on PATH.
if [ -n "${JAVA_HOME:-}" ]; then
  if [ ! -x "$JAVA_HOME/bin/java" ] && [ ! -x "$JAVA_HOME/jre/sh/java" ]; then
    echo "ERROR: JAVA_HOME ($JAVA_HOME) contains no executable java for check-kotlin-generated-drift.sh" >&2
    exit 2
  fi
elif ! command -v java >/dev/null 2>&1; then
  echo "ERROR: java is required for check-kotlin-generated-drift.sh (set JAVA_HOME or put java on PATH)" >&2
  exit 2
fi

GENERATED_DIR="$ROOT_DIR/kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated"

# Regenerate into a unique subdir of kotlin/build (gitignored, so this stays
# non-mutating w.r.t. the committed tree). Explicit XXXXXX template so mktemp
# behaves identically under GNU and BSD/macOS. A failure to create the temp
# directory means the check could not run → exit 2 (not the default 1).
BUILD_DIR="$ROOT_DIR/kotlin/build"
mkdir -p "$BUILD_DIR" || { echo "ERROR: could not create $BUILD_DIR — cannot check drift" >&2; exit 2; }
TMP_OUT="$(mktemp -d "$BUILD_DIR/kt-drift-check.XXXXXX")" || { echo "ERROR: could not create a temporary output directory under $BUILD_DIR — cannot check drift" >&2; exit 2; }
# Path RELATIVE to kotlin/ (the generator's working directory below). basename
# is a fixed prefix + [A-Za-z0-9] suffix, so this is always clean.
TMP_REL="build/$(basename "$TMP_OUT")"
GEN_LOG="$TMP_OUT.log"
trap 'rm -rf "$TMP_OUT" "$GEN_LOG"' EXIT

echo "==> Regenerating Kotlin SDK into kotlin/$TMP_REL ..."
# Pass ONLY clean, fixed RELATIVE paths to the generator, exactly like
# `make kt-generate-services`. This sidesteps Gradle's --args string tokenizer
# entirely: because no repo/TMPDIR path characters (spaces, quotes, backslashes)
# ever appear in the argument string, no quoting is needed and no path can be
# mis-split. The generator runs with kotlin/ as its working directory, so
# ../openapi.json, ../behavior-model.json, and build/... all resolve there.
#
# Gradle output goes to a log file (it can be voluminous on first run), printed
# only on failure — same pattern as check-go-generated-drift.sh. A generator/
# Gradle failure means the check could not run → map ANY non-zero exit to 2 (the
# reserved "unable to run" code), not 1 (drift). The `|| gen_status=$?` keeps
# set -e from firing on Gradle's non-zero exit.
gen_status=0
(cd "$ROOT_DIR/kotlin" && \
  ./gradlew --quiet :generator:run \
    --args="--openapi ../openapi.json --behavior ../behavior-model.json --output $TMP_REL") > "$GEN_LOG" 2>&1 || gen_status=$?
if [ "$gen_status" -ne 0 ]; then
  echo "ERROR: Kotlin generator failed (gradle exit $gen_status) — cannot check drift:" >&2
  # Only read the log if it exists and is non-empty: if the redirection itself
  # failed (e.g. a full disk), an unconditional cat would fail under set -e and
  # abort with status 1 before the reserved exit 2 is reached.
  if [ -s "$GEN_LOG" ]; then
    cat "$GEN_LOG" >&2
  fi
  exit 2
fi

echo "==> Diffing against committed kotlin/.../generated/ ..."
# A missing committed tree (accidental delete/rename) is drift, not a tool
# error: `diff` would exit >=2 on the missing operand, which the branch below
# reserves for genuine I/O failures. Handle it explicitly as drift (exit 1).
if [ ! -d "$GENERATED_DIR" ]; then
  echo "ERROR: committed generated tree is missing: $GENERATED_DIR"
  echo "       Run 'make kt-generate-services' to regenerate it."
  exit 1
fi

# diff exits 0 = identical, 1 = differences (drift), >=2 = trouble (e.g. an
# unreadable path mid-comparison). Distinguish them so a diff failure is not
# misreported as drift and maps to the reserved toolchain/error code 2. The
# `|| diff_status=$?` keeps set -e from firing on diff's non-zero exit.
diff_status=0
diff_output=$(diff -rq "$GENERATED_DIR" "$TMP_OUT" 2>&1) || diff_status=$?
if [ "$diff_status" -eq 0 ]; then
  echo "No drift detected."
  exit 0
elif [ "$diff_status" -eq 1 ]; then
  echo "ERROR: Generated Kotlin is out of date. Run 'make kt-generate-services', then"
  echo "       delete any path the diff below reports as 'Only in $GENERATED_DIR':"
  echo "       the generator rewrites the tree but does not prune unexpected extra files."
  echo "$diff_output"
  exit 1
else
  echo "ERROR: diff failed (exit $diff_status) while comparing generated Kotlin trees:" >&2
  echo "$diff_output" >&2
  exit 2
fi
