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
a doc comment, so a substring search would happily bless a phantom row.

Comments are therefore removed before matching, by a scanner rather than a
line-prefix test — inside an unstarred `/* ... */` the interior lines begin with
ordinary code characters — and the stripped text is matched whole rather than
line by line, so a call split across lines is still seen. Both directions of
that matter: counting a doc example as a read lets a phantom table row pass,
and missing a real read lets an undocumented variable pass.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent

NAME = r"(?P<name>[A-Z_][A-Z0-9_]*)"

# Quote styles that actually delimit a string literal in each language. Getting
# this wrong fails *open*: a read the pattern cannot see is a read the gate will
# swear does not exist, so it would report a phantom row as fine and an
# undocumented variable as documented. The closing quote is a backreference, so
# a mismatched pair cannot match.
Q = r"(?P<q>[\"'])"  # Ruby, Python, TypeScript
GO_Q = r"(?P<q>[\"`])"  # Go has interpreted and raw string literals, no single quotes
DQ = r"(?P<q>\")"  # Swift and Kotlin have only double-quoted strings
ENDQ = r"(?P=q)"

# Where each SDK's shipping source lives, and how that language reads an env var.
SDKS = {
    "Go": {
        "readme": "go/README.md",
        "source": "go/pkg",
        "comments": "slash",
        "suffixes": (".go",),
        "patterns": [
            rf"os\.Getenv\(\s*{GO_Q}{NAME}{ENDQ}",
            rf"os\.LookupEnv\(\s*{GO_Q}{NAME}{ENDQ}",
        ],
    },
    "Ruby": {
        "readme": "ruby/README.md",
        "source": "ruby/lib",
        "comments": "hash",
        "suffixes": (".rb",),
        "patterns": [
            rf"ENV\[\s*{Q}{NAME}{ENDQ}",
            rf"ENV\.fetch\(\s*{Q}{NAME}{ENDQ}",
        ],
    },
    "Python": {
        "readme": "python/README.md",
        "source": "python/src",
        "comments": "hash",
        "suffixes": (".py",),
        "patterns": [
            rf"os\.environ\.get\(\s*{Q}{NAME}{ENDQ}",
            rf"os\.environ\[\s*{Q}{NAME}{ENDQ}",
            rf"os\.getenv\(\s*{Q}{NAME}{ENDQ}",
        ],
    },
    "TypeScript": {
        "readme": "typescript/README.md",
        "source": "typescript/src",
        "comments": "slash",
        "suffixes": (".ts",),
        "patterns": [
            rf"process\.env\.{NAME}",
            rf"process\.env\[\s*{Q}{NAME}{ENDQ}",
        ],
    },
    "Swift": {
        "readme": "swift/README.md",
        "source": "swift/Sources",
        "comments": "slash",
        "suffixes": (".swift",),
        "patterns": [rf"ProcessInfo\.processInfo\.environment\[\s*{DQ}{NAME}{ENDQ}"],
    },
    # kotlin/sdk/src, not just commonMain: jvmMain ships in the same artifact, so
    # scoping to commonMain would let a platform-specific read bypass this gate.
    # The source-set test directories (commonTest, jvmTest) are excluded below.
    "Kotlin": {
        "readme": "kotlin/README.md",
        "source": "kotlin/sdk/src",
        "comments": "slash",
        "suffixes": (".kt",),
        "patterns": [rf"System\.getenv\(\s*{DQ}{NAME}{ENDQ}"],
    },
}

ROOT_README = "README.md"

# The root README states these three read nothing. Invariant 4 holds it true.
NO_ENV_SDKS = ("TypeScript", "Swift", "Kotlin")

# Only variables in these families are in scope; the SDKs read no others.
VAR_RE = re.compile(r"\b((?:BASECAMP|XDG)_[A-Z0-9_]+)\b")

FILENAME_TEST_MARKERS = ("_test.", "test_", ".test.", "_spec.")

# Quote characters that open a string literal, per comment style. String bodies
# are kept — the variable name lives inside one — but they are skipped over so a
# `#` or `//` inside a string is not mistaken for the start of a comment.
STRING_QUOTES = {"hash": "\"'", "slash": "\"'`"}


