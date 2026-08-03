#!/usr/bin/env python3
"""Self-test for check-readme-env-vars.py, driven by synthetic repos.

The gate answers "does this SDK really read this variable?", and its failure
mode is silent: if a read pattern misses, the gate does not error — it reports
that nothing reads the variable, which reads as a README bug rather than a gate
bug. Two such defects already shipped in this script (a commonMain-only Kotlin
root that could not see jvmMain, and an absolute-path test filter that skipped
the entire repo when the checkout sat under a directory named `Tests`), so the
gate needs its own tests against inputs whose correct answer is known.

Each case builds a throwaway tree, points the gate's module-level constants at
it, and asserts on the failures returned.
"""

from __future__ import annotations

import importlib.util
import shutil
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent

spec = importlib.util.spec_from_file_location("gate", HERE / "check-readme-env-vars.py")
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

FAILURES: list[str] = []


def build(root: Path, files: dict[str, str]) -> None:
    for rel, body in files.items():
        path = root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(body, encoding="utf-8")


def run_gate(root: Path, sdks: dict, no_env_sdks: tuple = ()) -> list[str]:
    """Run the gate against a synthetic repo, returning its failure list."""
    old_repo, old_sdks, old_no_env = gate.REPO, gate.SDKS, gate.NO_ENV_SDKS
    gate.REPO, gate.SDKS, gate.NO_ENV_SDKS = root, sdks, no_env_sdks
    try:
        failures: list[str] = []
        # Re-implement main()'s collection by calling it and capturing stderr is
        # brittle; instead drive the same helpers main() uses.
        reads = {name: gate.real_reads(s) for name, s in sdks.items()}
        for sdk, s in sdks.items():
            readme = root / s["readme"]
            if not readme.is_file():
                continue
            for lineno, cells in gate.table_rows(readme):
                for var in gate.VAR_RE.findall(cells[0]):
                    if var not in reads[sdk]:
                        failures.append(f"forward:{sdk}:{var}")
            text = readme.read_text(encoding="utf-8")
            for var in sorted(reads[sdk]):
                if var not in text:
                    failures.append(f"reverse:{sdk}:{var}")
        for sdk in no_env_sdks:
            if reads.get(sdk):
                failures.append(f"noenv:{sdk}:{sorted(reads[sdk])[0]}")
        return failures
    finally:
        gate.REPO, gate.SDKS, gate.NO_ENV_SDKS = old_repo, old_sdks, old_no_env


def check(name: str, actual, expected) -> None:
    if actual == expected:
        print(f"  ok   {name}")
    else:
        print(f"  FAIL {name}\n         expected: {expected}\n         actual:   {actual}")
        FAILURES.append(name)


# Reuse the real per-SDK specs verbatim, so the fixtures exercise the patterns
# and comment style that actually ship rather than a copy that can drift.
PY_SDK = {"Python": gate.SDKS["Python"]}
RB_SDK = {"Ruby": gate.SDKS["Ruby"]}
TS_SDK = {"TypeScript": gate.SDKS["TypeScript"]}

TABLE = "| Variable | Description |\n|---|---|\n| `{var}` | thing |\n"


