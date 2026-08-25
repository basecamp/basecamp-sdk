#!/usr/bin/env bash
# Self-test for scripts/promote-migrating.sh. Each case builds a scratch file,
# runs the script, and asserts exit code plus resulting state. The backward-
# bump cases are the point: the first version of this script matched "# vX"
# anywhere, so a historical heading waved a rolled-back bump through.
set -u
SCRIPT="$(dirname "$0")/promote-migrating.sh"
# Pin the current-version authority so the repo's real constant cannot leak
# into scratch-file cases; individual cases override it to probe the gate.
# Pin the shipped-releases authority: the suite must not depend on the
# checkout's real tag state (CI checkouts are shallow and tagless).
export PROMOTE_MIGRATING_RELEASED="0.14.0 0.15.0"
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

# 10. --check accepts no-notes forward past a RELEASED heading (v0.15.0 has a
#     real tag), refuses backward
fresh f2.md "# v0.15.0" "# v0.14.0"
bash "$SCRIPT" --check 0.15.1 "$DIR/f2.md" >/dev/null 2>&1; check "--check accepts no-notes forward" 0 $?
bash "$SCRIPT" --check 0.14.0 "$DIR/f2.md" >/dev/null 2>&1; check "--check refuses backward release" 1 $?

# 11. a code-block heading below the newest section does not confuse the
#     first-heading scan (the guide quotes old headings in fences)
{ echo "# Migrating"; echo; echo "# v0.16.0"; echo; echo '```'; echo "# v0.12.0"; echo '```'; } > "$DIR/g.md"
bash "$SCRIPT" --check 0.16.0 "$DIR/g.md" >/dev/null 2>&1; check "quoted old heading below top ignored" 0 $?

# 12. promote refuses a target older than the newest released section
fresh h.md "# Unreleased" "# v0.15.0"
bash "$SCRIPT" 0.9.0 "$DIR/h.md" >/dev/null 2>&1; check "promote refuses backward target" 1 $?

# 13. a two-digit component beats a one-digit one (component-wise, not lexicographic)
fresh i.md "# Unreleased" "# v0.9.0"
PROMOTE_MIGRATING_RELEASED="0.9.0" bash "$SCRIPT" 0.10.0 "$DIR/i.md" >/dev/null 2>&1; check "0.10.0 is newer than 0.9.0" 0 $?
grep -qxF "# v0.10.0" "$DIR/i.md"; check "0.10.0 promoted" 0 $?

# 14. a misplaced "# Unreleased" below the promoted top is refused, both branches
fresh j.md "# v0.16.0" "# Unreleased" "# v0.15.0"
bash "$SCRIPT" 0.16.0 "$DIR/j.md" >/dev/null 2>&1; check "idempotent branch refuses misplaced Unreleased" 1 $?
bash "$SCRIPT" --check 0.16.1 "$DIR/j.md" >/dev/null 2>&1; check "no-notes branch refuses misplaced Unreleased" 1 $?

# 15. the shipped tags, not the headings or the constants, are the rollback
#     authority: after a no-notes 0.16.0 release the newest heading lags, and
#     a bump or a hand-edited --check between them must still refuse
fresh k.md "# v0.15.0" "# v0.14.0"
PROMOTE_MIGRATING_RELEASED="0.15.0 0.16.0" bash "$SCRIPT" 0.15.5 "$DIR/k.md" >/dev/null 2>&1
check "no-notes bump below the newest shipped release refused" 1 $?
fresh l.md "# Unreleased" "# v0.15.0"
PROMOTE_MIGRATING_RELEASED="0.15.0 0.16.0" bash "$SCRIPT" 0.15.5 "$DIR/l.md" >/dev/null 2>&1
check "promote below the newest shipped release refused" 1 $?
PROMOTE_MIGRATING_RELEASED="0.15.0 0.16.0" bash "$SCRIPT" --check 0.15.5 "$DIR/k.md" >/dev/null 2>&1
check "hand-edited constants cannot sneak an unshipped rollback past --check" 1 $?
PROMOTE_MIGRATING_RELEASED="0.15.0 0.16.0" bash "$SCRIPT" --check 0.16.5 "$DIR/k.md" >/dev/null 2>&1
check "--check past the newest shipped release accepted" 0 $?

# 16. more than one "# Unreleased" heading is refused before any mutation
fresh m.md "# Unreleased" "# v0.15.0"
printf '# Unreleased\n\nstray\n' >> "$DIR/m.md"
cp "$DIR/m.md" "$DIR/m.before"
bash "$SCRIPT" 0.16.0 "$DIR/m.md" >/dev/null 2>&1; check "duplicate Unreleased refused" 1 $?
diff -q "$DIR/m.md" "$DIR/m.before" >/dev/null; check "duplicate refusal leaves file untouched" 0 $?

