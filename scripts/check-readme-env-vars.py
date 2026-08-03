#!/usr/bin/env python3
"""Keep README environment-variable tables honest against SDK source.

READMEs have repeatedly claimed environment variables that no SDK reads
(a phantom `BASECAMP_ACCOUNT_ID` row), or attributed a real variable to the
wrong SDK (`XDG_CACHE_HOME` credited to Ruby, which never reads it). Both are
invisible to every other gate in `make check`, because nothing here is
generated -- the prose is hand-written and drifts silently.

This checks four invariants:

  1. Forward, per SDK: every variable named in an SDK README's env-var table is
     genuinely read by that SDK's source.
  2. Forward, root README: the "Read by" column names SDKs, and each one must
     genuinely read that variable.
  3. Reverse, per SDK: every variable an SDK genuinely reads is named somewhere
     in that SDK's README, so a new read cannot ship undocumented.
  4. The root README's "TypeScript, Swift, and Kotlin read no environment
     variables at all" sentence still holds.

"Genuinely reads" means an env-read call site in non-test source, not a bare
mention. That distinction is the whole point: every `process.env.BASECAMP_*`
in typescript/src and every `ENV["BASECAMP_CLIENT_ID"]` in ruby/lib sits inside
a doc comment, so a substring search would happily bless a phantom row. Lines
whose first non-whitespace characters open a comment (`#`, `//`, `*`, `/*`) are
excluded for exactly that reason.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent

# Where each SDK's shipping source lives, and how that language reads an env var.
SDKS = {
    "Go": {
        "readme": "go/README.md",
        "source": "go/pkg",
        "suffixes": (".go",),
        "patterns": [r'os\.Getenv\(\s*"([A-Z_][A-Z0-9_]*)"', r'os\.LookupEnv\(\s*"([A-Z_][A-Z0-9_]*)"'],
    },
    "Ruby": {
        "readme": "ruby/README.md",
        "source": "ruby/lib",
        "suffixes": (".rb",),
        "patterns": [r'ENV\[\s*"([A-Z_][A-Z0-9_]*)"', r'ENV\.fetch\(\s*"([A-Z_][A-Z0-9_]*)"'],
    },
    "Python": {
        "readme": "python/README.md",
        "source": "python/src",
        "suffixes": (".py",),
        "patterns": [
            r'os\.environ\.get\(\s*"([A-Z_][A-Z0-9_]*)"',
            r'os\.environ\[\s*"([A-Z_][A-Z0-9_]*)"',
            r'os\.getenv\(\s*"([A-Z_][A-Z0-9_]*)"',
        ],
    },
    "TypeScript": {
        "readme": "typescript/README.md",
        "source": "typescript/src",
        "suffixes": (".ts",),
        "patterns": [r'process\.env\.([A-Z_][A-Z0-9_]*)', r'process\.env\[\s*"([A-Z_][A-Z0-9_]*)"'],
    },
    "Swift": {
        "readme": "swift/README.md",
        "source": "swift/Sources",
        "suffixes": (".swift",),
        "patterns": [r'ProcessInfo\.processInfo\.environment\[\s*"([A-Z_][A-Z0-9_]*)"'],
    },
    "Kotlin": {
        "readme": "kotlin/README.md",
        "source": "kotlin/sdk/src/commonMain",
        "suffixes": (".kt",),
        "patterns": [r'System\.getenv\(\s*"([A-Z_][A-Z0-9_]*)"'],
    },
}

ROOT_README = "README.md"

# The root README states these three read nothing. Invariant 4 holds it true.
NO_ENV_SDKS = ("TypeScript", "Swift", "Kotlin")

# Only variables in these families are in scope; the SDKs read no others.
VAR_RE = re.compile(r"\b((?:BASECAMP|XDG)_[A-Z0-9_]+)\b")

COMMENT_STARTS = ("#", "//", "*", "/*")
TEST_MARKERS = ("_test.", "test_", "/tests/", "/test/", ".test.", "Tests/", "spec/")


def is_test(path: Path) -> bool:
    text = str(path)
    return any(marker in text for marker in TEST_MARKERS)


def real_reads(spec: dict) -> dict[str, list[str]]:
    """Env vars genuinely read by this SDK, mapped to 'file:line' call sites."""
    root = REPO / spec["source"]
    found: dict[str, list[str]] = {}
    if not root.is_dir():
        return found
    compiled = [re.compile(p) for p in spec["patterns"]]
    for path in sorted(root.rglob("*")):
        if not path.is_file() or path.suffix not in spec["suffixes"] or is_test(path):
            continue
        for lineno, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            if line.lstrip().startswith(COMMENT_STARTS):
                continue  # a doc-comment example is not a read
            for pattern in compiled:
                for name in pattern.findall(line):
                    if VAR_RE.fullmatch(name):
                        found.setdefault(name, []).append(f"{path.relative_to(REPO)}:{lineno}")
    return found


def table_rows(readme: Path) -> list[tuple[int, list[str]]]:
    """Markdown table rows, as (line number, cells)."""
    rows = []
    for lineno, line in enumerate(readme.read_text(encoding="utf-8").splitlines(), 1):
        stripped = line.strip()
        if not stripped.startswith("|"):
            continue
        cells = [c.strip() for c in stripped.strip("|").split("|")]
        if all(set(c) <= set("-: ") for c in cells):
            continue  # separator row
        rows.append((lineno, cells))
    return rows


def main() -> int:
    failures: list[str] = []
    reads = {name: real_reads(spec) for name, spec in SDKS.items()}

    # 1. Forward, per SDK.
    for sdk, spec in SDKS.items():
        readme = REPO / spec["readme"]
        if not readme.is_file():
            continue
        for lineno, cells in table_rows(readme):
            for var in VAR_RE.findall(cells[0]):
                if var not in reads[sdk]:
                    failures.append(
                        f"{spec['readme']}:{lineno}: table names {var}, but no "
                        f"read of it exists in {spec['source']}/"
                    )

    # 2. Forward, root README: honor the "Read by" column.
    root = REPO / ROOT_README
    for lineno, cells in table_rows(root):
        if len(cells) < 2:
            continue
        vars_in_row = VAR_RE.findall(cells[0])
        if not vars_in_row:
            continue
        claimed = [sdk for sdk in SDKS if re.search(rf"\b{sdk}\b", cells[1])]
        if not claimed:
            failures.append(
                f"{ROOT_README}:{lineno}: row for {', '.join(vars_in_row)} names no "
                f"SDK in its 'Read by' column"
            )
        for var in vars_in_row:
            for sdk in claimed:
                if var not in reads[sdk]:
                    failures.append(
                        f"{ROOT_README}:{lineno}: credits {var} to {sdk}, but no read "
                        f"of it exists in {SDKS[sdk]['source']}/"
                    )

    # 3. Reverse, per SDK: a real read must be documented.
    for sdk, spec in SDKS.items():
        readme = REPO / spec["readme"]
        if not readme.is_file():
            continue
        text = readme.read_text(encoding="utf-8")
        for var, sites in sorted(reads[sdk].items()):
            if var not in text:
                failures.append(
                    f"{spec['readme']}: {sdk} reads {var} ({sites[0]}) but the README "
                    f"never mentions it"
                )

    # 4. The root README's "read no environment variables at all" sentence.
    for sdk in NO_ENV_SDKS:
        if reads[sdk]:
            detail = ", ".join(f"{v} ({s[0]})" for v, s in sorted(reads[sdk].items()))
            failures.append(
                f"{ROOT_README}: claims {sdk} reads no environment variables, but "
                f"found: {detail}"
            )

    if failures:
        print("README environment-variable check FAILED:\n", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        print(
            f"\n{len(failures)} problem(s). Fix the README, or the source, so the "
            f"two agree.",
            file=sys.stderr,
        )
        return 1

    total = sum(len(r) for r in reads.values())
    print(f"README env-var check passed ({total} real read sites across {len(SDKS)} SDKs).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
