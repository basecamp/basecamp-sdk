#!/usr/bin/env python3
"""Refuse a doc comment that documents nothing because another one follows it.

A `/** ... */` block documents the declaration that comes next. If the next
thing is another `/** ... */` block, the first one is attached to nothing: it
renders in no IDE tooltip, no generated page, and no `@see` link resolves to it.
The prose survives in the file and reads as documentation of whatever is below,
which is how it stops describing anything at all -- `Pagination.kt` carried a
`rel="next"` block above the WRONG function for long enough that the function it
described had no documentation at all.

This shape kept being found by hand, one instance at a time, by people reading
for something else. It is a file-wide question with a mechanical answer, so it
should not need another pair of eyes: eleven instances were in the tree when
this gate was written, and one pass finds all of them.

The two generated instances mattered most: `ServiceAccessors.kt` and `Types.kt`
each got theirs from an emitter, so editing the output would have been undone by
the next generation run. Both emitters now write the banner as `/*`, which is
what this gate wants and what the hand-written banners (`Mentions.kt`, `Urls.kt`,
`Device.kt`, `Discovery.kt`) already use.

WHAT IS AND IS NOT FLAGGED

  Flagged: a `/** ... */` whose next non-whitespace content opens another
  `/** ... */`. There is no benign reading of that shape -- a doc comment is not
  a declaration, so the first block cannot be documenting the second.

  Not flagged: `/* ... */` with one star. That is an ordinary block comment --
  prose about the file rather than documentation of a declaration -- and it is
  the FIX this gate steers toward, so flagging it would fight the repair. `/**/`
  and `/***/` are empty comments, not doc comments.

  Not flagged, IN TYPESCRIPT ONLY: a doc comment that is the first thing in the
  file. JSDoc and TSDoc give that position a meaning of its own (file- or
  module-level documentation), and this gate does not adjudicate whether a given
  leading block earns it. What the exemption actually rescues is the smaller
  population: not every TypeScript file that opens with a doc comment, but the
  ones where the next thing is another doc comment. It is a hole, and a narrow
  one: the block has to START the file, not merely sit near the top. (No count
  is given, deliberately. Every count stated in this file so far has drifted at
  least once -- two of them on a single rebase -- and a number nobody
  re-derives is the exact defect this gate exists to catch.)

  The exemption is a property of JSDoc, not of the shape, so it does NOT apply
  to Kotlin or Java. Neither language has a file-level doc comment at all -- a
  `package-info.java` Javadoc documents the package DECLARATION below it, and
  Kotlin has no equivalent -- so a leading orphan there is a defect like any
  other, which is why the banner in `ServiceAccessors.kt`, six lines down, was
  never a convention. No Kotlin or Java file in the tree opens with one.

WHY A LEXER AND NOT A GREP

The first hand-written sweep on the port branch matched `*/` immediately
followed by `/**` and walked straight past every instance with a blank line
between them. The second matched the blank line too and still could not see an
orphan followed by a ONE-LINE `/** ... */`, because a one-line doc comment both
opens and closes on the same line -- which is precisely the shape of
`ServiceAccessors.kt`, whose next line is `/** Account operations. */`. A
line-oriented pattern also ends a block at the first line ending in `*/`, and a
KDoc containing a code sample can carry one of those in its own body.

So comments are found by scanning, not matching:

  - String literals are skipped, so a `/**` inside one is not a comment.
    Kotlin raw strings, Java text blocks and TypeScript backtick template
    literals are recognised as their own forms.
  - Block comments NEST in Kotlin and do not in TypeScript or Java, so the
    scanner counts depth only where the language does.
  - A block ends where its `*/` is, not where a line ends.
  - A single- or double-quoted literal ends at a newline if it has not closed.
    None of these languages let one cross a line, so the alternative is not a
    longer string -- it is a misread that runs to the end of the file.
  - A triple-quoted literal follows its own language. A Kotlin raw string has no
    escapes and closes on the LAST three quotes of a run, so
    `Pagination.kt` line 118 -- in the very file this gate was written to sweep
    -- is a raw string whose content ends with a quote; closing on the first
    three leaves a stray one behind. A Java text block DOES process escapes, so
    it closes on the first unescaped delimiter and has no run rule, and its
    opener must be followed by a line terminator.
  - A REGEX literal is lexed, in TypeScript. It has to be: a regex can carry a
    quote or a backtick inside a character class, and either one read as a
    string opener takes the rest of the file out of view.
    `typescript/src/services/mentions.ts` has both -- `/["'<>&]/` and a class
    holding a backtick -- and between them they hid 76% and then 34 lines of
    the one file this gate is most often pointed at. A `/` opens a regex where
    a value cannot already have ended; the scan is line-bounded and
    class-aware, so a wrong guess costs at most the rest of one line.
  - ...EXCEPT that a misread backtick is not bounded at all, because a template
    literal may legally cross lines. So the two readings are weighed by cost,
    not symmetrically: where the candidate SPAN contains a backtick -- regex or
    division, the scanner does not know which, that being the point -- the regex
    reading wins even if the previous token says division. That is safe in the
    strong sense, not the hopeful one: skipping text only removes entries from
    the span list, and the adjacency test reads RAW source between two spans
    rather than the lexer's view of it, so skipped text is still non-whitespace
    to that test and still breaks the walk. The override cannot invent an
    orphan. The reading it overrides runs to the next backtick ANYWHERE.
  - What the scanner opens and cannot close, it REPORTS. A run-away comment or
    literal is indistinguishable from a clean file at the exit, and "clean" is
    the wrong answer to give when the truth is "unreadable". The same posture
    covers a file whose language is not modelled at all: it is named, not
    skipped in silence.

WHAT THIS DOES NOT MODEL

The regex rule is a heuristic on the previous significant token, not the
parser's own state, and there are several positions where it guesses division:
after `)`, after `}`, after a plain identifier, and after any keyword missing
from `REGEX_KEYWORDS`. A regex written in one of those hides whatever else is
on its line -- but only that, because the backtick override above is what took
the unbounded case off the table. A differential against the TypeScript
PARSER -- a real parser, not a second lexer sharing the same blind spot -- over
every TypeScript file in this tree finds no divergence.

JSX is not modelled, and that is why `.tsx` and `.jsx` are excluded rather than
covered badly: JSX text is neither string nor code, so an apostrophe in
`<p>don't</p>` reads as a quote here and hands the next real string's contents
to the scanner as code -- flagging valid source, which is the failure direction
that gets a gate deleted. The tree has no such file; if one appears the gate
names it instead of walking past it.

THE LARGER HALF OF THIS FAMILY IS OUT OF REACH

A doc comment on the WRONG declaration is the same defect and this gate cannot
see it: it only knows that a doc comment followed by a doc comment documents
nothing. `typescript/src/services/recordings-extensions.ts` has a live instance
-- a block describing `wireInteger` sits on the `INT64_LIMIT` constant above it
-- and it passes here. "Eleven instances, all fixed" means eleven of THIS shape,
not that the family is gone.

A template literal's `${ ... }` interpolation is skipped as part of the literal
rather than lexed as code, so a comment written inside one is invisible -- it is
also not documentation of anything. A backtick literal is allowed to cross
newlines, because in TypeScript it genuinely can.

Go, Rust and Swift are out of scope, not under-covered: their documentation is
written with LINE comments (`//`, `///`), and a run of those followed by another
run is an ordinary and legal shape, so "a doc comment followed by a doc comment"
is not a defect there. Verified rather than assumed -- no `.go`, `.rs` or
`.swift` file in the tree opens a `/**` block comment at all. (The one textual
match is `conformance/tests/**/*.json`, a glob inside a `//` line.) Java IS in
scope: `spec/smithy-bare-arrays` documents with Javadoc, which is the same
`/** ... */` block under a different name and has the same defect.
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent

# Build outputs, dependency trees and vendored code: not ours to document.
SKIP_DIRS = {
    ".git", ".gradle", ".build", "build", "target", "node_modules", "dist",
    "vendor", ".venv", "venv", "__pycache__",
}


class Language:
    """What the scanner has to know per language, one flag per real difference."""

    def __init__(self, nests: bool, triple_quote: bool, triple_escapes: bool,
                 backtick: bool, regex: bool, leading_exempt: bool) -> None:
        self.nests = nests            # do block comments nest?
        self.triple_quote = triple_quote  # raw strings / text blocks
        self.triple_escapes = triple_escapes  # does `\\` escape inside one?
        self.backtick = backtick      # `template ${literal}`, may cross lines
        self.regex = regex            # /literal/ as its own form
        self.leading_exempt = leading_exempt  # is a leading doc comment a file doc?


KOTLIN = Language(nests=True, triple_quote=True, triple_escapes=False,
                  backtick=False, regex=False, leading_exempt=False)
TYPESCRIPT = Language(nests=False, triple_quote=False, triple_escapes=False,
                      backtick=True, regex=True, leading_exempt=True)
# Java text blocks DO process escapes -- `\\"""` is the documented way to write a
# triple quote inside one -- where a Kotlin raw string has no escapes at all.
JAVA = Language(nests=False, triple_quote=True, triple_escapes=True,
                backtick=False, regex=False, leading_exempt=False)

LANGUAGES = {
    ".kt": KOTLIN,
    ".kts": KOTLIN,
    ".ts": TYPESCRIPT,
    ".mts": TYPESCRIPT,
    ".cts": TYPESCRIPT,
    ".java": JAVA,
}

# JSX is not modelled, and `.tsx`/`.jsx` are therefore NOT in the table above.
# JSX text is not a string and not code: an apostrophe in `<p>don't</p>` reads
# as a quote here and hands the next real string's contents to the scanner as
# code, which flags valid source. That is the failure direction that gets a gate
# deleted. There are no such files in the tree; if one appears, the gate says so
# rather than passing silently over it.
UNMODELLED_SUFFIXES = {".tsx", ".jsx"}


