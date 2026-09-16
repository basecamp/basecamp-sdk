#!/usr/bin/env python3
"""Self-test for check-orphaned-doc-comments.py, driven by synthetic trees.

The live run only ever proves the gate can say YES. Everything that makes this
gate worth having is in the cases where it must say NO, and a false negative is
worse here than no gate at all: it turns "nobody looked" into "the checker says
it is fine". Two such cases were already missed by hand on the branch that found
the defect:

  - An orphan whose follower is a ONE-LINE doc comment. A one-line `/** ... */`
    both opens and closes, so a pattern looking for a multi-line opener walks
    past it. That is the shape of `ServiceAccessors.kt`, the instance the sweep
    was actually run to find.
  - An orphan with a blank line before the follower. The first sweep matched
    `*/` immediately followed by `/**` and saw none of the real instances.

Review then found a further run of them in the gate ITSELF, across three rounds
and two reviewers. They fall into four classes, and each instance below is a
case rather than a note in a commit message — every one checked against the
previous version of the gate, to confirm it is a regression test and not
decoration:

  - **A literal the scanner closed in the wrong place**, which then blinds it
    for the rest of the file. A Kotlin raw string closes on the LAST three
    quotes of a run (`Pagination.kt` line 118 holds one, so 58% of the file this
    gate was written to sweep was unreadable by it); a Java text block closes on
    the first UNESCAPED delimiter instead, because unlike Kotlin it processes
    escapes. One rule for both was wrong twice.
  - **A form the scanner did not know was a form.** A regex literal can carry a
    quote (`/["\'<>&]/` in `mentions.ts`, 76% of that file), a BACKTICK (also in
    `mentions.ts` — and a backtick may cross lines, so bounding the quote only
    moved the hole), or an escaped slash read as a `//` comment (`pkce.ts`,
    `sgid.ts`). Four positions where the previous-token heuristic guesses
    division are covered here, each with a backtick in the regex.
  - **A language left out or let in wrongly.** Java was omitted although
    `spec/smithy-bare-arrays` documents with Javadoc; JSX was in the table
    although the scanner cannot read it.
  - **A false positive**, the direction that gets a gate deleted: a UTF-8 BOM
    costing a correct file its exemption, a JSDoc convention leaking into
    Kotlin, and a stray quote handing a real string's contents to the scanner
    as code.

The scanner now also REPORTS what it cannot read: an unterminated block comment
or triple-quoted literal is a gate defect, and a gate defect that presents as
"clean" is the thing this file exists to prevent.

The opposite direction is tested just as hard, because a gate that fails on
correct source gets deleted: `/*` banners (the FIX this gate steers toward),
empty `/**/` comments, a `/**` written inside a string or a template literal,
a block that closes mid-line, CRLF, and a BOM all have to pass. Each of those
cases was checked to FLIP when the rule it guards is removed — the first
template-literal case did not, and guarded nothing.
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

spec = importlib.util.spec_from_file_location("gate", HERE / "check-orphaned-doc-comments.py")
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

FAILURES: list[str] = []


def build(root: Path, files: dict[str, str]) -> None:
    for rel, body in files.items():
        path = root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(body, encoding="utf-8")


def check(name: str, actual, expected) -> None:
    if actual == expected:
        print(f"  ok   {name}")
    else:
        print(f"  FAIL {name}\n         expected: {expected}\n         actual:   {actual}")
        FAILURES.append(name)


def run(tmp: Path, name: str, files: dict[str, str]) -> list[str]:
    """Build a synthetic tree and return the gate's findings, identity only.

    The remediation half of each message is dropped so every case can assert on
    something short. That would let the remediation text rot unnoticed, so
    `messages_are_actionable` below asserts the whole of both message shapes.
    """
    root = tmp / name
    root.mkdir(parents=True)
    build(root, files)
    return [f.split(" -- ")[0].split(": doc comment")[0] for f in gate.check(root)]


def jsx_root(tmp: Path) -> Path:
    """A tree holding one `.tsx` file, which the gate must name rather than skip."""
    root = tmp / "jsx"
    root.mkdir(parents=True)
    build(root, {"a.tsx": JSX_TSX})
    return root


def messages_are_actionable(tmp: Path) -> list[str]:
    """Assert the full text of both message shapes, which `run` truncates away.

    A finding a reader cannot act on is a finding that gets ignored, and the
    remediation half is the only part that says what to DO -- including the
    distinction the second shape rests on, that a scanner problem is a defect in
    this gate rather than in the source it is reading.
    """
    root = tmp / "messages"
    root.mkdir(parents=True)
    build(root, {"a.kt": ADJACENT, "b.kt": UNTERMINATED_RAW_STRING_KT})
    return sorted(gate.check(root))


def run_main(tmp: Path, name: str, files: dict[str, str]) -> int:
    """Drive the shipped entrypoint, which is what the Makefile branches on.

    `run` above calls `check()` so every case can assert on a compact
    `path:line` rather than on prose. What that cannot cover is `main()` itself
    -- the exit code the recipe and the CI step read. Asserting only the code,
    never the message, is what keeps this cheap.
    """
    root = tmp / name
    root.mkdir(parents=True)
    build(root, files)
    old = gate.REPO
    gate.REPO = root
    try:
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            return gate.main()
    finally:
        gate.REPO = old


# The defect, in the two spellings a hand sweep missed, plus the one it caught.
ONE_LINE_FOLLOWER = """package p

