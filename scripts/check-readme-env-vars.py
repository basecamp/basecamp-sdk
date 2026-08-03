#!/usr/bin/env python3
"""Keep README environment-variable tables honest against SDK source.

READMEs have repeatedly claimed environment variables that no SDK reads
(a phantom `BASECAMP_ACCOUNT_ID` row), or attributed a real variable to the
wrong SDK (`XDG_CACHE_HOME` credited to Ruby, which never reads it). Both are
invisible to every other gate in `make check`, because nothing here is
generated -- the prose is hand-written and drifts silently.

This checks five invariants:

  1. Forward, per SDK: every variable named in an SDK README's env-var table is
     genuinely read by that SDK's source.
  2. Forward, root README: the "Read by" column names SDKs, and each one must
     genuinely read that variable.
  3. Reverse, per SDK: every variable an SDK genuinely reads is named somewhere
     in that SDK's README, so a new read cannot ship undocumented.
  4. The root README's "TypeScript, Swift, and Kotlin read no environment
     variables at all" sentence still holds.
  5. Prose attribution: a sentence of the form "<SDK> reads `VAR`" must be true.

Invariant 5 exists because 1 and 2 only police table rows, and the attribution
bug that prompted this gate was in prose, not a table: the root README said the
XDG variables were the ones "Go and Ruby use to site their cache and config
directories" when Ruby never reads `XDG_CACHE_HOME` at all. With only the table
checks, rewording that sentence back to credit Ruby with `XDG_CACHE_HOME` passes
silently -- verified by doing exactly that and watching the gate report success.

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

That scanner is a lexer, not a parser, and the distinction is worth stating
plainly rather than discovering later. It models what these six languages
actually do with comments, string literals, and interpolation — including the
parts that differ, which is why `LANG_FLAGS` exists: Swift and Kotlin nest block
comments and Go and TypeScript do not; a triple-quoted literal is documentation
in Python and an ordinary string in Kotlin; only Swift has raw strings. It does
*not* model
preprocessor conditionals, macros, or heredocs, and it assumes source is
syntactically valid — a file with an unterminated literal is consumed to the
end rather than resynchronised.

The failure directions are not symmetric, and every fix here has been argued in
those terms. Masking too much hides a real read, so an undocumented variable
ships silently. Masking too little promotes example text to a read, which either
fails CI on a correct README or lets a phantom row pass behind a bogus
no-environment break. Neither is acceptable, so both directions carry tests.
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
            # Parentheses are optional in Ruby, so `ENV.fetch "FOO", nil` is a
            # real read. Requiring `(` made it invisible, which is the fail-open
            # direction: the gate would report that nothing reads the variable.
            rf"ENV\.fetch[(\s]\s*{Q}{NAME}{ENDQ}",
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

# Lexical quirks that are not implied by the comment style. Getting these wrong
# is not academic: each was a real miss found in review.
#
#   triple:  Python/Ruby `"""`, and Swift/Kotlin multiline `"""` -- which are
#            ordinary strings, not documentation.
#   nested:  Swift and Kotlin allow `/* /* */ */`; Go and TypeScript end the
#            comment at the first `*/`, so nesting them there would swallow code.
#   raw:     Swift `#"..."#`, whose interpolation is `\#(...)`.
LANG_FLAGS = {
    "Go": {"triple": False, "nested": False, "raw": False},
    "Ruby": {"triple": True, "nested": False, "raw": False},
    "Python": {"triple": True, "nested": False, "raw": False},
    "TypeScript": {"triple": False, "nested": False, "raw": False},
    "Swift": {"triple": True, "nested": True, "raw": True},
    "Kotlin": {"triple": True, "nested": True, "raw": False},
}
DEFAULT_FLAGS = {"triple": True, "nested": True, "raw": True}

# Attach each SDK's lexical flags, so the scanner is driven by the language it
# is actually reading rather than by the comment style alone.
for _sdk, _spec in SDKS.items():
    _spec["flags"] = LANG_FLAGS[_sdk]

ROOT_README = "README.md"

# The root README states these three read nothing. Invariant 4 holds it true.
NO_ENV_SDKS = ("TypeScript", "Swift", "Kotlin")

# Only variables in these families are in scope; the SDKs read no others.
VAR_RE = re.compile(r"\b((?:BASECAMP|XDG)_[A-Z0-9_]+)\b")

FILENAME_TEST_MARKERS = ("_test.", "test_", ".test.", "_spec.")

# Invariant 5. Only the active voice with SDK names as the subject. The passive
# "`BASECAMP_TOKEN` is read by `AuthManager`", the symbol subject "`Config()`
# reads no environment", and "`DefaultConfig` consults `XDG_CACHE_HOME`" all name
# a *symbol* rather than an SDK, so none match -- which is what keeps this quiet
# on the per-SDK READMEs, where every one of those sentences ships today.
#
# The subject may be compound, because the sentence this gate exists to catch
# was: "XDG_CACHE_HOME / XDG_CONFIG_HOME, which Go and Ruby use to site their
# cache and config directories". A single-name subject would have missed it, and
# so would a "reads"-only verb -- both were checked against that literal string.
SDK_ALT = "|".join(SDKS)
SDK_SUBJECT = rf"(?:{SDK_ALT})(?:(?:,\s*|\s+and\s+|,\s*and\s+)(?:{SDK_ALT}))*"
SDK_READS_RE = re.compile(
    rf"\b(?P<subject>{SDK_SUBJECT})\s+(?:reads?|uses?|consults?|honou?rs?)\b"
)
SDK_NAME_RE = re.compile(rf"\b({SDK_ALT})\b")

# A claim runs to the next such claim or the end of its sentence, whichever
# comes first, so "Go reads `A` ... and Ruby reads `B`" splits at "Ruby".
CLAIM_END_RE = re.compile(r"\.\s|\n")

# Quote characters that open a string literal, per comment style. String bodies
# are kept — the variable name lives inside one — but they are skipped over so a
# `#` or `//` inside a string is not mistaken for the start of a comment.
STRING_QUOTES = {"hash": "\"'", "slash": "\"'`"}




def matching_delimiter(text: str, start: int, open_ch: str, close_ch: str,
                       style: str | None = None, flags: dict | None = None) -> int:
    """Index of the delimiter closing the one already opened, honouring nesting.

    Skips over string literals when told the language, because a delimiter
    inside one is data: in `${foo("}", process.env.X)}` the quoted `}` is not the
    end of the interpolation, and treating it as such truncated the hole and
    hid the read behind it.

    Returns len(text) when unterminated, so a malformed literal consumes the
    rest of the file rather than silently resyncing mid-expression.
    """
    depth = 1
    k = start
    n = len(text)
    quotes = STRING_QUOTES[style] if style else ""
    while k < n:
        if quotes and text[k] in quotes:
            literal_end, _ = scan_literal(text, k, style, flags)
            if literal_end > k:
                k = literal_end
                continue
        if text[k] == open_ch:
            depth += 1
        elif text[k] == close_ch:
            depth -= 1
            if depth == 0:
                return k
        k += 1
    return n


def has_fstring_prefix(text: str, i: int) -> bool:
    """Whether the literal opening at `i` carries a Python f prefix."""
    prefix_start = i
    while prefix_start > 0 and text[prefix_start - 1].isalpha():
        prefix_start -= 1
    return "f" in text[prefix_start:i].lower()


def raw_string_hashes(text: str, i: int) -> int:
    """Number of `#` opening a Swift raw string at `i`, or 0 if this is not one."""
    k = i
    while k < len(text) and text[k] == "#":
        k += 1
    return k - i if k > i and k < len(text) and text[k] == '"' else 0


def brace_holes(text: str, start: int, stop: int, style: str, flags: dict,
                opener: str, closer: str) -> list[tuple[int, int]]:
    """Interpolation holes in a literal body, skipping escaped openers."""
    holes = []
    j = start
    while j < stop:
        # A doubled opener is an escape -- literal text, not an expression.
        if text.startswith(opener * 2, j):
            j += 2 * len(opener)
            continue
        if text.startswith(opener, j):
            close = matching_delimiter(text, j + len(opener), opener[-1], closer, style, flags)
            holes.append((j + len(opener), min(close, stop)))
            j = close + 1
            continue
        j += 1
    return holes


def scan_literal(text: str, i: int, style: str,
                 flags: dict | None = None) -> tuple[int, list[tuple[int, int]]]:
    """Extent of the literal opening at `i`, plus its interpolation holes.

    Returns (end, holes) where end is one past the closing quote and each hole is
    a half-open range of executable text inside the literal.
    """
    n = len(text)
    flags = DEFAULT_FLAGS if flags is None else flags

    # Swift raw strings: #"..."# , ##"..."## , and the #"""..."""# forms. The
    # hash count is part of both delimiters, and interpolation becomes \#(...).
    if flags["raw"]:
        hashes = raw_string_hashes(text, i)
        if hashes:
            fence = "#" * hashes
            body_open = i + hashes
            triple = text[body_open : body_open + 3] == '"""'
            open_len = 3 if triple else 1
            close_token = ('"""' if triple else '"') + fence
            body_start = body_open + open_len
            close_at = text.find(close_token, body_start)
            end = n if close_at == -1 else close_at + len(close_token)
            body_stop = close_at if close_at != -1 else n
            holes = brace_holes(text, body_start, body_stop, style, flags, "\\" + fence + "(", ")")
            return end, holes

    triple_quote = None
    if flags["triple"]:
        candidate = text[i : i + 3]
        if style == "hash" and candidate in ('"""', "'''"):
            triple_quote = candidate
        elif style == "slash" and candidate == '"""':
            triple_quote = candidate

    if triple_quote:
        close_at = text.find(triple_quote, i + 3)
        end = n if close_at == -1 else close_at + 3
        body_stop = close_at if close_at != -1 else n
        holes = []
        if style == "hash":
            # Only an f-string's braces are code; a plain docstring's are text.
            if has_fstring_prefix(text, i):
                holes = brace_holes(text, i + 3, body_stop, style, flags, "{", "}")
        else:
            # Swift and Kotlin multiline strings are ordinary strings, not
            # documentation, and both interpolate.
            holes = brace_holes(text, i + 3, body_stop, style, flags, "${", "}")
            holes += brace_holes(text, i + 3, body_stop, style, flags, "\\(", ")")
        return end, holes

    quote = text[i]
    openers = interpolation_openers(text, i, quote, style)
    swift_interp = ("\\(", ")") in openers
    j = i + 1
    holes = []
    while j < n:
        if text[j] == "\\":
            # Swift interpolation opens with a backslash, so it has to be tested
            # before the generic escape skip swallows the paren.
            if swift_interp and text.startswith("\\(", j):
                end = matching_delimiter(text, j + 2, "(", ")", style, flags)
                holes.append((j + 2, end))
                j = end + 1
                continue
            j += 2
            continue
        if text[j] == quote:
            j += 1
            break
        # An unterminated single-line literal ends at the newline; Go and
        # TypeScript backtick literals legitimately span lines.
        if text[j] == "\n" and quote != "`":
            break
        opened = False
        for opener, closer in openers:
            if opener != "\\(" and text.startswith(opener, j):
                end = matching_delimiter(text, j + len(opener), "{", closer, style, flags)
                holes.append((j + len(opener), end))
                j = end + 1
                opened = True
                break
        if opened:
            continue
        j += 1
    return j, holes