# A `/` opens a regex literal only where a VALUE cannot already have ended. The
# previous significant character decides it: after an identifier, a literal, `)`
# or `]` the `/` is division. These keywords end in a word character but are not
# values, so a `/` after one of them is a regex.
REGEX_KEYWORDS = frozenset((
    "return", "typeof", "instanceof", "in", "of", "new", "delete", "void",
    "case", "do", "else", "yield", "await", "throw", "default",
))
VALUE_ENDERS = frozenset(")]}\"'`")


def opens_regex(src: str, prev: str | None, prev_word: str) -> bool:
    """Whether a `/` at this point opens a regex rather than dividing."""
    if prev is None:
        return True
    if prev in VALUE_ENDERS or prev.isdigit():
        return False
    if prev.isalpha() or prev == "_" or prev == "$":
        return prev_word in REGEX_KEYWORDS
    return True


def skip_regex(src: str, i: int) -> int:
    """Consume a regex literal starting at `i`, or return -1 if it is not one.

    Line-bounded, because a regex literal cannot contain a raw newline. That is
    what makes a wrong guess cheap: the worst case is skipping the rest of one
    line. A `/` inside a bracket class does not close the literal, which is the
    shape that matters here: `typescript/src/services/mentions.ts` has one class
    holding a double quote and another holding a BACKTICK, and each of those
    characters opens a phantom literal in a scanner that does not lex this form.
    """
    n = len(src)
    j = i + 1
    in_class = False
    while j < n:
        c = src[j]
        if c == "\\":
            j += 2
            continue
        if c == "\n":
            return -1
        if in_class:
            if c == "]":
                in_class = False
        elif c == "[":
            in_class = True
        elif c == "/":
            j += 1
            while j < n and (src[j].isalpha()):  # flags
                j += 1
            return j
        j += 1
    return -1