# 17. an unobtainable shipped list is fatal, not fail-open: a copy of the
#     script outside the repo has no origin to ask
cp "$SCRIPT" "$DIR/stray-promote.sh"
fresh n.md "# Unreleased" "# v0.15.0"
env -u PROMOTE_MIGRATING_RELEASED bash "$DIR/stray-promote.sh" 0.16.0 "$DIR/n.md" >/dev/null 2>&1
check "unobtainable shipped list is fatal" 1 $?

# 18. promoting NEW notes to the version already shipped is refused: after a
#     no-notes 0.16.0 release, a fresh Unreleased belongs to a later release
fresh o.md "# Unreleased" "# v0.15.0"
PROMOTE_MIGRATING_RELEASED="0.15.0 0.16.0" bash "$SCRIPT" 0.16.0 "$DIR/o.md" >/dev/null 2>&1
check "equal-version promotion of new notes refused" 1 $?

# 19. correcting a bump before commit re-promotes the pending heading (no tag
#     names v0.99.0) instead of stranding its notes — in EITHER direction —
#     and --check refuses the state instead of calling it no-notes
fresh p.md "# v0.99.0" "# v0.15.0"
bash "$SCRIPT" 0.99.1 "$DIR/p.md" >/dev/null 2>&1
check "pending heading re-promoted on corrected bump" 0 $?
grep -qxF "# v0.99.1" "$DIR/p.md" && ! grep -qxF "# v0.99.0" "$DIR/p.md"
check "pending heading renamed" 0 $?
fresh p2.md "# v0.99.0" "# v0.15.0"
bash "$SCRIPT" 0.17.0 "$DIR/p2.md" >/dev/null 2>&1
check "pending heading re-promoted DOWNWARD to a still-forward target" 0 $?
grep -qxF "# v0.17.0" "$DIR/p2.md" && ! grep -qxF "# v0.99.0" "$DIR/p2.md"
check "downward correction renamed the heading" 0 $?
fresh q.md "# v0.99.0" "# v0.15.0"
bash "$SCRIPT" --check 0.99.1 "$DIR/q.md" >/dev/null 2>&1
check "--check refuses a pending unreleased heading" 1 $?

# 20. a malformed target is refused with the format diagnostic
fresh r.md "# Unreleased" "# v0.15.0"
ERR=$(bash "$SCRIPT" dev "$DIR/r.md" 2>&1 >/dev/null)
check "malformed target version is fatal" 1 $?
printf '%s' "$ERR" | grep -q "is not X.Y.Z"; check "refusal names the malformed target" 0 $?

# 21. a checkout that knows no release tags fails closed when release status
#     matters, instead of renaming real history as "pending"
fresh s.md "# v0.15.0" "# v0.14.0"
PROMOTE_MIGRATING_RELEASED= bash "$SCRIPT" 0.15.1 "$DIR/s.md" >/dev/null 2>&1
check "tagless checkout fails closed on the pending question" 1 $?
cp "$DIR/s.md" "$DIR/s.before" 2>/dev/null
PROMOTE_MIGRATING_RELEASED= bash "$SCRIPT" 0.15.1 "$DIR/s.md" >/dev/null 2>&1 || true
diff -q "$DIR/s.md" "$DIR/s.before" >/dev/null; check "tagless refusal leaves the file untouched" 0 $?

# 22. an already-tagged target cannot pass --check, whatever the headings say
fresh t0.md "# v0.15.0" "# v0.14.0"
bash "$SCRIPT" --check 0.15.0 "$DIR/t0.md" >/dev/null 2>&1
check "--check refuses an already-tagged target" 1 $?

# 23. the real remote-authority paths, exercised against a file:// origin —
#     no overrides, so the script's own git ls-remote runs. A tag pushed to
#     the origin is released; a LOCAL-ONLY tag (the residue of a release
#     whose tag push failed) is not; an unreachable origin fails closed.
FIX="$DIR/fixture-repo"; ORIGIN="$DIR/origin.git"
git init -q --bare "$ORIGIN"
git init -q "$FIX" && git -C "$FIX" remote add origin "$ORIGIN"
mkdir -p "$FIX/scripts" "$FIX/go/pkg/basecamp"
cp "$SCRIPT" "$FIX/scripts/promote-migrating.sh"
printf 'package basecamp\n\nconst Version = "0.6.0"\n' > "$FIX/go/pkg/basecamp/version.go"
git -C "$FIX" -c user.email=t@t -c user.name=t commit -q --allow-empty -m fixture
git -C "$FIX" tag v0.5.0 && git -C "$FIX" push -q origin v0.5.0
git -C "$FIX" tag v0.6.0   # local only: never pushed
FSCRIPT="$FIX/scripts/promote-migrating.sh"