def strip_noncode(text: str, style: str) -> str:
    """Blank comments and doc-only string blocks, preserving every offset.

    Removed characters become spaces and newlines are kept, so match offsets
    still map to the right line of the original file.

    A line-prefix test is not enough: in an unstarred block comment the interior
    lines start with ordinary code characters, so `/*\\nprocess.env.FOO\\n*/`
    would otherwise count as a real read.
    """
    out = list(text)
    n = len(text)
    quotes = STRING_QUOTES[style]
    i = 0

    def blank(start: int, end: int) -> None:
        for k in range(start, min(end, n)):
            if out[k] != "\n":
                out[k] = " "

    while i < n:
        # Python/Ruby docstrings: string literals, but used as documentation, so
        # an example inside one is not a read.
        if style == "hash" and text[i : i + 3] in ('"""', "'''"):
            quote = text[i : i + 3]
            end = text.find(quote, i + 3)
            end = n if end == -1 else end + 3
            blank(i, end)
            i = end
            continue
        if text[i] in quotes:
            quote = text[i]
            j = i + 1
            while j < n:
                if text[j] == "\\":
                    j += 2
                    continue
                if text[j] == quote:
                    j += 1
                    break
                # An unterminated single-line literal ends at the newline; Go and
                # TypeScript backtick literals legitimately span lines.
                if text[j] == "\n" and quote != "`":
                    break
                j += 1
            i = j
            continue
        if style == "slash" and text.startswith("//", i):
            end = text.find("\n", i)
            blank(i, n if end == -1 else end)
            i = n if end == -1 else end
            continue
        if style == "slash" and text.startswith("/*", i):
            end = text.find("*/", i + 2)
            end = n if end == -1 else end + 2
            blank(i, end)
            i = end
            continue
        if style == "hash" and text[i] == "#":
            end = text.find("\n", i)
            blank(i, n if end == -1 else end)
            i = n if end == -1 else end
            continue
        # Ruby block comment: =begin/=end, each at column 0.
        if style == "hash" and text.startswith("=begin", i) and (i == 0 or text[i - 1] == "\n"):
            end = text.find("\n=end", i)
            if end == -1:
                end = n
            else:
                nl = text.find("\n", end + 1)
                end = n if nl == -1 else nl
            blank(i, end)
            i = end
            continue
        i += 1
    return "".join(out)


def is_test(rel_path: Path) -> bool:
    """Test code, by directory or by filename.

    Takes a repo-relative path: matching against an absolute one would consult
    the checkout's parent directories, so a clone living under any directory
    named e.g. `Tests` would silently skip every file in the repo.

    Directory matching has to understand Kotlin/Swift source-set naming
    (`commonTest`, `jvmTest`, `Tests`) as well as the plain `test`/`tests`/`spec`
    directories the other SDKs use.
    """
    for part in rel_path.parts[:-1]:
        lowered = part.lower()
        if lowered in ("test", "tests", "spec", "specs"):
            return True
        if part.endswith(("Test", "Tests")):
            return True
    return any(marker in rel_path.name for marker in FILENAME_TEST_MARKERS)


def real_reads(spec: dict) -> dict[str, list[str]]:
    """Env vars genuinely read by this SDK, mapped to 'file:line' call sites."""
    root = REPO / spec["source"]
    found: dict[str, list[str]] = {}
    if not root.is_dir():
        return found
    compiled = [re.compile(p) for p in spec["patterns"]]
    for path in sorted(root.rglob("*")):
        if not path.is_file() or path.suffix not in spec["suffixes"]:
            continue
        rel = path.relative_to(REPO)
        if is_test(rel):
            continue
        # Scan the comment-stripped file whole rather than line by line: the
        # patterns allow whitespace after the opening paren/bracket, so a call
        # split across lines — os.getenv(\n    "BASECAMP_FOO") — is still a read,
        # and a per-line scan would silently miss it.
        text = strip_noncode(path.read_text(encoding="utf-8", errors="replace"), spec["comments"])
        for pattern in compiled:
            # finditer + the named group, not findall: these patterns carry a
            # quote group too, and findall would hand back tuples.
            for match in pattern.finditer(text):
                name = match.group("name")
                if VAR_RE.fullmatch(name):
                    lineno = text.count("\n", 0, match.start()) + 1
                    found.setdefault(name, []).append(f"{rel}:{lineno}")
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