/**
 * A banner about the file.
 */

/** Account operations. */
val x: Int = 1
"""

BLANK_LINE_BETWEEN = """package p

/**
 * Orphan.
 */

/**
 * Attached.
 */
fun f() {}
"""

ADJACENT = """package p

/**
 * Orphan.
 */
/**
 * Attached.
 */
fun f() {}
"""

# The fix, and the shapes that must keep passing.
BANNER_FIX = """package p

/*
 * A banner about the file. Prose, not documentation.
 */

/** Account operations. */
val x: Int = 1
"""

CLEAN = """package p

/**
 * Attached.
 */
fun f() {}
"""

EMPTY_COMMENTS = """package p

/**/
/***/
/** Attached. */
fun f() {}
"""

# Kotlin nests block comments; TypeScript does not. Each file is written for its
# own language and BOTH must be flagged, which is what pins the difference: read
# as non-nesting, the Kotlin block ends at `inner */` and its orphan goes away;
# read as nesting, the TypeScript file becomes one comment swallowing everything
# below and its orphan goes away too. Each case fails under the other's rule.
NESTED_KT = """package p

/**
 * A /* inner */ still A
 */
/** B */
fun f() {}
"""

NESTED_TS = """/* not the leading doc comment */
/** A /* inner */
/** B */
export const x = 1;
"""

# A `/**` written inside a string literal is not a doc comment. Masking too much
# is the failure direction that HIDES a real orphan, so both of these carry one:
# a scanner blind to the literal opens a comment there and never reaches it.
STRING_OPENER_KT = """package p

val opener = "/**"

/** A */
/** B */
fun f() {}
"""

TEMPLATE_OPENER_TS = """import { y } from "./y";

const t = `/**`;

/** A */
/** B */
export const x = 1;
"""

# A Kotlin raw string may CLOSE on a quote: the delimiter is the last three of
# the run, so this literal's content is `rel="next"`. Taking the first three
# leaves a stray `"` that opens a phantom literal and blinds the scanner for
# everything below. `Pagination.kt` carries this exact line.
Q3 = '"' * 3
RAW_STRING_CLOSING_QUOTE_KT = (
    "package p\n"
    "\n"
    "fun f(part: String) = part.contains(" + Q3 + 'rel="next"' + Q3 + ")\n"
    "\n"
    "/**\n"
    " * Orphan.\n"
    " */\n"
    "/** Attached. */\n"
    "val x: Int = 1\n"
)

# A block ends at its `*/`, not at the end of the line the `*/` is on. That rule
# was documented and never exercised. Here the first block closes mid-line and
# `val x` follows it, so it documents something and there is nothing to flag —
# but a line-oriented scanner swallows `val x` with the comment, finds `/** B */`
# adjacent, and reports an orphan that is not there.
CLOSES_MID_LINE_KT = """package p

/** A */ val x: Int = 1
/** B */
fun f() {}
"""

# CRLF is not a defect.
CRLF_KT = (
    "package p\r\n\r\n/**\r\n * Orphan.\r\n */\r\n/** Attached. */\r\nfun f() {}\r\n"
)

# Neither a `//` note nor a `/*` aside is a declaration, so the doc comment
# above one is still attached to nothing.
LINE_COMMENT_BETWEEN = """package p

/**
 * Orphan.
 */
// An implementation note.
/**
 * Attached.
 */
fun f() {}
"""

ASIDE_BETWEEN = """package p

/**
 * Orphan.
 */
/* An aside. */
/**
 * Attached.
 */
fun f() {}
"""

# Real code between the two is the whole point: the first one documents it.
CODE_BETWEEN = """package p