def main() -> int:
    tmp = Path(tempfile.mkdtemp(prefix="check-readme-env-vars-test-"))
    try:
        # 1. Single-quoted reads count. A double-quote-only pattern would report
        #    "no read exists" and the reverse check would pass in silence.
        root = tmp / "quotes-py"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_SINGLE"),
            "python/src/c.py": "v = os.environ.get('BASECAMP_SINGLE')\n",
        })
        check("python single-quoted read is seen", run_gate(root, PY_SDK), [])

        root = tmp / "quotes-rb"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_SINGLE"),
            "ruby/lib/c.rb": "v = ENV['BASECAMP_SINGLE']\n",
        })
        check("ruby single-quoted read is seen", run_gate(root, RB_SDK), [])

        root = tmp / "quotes-ts"
        build(root, {
            "typescript/README.md": TABLE.format(var="BASECAMP_SINGLE"),
            "typescript/src/c.ts": "const v = process.env['BASECAMP_SINGLE'];\n",
        })
        check("typescript single-quoted read is seen", run_gate(root, TS_SDK), [])

        # 2. A mismatched quote pair is not a string literal and must not match.
        root = tmp / "mismatched"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_BAD"),
            "python/src/c.py": "v = os.environ.get(\"BASECAMP_BAD')\n",
        })
        check("mismatched quotes do not count as a read",
              run_gate(root, PY_SDK), ["forward:Python:BASECAMP_BAD"])

        # 3. A documented variable nothing reads — the phantom-row defect.
        root = tmp / "phantom"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_PHANTOM"),
            "python/src/c.py": "v = 1\n",
        })
        check("phantom table row is caught",
              run_gate(root, PY_SDK), ["forward:Python:BASECAMP_PHANTOM"])

        # 4. A read nobody documented — the reverse direction.
        root = tmp / "undocumented"
        build(root, {
            "python/README.md": "no tables here\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_SECRET")\n',
        })
        check("undocumented read is caught",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_SECRET"])

        # 5. Doc-comment examples are not reads. This is what makes a plain
        #    substring search useless for this job.
        root = tmp / "comments"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_COMMENTED"),
            "python/src/c.py": '# v = os.environ.get("BASECAMP_COMMENTED")\n',
        })
        check("commented-out read does not satisfy the table",
              run_gate(root, PY_SDK), ["forward:Python:BASECAMP_COMMENTED"])

        # A whole JSDoc block, delimiters included — this is the shape that
        # actually ships in typescript/src, and every BASECAMP_* there is inside
        # one. A bare " * ..." fragment with no opening /* is not valid source
        # and must not be what the fixture asserts on.
        root = tmp / "comments-ts"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts": (
                "/**\n * const client = createBasecampClient({\n"
                " *   accessToken: process.env.BASECAMP_TOKEN!,\n * });\n */\n"
                "export const x = 1;\n"
            ),
        })
        check("jsdoc example is not a read (no-env claim survives)",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        # 5b. Interior lines of an unstarred block comment begin with ordinary
        #     code characters, so a line-prefix test would count them.
        root = tmp / "blockcomment"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts": "/*\nprocess.env.BASECAMP_DOC\n*/\n",
        })
        check("unstarred block comment is not a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        root = tmp / "docstring"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_DOCSTR"),
            "python/src/c.py": 'def f():\n    """Example: os.environ.get("BASECAMP_DOCSTR")"""\n    return 1\n',
        })
        check("python docstring example is not a read",
              run_gate(root, PY_SDK), ["forward:Python:BASECAMP_DOCSTR"])

        # 5c. A `#` inside a string literal does not start a comment, so a read
        #     later on the same line must still be seen.
        root = tmp / "hashinstring"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_AFTERHASH"),
            "python/src/c.py": 'u = "http://h/#frag"\nv = os.environ.get("BASECAMP_AFTERHASH")\n',
        })
        check("hash inside a string does not hide a later read",
              run_gate(root, PY_SDK), [])

        # 5d. Reads split across lines. A per-line scan misses these entirely,
        #     which fails open on the reverse and no-env checks.
        root = tmp / "multiline-py"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_MULTI"),
            "python/src/c.py": 'v = os.getenv(\n    "BASECAMP_MULTI"\n)\n',
        })
        check("multiline python read is seen", run_gate(root, PY_SDK), [])

        root = tmp / "multiline-rb"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_MULTI"),
            "ruby/lib/c.rb": 'v = ENV.fetch(\n  "BASECAMP_MULTI", nil\n)\n',
        })
        check("multiline ruby read is seen", run_gate(root, RB_SDK), [])

        root = tmp / "multiline-ts"
        build(root, {
            "typescript/README.md": "mentions BASECAMP_MULTI\n",
            "typescript/src/c.ts": 'const v = process.env[\n  "BASECAMP_MULTI"\n];\n',
        })
        check("multiline typescript read breaks the no-env claim",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_MULTI"])

        # 6. The no-env claim breaks on a real read.
        root = tmp / "noenv"
        build(root, {
            "typescript/README.md": "mentions BASECAMP_REAL\n",
            "typescript/src/c.ts": "const v = process.env.BASECAMP_REAL;\n",
        })
        check("real read breaks the no-env claim",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_REAL"])

        # 7. Test code is excluded, including Kotlin/Swift source-set naming...
        root = tmp / "testdirs"
        build(root, {
            "python/README.md": "no tables\n",
            "python/src/tests/c.py": 'v = os.environ.get("BASECAMP_INTEST")\n',
            "python/src/commonTest/c.py": 'v = os.environ.get("BASECAMP_INSET")\n',
            "python/src/c_test.py": 'v = os.environ.get("BASECAMP_INFILE")\n',
        })
        check("test dirs and files are excluded", run_gate(root, PY_SDK), [])

        # ...but a nested non-test directory under the source root is not.
        root = tmp / "nested"
        build(root, {
            "python/README.md": "no tables\n",
            "python/src/basecamp/deep/c.py": 'v = os.environ.get("BASECAMP_DEEP")\n',
        })
        check("nested real source is scanned",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_DEEP"])

        # 8. The repo living under a directory named `Tests` must not disable
        #    the whole gate (the absolute-path bug).
        root = tmp / "Tests" / "checkout"
        build(root, {
            "python/README.md": "no tables\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_UNDER_TESTS")\n',
        })
        check("checkout under a Tests/ parent still scans",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_UNDER_TESTS"])

        # 9. Out-of-family variables are ignored; the gate only polices
        #    BASECAMP_*/XDG_*, not every env var an SDK might touch.
        root = tmp / "outoffamily"
        build(root, {
            "python/README.md": "no tables\n",
            "python/src/c.py": 'v = os.environ.get("HTTP_PROXY")\n',
        })
        check("unrelated env vars are ignored", run_gate(root, PY_SDK), [])
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    if FAILURES:
        print(f"\n{len(FAILURES)} self-test failure(s): {', '.join(FAILURES)}", file=sys.stderr)
        return 1
    print("\ncheck-readme-env-vars self-test passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