def mask_literals(text: str, lo: int, hi: int, style: str, in_string: bytearray,
                  flags: dict | None = None) -> None:
    """Mask string literals in [lo, hi), recursing through interpolation holes.

    Called on the inside of an interpolation, which is code: any literal *there*
    is data again. Without this, `${"process.env.BASECAMP_FAKE"}` would count as
    a read, because the hole was un-masked wholesale.
    """
    flags = DEFAULT_FLAGS if flags is None else flags
    quotes = STRING_QUOTES[style]
    i = lo
    while i < hi:
        if text[i] in quotes or (flags["raw"] and raw_string_hashes(text, i)):
            end, holes = scan_literal(text, i, style, flags)
            if end <= i:
                i += 1
                continue
            for k in range(i, min(end, hi)):
                in_string[k] = 1
            for hole_start, hole_end in holes:
                for k in range(hole_start, min(hole_end, hi)):
                    in_string[k] = 0
                mask_literals(text, hole_start, min(hole_end, hi), style, in_string, flags)
            i = end
            continue
        i += 1


def interpolation_openers(text: str, i: int, quote: str, style: str) -> list[tuple[str, str]]:
    """Interpolation delimiters valid inside the literal opening at `i`.

    Interpolations are the one part of a string literal that is executable, so
    they must stay visible to the read patterns. Each language spells them
    differently, and spelling them wrong fails *open* -- a masked read is a read
    the gate swears does not exist.
    """
    if quote == "`":
        return [("${", "}")]  # TypeScript and Kotlin raw strings
    if style == "slash":
        if quote == '"':
            return [("${", "}"), ("\\(", ")")]  # Kotlin, and Swift's \( ... )
        return []
    openers = []
    if quote == '"':
        openers.append(("#{", "}"))  # Ruby
    # Python f-strings, whose braces are code. Only with an `f` prefix: an
    # ordinary "{...}" is literal text, and treating it as code would let a
    # documentation example count as a read.
    if has_fstring_prefix(text, i):
        openers.append(("{", "}"))
    return openers


