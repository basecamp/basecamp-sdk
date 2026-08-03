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

import contextlib
import importlib.util
import io
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


def run_gate(root: Path, sdks: dict, no_env_sdks: tuple = (),
             check_root_table: bool = False) -> list[str]:
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
            documented = gate.affirmative_mentions(readme)
            for var in sorted(reads[sdk]):
                if var not in documented:
                    failures.append(f"reverse:{sdk}:{var}")
            for _, claimed_sdk, named in gate.prose_claims(readme):
                for var in named:
                    if var not in reads.get(claimed_sdk, {}):
                        failures.append(f"prose:{claimed_sdk}:{var}")
        # Invariant 2, in the order main() reports it: a row must name an SDK,
        # every SDK it names must really read the variable, and every SDK that
        # really reads it must be named. Reproducing only the last of the three
        # left the first two with no coverage at all -- and the root table is
        # what the READMEs get wrong most often.
        if check_root_table:
            root_readme = root / gate.ROOT_README
            if root_readme.is_file():
                for _lineno, cells in gate.table_rows(root_readme):
                    if len(cells) < 2:
                        continue
                    row_vars = gate.VAR_RE.findall(cells[0])
                    if not row_vars:
                        continue
                    claimed = [s for s in sdks if gate.re.search(rf"\b{s}\b", cells[1])]
                    if not claimed:
                        failures.append(f"rootnosdk:{','.join(row_vars)}")
                    for var in row_vars:
                        for s in claimed:
                            if var not in reads[s]:
                                failures.append(f"rootcredits:{s}:{var}")
                        for s in sdks:
                            if var in reads[s] and s not in claimed:
                                failures.append(f"rootomits:{s}:{var}")
                # Invariant 2b: the row loop above only ever sees variables that
                # already have a row, so a real read with no row at all slips
                # past every check. Named anywhere affirmative counts.
                root_documented = gate.affirmative_mentions(root_readme)
                for s in sdks:
                    for var in sorted(reads[s]):
                        if var not in root_documented:
                            failures.append(f"rootmissing:{s}:{var}")
        # Invariant 4 is unscoped: "reads no environment variables at all" is
        # broken by `process.env.HOME` too, and the scoped inventory drops it.
        for sdk in no_env_sdks:
            every = gate.real_reads(sdks[sdk], scoped=False)
            if every:
                failures.append(f"noenv:{sdk}:{sorted(every)[0]}")
            # ...and a wholesale grab, which names no variable and so cannot
            # appear in the inventory above at all. Only when that inventory is
            # empty, matching main(): a named read already fails the claim.
            elif gate.env_api_sites(sdk, sdks[sdk]):
                failures.append(f"envapi:{sdk}")
        return failures
    finally:
        gate.REPO, gate.SDKS, gate.NO_ENV_SDKS = old_repo, old_sdks, old_no_env