def opens_triple(src: str, i: int, lang: Language) -> bool:
    """Whether the triple quote at `i` opens a triple-quoted literal.

    A Java text block's opening delimiter must be followed by a line terminator;
    with anything else on the line it is an empty string followed by a string. A
    Kotlin raw string has no such rule and may open and close on one line.
    """
    if not lang.triple_escapes:
        return True
    j = i + 3
    while j < len(src) and src[j] in " \t":
        j += 1
    return j < len(src) and src[j] == "\n"


def close_of_triple(src: str, i: int, lang: Language) -> int:
    """Offset just past the closing delimiter, or -1 if it never closes.

    The two languages genuinely differ and sharing one rule got both wrong:

      - Kotlin raw strings have NO escapes, and close on the LAST three quotes
        of the run. `Pagination.kt` line 118 holds a raw string whose content
        ends with a quote; closing on the first three leaves a stray quote that
        opens a phantom literal and blinds the scanner for the rest of the file.
      - Java text blocks DO process escapes, and an escaped quote is the
        documented way to write a triple quote inside one, so the close is the
        first UNESCAPED delimiter and there is no run rule.
    """
    n = len(src)
    j = i + 3
    if lang.triple_escapes:
        while j < n:
            if src[j] == "\\":
                j += 2
                continue
            if src.startswith('"""', j):
                return j + 3
            j += 1
        return -1
    end = src.find('"""', j)
    if end < 0:
        return -1
    run = end
    while run < n and src[run] == '"':
        run += 1
    return run


