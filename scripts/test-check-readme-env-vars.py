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
            for _, claimed_sdk, named in gate.prose_claims(readme):
                for var in named:
                    if var not in reads.get(claimed_sdk, {}):
                        failures.append(f"prose:{claimed_sdk}:{var}")
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
            "typescript/README.md": "mentions BASECAMP_TMPL\n",
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
            "swift/README.md": "mentions BASECAMP_SW\n",
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
            "kotlin/README.md": "mentions BASECAMP_REAL\n",
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
            "swift/README.md": "mentions BASECAMP_RAW\n",
            "swift/Sources/c.swift":
                'let t = #"\\#(ProcessInfo.processInfo.environment["BASECAMP_RAW"]!)"#\n',
        })
        check("swift raw-string interpolation is a read",
              run_gate(root, SW_SDK, no_env_sdks=("Swift",)),
              ["noenv:Swift:BASECAMP_RAW"])

        root = tmp / "brace-in-nested-literal"
        build(root, {
            "typescript/README.md": "mentions BASECAMP_SECRET\n",
            "typescript/src/c.ts": 'const x = `${foo("}", process.env.BASECAMP_SECRET)}`;\n',
        })
        check("a quoted brace does not truncate the interpolation",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_SECRET"])

        # A delimiter inside a comment is not a delimiter, same as inside a
        # literal — otherwise the hole is truncated and the read behind it hides.
        root = tmp / "comment-in-hole"
        build(root, {
            "typescript/README.md": "mentions BASECAMP_SECRET\n",
            "typescript/src/c.ts":
                'const x = `${foo(/* } */ process.env.BASECAMP_SECRET)}`;\n',
        })
        check("a commented brace does not truncate the interpolation",
              run_gate(root, TS_SDK, no_env_sdks=("TypeScript",)),
              ["noenv:TypeScript:BASECAMP_SECRET"])

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
            "kotlin/README.md": "mentions BASECAMP_REAL\n",
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
            "typescript/README.md": "mentions BASECAMP_AFTER\n",
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
            "ruby/README.md": "mentions BASECAMP_TWO\n",
            "ruby/lib/c.rb": 'v = ENV["BASECAMP_TWO"]\n',
        })
        check("a two-SDK sentence attributes each variable to its own SDK",
              run_gate(root, PY_RB), [])

        # ...and the second clause is really checked, not merely parsed.
        root = tmp / "prose-split-bad"
        build(root, {
            "python/README.md": "Python reads `BASECAMP_ONE` and Ruby reads `BASECAMP_TWO` here.\n",
            "python/src/c.py": 'v = os.environ.get("BASECAMP_ONE")\n',
            "ruby/README.md": "mentions BASECAMP_TWO\n",
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
            "ruby/README.md": "mentions BASECAMP_BOTH\n",
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
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    if FAILURES:
        print(f"\n{len(FAILURES)} self-test failure(s): {', '.join(FAILURES)}", file=sys.stderr)
        return 1
    print("\ncheck-readme-env-vars self-test passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
