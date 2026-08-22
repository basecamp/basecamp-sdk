#!/usr/bin/env bash
# Self-test for scripts/promote-migrating.sh. Each case builds a scratch file,
# runs the script, and asserts exit code plus resulting state. The backward-
# bump cases are the point: the first version of this script matched "# vX"
# anywhere, so a historical heading waved a rolled-back bump through.
set -u
SCRIPT="$(dirname "$0")/promote-migrating.sh"
DIR=$(mktemp -d)
trap 'rm -rf "$DIR"' EXIT
FAILS=0
CASES=0

check() { # check <desc> <want_exit> <got_exit>
  CASES=$((CASES + 1))
  if [ "$2" != "$3" ]; then
    echo "FAIL: $1 (want exit $2, got $3)" >&2
    FAILS=$((FAILS + 1))
  fi
}

fresh() { # fresh <file> <headings...>
  local f="$DIR/$1"; shift
  { echo "# Migrating"; echo; for h in "$@"; do echo "$h"; echo; echo "body"; echo; done; } > "$f"
}

# 1. promote: Unreleased at top becomes the version heading
fresh a.md "# Unreleased" "# v0.15.0"
bash "$SCRIPT" 0.16.0 "$DIR/a.md" >/dev/null 2>&1; check "promote happy path" 0 $?
grep -qxF "# v0.16.0" "$DIR/a.md" && ! grep -qxF "# Unreleased" "$DIR/a.md"
check "heading replaced" 0 $?

# 2. idempotent second run, file unchanged
cp "$DIR/a.md" "$DIR/a.before"
bash "$SCRIPT" 0.16.0 "$DIR/a.md" >/dev/null 2>&1; check "idempotent rerun" 0 $?
diff -q "$DIR/a.md" "$DIR/a.before" >/dev/null; check "idempotent leaves file untouched" 0 $?

# 3. THE regression: backward bump to a version whose heading exists historically
fresh b.md "# v0.15.0" "# v0.14.0"
bash "$SCRIPT" 0.14.0 "$DIR/b.md" >/dev/null 2>&1; check "backward bump to historical heading refused" 1 $?

# 4. backward bump refused even without a matching historical heading
bash "$SCRIPT" 0.14.9 "$DIR/b.md" >/dev/null 2>&1; check "backward bump without heading refused" 1 $?

# 5. no-notes forward release accepted, file untouched
cp "$DIR/b.md" "$DIR/b.before"
bash "$SCRIPT" 0.15.1 "$DIR/b.md" >/dev/null 2>&1; check "no-notes forward release accepted" 0 $?
diff -q "$DIR/b.md" "$DIR/b.before" >/dev/null; check "no-notes leaves file untouched" 0 $?

# 6. Unreleased plus an existing target heading is a rollback, refused
fresh c.md "# Unreleased" "# v0.16.0" "# v0.15.0"
bash "$SCRIPT" 0.16.0 "$DIR/c.md" >/dev/null 2>&1; check "duplicate target heading refused" 1 $?

# 7. no release heading at all
printf '# Migrating\n\nprose only\n' > "$DIR/d.md"
bash "$SCRIPT" 0.16.0 "$DIR/d.md" >/dev/null 2>&1; check "headingless file refused" 1 $?

# 8. --check refuses an unpromoted Unreleased
fresh e.md "# Unreleased" "# v0.15.0"
bash "$SCRIPT" --check 0.16.0 "$DIR/e.md" >/dev/null 2>&1; check "--check refuses unpromoted notes" 1 $?

# 9. --check accepts the promoted top heading
fresh f.md "# v0.16.0" "# v0.15.0"
bash "$SCRIPT" --check 0.16.0 "$DIR/f.md" >/dev/null 2>&1; check "--check accepts promoted heading" 0 $?

# 10. --check accepts no-notes forward, refuses backward
bash "$SCRIPT" --check 0.16.1 "$DIR/f.md" >/dev/null 2>&1; check "--check accepts no-notes forward" 0 $?
bash "$SCRIPT" --check 0.15.0 "$DIR/f.md" >/dev/null 2>&1; check "--check refuses backward release" 1 $?

# 11. a code-block heading below the newest section does not confuse the
#     first-heading scan (the guide quotes old headings in fences)
{ echo "# Migrating"; echo; echo "# v0.16.0"; echo; echo '```'; echo "# v0.12.0"; echo '```'; } > "$DIR/g.md"
bash "$SCRIPT" --check 0.16.0 "$DIR/g.md" >/dev/null 2>&1; check "quoted old heading below top ignored" 0 $?

if [ "$FAILS" -gt 0 ]; then
  echo "test-promote-migrating: $FAILS of $CASES assertions failed" >&2
  exit 1
fi
echo "test-promote-migrating: passed — $CASES assertions."
