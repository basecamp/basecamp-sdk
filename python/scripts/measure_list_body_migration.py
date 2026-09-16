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


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", default="origin/main", help="git ref for the BEFORE column")
    parser.add_argument("--head", default="HEAD", help="git ref for the AFTER column")
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

        print(f"before `{base_sha}` -> after `{head_sha}`. Sync operation counts, from the AST: "
              f"{counts['paginated']} paginated, {counts['unpaginated']} unpaginated, "
              f"{counts['wrapped']} wrapped.\n")
        for family, bodies in (("paginated", BARE_ARRAY_BODIES), ("unpaginated", BARE_ARRAY_BODIES)):
            print(f"## {family}\n")
            print(table(before_root, after_root, family, bodies))
            print()
        print("## wrapped\n")
        print(table(before_root, after_root, "wrapped", ENVELOPE_BODIES))


if __name__ == "__main__":
    main()