def strip_noncode(text: str, style: str, flags: dict | None = None) -> tuple[str, bytearray]:
    """Blank comments and doc-only string blocks, preserving every offset.

    Returns the cleaned text plus a mask marking offsets that sit inside a
    string literal (opening quote included).

    Removed characters become spaces and newlines are kept, so match offsets
    still map to the right line of the original file.

    Two things this has to get right, having got both wrong before:

    * A line-prefix comment test is not enough — inside an unstarred block
      comment the interior lines begin with ordinary code characters, so
      `/*\\nprocess.env.FOO\\n*/` would count as a read.
    * String bodies must be *kept* (the variable name lives inside one) but
      still tracked, because an ordinary string such as
      `"process.env.BASECAMP_FAKE"` is data, not a read. Callers reject a match
      whose API prefix begins inside a literal; see `real_reads`.
    """
    out = list(text)
    n = len(text)
    in_string = bytearray(n)
    flags = DEFAULT_FLAGS if flags is None else flags
    quotes = STRING_QUOTES[style]
    i = 0

    def blank(start: int, end: int) -> None:
        for k in range(start, min(end, n)):
            if out[k] != "\n":
                out[k] = " "

    def take_literal(start: int) -> int:
        """Mask the literal at `start`, un-masking and recursing into its holes."""
        end, holes = scan_literal(text, start, style, flags)
        for k in range(start, min(end, n)):
            in_string[k] = 1
        # An interpolation is executable code that merely lives inside a literal,
        # so it is un-masked: `token=${process.env.BASECAMP_TOKEN}` is a genuine
        # read, and masking it would hide one. Literals *inside* that expression
        # are data again, hence the recursive re-mask.
        for hole_start, hole_end in holes:
            for k in range(hole_start, min(hole_end, n)):
                in_string[k] = 0
            mask_literals(text, hole_start, min(hole_end, n), style, in_string, flags)
        return end

    while i < n:
        # Python/Ruby docstrings: string literals used as documentation, so an
        # example inside one is not a read -- unless it is an f-string, whose
        # braces are executable. scan_literal makes that distinction; blanking
        # every triple-quoted literal outright hid real reads. Swift and Kotlin
        # `"""` are ordinary strings, so they fall through to take_literal.
        if flags["triple"] and style == "hash" and text[i : i + 3] in ('"""', "'''"):
            end, holes = scan_literal(text, i, style, flags)
            if holes:
                take_literal(i)
            else:
                blank(i, end)
            i = end
            continue
        if flags["raw"] and raw_string_hashes(text, i):
            i = take_literal(i)
            continue
        if text[i] in quotes:
            i = take_literal(i)
            continue
        if style == "slash" and text.startswith("//", i):
            end = text.find("\n", i)
            blank(i, n if end == -1 else end)
            i = n if end == -1 else end
            continue
        if style == "slash" and text.startswith("/*", i):
            # Swift and Kotlin nest block comments; Go and TypeScript end at the
            # first `*/`, so nesting them there would swallow the code after it.
            if flags["nested"]:
                depth = 1
                k = i + 2
                while k < n and depth:
                    if text.startswith("/*", k):
                        depth += 1
                        k += 2
                    elif text.startswith("*/", k):
                        depth -= 1
                        k += 2
                    else:
                        k += 1
                end = k
            else:
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
    return "".join(out), in_string


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
        text, in_string = strip_noncode(
            path.read_text(encoding="utf-8", errors="replace"),
            spec["comments"],
            spec.get("flags"),
        )
        for pattern in compiled:
            # finditer + the named group, not findall: these patterns carry a
            # quote group too, and findall would hand back tuples.
            for match in pattern.finditer(text):
                # The match starts at the API prefix (`os.getenv`, `process.env`),
                # which is code. If that prefix is itself inside a literal, this
                # is example text in a string, not a call — `"process.env.FOO"`
                # is data. The *name* is legitimately inside a literal, so only
                # the start offset is tested.
                if in_string[match.start()]:
                    continue
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