/**
 * Attached to x.
 */
val x: Int = 1

/**
 * Attached to f.
 */
fun f() {}
"""

# Byte 0 is the leading-comment position, which this gate does not judge.
LEADING_TS = """/**
 * Module documentation.
 */

/** Attached. */
export const x = 1;
"""

# ...and a byte-order mark must not cost that file its exemption. Decoded as a
# character rather than consumed, it shifts every offset by one and the leading
# block starts at index 1 — a FALSE POSITIVE on source that was already correct.
BOM_LEADING_TS = "﻿" + LEADING_TS

# An unterminated block comment is a GATE defect, not a source defect: the
# scanner cannot read past it, so it has to say so rather than report clean.
UNTERMINATED_BLOCK_KT = """package p

/**
 * Orphan.
 */
/** Attached. */
fun f() {}

/* this comment is never closed
"""

UNTERMINATED_RAW_STRING_KT = "package p\n\nval s = " + Q3 + "never closed\n"

NOT_LEADING_TS = """import { y } from "./y";

/**
 * Orphan.
 */

/** Attached. */
export const x = 1;
"""

# ...and that exemption is JSDoc's, not the shape's. Kotlin and Java have no
# file-level doc comment, so byte 0 carries no meaning there and a leading
# orphan is a defect like any other.
LEADING_KT = """/**
 * Orphan: Kotlin has no file-level KDoc for this to be.
 */

/** Attached. */
val x: Int = 1
"""

LEADING_JAVA = """/**
 * Orphan: Javadoc documents the declaration below, and that is another comment.
 */

/** Attached. */
class A {}
"""

# Javadoc is the same block comment under another name, and the same defect.
JAVA_ORPHAN = """package com.basecamp.smithy;

/**
 * Orphan.
 */
/**
 * Attached.
 */
class A {}
"""

# A regex literal carrying a quote. Before the newline rule, the quote opened a
# literal that ran to the end of the file and the orphan below vanished.
REGEX_QUOTE_TS = """import { y } from "./y";

const forbidden = /["'<>&]/;

/**
 * Orphan.
 */
/** Attached. */
export const x = 1;
"""

# The same trap one line at a time: an apostrophe with no partner.
LONE_QUOTE_TS = """const r = /'/;

/**
 * Orphan.
 */
/** Attached. */
export const x = 1;
"""

# A backtick literal genuinely does cross newlines, so the newline rule must not
# apply to it. The literal spans a full ORPHAN PAIR on purpose: the first version
# of this case held one `/** */` and passed whether or not the exemption existed,
# which made it the only guard on the rule and no guard at all.
MULTILINE_TEMPLATE_TS = """import { y } from "./y";

const t = `line one
/** A */
/** B */
line five`;

/** Attached. */
export const x = 1;
"""

# A regex may carry a BACKTICK in a character class. Unlexed, it opens a phantom
# template literal — and a template literal is exempt from the newline rule, so
# there is nothing to stop it. `mentions.ts` has this exact shape; before regex
# lexing it took 34 lines and five doc comments out of the gate's view.
BACKTICK_IN_REGEX_TS = (
    'import { y } from "./y";\n'
    "const forbidden = /[ @[\\\\^`{|}]/;\n"
    "\n"
    "/**\n"
    " * Orphan.\n"
    " */\n"
    "/** Attached. */\n"
    "export const x = 1;\n"
)

# `/\//g` is a regex whose body is an escaped slash. Read as a `//` line
# comment, it swallows the rest of its line. Live in `pkce.ts` and `sgid.ts`.
ESCAPED_SLASH_REGEX_TS = (
    "const r = /\\//g; /** Orphan, on the regex's own line. */\n"
    "/** Attached. */\n"
    "export const x = 1;\n"
)

# JSX text is neither string nor code, so the scanner is not pointed at it. A
# `.tsx` file is NAMED rather than skipped in silence, because a suffix quietly
# dropped from the language table is a false negative with no symptom.
JSX_TSX = "const a = <p>don't</p>; const s = '/** A */ /** B */';\n"

# The four positions where the heuristic guesses DIVISION, each with a backtick
# in the regex so the guess would otherwise run to the next backtick anywhere in
# the file rather than stopping at the line. Swap the backtick for a quote and
# every one of these is found without the override; that asymmetry is the bug
# the override exists to close.
def _division_guess(line: str) -> str:
    return (
        'import { y } from "./y";\n'
        + line + "\n"
        "/**\n"
        " * Orphan.\n"
        " */\n"
        "/** Attached. */\n"
        "export const x = 1;\n"
    )


DIVISION_GUESSES_TS = {
    "after a keyword outside the allowlist": _division_guess("export default /[`]/;"),
    "after `}`": _division_guess("if (a) { b() }\n/[`]/.test(s);"),
    "after `)`": _division_guess("if (a) /[`]/.test(s);"),
    "after an identifier": _division_guess("const n = a /[`]/ b;"),
}

# The other direction, and it is a FALSE POSITIVE: a stray quote from an unlexed
# regex closing on a real string's opening quote hands that string's contents to
# the scanner as code. Valid TypeScript, nothing to flag.
STRAY_QUOTE_TS = (
    'const RE = /"/, a = "/** A */ /** B */";\n'
    "\n"
    "/** Attached. */\n"
    "export const x = 1;\n"
)

# A Java text block processes escapes, so an escaped quote is how a triple quote
# is written inside one. Closing on it leaves a stray delimiter that swallows
# the rest of the file — where a Kotlin raw string, which has no escapes, must
# not be read that way.
TEXT_BLOCK_ESCAPE_JAVA = (
    "package p;\n"
    "\n"
    "class A {\n"
    '    String s = """\n'
    '        a \\""" b\n'
    '        """;\n'
    "\n"
    "    /**\n"
    "     * Orphan.\n"
    "     */\n"
    "    /** Attached. */\n"
    "    void f() {}\n"
    "}\n"
)


