#!/usr/bin/env python3
"""Keep README environment-variable tables honest against a pinned inventory.

READMEs have repeatedly claimed environment variables that no SDK reads
(a phantom `BASECAMP_ACCOUNT_ID` row), or attributed a real variable to the
wrong SDK (`XDG_CACHE_HOME` credited to Ruby, which never reads it). Both are
invisible to every other gate in `make check`, because nothing here is
generated -- the prose is hand-written and drifts silently.

#611 measured that the previous general-purpose, six-language lexer never
changed this gate's answer about the shipping tree across ten review rounds:
the inventory stayed fixed while every fix chased hypothetical inputs. A
lexer that cannot close, sitting on `make check`, is the shape that gets a
gate deleted the first time it misfires.

This gate therefore checks only what the evidence supports:

  1. Forward, per SDK: every variable named in an SDK README's env-var table
     is a member of the pinned inventory for that SDK.
  2. Forward, root README: the "Read by" column names SDKs, and each claimed
     (variable, SDK) pair is in the inventory. When a row names a variable
     the inventory knows about, the column must list exactly the SDKs that
     inventory pairs with it -- otherwise a second reader can appear in
     source, stay in the inventory, and still be omitted from the table.
  3. Read sites: every inventory member still has its pinned exact-match
     read site(s) in source. The substring either appears in the named file
     or it does not -- no comment stripping, no string masking, no language
     parsing.

The inventory is the source of truth. A new real read is supposed to fail
here until someone adds the pair and its needle; that is the cost of keeping
the gate small enough to trust. Moving the check into each SDK's own test
suite (Option 2 in #611) is deliberately out of scope for this change.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent

ROOT_README = "README.md"

# SDK display name -> path of the README whose env-var table this gate reads.
SDK_READMES = {
    "Go": "go/README.md",
    "Ruby": "ruby/README.md",
    "Python": "python/README.md",
    "TypeScript": "typescript/README.md",
    "Swift": "swift/README.md",
    "Kotlin": "kotlin/README.md",
    "Rust": "rust/basecamp-sdk/README.md",
}

# Variables the READMEs tabulate. Caller-side examples also mention
# BASECAMP_TOKEN / BASECAMP_ACCOUNT_ID inside fenced Quick Starts; those are
# not table rows and are not this gate's business.
VAR_RE = re.compile(r"\b(BASECAMP_[A-Z0-9_]+|XDG_[A-Z0-9_]+)\b")

# Exact inventory of variable/SDK pairs, each with the source substrings that
# prove a read still exists. Derived from current main by running the previous
# lexer once, then pinning its unique (variable, SDK) pairs (19) and the
# distinct call-form needles those sites share (22: Go's two
# `os.Getenv("BASECAMP_TOKEN")` call sites are one needle). Update this table
# when a real read is added, removed, or moved -- do not revive the lexer to
# rediscover it.
#
# Each needle is an exact substring of the named file. Quoting and call shape
# are part of the pin: `os.Getenv("BASECAMP_TOKEN")` is the Go read;
# `process.env.BASECAMP_TOKEN` in a TypeScript example is not in this table
# because TypeScript is not a reader.
INVENTORY: dict[tuple[str, str], list[tuple[str, str]]] = {
    ("Go", "BASECAMP_BASE_URL"): [
        ("go/pkg/basecamp/config.go", 'os.Getenv("BASECAMP_BASE_URL")'),
    ],
    ("Go", "BASECAMP_CACHE_DIR"): [
        ("go/pkg/basecamp/config.go", 'os.Getenv("BASECAMP_CACHE_DIR")'),
    ],
    ("Go", "BASECAMP_CACHE_ENABLED"): [
        ("go/pkg/basecamp/config.go", 'os.Getenv("BASECAMP_CACHE_ENABLED")'),
    ],
    ("Go", "BASECAMP_NO_KEYRING"): [
        ("go/pkg/basecamp/auth.go", 'os.Getenv("BASECAMP_NO_KEYRING")'),
    ],
    ("Go", "BASECAMP_PROJECT_ID"): [
        ("go/pkg/basecamp/config.go", 'os.Getenv("BASECAMP_PROJECT_ID")'),
    ],
    ("Go", "BASECAMP_TODOLIST_ID"): [
        ("go/pkg/basecamp/config.go", 'os.Getenv("BASECAMP_TODOLIST_ID")'),
    ],
    ("Go", "BASECAMP_TOKEN"): [
        ("go/pkg/basecamp/auth.go", 'os.Getenv("BASECAMP_TOKEN")'),
    ],
    ("Go", "XDG_CACHE_HOME"): [
        ("go/pkg/basecamp/config.go", 'os.Getenv("XDG_CACHE_HOME")'),
    ],
    ("Go", "XDG_CONFIG_HOME"): [
        ("go/pkg/basecamp/config.go", 'os.Getenv("XDG_CONFIG_HOME")'),
    ],
    ("Ruby", "BASECAMP_BASE_URL"): [
        ("ruby/lib/basecamp/config.rb", 'ENV.fetch("BASECAMP_BASE_URL"'),
        ("ruby/lib/basecamp/config.rb", 'ENV["BASECAMP_BASE_URL"]'),
    ],
    ("Ruby", "BASECAMP_MAX_RETRIES"): [
        ("ruby/lib/basecamp/config.rb", 'ENV.fetch("BASECAMP_MAX_RETRIES"'),
        ("ruby/lib/basecamp/config.rb", 'ENV["BASECAMP_MAX_RETRIES"]'),
    ],
    ("Ruby", "BASECAMP_TIMEOUT"): [
        ("ruby/lib/basecamp/config.rb", 'ENV.fetch("BASECAMP_TIMEOUT"'),
        ("ruby/lib/basecamp/config.rb", 'ENV["BASECAMP_TIMEOUT"]'),
    ],
    ("Ruby", "XDG_CONFIG_HOME"): [
        ("ruby/lib/basecamp/config.rb", 'ENV["XDG_CONFIG_HOME"]'),
    ],
    ("Python", "BASECAMP_BASE_URL"): [
        ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_BASE_URL"'),
    ],
    ("Python", "BASECAMP_MAX_RETRIES"): [
        ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_MAX_RETRIES"'),
    ],
    ("Python", "BASECAMP_TIMEOUT"): [
        ("python/src/basecamp/config.py", 'os.environ.get("BASECAMP_TIMEOUT"'),
    ],
    ("Rust", "BASECAMP_BASE_URL"): [
        ("rust/basecamp-sdk/src/config.rs", 'env_value("BASECAMP_BASE_URL")'),
    ],
    ("Rust", "BASECAMP_MAX_RETRIES"): [
        ("rust/basecamp-sdk/src/config.rs", 'env_value("BASECAMP_MAX_RETRIES")'),
    ],
    ("Rust", "BASECAMP_TIMEOUT"): [
        ("rust/basecamp-sdk/src/config.rs", 'env_value("BASECAMP_TIMEOUT")'),
    ],
}


def inventory_pairs() -> set[tuple[str, str]]:
    return set(INVENTORY)


def readers_of(var: str) -> list[str]:
    """SDKs the inventory pairs with this variable, in SDK_READMES order."""
    return [sdk for sdk in SDK_READMES if (sdk, var) in INVENTORY]


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


def check_read_sites(repo: Path = REPO) -> list[str]:
    """Fail when a pinned needle is missing from its file."""
    failures: list[str] = []
    for (sdk, var), sites in sorted(INVENTORY.items()):
        if not sites:
            failures.append(
                f"inventory: ({sdk}, {var}) has no pinned read sites"
            )
            continue
        missing = []
        for rel, needle in sites:
            path = repo / rel
            if not path.is_file():
                missing.append(f"{rel} (file missing)")
                continue
            text = path.read_text(encoding="utf-8", errors="replace")
            if needle not in text:
                missing.append(f"{rel} (missing {needle!r})")
        if missing:
            failures.append(
                f"inventory: {sdk} read of {var} lost site(s): "
                + "; ".join(missing)
            )
    return failures


def check_sdk_tables(repo: Path = REPO) -> list[str]:
    """Fail when an SDK README table names a variable outside the inventory."""
    failures: list[str] = []
    pairs = inventory_pairs()
    for sdk, rel in SDK_READMES.items():
        readme = repo / rel
        if not readme.is_file():
            continue
        for lineno, cells in table_rows(readme):
            for var in VAR_RE.findall(cells[0]):
                if (sdk, var) not in pairs:
                    failures.append(
                        f"{rel}:{lineno}: table names {var}, but ({sdk}, {var}) "
                        f"is not in the pinned inventory"
                    )
    return failures


def check_root_table(repo: Path = REPO) -> list[str]:
    """Fail when the root README's env-var table disagrees with the inventory."""
    failures: list[str] = []
    root = repo / ROOT_README
    if not root.is_file():
        return failures
    pairs = inventory_pairs()
    for lineno, cells in table_rows(root):
        if len(cells) < 2:
            continue
        vars_in_row = VAR_RE.findall(cells[0])
        if not vars_in_row:
            continue
        claimed = [sdk for sdk in SDK_READMES if re.search(rf"\b{sdk}\b", cells[1])]
        if not claimed:
            failures.append(
                f"{ROOT_README}:{lineno}: row for {', '.join(vars_in_row)} names no "
                f"SDK in its 'Read by' column"
            )
        for var in vars_in_row:
            expected = readers_of(var)
            for sdk in claimed:
                if (sdk, var) not in pairs:
                    failures.append(
                        f"{ROOT_README}:{lineno}: credits {var} to {sdk}, but "
                        f"({sdk}, {var}) is not in the pinned inventory"
                    )
            # Only enforce exact membership when the inventory knows the
            # variable. An unknown variable already failed the credit check
            # above for every claimed SDK; asking the column to match an empty
            # expected set would double-report the same defect.
            if expected:
                missing = [sdk for sdk in expected if sdk not in claimed]
                for sdk in missing:
                    failures.append(
                        f"{ROOT_README}:{lineno}: {sdk} is inventoried as reading "
                        f"{var}, but the 'Read by' column omits it"
                    )
    return failures


def collect_failures(repo: Path = REPO) -> list[str]:
    return check_read_sites(repo) + check_sdk_tables(repo) + check_root_table(repo)


def main() -> int:
    failures = collect_failures(REPO)

    if failures:
        print("README environment-variable check FAILED:\n", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        print(
            f"\n{len(failures)} problem(s). Fix the README, or update the pinned "
            f"inventory in scripts/check-readme-env-vars.py so the two agree.",
            file=sys.stderr,
        )
        return 1

    pairs = len(INVENTORY)
    sites = sum(len(s) for s in INVENTORY.values())
    print(
        f"README env-var check passed ({pairs} variable/SDK pairs, "
        f"{sites} pinned read sites, across {len(SDK_READMES)} SDKs)."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