def prose_lines(readme: Path) -> list[tuple[int, str]]:
    """README prose, as (line number, text), minus code blocks and table rows.

    Fenced blocks are shell and source examples, where "Go reads" would be a
    comment rather than a claim; table rows are already covered by invariants 1
    and 2, and scanning them here would double-report the same defect.
    """
    out = []
    fenced = False
    for lineno, line in enumerate(readme.read_text(encoding="utf-8").splitlines(), 1):
        if line.lstrip().startswith("```"):
            fenced = not fenced
            continue
        if fenced or line.strip().startswith("|"):
            continue
        out.append((lineno, line))
    return out


def prose_paragraphs(readme: Path) -> list[list[tuple[int, str]]]:
    """Prose grouped into paragraphs of consecutive lines.

    Markdown reflows, so an attribution can wrap: "Go reads" at the end of one
    line and the variable at the start of the next. Scanning line by line finds
    no claim at all in that case and invariant 5 silently stops enforcing a
    sentence it used to cover. A blank line ends a paragraph, and so does a gap
    in line numbers -- prose_lines drops tables and fenced blocks, and those
    genuinely separate one paragraph from the next.
    """
    groups: list[list[tuple[int, str]]] = []
    current: list[tuple[int, str]] = []
    for lineno, line in prose_lines(readme):
        if not line.strip() or (current and lineno != current[-1][0] + 1):
            if current:
                groups.append(current)
            current = []
        if line.strip():
            current.append((lineno, line))
    if current:
        groups.append(current)
    return groups