def run_main(root: Path, sdks: dict, no_env_sdks: tuple = ()) -> int:
    """Call the gate's real `main()` against a synthetic repo, returning its code.

    `run_gate` above drives the same helpers `main()` drives, which is what lets
    every case assert on a compact tag rather than on prose. What that cannot
    cover is `main()` itself: the order it wires the five invariants in, and the
    exit code the Makefile recipe and the CI step actually branch on. Nothing
    else here ever runs the shipped entrypoint.

    Asserting only the code, never the message text, is what keeps this cheap.
    Sharing the whole orchestration would mean rewriting every expectation
    against failure strings, trading a small drift risk for a large brittleness
    one in the file whose job is to be trustworthy; a smoke test on the return
    value buys the coverage without that.
    """
    old = gate.REPO, gate.SDKS, gate.NO_ENV_SDKS
    gate.REPO, gate.SDKS, gate.NO_ENV_SDKS = root, sdks, no_env_sdks
    try:
        with contextlib.redirect_stdout(io.StringIO()), \
             contextlib.redirect_stderr(io.StringIO()):
            return gate.main()
    finally:
        gate.REPO, gate.SDKS, gate.NO_ENV_SDKS = old


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
PY_RB = {"Python": gate.SDKS["Python"], "Ruby": gate.SDKS["Ruby"]}
SW_SDK = {"Swift": gate.SDKS["Swift"]}
KT_SDK = {"Kotlin": gate.SDKS["Kotlin"]}
GO_SDK = {"Go": gate.SDKS["Go"]}

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

        # Ruby parentheses are optional, so `ENV.fetch "FOO", nil` is a real read
        # that a `(`-only pattern reports as nonexistent.
        root = tmp / "ruby-no-parens"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_NOPAREN"),
            "ruby/lib/c.rb": 'v = ENV.fetch "BASECAMP_NOPAREN", nil\n',
        })
        check("ruby ENV.fetch without parentheses is seen",
              run_gate(root, RB_SDK), [])

        root = tmp / "quotes-ts"
        build(root, {
            "typescript/README.md": TABLE.format(var="BASECAMP_SINGLE"),
            "typescript/src/c.ts": "const v = process.env['BASECAMP_SINGLE'];\n",
        })
        check("typescript single-quoted read is seen", run_gate(root, TS_SDK), [])

        # Optional chaining is valid TypeScript and a real read; both spellings
        # were invisible to patterns that demanded the dot or bracket directly.
        root = tmp / "optional-chaining"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_OPT.\n",
            "typescript/src/c.ts": "const v = process.env?.BASECAMP_OPT;\n",
        })
        check("typescript optional-chained read is seen",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_OPT"])

        root = tmp / "optional-chaining-bracket"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_OPTB.\n",
            "typescript/src/c.ts": 'const v = process.env?.["BASECAMP_OPTB"];\n',
        })
        check("typescript optional-chained bracket read is seen",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_OPTB"])

        # "Reads no environment variables at all" is broken by any read, not
        # just a documented family. Scoping this one hid `process.env.HOME`
        # behind a claim that the SDK reads nothing.
        root = tmp / "noenv-unscoped"
        build(root, {
            "typescript/README.md": "The SDK reads nothing.\n",
            "typescript/src/c.ts": "const h = process.env.HOME;\n",
        })
        check("the no-environment claim sees reads outside the documented families",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:HOME"])

        # A shipping file whose name merely contains `test_` is not a test.
        root = tmp / "filename-not-a-test"
        build(root, {
            "python/README.md": "The SDK reads nothing.\n",
            "python/src/latest_config.py": 'v = os.environ.get("BASECAMP_LATEST")\n',
        })
        check("a shipping file named latest_* is scanned",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_LATEST"])

        # A regex literal is data. Its braces must not close an interpolation,
        # and its contents must not read as calls.
        root = tmp / "regex-in-hole"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_SECRET.\n",
            "typescript/src/c.ts":
                'const x = `${(/}/, process.env.BASECAMP_SECRET)}`;\n',
        })
        check("a regex brace does not truncate the interpolation",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_SECRET"])

        root = tmp / "regex-content"
        build(root, {
            "typescript/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "typescript/src/c.ts": "const re = /process.env.BASECAMP_FAKE/;\n",
        })
        check("regex contents are not a read",
              run_gate(root, TS_SDK), ["forward:TypeScript:BASECAMP_FAKE"])

        # ...but `/` after a value is division, not a regex. Getting this wrong
        # would swallow code up to the next slash and hide the read.
        root = tmp / "division-not-regex"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_DIV.\n",
            "typescript/src/c.ts": "const r = total / process.env.BASECAMP_DIV;\n",
        })
        check("a division slash is not a regex",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_DIV"])

        # A quoted destructuring key: the name sits inside a literal, so the
        # match has to start on the brace or comma to survive the string mask.
        root = tmp / "destructured-quoted-key"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_QK.\n",
            "typescript/src/c.ts": 'const { "BASECAMP_QK": token } = process.env;\n',
        })
        check("a quoted destructuring key is a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_QK"])

        # Kotlin triple-quoted strings are raw, so a trailing backslash does not
        # escape the terminator. Swift's are not raw, which is how \( works.
        root = tmp / "kotlin-triple-raw"
        build(root, {
            "kotlin/README.md": "The SDK reads BASECAMP_KA.\n",
            "kotlin/sdk/src/c.kt":
                'val s = """ends with a backslash \\"""\nval v = System.getenv("BASECAMP_KA")\n',
        })
        check("a kotlin triple-quoted string is raw",
              run_gate(root, KT_SDK, no_env_sdks=("Kotlin",)),
              ["noenv:Kotlin:BASECAMP_KA"])

        # A denial is not documentation. python/README.md really does say the
        # SDK never reads BASECAMP_TOKEN, so a substring test would bless it the
        # day the SDK started to.
        root = tmp / "negative-mention"
        build(root, {
            "python/README.md":
                "`BASECAMP_TOKEN` appears in the examples only because the caller "
                "reads it; the SDK never looks it up.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_TOKEN")\n',
        })
        check("a denial does not count as documentation",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_TOKEN"])

        # ...and an affirmative sentence still does, since that is how the XDG
        # variables are documented in go/README.md and ruby/README.md.
        root = tmp / "affirmative-mention"
        build(root, {
            "python/README.md":
                "The sole other environment read is `BASECAMP_AFFIRM`, for the cache.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_AFFIRM")\n',
        })
        check("an affirmative sentence counts as documentation",
              run_gate(root, PY_SDK), [])

        # Absence of a denial is not an affirmation. A neutral sentence claims
        # nothing, and the denial that follows it refers back by pronoun -- so
        # taking each sentence in isolation counted the README as documenting
        # the very read it goes on to disclaim.
        root = tmp / "neutral-then-denial"
        build(root, {
            "python/README.md":
                "`BASECAMP_NEUTRAL` appears in examples. The SDK never reads it.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_NEUTRAL")\n',
        })
        check("a neutral mention beside a denial is not documentation",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_NEUTRAL"])

        # ...and a sentence that denies nothing still has to claim something.
        # "reserved for future use" documents no read at all.
        root = tmp / "neutral-no-denial"
        build(root, {
            "python/README.md": "`BASECAMP_RESERVED` is reserved for future use.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_RESERVED")\n',
        })
        check("a neutral sentence with no claim is not documentation",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_RESERVED"])

        # The verbs the shipping READMEs actually use have to keep working, or
        # this fails CI on correct prose. `consults` is go/README.md's word for
        # the XDG pair, which has no table row anywhere.
        root = tmp / "affirmative-verbs"
        build(root, {
            "go/README.md":
                "`DefaultConfig` consults `XDG_CACHE_HOME` to site the cache directory.\n",
            "go/pkg/c.go": 'v := os.Getenv("XDG_CACHE_HOME")\n',
        })
        check("an affirmative verb documents a prose-only read",
              run_gate(root, GO_SDK), [])

        # ...but a table row is an affirmative claim in its own right, and the
        # denial usually qualifies one code path rather than the SDK. This is
        # go/README.md's real sentence about `StaticTokenProvider`.
        root = tmp / "denial-beside-table"
        build(root, {
            "go/README.md": TABLE.format(var="BASECAMP_TOKEN")
                + "\n`StaticTokenProvider` does not read `BASECAMP_TOKEN`.\n",
            "go/pkg/c.go": 'v := os.Getenv("BASECAMP_TOKEN")\n',
        })
        check("a table row survives a denial elsewhere",
              run_gate(root, GO_SDK), [])

        # A mention inside a fenced example is the *caller* reading its own
        # environment, which says nothing about what the SDK reads.
        root = tmp / "fenced-mention"
        build(root, {
            "python/README.md": "intro\n\n```python\nos.environ['BASECAMP_FENCED']\n```\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_FENCED")\n',
        })
        check("a fenced example does not document a read",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_FENCED"])

        # The root table must name every SDK that really reads the variable, not
        # just the ones already listed.
        root = tmp / "root-omits-reader"
        build(root, {
            "README.md": "| Variable | Read by |\n|---|---|\n| `BASECAMP_SHARED` | Ruby |\n",
            "python/README.md": "The SDK reads `BASECAMP_SHARED` from the environment.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_SHARED")\n',
            "ruby/README.md": "The SDK reads `BASECAMP_SHARED` from the environment.\n",
            "ruby/lib/c.rb": 'v = ENV["BASECAMP_SHARED"]\n',
        })
        check("the root table must name every reader",
              run_gate(root, PY_RB, check_root_table=True),
              ["rootomits:Python:BASECAMP_SHARED"])

        # ...and the forward direction of the same invariant: a 'Read by' column
        # may not credit an SDK that does not read the variable. This is the
        # half the root table gets wrong in practice -- it is how Ruby came to be
        # credited with XDG_CACHE_HOME -- so it needs its own case.
        root = tmp / "root-credits-nonreader"
        build(root, {
            "README.md":
                "| Variable | Read by |\n|---|---|\n| `BASECAMP_PHANTOM` | Python, Ruby |\n",
            "python/README.md": "The SDK reads `BASECAMP_PHANTOM` from the environment.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_PHANTOM")\n',
            "ruby/README.md": "no table here\n",
            "ruby/lib/c.rb": "v = 1\n",
        })
        check("the root table may not credit a non-reader",
              run_gate(root, PY_RB, check_root_table=True),
              ["rootcredits:Ruby:BASECAMP_PHANTOM"])

        # A row that names no SDK at all is the same defect wearing a disguise:
        # nothing to check against, so every other invariant passes it.
        root = tmp / "root-names-no-sdk"
        build(root, {
            "README.md":
                "| Variable | Read by |\n|---|---|\n| `BASECAMP_ORPHAN` | the CLI |\n",
            "python/README.md": "The SDK reads `BASECAMP_ORPHAN` from the environment.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_ORPHAN")\n',
        })
        check("a root row must name at least one SDK",
              run_gate(root, PY_SDK, check_root_table=True),
              ["rootnosdk:BASECAMP_ORPHAN", "rootomits:Python:BASECAMP_ORPHAN"])

        # A real read with no root row at all is invisible to the row loop --
        # it never gets a row to iterate. The root README is an inventory, so a
        # variable documented only in its own SDK's README still fails.
        root = tmp / "root-missing-row"
        build(root, {
            "README.md": "| Variable | Read by |\n|---|---|\n| `BASECAMP_OLD` | Python |\n",
            "python/README.md": "The SDK reads `BASECAMP_OLD` and `BASECAMP_NEW`.\n",
            "python/src/c.py":
                'a = os.environ.get("BASECAMP_OLD")\nb = os.environ.get("BASECAMP_NEW")\n',
        })
        check("a real read absent from the root README is caught",
              run_gate(root, PY_SDK, check_root_table=True),
              ["rootmissing:Python:BASECAMP_NEW"])

        # Ruby percent literals are data.
        root = tmp / "ruby-percent"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "ruby/lib/c.rb": "s = %q{ENV['BASECAMP_FAKE']}\n",
        })
        check("a ruby percent literal is not a read",
              run_gate(root, RB_SDK), ["forward:Ruby:BASECAMP_FAKE"])

        # ...but the uppercase forms interpolate, so a call inside one is real.
        root = tmp / "ruby-percent-interp"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_QINT"),
            "ruby/lib/c.rb": 'x = %Q{#{ENV["BASECAMP_QINT"]}}\n',
        })
        check("a %Q interpolation is a read", run_gate(root, RB_SDK), [])

        # A quote inside a percent literal is data. Delegating the delimiter walk
        # to the language-aware matcher let it open a phantom string and run past
        # the close, masking the read after it.
        root = tmp / "ruby-percent-quote"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_AFTERQ"),
            "ruby/lib/c.rb": 's = %q{"}\nv = ENV["BASECAMP_AFTERQ"]\n',
        })
        check("a quote inside a percent literal is data", run_gate(root, RB_SDK), [])

        # The delimiter can be any non-alphanumeric character, not just a bracket.
        root = tmp / "ruby-percent-bang"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "ruby/lib/c.rb": "s = %q!ENV['BASECAMP_FAKE']!\n",
        })
        check("an unpaired percent delimiter still delimits",
              run_gate(root, RB_SDK), ["forward:Ruby:BASECAMP_FAKE"])

        # %x is a command literal: lowercase, but it interpolates.
        root = tmp / "ruby-percent-x"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_XCMD"),
            "ruby/lib/c.rb": 's = %x{echo #{ENV["BASECAMP_XCMD"]}}\n',
        })
        check("a %x command literal interpolates", run_gate(root, RB_SDK), [])

        # Ruby regexes interpolate, and the `#` must not open a comment.
        root = tmp / "ruby-regex-interp"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_RXI"),
            "ruby/lib/c.rb": 'r = /#{ENV["BASECAMP_RXI"]}/\n',
        })
        check("a ruby regex interpolation is a read", run_gate(root, RB_SDK), [])

        # An escaped interpolation marker is literal text. The escape test has to
        # run after the opener test, because Swift's opener is itself a backslash
        # sequence and checking the escape first eats it.
        root = tmp / "ruby-escaped-interp"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_ESC"),
            "ruby/lib/c.rb": 'x = %Q{\\#{ENV["BASECAMP_ESC"]}}\n',
        })
        check("an escaped ruby interpolation marker is not a read",
              run_gate(root, RB_SDK), ["forward:Ruby:BASECAMP_ESC"])

        # ...and the same escape inside a regex literal, which reaches
        # brace_holes by a different path in strip_noncode.
        root = tmp / "ruby-regex-escaped-interp"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_RESC"),
            "ruby/lib/c.rb": "r = /\\#{ENV['BASECAMP_RESC']}/\n",
        })
        check("an escaped marker in a ruby regex is not a read",
              run_gate(root, RB_SDK), ["forward:Ruby:BASECAMP_RESC"])

        root = tmp / "ruby-regex-data"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_RXD"),
            "ruby/lib/c.rb": "r = /ENV['BASECAMP_RXD']/\n",
        })
        check("ruby regex contents are not a read",
              run_gate(root, RB_SDK), ["forward:Ruby:BASECAMP_RXD"])

        # Ruby calls may omit their parentheses, so the argument to `puts` is a
        # regex even though the preceding token is an identifier. Reading it as
        # division hands the regex body to the read patterns as code.
        root = tmp / "ruby-command-regex"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "ruby/lib/c.rb": "puts /ENV['BASECAMP_FAKE']/\n",
        })
        check("a regex argument to a parenthesis-less call is not a read",
              run_gate(root, RB_SDK), ["forward:Ruby:BASECAMP_FAKE"])

        # The inverse costs more than a phantom row: misread as division, the
        # `#` opens a line comment and a genuine interpolated read vanishes.
        root = tmp / "ruby-command-regex-interp"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_CMDI"),
            "ruby/lib/c.rb": "puts /#{ENV['BASECAMP_CMDI']}/\n",
        })
        check("an interpolated read in a command-form regex is still a read",
              run_gate(root, RB_SDK), [])

        # Spacing is what tells the two apart, so symmetric spacing stays
        # division: `a / b / c` must not swallow the read that follows it.
        root = tmp / "ruby-spaced-division"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_DIV"),
            "ruby/lib/c.rb": "n = total / count / 2\nv = ENV['BASECAMP_DIV']\n",
        })
        check("a spaced ruby division is not a command-form regex",
              run_gate(root, RB_SDK), [])

        # ...and so does divide-and-assign, whose `=` follows the slash.
        root = tmp / "ruby-divide-assign"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_DIVA"),
            "ruby/lib/c.rb": "n /= 2\nv = ENV['BASECAMP_DIVA']\n",
        })
        check("ruby divide-and-assign is not a command-form regex",
              run_gate(root, RB_SDK), [])

        # Spacing alone cannot decide this, so the call has to be one that takes
        # a regex. `total /x/ 2` is arithmetic after a local variable, and
        # guessing "regex" masks everything between the two operators -- the
        # fail-open direction, where a real read vanishes.
        root = tmp / "ruby-lvar-division"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_LVAR"),
            "ruby/lib/c.rb": 'total /ENV["BASECAMP_LVAR"].to_i / 2\n',
        })
        check("division after a local variable keeps its read visible",
              run_gate(root, RB_SDK), [])

        # The rule is Ruby's, not a general one: TypeScript has no
        # parenthesis-less call, so the same spelling there is arithmetic.
        root = tmp / "ts-not-command-regex"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_TSDIV.\n",
            "typescript/src/c.ts":
                "const n = total /count/ 2;\nconst v = process.env.BASECAMP_TSDIV;\n",
        })
        check("typescript keeps the division reading",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_TSDIV"])

        # Ruby has no triple-quoted literal: `"""x"""` is three adjacent
        # literals concatenated, and the middle one interpolates. Blanking the
        # run as a docstring lost the read inside it.
        root = tmp / "ruby-adjacent-quotes"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_RADJ"),
            "ruby/lib/c.rb": 'x = """#{ENV[\'BASECAMP_RADJ\']}"""\n',
        })
        check("ruby adjacent quotes are not a docstring",
              run_gate(root, RB_SDK), [])

        # A percent literal nested inside an interpolation is data too. Only the
        # top-level scanner knew that, so this came back out as a read.
        root = tmp / "ruby-percent-in-hole"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "ruby/lib/c.rb": 's = "#{%q{ENV[\'BASECAMP_FAKE\']}}"\n',
        })
        check("a percent literal inside an interpolation is not a read",
              run_gate(root, RB_SDK), ["forward:Ruby:BASECAMP_FAKE"])

        # Inside `#{...}` the ordinary rules apply again, so a quoted brace there
        # is a string. Counting it closed the literal early and let the rest of
        # the line -- and the read in it -- be eaten as a `#` comment.
        root = tmp / "ruby-percent-hole-quote"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_RQB"),
            "ruby/lib/c.rb": 's = %Q{#{"}"}, #{ENV["BASECAMP_RQB"]}}\n',
        })
        check("a quoted brace inside a percent hole does not close it",
              run_gate(root, RB_SDK), [])

        # A backtick command literal is data, like any other string...
        root = tmp / "ruby-backtick-data"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "ruby/lib/c.rb": 's = `echo ENV["BASECAMP_FAKE"]`\n',
        })
        check("a ruby backtick literal is not a read",
              run_gate(root, RB_SDK), ["forward:Ruby:BASECAMP_FAKE"])

        # ...and it interpolates, so the `#` must not open a comment.
        root = tmp / "ruby-backtick-interp"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_BTI"),
            "ruby/lib/c.rb": 's = `echo #{ENV["BASECAMP_BTI"]}`\n',
        })
        check("a ruby backtick interpolation is a read",
              run_gate(root, RB_SDK), [])

        # `%` after a value is modulo. Accepting `-` as a delimiter here ran the
        # literal to the end of the file, masking every read after it.
        root = tmp / "ruby-modulo-unary"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_MODU"),
            "ruby/lib/c.rb": '10%-ENV["BASECAMP_MODU"].to_i\n',
        })
        check("modulo before a unary operand is not a percent literal",
              run_gate(root, RB_SDK), [])

        # ...and `%` as modulo must not start one.
        root = tmp / "ruby-modulo"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_MOD"),
            "ruby/lib/c.rb": 'v = ENV["BASECAMP_MOD"]\nx = a % b\n',
        })
        check("a ruby modulo is not a percent literal", run_gate(root, RB_SDK), [])

        # A keyword ends in a letter, which otherwise reads as an identifier and
        # therefore as division.
        root = tmp / "regex-after-keyword"
        build(root, {
            "typescript/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "typescript/src/c.ts":
                "function f() { return /process.env.BASECAMP_FAKE/; }\n",
        })
        check("a regex after a keyword is still a regex",
              run_gate(root, TS_SDK), ["forward:TypeScript:BASECAMP_FAKE"])

        # An escaped delimiter does not close a triple-quoted literal.
        root = tmp / "escaped-triple"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "python/src/c.py": 'X = """a \\""" os.getenv(\'BASECAMP_FAKE\') b"""\n',
        })
        check("an escaped triple quote does not end the literal",
              run_gate(root, PY_SDK), ["forward:Python:BASECAMP_FAKE"])

        # Go backticks are raw: a trailing backslash does not escape the closing
        # delimiter, so the literal ends and the code after it is scanned.
        root = tmp / "go-backtick-raw"
        build(root, {
            "go/README.md": TABLE.format(var="BASECAMP_GO"),
            "go/pkg/c.go": 's := `raw\\`\nv := os.Getenv("BASECAMP_GO")\n',
        })
        check("a go backtick literal is raw", run_gate(root, GO_SDK), [])

        # A raw string keeps the backslash in its value, but the tokenizer still
        # lets it protect a quote: `r"\""` is one literal. Advancing a single
        # character made the escaped quote the terminator and left the rest of
        # the literal executable.
        root = tmp / "raw-escaped-quote"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "python/src/c.py": 'X = r"\\" os.getenv(\'BASECAMP_FAKE\')"\n',
        })
        check("an escaped quote does not end a raw string",
              run_gate(root, PY_SDK), ["forward:Python:BASECAMP_FAKE"])

        # In a raw f-string the backslash is literal, so it must not eat the
        # brace that opens the expression.
        root = tmp / "raw-fstring"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_RAWF"),
            "python/src/c.py": 'v = fr"\\{os.getenv(\'BASECAMP_RAWF\')}"\n',
        })
        check("a raw f-string interpolation is a read", run_gate(root, PY_SDK), [])

        # Destructuring is a read, and every name in the pattern is one.
        root = tmp / "destructured"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_DA and BASECAMP_DB.\n",
            "typescript/src/c.ts": "const { BASECAMP_DA, BASECAMP_DB } = process.env;\n",
        })
        check("destructured reads are seen, all of them",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_DA"])

        # Renaming reads the *key*, not the local name it is bound to. The
        # unanchored lookahead matched identifiers on the value side of `:` too,
        # so this invented a BASECAMP_TOKEN read the code never makes.
        root = tmp / "destructured-alias"
        build(root, {
            "typescript/README.md": TABLE.format(var="BASECAMP_TOKEN"),
            "typescript/src/c.ts": "const { OTHER: BASECAMP_TOKEN } = process.env;\n",
        })
        # The alias is not the key, so the phantom BASECAMP_TOKEN read is gone --
        # and OTHER, which *is* the key, correctly breaks the no-environment
        # claim. Both halves of that are the point.
        check("a destructuring alias is not the key that was read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["forward:TypeScript:BASECAMP_TOKEN", "noenv:TypeScript:OTHER"])

        # ...but destructuring something else is not an environment read.
        root = tmp / "destructured-other"
        build(root, {
            "typescript/README.md": TABLE.format(var="BASECAMP_NOTENV"),
            "typescript/src/c.ts": "const { BASECAMP_NOTENV } = someOtherObject;\n",
        })
        check("destructuring a non-env object is not a read",
              run_gate(root, TS_SDK), ["forward:TypeScript:BASECAMP_NOTENV"])

        # An escaped backslash is not Swift's interpolation marker.
        root = tmp / "swift-escaped-interp"
        build(root, {
            "swift/README.md": "no tables\n",
            "swift/Sources/c.swift":
                'let s = """\n\\\\(ProcessInfo.processInfo.environment["BASECAMP_FAKE"])\n"""\n',
        })
        check("an escaped backslash is not a swift interpolation",
              run_gate(root, SW_SDK, no_env_sdks=("Swift",)), [])

        # A Ruby quoted literal may span physical lines. Ending it at the newline
        # left the rest of the string executable.
        root = tmp / "ruby-multiline-literal"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "ruby/lib/c.rb": 's = "line one\nENV[\'BASECAMP_FAKE\']\nline three"\n',
        })
        check("a ruby literal spanning lines is not a read",
              run_gate(root, RB_SDK), ["forward:Ruby:BASECAMP_FAKE"])

        # ...and code after it is still code.
        root = tmp / "ruby-multiline-then-read"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_REAL"),
            "ruby/lib/c.rb": 's = "a\nb"\nv = ENV[\'BASECAMP_REAL\']\n',
        })
        check("a read after a multiline literal still counts",
              run_gate(root, RB_SDK), [])

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

        # 5b-ii. Example text inside an ordinary string is data, not a call.
        root = tmp / "stringliteral-ts"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts": 'const hint = "process.env.BASECAMP_FAKE";\n',
        })
        check("env syntax inside a string is not a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        root = tmp / "stringliteral-py"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "python/src/c.py": 'HINT = \'os.environ.get("BASECAMP_FAKE")\'\n',
        })
        check("env call quoted inside a string is not a read",
              run_gate(root, PY_SDK), ["forward:Python:BASECAMP_FAKE"])

        # ...while the real call on the very next line still registers, so the
        # mask rejects only the literal and not the file.
        root = tmp / "stringliteral-mixed"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_REALONE"),
            "python/src/c.py": (
                'HINT = "process.env.BASECAMP_FAKE"\n'
                'v = os.environ.get("BASECAMP_REALONE")\n'
            ),
        })
        check("a real read beside a string example still counts",
              run_gate(root, PY_SDK), [])

        # 5b-iii. An interpolation is executable code that happens to sit inside
        #     a literal. Masking it would hide a genuine read — the fail-open
        #     direction — so each language's spelling is exercised.
        root = tmp / "interp-ts"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_TMPL.\n",
            "typescript/src/c.ts": "const label = `token=${process.env.BASECAMP_TMPL}`;\n",
        })
        check("a template-literal interpolation is a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_TMPL"])

        root = tmp / "interp-rb"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_INTERP"),
            "ruby/lib/c.rb": 'v = "#{ENV[\'BASECAMP_INTERP\']}"\n',
        })
        check("a ruby #{} interpolation is a read", run_gate(root, RB_SDK), [])

        root = tmp / "interp-py"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_FSTR"),
            "python/src/c.py": 'v = f"{os.environ.get(\'BASECAMP_FSTR\')}"\n',
        })
        check("a python f-string interpolation is a read", run_gate(root, PY_SDK), [])

        # ...but the same braces without an `f` prefix are literal text.
        root = tmp / "interp-py-plain"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_NOTF"),
            "python/src/c.py": 'v = "{os.environ.get(\'BASECAMP_NOTF\')}"\n',
        })
        check("braces without an f prefix are not a read",
              run_gate(root, PY_SDK), ["forward:Python:BASECAMP_NOTF"])

        # A triple-quoted f-string is not documentation — its braces execute.
        # Blanking every triple-quoted literal outright hid the read entirely.
        root = tmp / "interp-py-triple"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_TRIPLE"),
            "python/src/c.py": 'X = f"""token={os.getenv(\'BASECAMP_TRIPLE\')}"""\n',
        })
        check("a triple-quoted f-string interpolation is a read",
              run_gate(root, PY_SDK), [])

        # A literal *inside* an interpolation is data again, so un-masking the
        # hole wholesale wrongly promoted example text to a read.
        root = tmp / "interp-nested-literal"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts": 'const x = `${"process.env.BASECAMP_FAKE"}`;\n',
        })
        check("a literal nested in an interpolation is not a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        root = tmp / "interp-swift"
        build(root, {
            "swift/README.md": "The SDK reads BASECAMP_SW.\n",
            "swift/Sources/c.swift":
                'let s = "tok=\\(ProcessInfo.processInfo.environment["BASECAMP_SW"])"\n',
        })
        check("a swift \\() interpolation is a read",
              run_gate(root, SW_SDK, no_env_sdks=("Swift",)), ["noenv:Swift:BASECAMP_SW"])

        # 5b-iv. Language-specific lexical forms. Each of these was a real miss:
        #     the scanner is driven by per-SDK flags, not by comment style alone.
        root = tmp / "kotlin-multiline"
        build(root, {
            "kotlin/README.md": "no tables\n",
            "kotlin/sdk/src/c.kt":
                'val s = """\n    System.getenv("BASECAMP_FAKE")\n    """.trimIndent()\n',
        })
        check("kotlin multiline string body is not a read",
              run_gate(root, KT_SDK, no_env_sdks=("Kotlin",)), [])

        # ...but an interpolation inside that multiline string still is.
        root = tmp / "kotlin-multiline-interp"
        build(root, {
            "kotlin/README.md": "The SDK reads BASECAMP_REAL.\n",
            "kotlin/sdk/src/c.kt": 'val s = """\ntok=${System.getenv("BASECAMP_REAL")}\n"""\n',
        })
        check("kotlin multiline interpolation is a read",
              run_gate(root, KT_SDK, no_env_sdks=("Kotlin",)),
              ["noenv:Kotlin:BASECAMP_REAL"])

        root = tmp / "swift-multiline"
        build(root, {
            "swift/README.md": "no tables\n",
            "swift/Sources/c.swift":
                'let s = """\nProcessInfo.processInfo.environment["BASECAMP_FAKE"]\n"""\n',
        })
        check("swift multiline string body is not a read",
              run_gate(root, SW_SDK, no_env_sdks=("Swift",)), [])

        root = tmp / "swift-raw"
        build(root, {
            "swift/README.md": "The SDK reads BASECAMP_RAW.\n",
            "swift/Sources/c.swift":
                'let t = #"\\#(ProcessInfo.processInfo.environment["BASECAMP_RAW"]!)"#\n',
        })
        check("swift raw-string interpolation is a read",
              run_gate(root, SW_SDK, no_env_sdks=("Swift",)),
              ["noenv:Swift:BASECAMP_RAW"])

        root = tmp / "brace-in-nested-literal"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_SECRET.\n",
            "typescript/src/c.ts": 'const x = `${foo("}", process.env.BASECAMP_SECRET)}`;\n',
        })
        check("a quoted brace does not truncate the interpolation",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_SECRET"])

        # A delimiter inside a comment is not a delimiter, same as inside a
        # literal — otherwise the hole is truncated and the read behind it hides.
        root = tmp / "comment-in-hole"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_SECRET.\n",
            "typescript/src/c.ts":
                'const x = `${foo(/* } */ process.env.BASECAMP_SECRET)}`;\n',
        })
        check("a commented brace does not truncate the interpolation",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_SECRET"])

        # A comment inside an interpolation is not code. The hole is un-masked
        # wholesale, so without blanking it the example text counts as a read.
        root = tmp / "comment-inside-hole"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts":
                'const x = `${foo(/* process.env.BASECAMP_FAKE */ 1)}`;\n',
        })
        check("a comment inside an interpolation is not a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        root = tmp / "comment-inside-hole-rb"
        build(root, {
            "ruby/README.md": TABLE.format(var="BASECAMP_FAKE"),
            "ruby/lib/c.rb": 'v = "#{foo( # ENV[\'BASECAMP_FAKE\']\n 1)}"\n',
        })
        check("a ruby comment inside an interpolation is not a read",
              run_gate(root, RB_SDK), ["forward:Ruby:BASECAMP_FAKE"])

        # A Swift raw string is a valid dictionary key, so the read pattern has
        # to accept the fences the lexer already understands.
        root = tmp / "swift-raw-key"
        build(root, {
            "swift/README.md": "The SDK reads BASECAMP_RAWKEY.\n",
            "swift/Sources/c.swift":
                'let v = ProcessInfo.processInfo.environment[#"BASECAMP_RAWKEY"#]\n',
        })
        check("a swift raw-string environment key is a read",
              run_gate(root, SW_SDK, no_env_sdks=("Swift",)),
              ["noenv:Swift:BASECAMP_RAWKEY"])

        # Interpolation is per language *and* per quote: TypeScript interpolates
        # in backticks only, so this is ordinary text and not a read.
        root = tmp / "ts-dq-not-interp"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts": 'const example = "${process.env.BASECAMP_FAKE}";\n',
        })
        check("typescript double quotes do not interpolate",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        # ...while the identical bytes in Kotlin are code.
        root = tmp / "kotlin-dq-interp"
        build(root, {
            "kotlin/README.md": "The SDK reads BASECAMP_REAL.\n",
            "kotlin/sdk/src/c.kt": 'val v = "${System.getenv("BASECAMP_REAL")}"\n',
        })
        check("kotlin double quotes do interpolate",
              run_gate(root, KT_SDK, no_env_sdks=("Kotlin",)),
              ["noenv:Kotlin:BASECAMP_REAL"])

        # Go interpolates nowhere, so neither spelling is a read.
        root = tmp / "go-no-interp"
        build(root, {
            "go/README.md": "no tables\n",
            "go/pkg/c.go": 'v := "${os.Getenv(\\"BASECAMP_FAKE\\")}"\n',
        })
        check("go strings never interpolate", run_gate(root, GO_SDK), [])

        root = tmp / "swift-nested-comment"
        build(root, {
            "swift/README.md": "no tables\n",
            "swift/Sources/c.swift":
                '/* outer /* inner */ ProcessInfo.processInfo.environment["BASECAMP_DOC"] */\n',
        })
        check("swift nested block comment stays a comment",
              run_gate(root, SW_SDK, no_env_sdks=("Swift",)), [])

        # ...while TypeScript does *not* nest, so code after the inner `*/` is
        # code. Nesting it everywhere would have swallowed a real read.
        root = tmp / "ts-unnested-comment"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_AFTER.\n",
            "typescript/src/c.ts": '/* outer /* inner */ const v = process.env.BASECAMP_AFTER;\n',
        })
        check("typescript block comments do not nest",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_AFTER"])

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
            "typescript/README.md": "The SDK reads BASECAMP_MULTI.\n",
            "typescript/src/c.ts": 'const v = process.env[\n  "BASECAMP_MULTI"\n];\n',
        })
        check("multiline typescript read breaks the no-env claim",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_MULTI"])

        # 6. The no-env claim breaks on a real read.
        root = tmp / "noenv"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_REAL.\n",
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

        # 8b. Prose attribution (invariant 5). The tables were never where the
        #     original XDG bug lived — it was a sentence — so a true sentence
        #     must pass and a false one must fail.
        root = tmp / "prose-true"
        build(root, {
            "python/README.md": "Python reads `BASECAMP_REALPROSE` on request.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_REALPROSE")\n',
        })
        check("true prose attribution passes", run_gate(root, PY_SDK), [])

        root = tmp / "prose-false"
        build(root, {
            "python/README.md": "Python reads `BASECAMP_PROSEPHANTOM` on request.\n",
            "python/src/c.py": "v = 1\n",
        })
        check("false prose attribution is caught",
              run_gate(root, PY_SDK), ["prose:Python:BASECAMP_PROSEPHANTOM"])

        # Two claims on one line must split at the second SDK, not pool their
        # variables — this is the shape the root README actually ships.
        root = tmp / "prose-split-ok"
        build(root, {
            "python/README.md": "Python reads `BASECAMP_ONE` and Ruby reads `BASECAMP_TWO` here.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_ONE")\n',
            "ruby/README.md": "The SDK reads BASECAMP_TWO.\n",
            "ruby/lib/c.rb": 'v = ENV["BASECAMP_TWO"]\n',
        })
        check("a two-SDK sentence attributes each variable to its own SDK",
              run_gate(root, PY_RB), [])

        # ...and the second clause is really checked, not merely parsed.
        root = tmp / "prose-split-bad"
        build(root, {
            "python/README.md": "Python reads `BASECAMP_ONE` and Ruby reads `BASECAMP_TWO` here.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_ONE")\n',
            "ruby/README.md": "The SDK reads BASECAMP_TWO.\n",
            "ruby/lib/c.rb": "v = 1\n",
        })
        check("a false second clause is caught",
              run_gate(root, PY_RB), ["prose:Ruby:BASECAMP_TWO"])

        # The passive voice names a symbol, not an SDK, and must not fire —
        # go/README.md ships exactly this sentence.
        root = tmp / "prose-passive"
        build(root, {
            "python/README.md": "`BASECAMP_PASSIVE` is read by `AuthManager.AccessToken`.\n",
            "python/src/c.py": "v = 1\n",
        })
        check("passive voice is not an SDK attribution", run_gate(root, PY_SDK), [])

        # Likewise a symbol subject: python/README.md says "a plain `Config()`
        # reads no environment at all".
        root = tmp / "prose-symbol-subject"
        build(root, {
            "python/README.md": "A plain `Config()` reads no environment; `BASECAMP_NOPE` is inert.\n",
            "python/src/c.py": "v = 1\n",
        })
        check("a symbol subject is not an SDK attribution", run_gate(root, PY_SDK), [])

        # The literal sentence this gate was built to catch: compound subject,
        # verb "use" rather than "reads", and the variables *before* the subject
        # in a trailing relative clause. Ruby has never read XDG_CACHE_HOME.
        root = tmp / "prose-original-defect"
        build(root, {
            "ruby/README.md": (
                "What the SDKs read on their own (the XDG directory variables aside: "
                "`XDG_CACHE_HOME` / `XDG_CONFIG_HOME`, which Ruby uses to site its "
                "cache and config directories):\n"
            ),
            "ruby/lib/c.rb": 'd = ENV["XDG_CONFIG_HOME"]\n',
        })
        check("the original 'uses' + variables-first sentence is caught",
              run_gate(root, RB_SDK), ["prose:Ruby:XDG_CACHE_HOME"])

        # A compound subject is checked against every SDK it names, not just the
        # first — Ruby is the false half here.
        root = tmp / "prose-compound"
        build(root, {
            "python/README.md": "`BASECAMP_BOTH`, which Python and Ruby use, matters.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_BOTH")\n',
            "ruby/README.md": "The SDK reads BASECAMP_BOTH.\n",
            "ruby/lib/c.rb": "v = 1\n",
        })
        check("a compound subject is checked against each SDK named",
              run_gate(root, PY_RB), ["prose:Ruby:BASECAMP_BOTH"])

        # The backward pass must not reach across a sentence boundary into the
        # previous claim's variables.
        root = tmp / "prose-backward-bounded"
        build(root, {
            "python/README.md": "Python reads `BASECAMP_MINE`. Ruby uses a config file.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_MINE")\n',
            "ruby/README.md": "no tables\n",
            "ruby/lib/c.rb": "v = 1\n",
        })
        check("the backward pass stops at the sentence boundary",
              run_gate(root, PY_RB), [])

        # Markdown reflows, so an attribution can wrap mid-sentence. A per-line
        # scan finds no claim at all and invariant 5 silently stops enforcing it.
        root = tmp / "prose-wrapped"
        build(root, {
            "python/README.md": "Python reads\n`BASECAMP_WRAPPED` on request.\n",
            "python/src/c.py": "v = 1\n",
        })
        check("a claim wrapped across lines is still checked",
              run_gate(root, PY_SDK), ["prose:Python:BASECAMP_WRAPPED"])

        # The variables-first form has to survive wrapping too.
        root = tmp / "prose-wrapped-backward"
        build(root, {
            "ruby/README.md": "`XDG_CACHE_HOME`, which\nRuby uses to site its cache.\n",
            "ruby/lib/c.rb": "v = 1\n",
        })
        check("a wrapped variables-first claim is still checked",
              run_gate(root, RB_SDK), ["prose:Ruby:XDG_CACHE_HOME"])

        # A blank line still separates paragraphs, so an unrelated later
        # paragraph cannot lend its variables to an earlier claim.
        root = tmp / "prose-paragraph-break"
        build(root, {
            "python/README.md": "Python reads nothing here.\n\n`BASECAMP_ELSEWHERE` is unrelated.\n",
            "python/src/c.py": "v = 1\n",
        })
        check("a blank line ends the paragraph", run_gate(root, PY_SDK), [])

        # Fenced examples are not claims.
        root = tmp / "prose-fenced"
        build(root, {
            "python/README.md": "intro\n\n```\nPython reads `BASECAMP_INFENCE`\n```\n",
            "python/src/c.py": "v = 1\n",
        })
        check("a fenced example is not a prose claim", run_gate(root, PY_SDK), [])

        # 9. Out-of-family variables are ignored; the gate only polices
        #    BASECAMP_*/XDG_*, not every env var an SDK might touch.
        root = tmp / "outoffamily"
        build(root, {
            "python/README.md": "no tables\n",
            "python/src/c.py": 'v = os.environ.get("HTTP_PROXY")\n',
        })
        check("unrelated env vars are ignored", run_gate(root, PY_SDK), [])

        # 10. Where a newline does and does not end a statement. Both of these
        #     are the same bytes in the two languages that spell a regex `/.../`,
        #     and they mean opposite things, so one rule cannot serve both.
        #
        #     JavaScript inserts no semicolon before `/`, because the slash
        #     parses as a continuation of the expression above it. So this is
        #     division and the read between the two slashes is real; treating
        #     the line-leading slash as a regex opener masks it, and an
        #     undocumented variable ships.
        root = tmp / "ts-newline-division"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_REAL.\n",
            "typescript/src/c.ts":
                "const ratio = 10\n/ process.env.BASECAMP_REAL / 2;\n",
        })
        check("a typescript division survives the line break",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_REAL"])

        # ...while a slash that continues an operator really does open a regex,
        # so the read spelled inside it is example text and not a read.
        root = tmp / "ts-newline-regex"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts":
                "const re =\n  /process.env.BASECAMP_FAKE/;\n",
        })
        check("a typescript regex after a line-broken operator is not a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        # Ruby is the other way round: a complete expression ends at the newline,
        # so a line-leading slash is a regex and its body is data.
        root = tmp / "ruby-newline-regex"
        build(root, {
            "ruby/README.md": "no tables\n",
            "ruby/lib/c.rb": "n = 10\n/ENV['BASECAMP_FAKE']/.match(s)\n",
        })
        check("a ruby line-leading slash is a regex",
              run_gate(root, RB_SDK), [])

        # 11. A `)` that closes a control-flow condition is followed by a
        #     statement, and a statement may begin with a regex. Reading it as
        #     division leaves the pattern text executable and invents a read.
        root = tmp / "ts-condition-regex"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts":
                "if (ready) /process.env.BASECAMP_FAKE/.test(value);\n",
        })
        check("a regex after a control-flow condition is not a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        # ...but every other `)` still ends a value, so the slash after it stays
        # division. This is the fail-open direction of the rule above: guess
        # "regex" here and the read between the slashes disappears.
        root = tmp / "ts-call-division"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_REAL.\n",
            "typescript/src/c.ts":
                "const n = size() / process.env.BASECAMP_REAL / 2;\n",
        })
        check("division after a call is not a condition regex",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_REAL"])

        # ...and the keyword has to be a statement head, not a property that
        # merely spells one. `obj.if(...)` is a legal call, so what follows it
        # is an expression and the slash is division.
        root = tmp / "ts-property-named-if"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_REAL.\n",
            "typescript/src/c.ts":
                "const n = obj.if(ready) / process.env.BASECAMP_REAL / 2;\n",
        })
        check("a property spelled like a keyword is not a condition",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_REAL"])

        # Ruby has statement modifiers, so `if (x) / 2` really is division
        # inside a trailing condition. The rule above is therefore TypeScript's
        # alone -- applying it here would mask the read in that condition.
        root = tmp / "ruby-modifier-if-division"
        build(root, {
            "ruby/README.md": "no tables\n",
            "ruby/lib/c.rb": "warn 'x' if (limit) / ENV['BASECAMP_REAL'].to_i / 2 > 1\n",
        })
        check("a ruby modifier-if condition keeps its division",
              run_gate(root, RB_SDK), ["reverse:Ruby:BASECAMP_REAL"])

        # 12. Kotlin's triple-quoted string is raw: a backslash is ordinary data
        #     and does not escape the `$` after it, so `${...}` still executes.
        #     Consuming the pair as an escape hid a real read -- and the root
        #     README's "Kotlin reads no environment variables" stayed true-looking.
        root = tmp / "kotlin-raw-escaped-dollar"
        build(root, {
            "kotlin/README.md": "The SDK reads BASECAMP_REAL.\n",
            "kotlin/sdk/src/c.kt":
                'val s = """\\${System.getenv("BASECAMP_REAL")}"""\n',
        })
        check("a backslash does not escape kotlin raw interpolation",
              run_gate(root, KT_SDK, no_env_sdks=("Kotlin",)),
              ["noenv:Kotlin:BASECAMP_REAL"])

        # Swift's plain `"""` is not raw, so there the backslash does escape and
        # `\\(` is literal text rather than an interpolation. Same shape, opposite
        # answer, which is why this is a per-language flag and not a global rule.
        root = tmp / "swift-multiline-escaped-interp"
        build(root, {
            "swift/README.md": "no tables\n",
            "swift/Sources/c.swift":
                'let s = """\n\\\\(ProcessInfo.processInfo.environment["BASECAMP_FAKE"])\n"""\n',
        })
        check("a backslash still escapes swift multiline interpolation",
              run_gate(root, SW_SDK, no_env_sdks=("Swift",)), [])

        # 13. `++` and `--` end a value, so the slash after one is division --
        #     but both of their characters are in REGEX_PRECEDERS, so the
        #     single-character heuristic called it a regex and masked the read
        #     between the slashes. Fail-open.
        root = tmp / "ts-postfix-division"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_REAL.\n",
            "typescript/src/c.ts":
                "let x = 1;\nconst n = x++ / process.env.BASECAMP_REAL / 2;\n",
        })
        check("division after a postfix increment keeps its read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_REAL"])

        # ...while a single `+` really does expect an operand, so the regex
        # reading must survive for it.
        root = tmp / "ts-plus-regex"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts":
                'const s = "" + /process.env.BASECAMP_FAKE/.source;\n',
        })
        check("a regex after a single plus is still a regex",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        # 14. "Reads no environment variables at all" is broken by a grab of the
        #     whole environment too, and that names no variable -- so the read
        #     patterns, which all require a quoted name, could never see it.
        root = tmp / "kotlin-whole-env"
        build(root, {
            "kotlin/README.md": "no tables\n",
            "kotlin/sdk/src/c.kt": 'val t = System.getenv()["BASECAMP_TOKEN"]\n',
        })
        check("a whole-environment grab breaks the no-env claim",
              run_gate(root, KT_SDK, no_env_sdks=("Kotlin",)), ["envapi:Kotlin"])

        root = tmp / "ts-whole-env"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts": "const cfg = { ...process.env };\n",
        })
        check("a spread of process.env breaks the no-env claim",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), ["envapi:TypeScript"])

        root = tmp / "swift-whole-env"
        build(root, {
            "swift/README.md": "no tables\n",
            "swift/Sources/c.swift":
                "let all = ProcessInfo.processInfo.environment\n",
        })
        check("passing the whole swift environment breaks the claim",
              run_gate(root, SW_SDK, no_env_sdks=("Swift",)), ["envapi:Swift"])

        # ...but the READMEs' own examples spell these APIs in prose and code
        # blocks, and the SDK sources carry them in doc comments. Counting those
        # would fail CI on a correct tree, which is how a gate gets deleted.
        root = tmp / "ts-whole-env-in-comment"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts":
                '// Callers do `process.env.BASECAMP_TOKEN` themselves.\n'
                'const s = "process.env";\n',
        })
        check("a documented environment API is not a touch",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        # 15. `!` is postfix in TypeScript's non-null assertion, so `value! / x`
        #     is division -- and prefix in negation, so `!/re/.test(s)` is a
        #     regex. Same character, opposite answers, decided by what is in
        #     front of it.
        root = tmp / "ts-nonnull-division"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_REAL.\n",
            "typescript/src/c.ts":
                "const n = value! / process.env.BASECAMP_REAL / 2;\n",
        })
        check("division after a non-null assertion keeps its read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_REAL"])

        root = tmp / "ts-negated-regex"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts":
                "const bad = !/process.env.BASECAMP_FAKE/.test(s);\n",
        })
        check("a negated regex is still a regex",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        # ...and the word matters, not its last letter: `return` ends in `n`,
        # which as a bare character would read as a value and mask the regex.
        root = tmp / "ts-return-negated-regex"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts":
                "function f(s: string) { return !/process.env.BASECAMP_FAKE/.test(s); }\n",
        })
        check("a negated regex after a keyword is still a regex",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        # Ruby spells both meanings too, and the word lists separate them there
        # as well: a bang method ends a value, a command-form call negates what
        # follows it.
        root = tmp / "ruby-bang-method-division"
        build(root, {
            "ruby/README.md": "no tables\n",
            "ruby/lib/c.rb": "n = save! / ENV['BASECAMP_REAL'].to_i / 2\n",
        })
        check("a ruby bang method ends a value",
              run_gate(root, RB_SDK), ["reverse:Ruby:BASECAMP_REAL"])

        root = tmp / "ruby-command-negated-regex"
        build(root, {
            "ruby/README.md": "no tables\n",
            "ruby/lib/c.rb": "puts !/ENV['BASECAMP_FAKE']/.match(s)\n",
        })
        check("a negated regex after a ruby command is still a regex",
              run_gate(root, RB_SDK), [])

        # 16. `process["env"]` is the same object as `process.env`. Node does not
        #     care which spelling reaches it, so neither may the gate -- in the
        #     named-read patterns and in the whole-environment detector both.
        root = tmp / "ts-bracket-env-named"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_REAL.\n",
            "typescript/src/c.ts": 'const t = process["env"].BASECAMP_REAL;\n',
        })
        check("a bracket-spelled env property is a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_REAL"])

        root = tmp / "ts-bracket-env-subscript"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_REAL.\n",
            "typescript/src/c.ts": 'const t = process["env"]["BASECAMP_REAL"];\n',
        })
        check("a fully bracket-spelled env read is a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_REAL"])

        root = tmp / "ts-bracket-env-destructured"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_REAL.\n",
            "typescript/src/c.ts": 'const { BASECAMP_REAL } = process["env"];\n',
        })
        check("destructuring a bracket-spelled env is a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_REAL"])

        root = tmp / "ts-bracket-env-wholesale"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts": 'const all = process["env"];\n',
        })
        check("a bracket-spelled whole environment breaks the claim",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), ["envapi:TypeScript"])

        # ...but `process` has other properties, and none of them are the
        # environment. Matching them would invent reads out of ordinary code.
        root = tmp / "ts-other-process-property"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts":
                'const v = process["version"];\nconst e = process.envelope;\n',
        })
        check("other process properties are not the environment",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        # `process?.env` is valid optional chaining and the same object again.
        root = tmp / "ts-optional-chained-process"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_REAL.\n",
            "typescript/src/c.ts": "const t = process?.env.BASECAMP_REAL;\n",
        })
        check("optional chaining before env is a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_REAL"])

        root = tmp / "ts-optional-chained-bracket"
        build(root, {
            "typescript/README.md": "The SDK reads BASECAMP_REAL.\n",
            "typescript/src/c.ts": 'const t = process?.["env"]?.BASECAMP_REAL;\n',
        })
        check("optional chaining with a bracket env is a read",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_REAL"])

        # 17. `from os import getenv` is the ordinary Python spelling, and the
        #     `os.`-qualified patterns reported those reads as nonexistent.
        root = tmp / "py-unqualified-getenv"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_UNQ"),
            "python/src/c.py":
                'from os import getenv\n\nv = getenv("BASECAMP_UNQ")\n',
        })
        check("an unqualified getenv is a read", run_gate(root, PY_SDK), [])

        root = tmp / "py-unqualified-environ"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_ENVQ"),
            "python/src/c.py":
                'from os import environ\n\nv = environ.get("BASECAMP_ENVQ")\n'
                'w = environ["BASECAMP_ENVQ"]\n',
        })
        check("an unqualified environ is a read", run_gate(root, PY_SDK), [])

        # ...and a differently-named function is not one, so an unrelated
        # helper cannot be promoted into the inventory.
        root = tmp / "py-lookalike-getenv"
        build(root, {
            "python/README.md": "no tables\n",
            "python/src/c.py": 'v = mygetenv("BASECAMP_FAKE")\n',
        })
        check("a lookalike helper is not an env read", run_gate(root, PY_SDK), [])

        # ...and neither is a same-named function from somewhere else. Whether
        # `getenv(...)` reads the environment is a fact about the file's
        # imports, so matching the bare name regardless invents a read.
        root = tmp / "py-foreign-getenv"
        build(root, {
            "python/README.md": "no tables\n",
            "python/src/c.py":
                'from helpers import getenv\n\nv = getenv("BASECAMP_FAKE")\n',
        })
        check("an unqualified getenv not imported from os is not a read",
              run_gate(root, PY_SDK), [])

        # Reading the imports settles aliases too, which no fixed pattern could.
        root = tmp / "py-aliased-getenv"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_ALIAS"),
            "python/src/c.py":
                'from os import getenv as ge\n\nv = ge("BASECAMP_ALIAS")\n',
        })
        check("an aliased os import is still a read", run_gate(root, PY_SDK), [])

        root = tmp / "py-aliased-environ"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_ENVA"),
            "python/src/c.py":
                'from os import environ as en\n\nv = en["BASECAMP_ENVA"]\n',
        })
        check("an aliased environ import is still a read",
              run_gate(root, PY_SDK), [])

        # 18. `const { env } = process` aliases the whole environment without
        #     ever spelling `process.env`.
        root = tmp / "ts-destructured-env-alias"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts":
                "const { env } = process;\nconst t = env.BASECAMP_TOKEN;\n",
        })
        check("destructuring env off process breaks the claim",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), ["envapi:TypeScript"])

        # ...but destructuring something else off it is not the environment.
        root = tmp / "ts-destructured-other"
        build(root, {
            "typescript/README.md": "no tables\n",
            "typescript/src/c.ts": "const { argv, pid } = process;\n",
        })
        check("destructuring other process fields is not the environment",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)), [])

        # 19. Tilde fences are code blocks too, and a comment inside one is not
        #     documentation. Left in prose, `# The SDK uses BASECAMP_TILDE`
        #     satisfied the reverse check for a read nothing else described.
        root = tmp / "tilde-fence"
        build(root, {
            "python/README.md":
                "intro\n\n~~~python\n# The SDK uses BASECAMP_TILDE\n~~~\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_TILDE")\n',
        })
        check("a tilde-fenced example is not documentation",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_TILDE"])

        # ...and the fence closes only on its own character, so a ``` line
        # inside a ~~~ block must not end it and hand back the rest as prose.
        root = tmp / "tilde-fence-mixed"
        build(root, {
            "python/README.md":
                "intro\n\n~~~\n```\n# The SDK uses BASECAMP_MIXED\n```\n~~~\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_MIXED")\n',
        })
        check("a backtick line inside a tilde fence does not end it",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_MIXED"])

        # A closing fence must be at least as long as the one that opened it,
        # so a ``` line inside a ```` block is content rather than its end.
        root = tmp / "long-fence"
        build(root, {
            "python/README.md":
                "intro\n\n````\n```\n# The SDK uses BASECAMP_LONG\n```\n````\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_LONG")\n',
        })
        check("a short fence does not close a longer one",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_LONG"])

        # A closing fence carries nothing but its run and trailing spaces, so a
        # ```-prefixed word inside a block is code rather than its end.
        root = tmp / "fence-info-string"
        build(root, {
            "python/README.md":
                "intro\n\n```\n```not-a-close\n# The SDK uses BASECAMP_INFO\n```\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_INFO")\n',
        })
        check("an info-string line does not close a fence",
              run_gate(root, PY_SDK), ["reverse:Python:BASECAMP_INFO"])

        # Swift wraps long member chains across lines, and the dots stay dots.
        root = tmp / "swift-wrapped-chain"
        build(root, {
            "swift/README.md": "The SDK reads BASECAMP_REAL.\n",
            "swift/Sources/c.swift":
                'let t = ProcessInfo\n  .processInfo\n  .environment["BASECAMP_REAL"]\n',
        })
        check("a wrapped swift environment chain is a read",
              run_gate(root, SW_SDK, no_env_sdks=("Swift",)),
              ["noenv:Swift:BASECAMP_REAL"])

        # `import os as X` rebinds the module, moving every qualified spelling.
        root = tmp / "py-module-alias"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_MODALIAS"),
            "python/src/c.py":
                'import os as operating_system\n\n'
                'v = operating_system.getenv("BASECAMP_MODALIAS")\n',
        })
        check("an aliased os module still binds", run_gate(root, PY_SDK), [])

        # A parenthesised import runs over as many lines as it likes, and a
        # backslash continues one. Both are ordinary Python.
        root = tmp / "py-paren-import"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_PAREN"),
            "python/src/c.py":
                'from os import (\n    getenv,\n)\n\nv = getenv("BASECAMP_PAREN")\n',
        })
        check("a parenthesised os import still binds",
              run_gate(root, PY_SDK), [])

        root = tmp / "py-backslash-import"
        build(root, {
            "python/README.md": TABLE.format(var="BASECAMP_BSLASH"),
            "python/src/c.py":
                'from os import \\\n    getenv\n\nv = getenv("BASECAMP_BSLASH")\n',
        })
        check("a backslash-continued os import still binds",
              run_gate(root, PY_SDK), [])

        # 20. The shipped entrypoint, end to end. Everything above drives the
        #     helpers `main()` drives, so until here the exit code that `make
        #     check` and the CI step branch on had no coverage at all -- a
        #     `main()` that collected every failure and then returned 0 would
        #     have passed this whole suite.
        root = tmp / "main-consistent"
        build(root, {
            "README.md": "Python reads `BASECAMP_MAIN`.\n",
            "python/README.md": TABLE.format(var="BASECAMP_MAIN"),
            "python/src/c.py": 'v = os.environ.get("BASECAMP_MAIN")\n',
        })
        check("main() exits 0 on a consistent repo", run_main(root, PY_SDK), 0)

        root = tmp / "main-phantom"
        build(root, {
            "README.md": "Python reads `BASECAMP_PHANTOM`.\n",
            "python/README.md": TABLE.format(var="BASECAMP_PHANTOM"),
            "python/src/c.py": "v = 1\n",
        })
        check("main() exits 1 on a phantom table row", run_main(root, PY_SDK), 1)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    if FAILURES:
        print(f"\n{len(FAILURES)} self-test failure(s): {', '.join(FAILURES)}", file=sys.stderr)
        return 1
    print("\ncheck-readme-env-vars self-test passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
