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
# Explicit template so this works identically under GNU and BSD/macOS mktemp.
TMP_OUT="$(mktemp -d "${TMPDIR:-/tmp}/kotlin-generated-drift.XXXXXX")"
# Canonicalize to an absolute path: a relative TMPDIR (e.g. TMPDIR=tmp) yields a
# relative TMP_OUT, and the generator runs from kotlin/ (below), which would
# resolve it against the wrong directory and falsely report drift.
TMP_OUT="$(cd "$TMP_OUT" && pwd)"
trap 'rm -rf "$TMP_OUT"' EXIT

echo "==> Regenerating Kotlin SDK into a temp directory..."
# Mirror `make kt-generate-services` but emit to an absolute temp path so the
# committed tree is untouched. The Gradle generator accepts absolute paths for
# --openapi/--behavior/--output.
#
# Gradle re-tokenizes the --args string on whitespace, honoring quotes, so each
# path is wrapped in double quotes: without it a repo or TMPDIR path containing a
# space would be split into multiple tokens and the generator (which consumes
# only the token immediately after each flag) would read/write the wrong path.
# Double (not single) quotes so a path containing an apostrophe also survives;
# the shell still expands the variables inside the outer double-quoted --args.
(cd "$ROOT_DIR/kotlin" && \
  ./gradlew --quiet :generator:run \
    --args="--openapi \"$ROOT_DIR/openapi.json\" --behavior \"$ROOT_DIR/behavior-model.json\" --output \"$TMP_OUT\"") > /dev/null

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
