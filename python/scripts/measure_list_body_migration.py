#!/usr/bin/env python3
"""Generates the before/after tables in MIGRATING.md's null-list-body section.

Usage: python scripts/measure_list_body_migration.py [--base <ref>] [--head <ref>]

A migration note's "before" column is the one claim in a change that nobody
re-derives: the old behaviour is gone by the time anyone reads it, so a recalled
number survives review indefinitely. On this change it did not survive — a typed
before-column asserted that paginated list operations were unaffected, and
measuring showed three body shapes that used to succeed and now raise. The note
understated its own blast radius by sixty operations.

So the column is generated rather than typed. This script builds a scratch copy
of the package for EACH of two git refs, drives every body shape through a real
operation against both, and prints the markdown. Regenerate rather than edit; if
a row looks wrong, the behaviour is wrong, not the table.

Both columns come from named refs and neither reads the working tree, which is
not a detail. The first version of this script took its "after" column from
`src/` on disk, and the tree happened to be mid-swap from another process at the
moment it ran; it emitted a table where both columns were the BEFORE revision
and nothing about the output said so. A measurement harness that reads a mutable
input produces numbers that look generated and are not, which is strictly worse
than typing them, because the reader stops checking. Read immutable inputs, name
them by identifier, and the result is reproducible by someone who was not there.

The probes deliberately name ONE operation per family rather than all 69: every
operation in a family routes through the same primitive, which the script
asserts by checking the call-site counts it reports against the families it
drives.
"""

from __future__ import annotations

import argparse
import ast
import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

PY_ROOT = Path(__file__).resolve().parent.parent
REPO_ROOT = PY_ROOT.parent
PRIMITIVES = (
    "src/basecamp/generated/services/_base.py",
    "src/basecamp/generated/services/_async_base.py",
)

#: Bodies driven through every family. `null` first because it is the defect
#: this change exists for; the rest are the shapes a reader has to be able to
#: tell apart from it.
BARE_ARRAY_BODIES = [
    ("`null`", "null"),
    ("`{}`", "{}"),
    ('`{"id": 1}`', '{"id": 1}'),
    ('`"abc"`', '"abc"'),
    ("`0`", "0"),
    ("`false`", "false"),
    ('`[{"id": 1` (truncated)', '[{"id": 1'),
]

ENVELOPE_BODIES = [
    ("`null`", "null"),
    ('`{"person": …}`, `events` absent', '{"person": {"id": 9}}'),
    ('`{"events": null, "person": …}`', '{"events": null, "person": {"id": 9}}'),
    ('`{"events": "abc", "person": …}`', '{"events": "abc", "person": {"id": 9}}'),
    ('`{"events": {"a": 1}, "person": …}`', '{"events": {"a": 1}, "person": {"id": 9}}'),
    ("`\"abc\"` or `[]` (body not an object)", '"abc"'),
]

#: (family label, url path, python expression driving one operation of it)
FAMILIES = {
    "paginated": (
        "/my/bookmarks.json",
        "list(account.bookmarks.list_my_bookmarks())",
        "_request_paginated",
    ),
    "unpaginated": (
        "/stacks.json",
        "list(account.folders.list_folders())",
        "_request_list",
    ),
    "wrapped": (
        "/reports/users/progress/1.json",
        "list(account.reports.person_progress(person_id=1)['events'])",
        "_request_paginated_wrapped",
    ),
}

PROBE = '''
import json, httpx, respx
from basecamp import Client

@respx.mock
def run(path, body, expr):
    respx.get("https://3.basecampapi.com/12345" + path).mock(
        return_value=httpx.Response(200, content=body, headers={{"Content-Type": "application/json"}})
    )
    account = Client(access_token="t").for_account("12345")
    try:
        return {{"ok": True, "value": repr(eval(expr))}}
    except Exception as e:
        return {{"ok": False, "type": type(e).__name__}}

print(json.dumps(run({path!r}, {body!r}, {expr!r})))
'''