def scan_comments(src: str, lang: Language) -> tuple[list[tuple[int, int]], list[str]]:
    """Return every comment's (start, end), in order, plus any lexing problems.

    Line comments are included, not just consumed: only a block comment opening
    `/**` can be a doc comment, but a `//` note sitting between two of them is
    part of what "the next thing" means.

    A problem is a construct this scanner opened and never closed. It is
    reported rather than swallowed because the two are indistinguishable from
    the outside: a run-away literal or comment consumes the rest of the file,
    every doc comment in it disappears, and the gate says "clean". Real source
    that compiles has neither, so a problem means the scanner is wrong about
    this file and must say so instead of passing it.
    """
    spans: list[tuple[int, int]] = []
    problems: list[str] = []
    prev: str | None = None       # last significant character of code
    prev_word: str = ""           # ...and the word it ends, if it is one
    i, n = 0, len(src)
    while i < n:
        c = src[i]

        if c in " \t\r\n":
            i += 1
            continue

        if lang.triple_quote and src.startswith('"""', i) and opens_triple(src, i, lang):
            end = close_of_triple(src, i, lang)
            if end < 0:
                problems.append(
                    f"line {src.count(chr(10), 0, i) + 1}: unterminated "
                    f"triple-quoted literal"
                )
                i = n
                continue
            prev, prev_word = '"', ""
            i = end
            continue

        if c == '"' or c == "'" or (lang.backtick and c == "`"):
            quote = c
            i += 1
            while i < n and src[i] != quote:
                if src[i] == "\\":
                    i += 2  # a backslash escapes the next character
                    continue
                # A single- or double-quoted literal cannot cross a newline in
                # any of these languages. Reaching one means this was never a
                # literal, so resync there rather than swallowing the rest of
                # the file. A backtick template CAN cross newlines and is
                # exempt -- which is why the regex form below has to be lexed
                # rather than bounded: a backtick inside an unlexed regex has
                # nothing to stop it.
                if src[i] == "\n" and quote != "`":
                    break
                i += 1
            if i < n and src[i] == quote:
                i += 1
            prev, prev_word = quote, ""
            continue

        if src.startswith("//", i):
            end = src.find("\n", i)
            end = n if end < 0 else end
            spans.append((i, end))
            i = end
            continue

        if src.startswith("/*", i):
            start = i
            depth = 1
            i += 2
            while i < n and depth > 0:
                if lang.nests and src.startswith("/*", i):
                    depth += 1
                    i += 2
                elif src.startswith("*/", i):
                    depth -= 1
                    i += 2
                else:
                    i += 1
            if depth > 0:
                problems.append(
                    f"line {src.count(chr(10), 0, start) + 1}: unterminated "
                    f"block comment"
                )
            spans.append((start, i))
            continue

        if lang.regex and c == "/":
            end = skip_regex(src, i)
            # The heuristic decides the normal case. It is overridden in one
            # direction only: if the candidate SPAN contains a BACKTICK, take
            # the regex reading even where the previous token says division.
            # A misread quote is bounded by the newline rule; a misread backtick
            # is not, because a template literal may legally cross lines, so it
            # runs to the next backtick ANYWHERE in the file. The two readings
            # are not symmetric in cost, so they are not weighed symmetrically.
            # Preferring the regex can only SKIP text, never invent a comment,
            # so this cannot fail correct source.
            if end > 0 and (opens_regex(src, prev, prev_word) or "`" in src[i:end]):
                prev, prev_word = ")", ""  # a regex literal is a value
                i = end
                continue

        if c.isalnum() or c == "_" or c == "$":
            j = i
            while j < n and (src[j].isalnum() or src[j] in "_$"):
                j += 1
            prev, prev_word = src[j - 1], src[i:j]
            i = j
            continue

        prev, prev_word = c, ""
        i += 1

    return spans, problems