def join_paragraph(paragraph: list[tuple[int, str]]):
    """Join a paragraph's lines, returning the text and an offset->line mapper.

    Lines are joined with a single space so a wrapped "Go reads\\n`VAR`" reads as
    one sentence, and every claim is still reported at the source line where its
    subject appears.
    """
    parts: list[str] = []
    starts: list[tuple[int, int]] = []
    position = 0
    for lineno, line in paragraph:
        if parts:
            parts.append(" ")
            position += 1
        starts.append((position, lineno))
        parts.append(line)
        position += len(line)

    def line_at(offset: int) -> int:
        found = starts[0][1]
        for start, lineno in starts:
            if start <= offset:
                found = lineno
            else:
                break
        return found

    return "".join(parts), line_at


def prose_claims(readme: Path) -> list[tuple[int, str, list[str]]]:
    """Every "<SDK> reads `VAR`" claim, as (line number, sdk, variables).

    A compound subject yields one claim per SDK named, so "Go and Ruby use `X`"
    is checked against Go *and* Ruby.

    Variables are taken from after the verb -- "Go reads `X`" -- and, only when
    nothing follows, from before the subject. The backward pass is not
    hypothetical: the sentence that motivated this gate put the variables first
    and the subject in a trailing relative clause ("`XDG_CACHE_HOME` /
    `XDG_CONFIG_HOME`, which Go and Ruby use to site their cache and config
    directories"), where a forward-only scan sees no variables and reports
    nothing. Both passes stop at a sentence boundary or a neighbouring claim, so
    one clause cannot borrow another's variables.
    """
    claims = []
    for paragraph in prose_paragraphs(readme):
        text, line_at = join_paragraph(paragraph)
        matches = list(SDK_READS_RE.finditer(text))
        for index, match in enumerate(matches):
            forward_start = match.end()
            forward_stop = matches[index + 1].start() if index + 1 < len(matches) else len(text)
            terminator = CLAIM_END_RE.search(text, forward_start, forward_stop)
            if terminator:
                forward_stop = terminator.start()
            named = VAR_RE.findall(text[forward_start:forward_stop])

            if not named:
                # Back to the start of this sentence, but never past the previous
                # claim -- otherwise "Go reads `X`. Ruby uses it too" would hand
                # Go's variable to Ruby.
                backward_start = matches[index - 1].end() if index else 0
                for boundary in CLAIM_END_RE.finditer(text, backward_start, match.start()):
                    backward_start = boundary.end()
                named = VAR_RE.findall(text[backward_start:match.start()])

            for sdk in SDK_NAME_RE.findall(match.group("subject")):
                if named:
                    claims.append((line_at(match.start()), sdk, named))
    return claims


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

    # 5. Prose attribution, every README: "<SDK> reads `VAR`" must be true.
    for readme_rel in [ROOT_README] + [s["readme"] for s in SDKS.values()]:
        readme = REPO / readme_rel
        if not readme.is_file():
            continue
        for lineno, sdk, named in prose_claims(readme):
            for var in named:
                if var not in reads[sdk]:
                    failures.append(
                        f"{readme_rel}:{lineno}: prose says {sdk} reads {var}, but no "
                        f"read of it exists in {SDKS[sdk]['source']}/"
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

    # Count both, and name them accurately. `len(r)` is distinct variables, not
    # call sites -- reporting it as "read sites" understated the scan by a third
    # (16 vs 23), which is the same species of not-quite-true claim this gate
    # exists to catch.
    variables = sum(len(r) for r in reads.values())
    sites = sum(len(s) for r in reads.values() for s in r.values())
    print(
        f"README env-var check passed ({variables} variable/SDK pairs, "
        f"{sites} read sites, across {len(SDKS)} SDKs)."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