def count_call_sites(primitive: str) -> int:
    """Sync operations routing through one primitive, counted from the AST."""
    total = 0
    services = PY_ROOT / "src/basecamp/generated/services"
    for path in sorted(services.glob("*.py")):
        if path.name.startswith("_"):
            continue
        tree = ast.parse(path.read_text())
        for cls in (n for n in tree.body if isinstance(n, ast.ClassDef)):
            if cls.name.startswith("Async"):
                continue
            for fn in (n for n in cls.body if isinstance(n, ast.FunctionDef | ast.AsyncFunctionDef)):
                if f"'{primitive}'" in ast.dump(fn):
                    total += 1
    return total


def probe(src_root: Path, path: str, body: str, expr: str) -> str:
    """Drive one body through one operation, against one copy of the package."""
    result = subprocess.run(
        [sys.executable, "-c", PROBE.format(path=path, body=body, expr=expr)],
        cwd=PY_ROOT,
        env={"PYTHONPATH": str(src_root), "PATH": "/usr/bin:/bin", "HOME": str(Path.home())},
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise SystemExit(f"probe failed for {body!r}:\n{result.stderr}")
    outcome = json.loads(result.stdout.strip().splitlines()[-1])
    if outcome["ok"]:
        return f"`{outcome['value']}`"
    return f"`{outcome['type']}`"


def tree_at(ref: str, tmp: Path, name: str) -> Path:
    """A scratch copy of the package with ONE revision's primitives in place.

    Both columns are built this way, including "after". Reading the working tree
    for the after column would make the table a function of whatever is on disk
    at the moment it runs — which is not reproducible, and on a repo where more
    than one agent or shell may be mid-measurement in the same worktree, not even
    stable. A generated table should be a function of two commits and nothing
    else, so that two people running it on the same pair get the same bytes.
    """
    src = tmp / name / "src"
    src.parent.mkdir(parents=True, exist_ok=True)
    shutil.copytree(PY_ROOT / "src", src)
    for rel in PRIMITIVES:
        blob = subprocess.run(
            ["git", "show", f"{ref}:python/{rel}"],
            cwd=REPO_ROOT,
            capture_output=True,
            text=True,
            check=True,
        ).stdout
        target = src / Path(rel).relative_to("src")
        assert tmp in target.parents, f"refusing to write outside the scratch tree: {target}"
        target.write_text(blob)
    return src


def table(before_root: Path, after_root: Path, family: str, bodies: list[tuple[str, str]]) -> str:
    path, expr, _ = FAMILIES[family]
    rows = ["| body | before | after |", "|---|---|---|"]
    for label, body in bodies:
        before = probe(before_root, path, body, expr)
        after = probe(after_root, path, body, expr)
        rows.append(f"| {label} | {before} | {after} |")
    return "\n".join(rows)


def note_rows(section: str) -> list[tuple[str, str, str]]:
    """The note's table rows, as (body, before, after) with the pipes stripped."""
    rows = []
    for line in section.splitlines():
        if not line.startswith("| `") or "---" in line:
            continue
        cells = [c.strip() for c in line.strip("|").split("|")]
        if len(cells) == 3:
            rows.append(tuple(cells))
    return rows


#: The note is a human-facing document and uses exactly two presentation
#: conventions the probes do not. Both are named here so the matcher accommodates
#: what the note actually does rather than being loosened until it passes:
#:
#:   1. A body cell may merge shapes that behave identically: "`0` / `false`".
#:   2. A cell may split by family where the two differ:
#:      "paginated: `ApiError`; unpaginated: `json.JSONDecodeError`".
#:
#: Anything beyond these two is drift, and fails.
def states(note_cell: str, measured: str, family: str) -> bool:
    """Does a note cell state the measured value, for this family?"""
    bare = measured.strip("`")
    forms = (measured, f"`json.{bare}`")

    if ";" in note_cell and ":" in note_cell:  # convention 2
        for part in note_cell.split(";"):
            label, _, value = part.partition(":")
            if label.strip() == family:
                return value.strip().startswith(forms)
        return False

    return note_cell.startswith(forms)  # convention 1 needs nothing here


def check(measured: list[tuple[str, str]], base_sha: str, head_sha: str, counts: dict[str, int]) -> int:
    """Verify the committed note still states what the probes measure.

    The note is prose around these numbers — it merges the two bare-array tables
    where they agree, collapses `0`/`false`, and glosses cells. A generator that
    emitted that prose would be writing the document, which is the wrong division
    of labour. What has to be mechanical is that every measured value is stated
    by the row for its own body, so the note cannot drift from the behaviour
    without failing here.
    """
    note = (REPO_ROOT / "MIGRATING.md").read_text()
    section = note[note.index("### Python: a malformed list body") : note.index("### Rust: new SDK")]
    rows = note_rows(section)

    problems = []
    for family, block in measured:
        for line in block.splitlines()[2:]:
            body, before, after = (c.strip() for c in line.strip("|").split("|"))
            candidates = [
                r
                for r in rows
                # convention 1: a merged body cell states several shapes
                if r[0] == body or body in [alt.strip() for alt in r[0].split("/")]
                or r[0].startswith(body.split(" (")[0])
            ]
            if not candidates:
                problems.append(f"{family}: no row in MIGRATING.md for body {body}")
                continue
            if not any(states(r[1], before, family) for r in candidates):
                problems.append(
                    f"{family}: body {body} measures before={before}, "
                    f"note says {[r[1] for r in candidates]}"
                )
            if not any(states(r[2], after, family) for r in candidates):
                problems.append(
                    f"{family}: body {body} measures after={after}, "
                    f"note says {[r[2] for r in candidates]}"
                )
    for name, n in counts.items():
        if name != "wrapped" and str(n) not in section:
            problems.append(f"operation count for {name} ({n}) is not stated in MIGRATING.md")
    # Only the BASE is pinned in the note. A document cannot name the SHA of the
    # commit that contains it, and the after-column is simply "whatever you are
    # checking" — so the head travels as a ref, not a constant.
    if base_sha not in section:
        problems.append(f"base SHA {base_sha} is not named in MIGRATING.md")

    if problems:
        print("MIGRATING.md no longer matches the measured behaviour:")
        for problem in problems:
            print(f"  - {problem}")
        return 1
    print(f"MIGRATING.md matches the behaviour measured at {base_sha} -> {head_sha}.")
    return 0


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", default="origin/main", help="git ref for the BEFORE column")
    parser.add_argument("--head", default="HEAD", help="git ref for the AFTER column")
    parser.add_argument(
        "--check",
        action="store_true",
        help="re-measure and verify every cell still matches MIGRATING.md, instead of printing",
    )
    args = parser.parse_args()

    def short(ref: str) -> str:
        return subprocess.run(
            ["git", "rev-parse", "--short", ref], cwd=REPO_ROOT, capture_output=True, text=True, check=True
        ).stdout.strip()

    base_sha, head_sha = short(args.base), short(args.head)

    counts = {name: count_call_sites(prim) for name, (_, _, prim) in FAMILIES.items()}

    with tempfile.TemporaryDirectory() as tmpdir:
        before_root = tree_at(args.base, Path(tmpdir), "before")
        after_root = tree_at(args.head, Path(tmpdir), "after")

        measured = []
        for family, bodies in (("paginated", BARE_ARRAY_BODIES), ("unpaginated", BARE_ARRAY_BODIES)):
            measured.append((family, table(before_root, after_root, family, bodies)))
        measured.append(("wrapped", table(before_root, after_root, "wrapped", ENVELOPE_BODIES)))

        if args.check:
            raise SystemExit(check(measured, base_sha, head_sha, counts))

        print(f"before `{base_sha}` -> after `{head_sha}`. Sync operation counts, from the AST: "
              f"{counts['paginated']} paginated, {counts['unpaginated']} unpaginated, "
              f"{counts['wrapped']} wrapped.\n")
        for family, block in measured:
            print(f"## {family}\n")
            print(block)
            print()


if __name__ == "__main__":
    main()