def is_doc_comment(src: str, start: int, end: int) -> bool:
    """A doc comment opens `/**` and has a body: `/**/` and `/***/` do not."""
    body = src[start:end]
    return body.startswith("/**") and body not in ("/**/", "/***/")


def orphans_in(src: str, lang: Language) -> tuple[list[tuple[int, int]], list[str]]:
    """Return (orphan offset, following doc-comment offset) pairs, plus problems.

    "Next" skips whitespace AND any comment that is not itself a doc comment: a
    `//` note or a `/*` aside between two doc comments is not a declaration
    either, so the first block is just as detached with one in the way. Anything
    else ends the walk -- that is code, and the doc comment is doing its job.
    """
    spans, problems = scan_comments(src, lang)
    found = []
    for i, (start, end) in enumerate(spans):
        if not is_doc_comment(src, start, end):
            continue
        if start == 0 and lang.leading_exempt:
            continue  # the module-comment position; see the module docstring
        cursor = end
        for next_start, next_end in spans[i + 1:]:
            if src[cursor:next_start].strip():
                break  # real code in between: the doc comment documents it
            if is_doc_comment(src, next_start, next_end):
                found.append((start, next_start))
                break
            cursor = next_end
    return found, problems


def source_files(root: Path):
    """In-scope source under `root`, in a stable order.

    `os.walk` rather than `rglob`, for two reasons a reviewer should not have to
    rediscover: pruning happens BEFORE descending, so a populated
    `node_modules` is never walked at all, and it does not follow directory
    symlinks, so a link back up the tree cannot loop.
    """
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
        for name in sorted(filenames):
            path = Path(dirpath) / name
            if path.suffix in LANGUAGES and path.is_file():
                yield path


def unmodelled_files(root: Path) -> list[Path]:
    """Files this gate would want but cannot lex. See UNMODELLED_SUFFIXES."""
    found = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
        for name in sorted(filenames):
            path = Path(dirpath) / name
            if path.suffix in UNMODELLED_SUFFIXES and path.is_file():
                found.append(path)
    return found


def line_of(src: str, offset: int) -> int:
    return src.count("\n", 0, offset) + 1


def check(root: Path) -> list[str]:
    failures: list[str] = []
    for path in source_files(root):
        # utf-8-sig, so a byte-order mark is consumed rather than decoded to a
        # character that shifts every offset by one -- which would put a leading
        # JSDoc block at index 1 and cost it the leading-comment exemption.
        src = path.read_text(encoding="utf-8-sig", errors="replace")
        rel = path.relative_to(root)
        orphans, problems = orphans_in(src, LANGUAGES[path.suffix])
        for start, following in orphans:
            failures.append(
                f"{rel}:{line_of(src, start)}: doc comment documents nothing -- "
                f"the next thing is another doc comment, at line "
                f"{line_of(src, following)}. Attach it to the declaration it "
                f"describes, or make it a `/*` file banner if it is prose about "
                f"the file."
            )
        for problem in problems:
            failures.append(
                f"{rel}:{problem} -- the scanner could not read this file to the "
                f"end, so any orphan below that point is invisible. This is a "
                f"gate defect, not a source defect: report it rather than "
                f"working around it."
            )
    for path in unmodelled_files(root):
        failures.append(
            f"{path.relative_to(root)}: this gate does not model JSX -- an "
            f"apostrophe in element text reads as a quote and flags valid "
            f"source. The tree had no such file when the exclusion was written. "
            f"Teach the scanner JSX, or widen UNMODELLED_SUFFIXES deliberately: "
            f"the one thing not to do is leave the file silently unswept."
        )
    return failures


def main() -> int:
    failures = check(REPO)
    if failures:
        print("Orphaned doc comments:", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        return 1
    print("OK: no orphaned doc comments")
    return 0


if __name__ == "__main__":
    sys.exit(main())