def main() -> int:
    tmp = Path(tempfile.mkdtemp(prefix="check-orphaned-doc-comments-test-"))
    try:
        print("the defect, in every spelling:")
        check("one-line follower (the ServiceAccessors shape)",
              run(tmp, "t01", {"a.kt": ONE_LINE_FOLLOWER}), ["a.kt:3"])
        check("blank line between",
              run(tmp, "t02", {"a.kt": BLANK_LINE_BETWEEN}), ["a.kt:3"])
        check("adjacent, no blank line",
              run(tmp, "t03", {"a.kt": ADJACENT}), ["a.kt:3"])
        check("line comment between",
              run(tmp, "t04", {"a.kt": LINE_COMMENT_BETWEEN}), ["a.kt:3"])
        check("non-doc aside between",
              run(tmp, "t05", {"a.kt": ASIDE_BETWEEN}), ["a.kt:3"])
        check("not leading: byte 0 is the only exempt position",
              run(tmp, "t06", {"a.ts": NOT_LEADING_TS}), ["a.ts:3"])
        check("string literal cannot open a comment and hide the orphan",
              run(tmp, "t07", {"a.kt": STRING_OPENER_KT}), ["a.kt:5"])
        check("template literal cannot open a comment and hide the orphan",
              run(tmp, "t08", {"a.ts": TEMPLATE_OPENER_TS}), ["a.ts:5"])
        check("TypeScript block comments do not nest",
              run(tmp, "t09", {"a.ts": NESTED_TS}), ["a.ts:2"])
        check("Kotlin block comments nest",
              run(tmp, "t10", {"a.kt": NESTED_KT}), ["a.kt:3"])
        check("a regex literal's quote does not turn the file off",
              run(tmp, "t11", {"a.ts": REGEX_QUOTE_TS}), ["a.ts:5"])
        check("a lone quote in a regex does not turn the file off",
              run(tmp, "t12", {"a.ts": LONE_QUOTE_TS}), ["a.ts:3"])
        check("Javadoc is the same defect",
              run(tmp, "t13", {"A.java": JAVA_ORPHAN}), ["A.java:3"])
        check("byte 0 carries no exemption in Kotlin",
              run(tmp, "t14", {"a.kt": LEADING_KT}), ["a.kt:1"])
        check("byte 0 carries no exemption in Java",
              run(tmp, "t15", {"A.java": LEADING_JAVA}), ["A.java:1"])
        check("a raw string closing on a quote does not blind the scanner",
              run(tmp, "t16", {"a.kt": RAW_STRING_CLOSING_QUOTE_KT}), ["a.kt:5"])
        check("CRLF is read the same as LF",
              run(tmp, "t17", {"a.kt": CRLF_KT}), ["a.kt:3"])
        check("a backtick in a regex class does not open a template literal",
              run(tmp, "r01", {"a.ts": BACKTICK_IN_REGEX_TS}), ["a.ts:4"])
        check("an escaped slash in a regex is not a line comment",
              run(tmp, "r02", {"a.ts": ESCAPED_SLASH_REGEX_TS}), ["a.ts:1"])
        check("a Java text block's escaped delimiter does not close it",
              run(tmp, "r03", {"A.java": TEXT_BLOCK_ESCAPE_JAVA}), ["A.java:8"])
        for n, (label, body) in enumerate(DIVISION_GUESSES_TS.items()):
            orphan_line = body.split("\n").index("/**") + 1
            check(f"a backtick regex {label}",
                  run(tmp, f"r1{n}", {"a.ts": body}), [f"a.ts:{orphan_line}"])

        print("what the scanner cannot read, it reports:")
        check("unterminated block comment",
              run(tmp, "t18", {"a.kt": UNTERMINATED_BLOCK_KT}),
              ["a.kt:3", "a.kt:line 9: unterminated block comment"])
        check("unterminated raw string",
              run(tmp, "t19", {"a.kt": UNTERMINATED_RAW_STRING_KT}),
              ["a.kt:line 3: unterminated triple-quoted literal"])

        print("correct source, which must keep passing:")
        check("clean: doc then declaration",
              run(tmp, "t20", {"a.kt": CLEAN}), [])
        check("the fix: a `/*` banner is prose, not documentation",
              run(tmp, "t21", {"a.kt": BANNER_FIX}), [])
        check("`/**/` and `/***/` are empty comments, not doc comments",
              run(tmp, "t22", {"a.kt": EMPTY_COMMENTS}), [])
        check("code between two doc comments",
              run(tmp, "t24", {"a.kt": CODE_BETWEEN}), [])
        check("byte 0 in TypeScript is the module-comment position",
              run(tmp, "t25", {"a.ts": LEADING_TS}), [])
        check("a backtick literal may cross newlines",
              run(tmp, "t26", {"a.ts": MULTILINE_TEMPLATE_TS}), [])
        check("a block ends at its `*/`, not at the end of that line",
              run(tmp, "t27", {"a.kt": CLOSES_MID_LINE_KT}), [])
        check("a byte-order mark does not cost a file its exemption",
              run(tmp, "t28", {"a.ts": BOM_LEADING_TS}), [])
        check("a string's contents are not read as code after a regex",
              run(tmp, "r04", {"a.ts": STRAY_QUOTE_TS}), [])

        print("scope:")
        check("build outputs are not ours to document",
              run(tmp, "t30", {"kotlin/build/a.kt": ADJACENT,
                               "node_modules/b.ts": NOT_LEADING_TS,
                               "typescript/dist/c.ts": NOT_LEADING_TS,
                               "rust/target/d.kt": ADJACENT,
                               "go/vendor/e.kt": ADJACENT,
                               "f/.gradle/g.kt": ADJACENT,
                               "h/.build/i.kt": ADJACENT}), [])
        check("every in-scope suffix is read",
              run(tmp, "t31", {"a.kts": ADJACENT, "b.mts": NOT_LEADING_TS,
                               "C.java": JAVA_ORPHAN}),
              ["C.java:3", "a.kts:3", "b.mts:3"])
        check("a file whose language is not modelled is named, not skipped",
              [f.split(":")[0] for f in gate.check(jsx_root(tmp))], ["a.tsx"])
        check("line-doc languages are out of scope, not under-covered",
              run(tmp, "t32", {"a.swift": ADJACENT, "b.rs": ADJACENT,
                               "c.go": ADJACENT}), [])

        print("the messages themselves:")
        check("both message shapes say what to do about them",
              messages_are_actionable(tmp), [
                  "a.kt:3: doc comment documents nothing -- the next thing is "
                  "another doc comment, at line 6. Attach it to the declaration "
                  "it describes, or make it a `/*` file banner if it is prose "
                  "about the file.",
                  "b.kt:line 3: unterminated triple-quoted literal -- the "
                  "scanner could not read this file to the end, so any orphan "
                  "below that point is invisible. This is a gate defect, not a "
                  "source defect: report it rather than working around it.",
              ])

        print("the shipped entrypoint:")
        check("main() exits 1 on an orphan",
              run_main(tmp, "t40", {"a.kt": ADJACENT}), 1)
        check("main() exits 0 on a clean tree",
              run_main(tmp, "t41", {"a.kt": CLEAN}), 0)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    if FAILURES:
        print(f"\n{len(FAILURES)} case(s) failed: {', '.join(FAILURES)}", file=sys.stderr)
        return 1
    print("\nOK: the gate accepts and refuses the right source")
    return 0


if __name__ == "__main__":
    sys.exit(main())