fresh t1.md "# v0.5.0" "# v0.4.0"
cp "$DIR/t1.md" "$DIR/t1.before"
env -u PROMOTE_MIGRATING_RELEASED bash "$FSCRIPT" 0.6.1 "$DIR/t1.md" >/dev/null 2>&1
check "remote-present tag reads as released (real ls-remote)" 0 $?
diff -q "$DIR/t1.md" "$DIR/t1.before" >/dev/null
check "released heading took the no-notes path without mutation" 0 $?

fresh t2.md "# v0.6.0" "# v0.5.0"
env -u PROMOTE_MIGRATING_RELEASED bash "$FSCRIPT" 0.6.1 "$DIR/t2.md" >/dev/null 2>&1
check "local-only tag reads as pending (re-promoted)" 0 $?
grep -qxF "# v0.6.1" "$DIR/t2.md"; check "failed-push residue heading carried forward" 0 $?

fresh t3.md "# v0.5.0" "# v0.4.0"
env -u PROMOTE_MIGRATING_RELEASED bash "$FSCRIPT" --check 0.5.0 "$DIR/t3.md" >/dev/null 2>&1
check "real remote refuses an already-tagged --check target" 1 $?

git -C "$FIX" remote set-url origin "$DIR/no-such-origin.git"
fresh t4.md "# v0.5.0" "# v0.4.0"
env -u PROMOTE_MIGRATING_RELEASED bash "$FSCRIPT" 0.6.1 "$DIR/t4.md" >/dev/null 2>&1
check "unreachable remote fails closed" 1 $?

# 24. fenced examples are prose to a reader and NOTHING to the heading
#     judgments: a code block quoting "# Unreleased" is not a duplicate, and
#     one quoting the target heading is not a rollback
{ echo "# Migrating"; echo; echo "# Unreleased"; echo; echo '```'; echo "# Unreleased"; echo "# v0.16.0"; echo '```'; echo; echo "# v0.15.0"; } > "$DIR/u.md"
bash "$SCRIPT" 0.16.0 "$DIR/u.md" >/dev/null 2>&1; check "fenced headings are invisible to promotion" 0 $?
grep -qxF "# v0.16.0" "$DIR/u.md"; check "promotion landed despite fenced examples" 0 $?

# 25. a guide large enough that grep's early exit outruns the prose pass:
#     under pipefail a piped judgment would report the producer's SIGPIPE as
#     no-match and wave a rollback through; the captured-prose form must not
{ echo "# Migrating"; echo; echo "# Unreleased"; echo; echo "# v0.16.0"; echo; for i in $(seq 1 8000); do echo "filler prose line $i to outlast the pipe buffer after the early match"; done; } > "$DIR/v.md"
PROMOTE_MIGRATING_RELEASED="0.15.0" bash "$SCRIPT" 0.16.0 "$DIR/v.md" >/dev/null 2>&1
check "duplicate target still refused on a large guide" 1 $?

# 26. an abandoned unshipped section below a fresh Unreleased is refused
#     before promotion can orphan its notes forever
fresh w.md "# Unreleased" "# v0.16.0" "# v0.15.0"
PROMOTE_MIGRATING_RELEASED="0.15.0" bash "$SCRIPT" 0.17.0 "$DIR/w.md" >/dev/null 2>&1
check "abandoned unshipped section refused" 1 $?

# 27. the rewrite passes skip fences too: a fenced "# Unreleased" BEFORE the
#     real section must survive promotion untouched
{ echo "# Migrating"; echo; echo '```'; echo "# Unreleased"; echo '```'; echo; echo "# Unreleased"; echo; echo "# v0.15.0"; } > "$DIR/x.md"
bash "$SCRIPT" 0.16.0 "$DIR/x.md" >/dev/null 2>&1
check "promotion with a leading fenced example succeeds" 0 $?
FENCED_KEPT=$(awk '/^```/{f=!f; next} f && $0 == "# Unreleased"' "$DIR/x.md" | wc -l | tr -d ' ')
[ "$FENCED_KEPT" = "1" ]; check "the fenced example survived; the real heading promoted" 0 $?
grep -qxF "# v0.16.0" "$DIR/x.md"; check "real heading became the version" 0 $?

# 28. tilde and indented fences are fences too
{ echo "# Migrating"; echo; echo "~~~"; echo "# Unreleased"; echo "~~~"; echo; echo "   \`\`\`"; echo "# v0.16.0"; echo "   \`\`\`"; echo; echo "# Unreleased"; echo; echo "# v0.15.0"; } > "$DIR/y.md"
bash "$SCRIPT" 0.16.0 "$DIR/y.md" >/dev/null 2>&1
check "tilde and indented fences are invisible to the judgments" 0 $?
grep -qxF "# v0.16.0" "$DIR/y.md"; check "promotion landed past exotic fences" 0 $?

if [ "$FAILS" -gt 0 ]; then
  echo "test-promote-migrating: $FAILS of $CASES assertions failed" >&2
  exit 1
fi
echo "test-promote-migrating: passed — $CASES assertions."
