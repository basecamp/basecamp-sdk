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
*not* model preprocessor conditionals, macros, or heredocs, and it assumes source is
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
            # `?.` is valid optional chaining, and a read the plain patterns
            # reported as nonexistent.
            rf"process\.env\??\.{NAME}",
            rf"process\.env(?:\?\.)?\[\s*{Q}{NAME}{ENDQ}",
            # `const { BASECAMP_TOKEN } = process.env` is a read too. The name
            # comes *before* process.env, so this is a zero-width lookahead:
            # a consuming pattern would bind only one name per brace group,
            # which is the "looks like coverage" failure all over again.
            #
            # Anchored on the brace or comma so it matches destructuring *keys*
            # only. Unanchored it also matched the local name in
            # `const { OTHER: BASECAMP_TOKEN } = process.env`, where the key --
            # the variable actually read -- is OTHER and BASECAMP_TOKEN is just
            # what it was renamed to.
            rf"[{{,]\s*{NAME}\b(?=[^{{}}]*\}}\s*=\s*process\.env)",
            # ...and the quoted-key form, `{ "BASECAMP_TOKEN": token }`. The
            # match has to start on the brace or comma, because the name is
            # inside a literal and would be rejected as string data.
            rf"[{{,]\s*{Q}{NAME}{ENDQ}\s*:(?=[^{{}}]*\}}\s*=\s*process\.env)",
        ],
    },
    "Swift": {
        "readme": "swift/README.md",
        "source": "swift/Sources",
        "comments": "slash",
        "suffixes": (".swift",),
        # `#*` around the quotes: a raw string is a valid dictionary key, so
        # environment[#"BASECAMP_TOKEN"#] is a real read that a bare-quote
        # pattern reports as nonexistent.
        "patterns": [rf"ProcessInfo\.processInfo\.environment\[\s*#*{DQ}{NAME}{ENDQ}#*"],
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
#   triple:  Python `"""`, and Swift/Kotlin multiline `"""` -- which are
#            ordinary strings, not documentation. Not Ruby, which has no such
#            literal: there `"""x"""` is three adjacent literals concatenated.
#   nested:  Swift and Kotlin allow `/* /* */ */`; Go and TypeScript end the
#            comment at the first `*/`, so nesting them there would swallow code.
#   raw:     Swift `#"..."#`, whose interpolation is `\#(...)`.
#   interp:  which quote character interpolates, and how. Keyed by the opening
#            quote, because it differs *within* a language: TypeScript
#            interpolates in backticks only, so `"${x}"` is plain text, and Go
#            interpolates nowhere at all.
#   fstring: Python only, where the `f` prefix decides whether braces are code.
#   triple_raw: Kotlin, whose multiline strings have no escapes at all.
#            Swift's do, which is how `\(` interpolates there.
#   regex:   TypeScript `/.../` literals, whose contents are data. Enabled only
#            there: `/` is division far more often than not, and the two are
#            told apart by the preceding token, so this stays conservative.
#   command_regex: Ruby only, where a call may omit its parentheses -- so the
#            token before the slash in `puts /re/` is an identifier and it is
#            still a regex. TypeScript has no such form, and reading one there
#            would swallow the arithmetic it is actually looking at.
#   multiline: Ruby, where an ordinary quoted literal may span physical lines.
#            Everywhere else a newline ends it, which is what bounds the damage
#            from an unbalanced quote.
#   newline_operand: Ruby, where a line break ends a complete expression, so a
#            slash at the start of the next line opens a regex. JavaScript
#            inserts no semicolon before `/`, so there the same slash continues
#            the expression above and is division.
#   condition_regex: TypeScript, where `if (...)` is followed by a statement and
#            a statement may begin with a regex. Off for Ruby, whose statement
#            modifiers put a genuine division after the same `)`.
LANG_FLAGS = {
    "Go": {
        "triple": False, "nested": False, "raw": False, "fstring": False,
        "multiline": False,
        "regex": False,
        "command_regex": False,
        "newline_operand": False,
        "condition_regex": False,
        "backtick_string": False,
        "backtick_raw": True,
        "triple_raw": False,
        "percent": False,
        "interp": {},
    },
    # Ruby has no triple-quoted literal. `"""x"""` is three adjacent literals
    # concatenated -- `""`, `"x"`, `""` -- and the middle one interpolates, so
    # treating the run as a docstring blanked executable code and lost the read
    # inside it. Only Python's `"""` is documentation.
    "Ruby": {
        "triple": False, "nested": False, "raw": False, "fstring": False,
        "multiline": True,
        "regex": True,
        "command_regex": True,
        # A complete Ruby expression ends at the newline, so the next line's
        # leading slash is a regex. `if (x) / 2` after a statement modifier is
        # not, which is why the condition rule is off here.
        "newline_operand": True,
        "condition_regex": False,
        "backtick_raw": False,
        # Ruby's backtick is a command literal, and it interpolates like a
        # double-quoted string. Python shares this comment style and has no such
        # literal, which is why this is a flag rather than a `hash` quote.
        "backtick_string": True,
        "triple_raw": False,
        "percent": True,
        "interp": {'"': [("#{", "}")], "`": [("#{", "}")]},
    },
    "Python": {
        "triple": True, "nested": False, "raw": False, "fstring": True,
        "multiline": False,
        "regex": False,
        "command_regex": False,
        "newline_operand": False,
        "condition_regex": False,
        "backtick_string": False,
        "backtick_raw": False,
        "triple_raw": False,
        "percent": False,
        "interp": {},
    },
    "TypeScript": {
        "triple": False, "nested": False, "raw": False, "fstring": False,
        "multiline": False,
        "regex": True,
        "command_regex": False,
        # No automatic semicolon is inserted before `/`, so a line-leading slash
        # continues the expression above it and is division. `if (...)` is
        # followed by a statement, which may begin with a regex.
        "newline_operand": False,
        "condition_regex": True,
        "backtick_string": False,
        "backtick_raw": False,
        "triple_raw": False,
        "percent": False,
        "interp": {"`": [("${", "}")]},
    },
    "Swift": {
        "triple": True, "nested": True, "raw": True, "fstring": False,
        "multiline": False,
        "regex": False,
        "command_regex": False,
        "newline_operand": False,
        "condition_regex": False,
        "backtick_string": False,
        "backtick_raw": False,
        "triple_raw": False,
        "percent": False,
        "interp": {'"': [("\\(", ")")]},
    },
    "Kotlin": {
        "triple": True, "nested": True, "raw": False, "fstring": False,
        "multiline": False,
        "regex": False,
        "command_regex": False,
        "newline_operand": False,
        "condition_regex": False,
        "backtick_string": False,
        "backtick_raw": False,
        "triple_raw": True,
        "percent": False,
        "interp": {'"': [("${", "}")]},
    },
}
# Permissive union, used only when no language is supplied.
DEFAULT_FLAGS = {
    "triple": True, "nested": True, "raw": True, "fstring": True, "multiline": True, "regex": True, "command_regex": True, "newline_operand": True, "condition_regex": True, "backtick_string": False, "backtick_raw": False, "triple_raw": False, "percent": True,
    "interp": {"`": [("${", "}")], '"': [("${", "}"), ("\\(", ")"), ("#{", "}")]},
}

# Attach each SDK's lexical flags, so the scanner is driven by the language it
# is actually reading rather than by the comment style alone.
for _sdk, _spec in SDKS.items():
    _spec["flags"] = LANG_FLAGS[_sdk]

ROOT_README = "README.md"

# The root README states these three read nothing. Invariant 4 holds it true.
NO_ENV_SDKS = ("TypeScript", "Swift", "Kotlin")

# Only variables in these families are in scope; the SDKs read no others.
VAR_RE = re.compile(r"\b((?:BASECAMP|XDG)_[A-Z0-9_]+)\b")

# Anchored, not merely contained. `test_` matched anywhere in the name made
# `latest_config.py`, `contest_helper.go` and `protest_mode.rb` all test files,
# and a read added to any shipping file so named would bypass every check here.
# The dotted markers are already anchored by their own `.`.
FILENAME_TEST_PREFIXES = ("test_",)
FILENAME_TEST_MARKERS = ("_test.", ".test.", "_spec.")

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


def string_quotes(style: str | None, flags: dict | None = None) -> str:
    """Quote characters that open a literal in this language.

    Keyed by comment style *and* flags, because Ruby and Python share the `hash`
    style and only Ruby has backtick command literals. Putting the backtick in
    the shared set would have a stray one in Python open a literal; leaving it
    out entirely -- which is what happened -- left Ruby's ``echo `` bodies
    executable, so `` `echo ENV["X"]` `` counted as a read while the `#` of
    `` `echo #{ENV["X"]}` `` opened a comment and hid a real one.
    """
    if not style:
        return ""
    quotes = STRING_QUOTES[style]
    if flags and flags.get("backtick_string"):
        quotes += "`"
    return quotes




# A `/` starts a regex only where an operand is expected. After a value -- an
# identifier, literal, or closing bracket -- it is division. Testing the
# preceding token is the standard heuristic and errs toward division, which is
# the safe way round: mistaking division for a regex would swallow code.
REGEX_PRECEDERS = set("=(,:[!&|?{};+-*%<>~^")
# ...and after a keyword, whose last character is a letter and would otherwise
# read as the end of an identifier, i.e. division.
REGEX_KEYWORDS = {
    "return", "typeof", "instanceof", "in", "of", "new", "delete", "void",
    "case", "do", "else", "yield", "await", "throw",
    # Ruby positions where an operand is expected.
    "when", "if", "unless", "elsif", "and", "or", "not", "then", "while", "until",
}


# Methods that idiomatically take a bare regex as a first argument without
# parentheses. Spacing alone cannot decide this: `total /x/ 2` and `puts /x/ 2`
# are spelled identically, and Ruby itself tells them apart only by knowing
# whether `total` is a local variable in scope -- which a lexer does not.
#
# So the rule is deliberately narrow rather than clever. A name on this list
# with a regex-shaped argument is a regex; everything else stays division. That
# keeps `total /ENV["X"].to_i / 2` arithmetic, where guessing "regex" would mask
# the read between the two operators and let an undocumented variable ship --
# the fail-open direction, and the one worth erring away from.
REGEX_COMMANDS = {
    "puts", "print", "p", "pp", "warn", "raise", "fail",
    "match", "grep", "scan", "split", "sub", "gsub", "index",
    "assert_match", "refute_match",
}


def command_argument(text: str, i: int, word: str, spaced: bool, flags: dict) -> bool:
    """Whether the `/` at `i` opens a regex passed to a call without parentheses.

    Ruby lets a method drop its parentheses, so `puts /re/` passes a regex whose
    preceding token is an ordinary identifier -- which the rule above reads as
    division. Both directions of that are wrong, and the second is the worse:
    `puts /ENV['X']/` hands the regex body to the read patterns and invents a
    read, while `puts /#{ENV['X']}/` stays division, the `#` then opens a line
    comment, and a genuine interpolated read disappears.

    Two things have to hold, because either alone is ambiguous. The call has to
    be one this list knows takes a regex, and the spacing has to be the argument
    form Ruby's own parser looks for -- a space before the slash and none after
    it, the spelling MRI warns about as `ambiguous first argument`. Every other
    spacing stays division, `/=` included.

    Ruby only. TypeScript has no parenthesis-less call, so `total /count/ 2`
    there is arithmetic and this reading would swallow it.

    Being wrong here is bounded twice over: the name must be on the list, and
    the caller still requires a closing `/` on the same line, so a lone slash
    falls back to division on its own.
    """
    if not flags.get("command_regex") or not spaced:
        return False
    if word not in REGEX_COMMANDS:
        return False
    after = text[i + 1 : i + 2]
    return bool(after) and not after.isspace() and after != "="


# Statement heads whose parenthesised condition is followed by a statement,
# rather than by more of an expression.
CONDITION_KEYWORDS = {"if", "for", "while", "switch", "catch", "with"}


def condition_paren(text: str, close: int, flags: dict) -> bool:
    """Whether the `)` at `close` ends an `if (...)`-style condition.

    A `)` normally ends a value, so a `/` after it is division. The exception is
    a control-flow condition: what follows that is a *statement*, and a
    statement may begin with a regex. `if (ready) /re/.test(value)` is valid
    TypeScript, and reading its slash as division leaves the pattern text
    executable -- so the gate invents an environment read out of regex data.

    TypeScript only, and that restriction is load-bearing rather than cautious.
    Ruby has statement modifiers, so in `warn 'x' if (limit) / n / 2 > 1` the
    identical `)` really is followed by division, and masking to the next slash
    would swallow whatever sits between them. That is the fail-open direction,
    which is the one worth erring away from.

    The backward walk counts parentheses without skipping strings or comments,
    so a `)` written inside a literal in the condition can throw the depth off.
    That resolves to "not a condition" and falls back to division, which leaves
    the pattern text visible: the gate over-reports rather than under-reports.
    """
    if not flags.get("condition_regex"):
        return False
    depth = 1
    k = close - 1
    while k >= 0:
        if text[k] == ")":
            depth += 1
        elif text[k] == "(":
            depth -= 1
            if depth == 0:
                break
        k -= 1
    if depth:
        return False
    k -= 1
    while k >= 0 and text[k] in " \t\r\n":
        k -= 1
    word_end = k + 1
    while k >= 0 and (text[k].isalnum() or text[k] == "_"):
        k -= 1
    if text[k + 1 : word_end] not in CONDITION_KEYWORDS:
        return False
    # ...but only as a statement head. `obj.if(ready) / x / 2` is a call on a
    # property that merely spells a keyword -- legal since ES5 -- and what
    # follows it is still an expression, so the slash there is division.
    # Without this the mask would run to the next slash and eat the read
    # between them, which is the fail-open direction this whole rule is
    # supposed to be avoiding.
    while k >= 0 and text[k] in " \t":
        k -= 1
    return k < 0 or text[k] != "."


def operand_position(text: str, i: int, flags: dict) -> bool:
    """Whether an operand may begin at `i`, rather than a binary operator.

    `/` and `%` are both overloaded the same way -- regex or division, percent
    literal or modulo -- and both are decided by what precedes them, so both ask
    here. Getting `%` wrong is the more violent of the two: an accepted literal
    with a punctuation delimiter that never recurs consumes the rest of the file.
    """
    k = i - 1
    # Whether anything separated the operator from the token before it. Ruby
    # needs it to tell a command-form argument from arithmetic, and the scan
    # below discards it.
    spaced = k >= 0 and text[k] in " \t"
    while k >= 0 and text[k] in " \t":
        k -= 1
    if k < 0:
        return True
    if text[k] in "\r\n":
        # Whether a line break ends the statement is the whole difference
        # between the two languages that spell a regex `/.../`, so it cannot be
        # one rule. Ruby ends a complete expression at the newline, so a
        # line-leading slash opens a regex. JavaScript inserts no semicolon
        # before `/` -- the slash parses as a continuation of the line above --
        # so `const r = 10\n/ process.env.X / 2` is division, and calling it a
        # regex masks the read between the slashes and ships it undocumented.
        if flags.get("newline_operand"):
            return True
        while k >= 0 and text[k] in " \t\r\n":
            k -= 1
        if k < 0:
            return True
    if text[k] in REGEX_PRECEDERS:
        return True
    if text[k] == ")":
        return condition_paren(text, k, flags)
    if not (text[k].isalnum() or text[k] == "_"):
        return False
    word_end = k + 1
    while k >= 0 and (text[k].isalnum() or text[k] == "_"):
        k -= 1
    word = text[k + 1 : word_end]
    return word in REGEX_KEYWORDS or command_argument(text, i, word, spaced, flags)


def regex_extent(text: str, i: int, flags: dict) -> int:
    """End of a regex literal starting at `i`, else `i`.

    TypeScript and Ruby both spell it `/.../` and both overload `/` as division,
    so the same preceding-token test decides. Ruby's `#{...}` interpolation
    inside one is handled by the caller, which also has to run this before the
    comment check -- otherwise the `#` opens a line comment and eats the rest.
    """
    if not flags or not flags.get("regex") or text[i] != "/":
        return i
    if text.startswith("//", i) or text.startswith("/*", i):
        return i
    if not operand_position(text, i, flags):
        return i
    j = i + 1
    in_class = False
    n = len(text)
    while j < n:
        c = text[j]
        if c == "\\":
            j += 2
            continue
        if c == "\n":
            return i  # unterminated on its line: not a regex after all
        if c == "[":
            in_class = True
        elif c == "]":
            in_class = False
        elif c == "/" and not in_class:
            return j + 1
        j += 1
    return i


# Ruby percent literals -- %q{...}, %w[...], %(...) -- are ordinary data, and
# the scanner saw only quote characters, leaving their contents executable.
PERCENT_DELIMS = {"{": "}", "[": "]", "(": ")", "<": ">"}
# %q %w %i %s are data; %Q %W %I, %r, %x and the bare form interpolate.
PERCENT_INTERPOLATING = {"", "r", "x"}


def percent_literal_extent(text: str, i: int, flags: dict) -> tuple[int, list[tuple[int, int]]]:
    """End of a Ruby percent literal starting at `i` (else `i`), plus its holes.

    The delimiter may be any non-alphanumeric character, not just a bracket:
    `%q!...!` is as valid as `%q{...}`. Paired delimiters nest, unpaired ones do
    not. Inside the body a quote is ordinary data, so the walk here is a plain
    scan rather than the language-aware `matching_delimiter` -- delegating to
    that let a `"` inside `%q{"}` open a phantom string and run past the close.
    """
    if not flags or not flags.get("percent") or text[i] != "%":
        return i, []
    # `%` is modulo wherever a value just ended. Skipping this test, `10%-x`
    # read the `-` as a bare delimiter, and with no second `-` on the way the
    # literal ran to the end of the file and masked every read after it.
    if not operand_position(text, i, flags):
        return i, []
    n = len(text)
    j = i + 1
    letter = ""
    if j < n and text[j].isalpha():
        letter = text[j]
        j += 1
    if j >= n:
        return i, []
    opener = text[j]
    if opener.isalnum() or opener.isspace() or opener == "=":
        return i, []  # `a % b`, `%=`, and format strings are not literals
    closer = PERCENT_DELIMS.get(opener)
    interpolates = letter.lower() in PERCENT_INTERPOLATING or letter.isupper()
    body_start = j + 1
    k = body_start
    depth = 1
    while k < n:
        if text[k] == "\\":
            k += 2
            continue
        # Inside `#{...}` the language's ordinary rules apply again, so a quoted
        # brace there is a string and not the end of anything. The plain scan
        # counted it and closed the literal early, which left the rest of the
        # line -- and any real read in it -- to be eaten as a `#` comment.
        # Quotes in the surrounding body stay data, which is why only the hole
        # is handed to the language-aware matcher.
        if interpolates and text.startswith("#{", k):
            k = matching_delimiter(text, k + 2, "{", "}", "hash", flags) + 1
            continue
        if closer is None:
            if text[k] == opener:
                break
        else:
            if text[k] == opener:
                depth += 1
            elif text[k] == closer:
                depth -= 1
                if depth == 0:
                    break
        k += 1
    body_stop = min(k, n)
    holes = []
    if interpolates:
        holes = brace_holes(text, body_start, body_stop, "hash", flags, "#{", "}")
    return min(body_stop + 1, n), holes


def comment_extent(text: str, i: int, style: str | None, flags: dict | None = None) -> int:
    """End of the comment starting at `i`, or `i` when none starts there.

    Shared by the top-level scan and the delimiter walk so the two cannot
    disagree about where a comment ends -- they did, and a commented brace
    truncated an interpolation as a result.
    """
    if not style:
        return i
    flags = DEFAULT_FLAGS if flags is None else flags
    n = len(text)
    if style == "slash" and text.startswith("//", i):
        end = text.find("\n", i)
        return n if end == -1 else end
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
            return k
        end = text.find("*/", i + 2)
        return n if end == -1 else end + 2
    if style == "hash" and text[i] == "#":
        end = text.find("\n", i)
        return n if end == -1 else end
    return i


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
    quotes = string_quotes(style, flags)
    while k < n:
        if quotes and text[k] in quotes:
            literal_end, _ = scan_literal(text, k, style, flags)
            if literal_end > k:
                k = literal_end
                continue
        # A delimiter inside a comment is not a delimiter either: in
        # `${foo(/* } */ x)}` the commented brace must not end the hole.
        comment_end = comment_extent(text, k, style, flags)
        if comment_end > k:
            k = comment_end
            continue
        regex_end = regex_extent(text, k, flags)
        if regex_end > k:
            k = regex_end
            continue
        if text[k] == open_ch:
            depth += 1
        elif text[k] == close_ch:
            depth -= 1
            if depth == 0:
                return k
        k += 1
    return n


def has_raw_prefix(text: str, i: int) -> bool:
    """Whether the literal opening at `i` carries a Python `r` prefix.

    In a raw string a backslash is an ordinary character, so `fr"\\{x}"` still
    interpolates -- consuming the backslash as an escape swallowed the brace and
    hid the expression.
    """
    start = i
    while start > 0 and text[start - 1].isalpha():
        start -= 1
    return "r" in text[start:i].lower()


def has_fstring_prefix(text: str, i: int) -> bool:
    """Whether the literal opening at `i` carries a Python f prefix."""
    prefix_start = i
    while prefix_start > 0 and text[prefix_start - 1].isalpha():
        prefix_start -= 1
    return "f" in text[prefix_start:i].lower()


def find_unescaped(text: str, token: str, start: int, raw: bool = False) -> int:
    """Index of `token` at or after `start`, skipping backslash-escaped ones.

    An escaped delimiter inside a triple-quoted literal does not close it, and
    treating it as the terminator left the rest of the literal scanned as code.
    """
    j = start
    n = len(text)
    while j < n:
        if not raw and text[j] == "\\":
            j += 2
            continue
        if text.startswith(token, j):
            return j
        j += 1
    return -1


def raw_string_hashes(text: str, i: int) -> int:
    """Number of `#` opening a Swift raw string at `i`, or 0 if this is not one."""
    k = i
    while k < len(text) and text[k] == "#":
        k += 1
    return k - i if k > i and k < len(text) and text[k] == '"' else 0


def brace_holes(text: str, start: int, stop: int, style: str, flags: dict,
                opener: str, closer: str, raw: bool = False) -> list[tuple[int, int]]:
    """Interpolation holes in a literal body, skipping escaped openers.

    `raw` says the literal processes no escapes, so a backslash inside it is
    ordinary data and protects nothing. Kotlin's triple-quoted string is the
    case that matters: there `\\${x}` still interpolates, because the backslash
    is literal text and the `${` after it is the template marker regardless.
    Consuming the pair as an escape skipped the `$`, dropped the hole, and
    masked a real `System.getenv` read -- the fail-open direction, and one the
    root README's "Kotlin reads no environment variables" would have kept
    looking true through.
    """
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
        # A backslash escapes what follows, so `\#{...}` in Ruby is literal text.
        # This has to be tested *after* the opener, because Swift's opener is
        # itself `\(` -- checking the escape first would eat it and lose every
        # Swift interpolation. And not at all in a raw literal, where there is
        # no escape to honour.
        if not raw and text[j] == "\\":
            j += 2
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
            # Raw: the escape is `\#(` with the fence, so a lone backslash here
            # is data. `#"\\#(x)"#` is a literal backslash then an interpolation.
            holes = brace_holes(text, body_start, body_stop, style, flags,
                                "\\" + fence + "(", ")", raw=True)
            return end, holes

    triple_quote = None
    if flags["triple"]:
        candidate = text[i : i + 3]
        if style == "hash" and candidate in ('"""', "'''"):
            triple_quote = candidate
        elif style == "slash" and candidate == '"""':
            triple_quote = candidate

    if triple_quote:
        triple_is_raw = flags.get("triple_raw") or (
            style == "hash" and has_raw_prefix(text, i))
        close_at = find_unescaped(text, triple_quote, i + 3, raw=triple_is_raw)
        end = n if close_at == -1 else close_at + 3
        body_stop = close_at if close_at != -1 else n
        holes = []
        if style == "hash":
            # Only an f-string's braces are code; a plain docstring's are text.
            if has_fstring_prefix(text, i):
                holes = brace_holes(text, i + 3, body_stop, style, flags, "{", "}",
                                    raw=triple_is_raw)
        else:
            # Swift and Kotlin multiline strings are ordinary strings, not
            # documentation, and both interpolate.
            # Whatever this language interpolates in a double-quoted string, it
            # interpolates in a multiline one: `${...}` in Kotlin, `\(...)` in
            # Swift, and nothing at all elsewhere.
            # Whether a backslash in there escapes anything is where the two
            # part company: Kotlin's `"""` is raw and Swift's is not, so the
            # same `\${...}` bytes are a live read in one and text in the other.
            for opener, closer in flags["interp"].get('"', []):
                holes += brace_holes(text, i + 3, body_stop, style, flags, opener, closer,
                                     raw=triple_is_raw)
        return end, holes

    quote = text[i]
    openers = interpolation_openers(text, i, quote, style, flags)
    swift_interp = ("\\(", ")") in openers
    # Two different kinds of raw, and they differ exactly where it matters. In
    # Python's the backslash stays in the value but still keeps the next quote
    # from closing the literal, so `r"\""` is one string. In Go's backtick
    # literal the backslash is nothing at all, so a trailing one cannot protect
    # the delimiter and the literal simply ends.
    py_raw = style == "hash" and has_raw_prefix(text, i)
    raw = py_raw or (quote == "`" and flags.get("backtick_raw"))
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
            if raw:
                if py_raw and text[j + 1 : j + 2] == quote:
                    j += 2
                    continue
                j += 1
                continue
            j += 2
            continue
        if text[j] == quote:
            j += 1
            break
        # An unterminated single-line literal ends at the newline; Go and
        # TypeScript backtick literals legitimately span lines.
        if text[j] == "\n" and quote != "`" and not flags["multiline"]:
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
    quotes = string_quotes(style, flags)
    i = lo
    while i < hi:
        # A comment inside the expression is not code either. The hole was
        # un-masked wholesale, so without this `${foo(/* process.env.X */ 1)}`
        # counts as a read. The mask means "not executable", not "in a string".
        comment_end = comment_extent(text, i, style, flags)
        if comment_end > i:
            for k in range(i, min(comment_end, hi)):
                in_string[k] = 1
            i = comment_end
            continue
        regex_end = regex_extent(text, i, flags)
        if regex_end > i:
            for k in range(i, min(regex_end, hi)):
                in_string[k] = 1
            i = regex_end
            continue
        # A percent literal is data here too. The top-level scanner knew that
        # and this one did not, so `"#{%q{ENV['X']}}"` -- data nested one level
        # inside an interpolation -- came back out as a read.
        percent_end, percent_holes = percent_literal_extent(text, i, flags)
        if percent_end > i:
            for k in range(i, min(percent_end, hi)):
                in_string[k] = 1
            for hole_start, hole_end in percent_holes:
                for k in range(hole_start, min(hole_end, hi)):
                    in_string[k] = 0
                mask_literals(text, hole_start, min(hole_end, hi), style, in_string, flags)
            i = percent_end
            continue
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


def interpolation_openers(text: str, i: int, quote: str, style: str,
                          flags: dict | None = None) -> list[tuple[str, str]]:
    """Interpolation delimiters valid inside the literal opening at `i`.

    Interpolations are the one part of a string literal that is executable, so
    they must stay visible to the read patterns. Which spelling is valid depends
    on the language *and* the quote: TypeScript interpolates in backticks only,
    so `"${x}"` is ordinary text there while the same bytes are code in Kotlin.
    Applying one language's rule to another is wrong in both directions -- too
    narrow hides a real read, too wide promotes example text to one.
    """
    flags = DEFAULT_FLAGS if flags is None else flags
    openers = list(flags["interp"].get(quote, []))
    # Python f-strings, whose braces are code. Only with an `f` prefix: an
    # ordinary "{...}" is literal text, and treating it as code would let a
    # documentation example count as a read.
    if flags["fstring"] and has_fstring_prefix(text, i):
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
    quotes = string_quotes(style, flags)
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
        # Regex before comment: Ruby interpolates with `#{...}`, and the comment
        # branch would eat the `#` and the rest of the line with it. regex_extent
        # already refuses `//` and `/*`, so TypeScript comments are unaffected.
        regex_end = regex_extent(text, i, flags)
        if regex_end > i:
            for k in range(i, min(regex_end, n)):
                in_string[k] = 1
            if style == "hash":
                for hole_start, hole_end in brace_holes(
                    text, i + 1, max(i + 1, regex_end - 1), style, flags, "#{", "}"
                ):
                    for k in range(hole_start, min(hole_end, n)):
                        in_string[k] = 0
                    mask_literals(text, hole_start, min(hole_end, n), style, in_string, flags)
            i = regex_end
            continue
        comment_end = comment_extent(text, i, style, flags)
        if comment_end > i:
            blank(i, comment_end)
            i = comment_end
            continue
        percent_end, percent_holes = percent_literal_extent(text, i, flags)
        if percent_end > i:
            for k in range(i, min(percent_end, n)):
                in_string[k] = 1
            for hole_start, hole_end in percent_holes:
                for k in range(hole_start, min(hole_end, n)):
                    in_string[k] = 0
                mask_literals(text, hole_start, min(hole_end, n), style, in_string, flags)
            i = percent_end
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
    name = rel_path.name
    return name.startswith(FILENAME_TEST_PREFIXES) or any(
        marker in name for marker in FILENAME_TEST_MARKERS)


def real_reads(spec: dict, scoped: bool = True) -> dict[str, list[str]]:
    """Env vars genuinely read by this SDK, mapped to 'file:line' call sites.

    `scoped` keeps only the `BASECAMP_*` and `XDG_*` families, which is right for
    the documentation invariants -- those are the only variables the READMEs
    tabulate. It is wrong for invariant 4, which asserts an SDK reads *no*
    environment at all: filtered, a new `process.env.HOME` disappeared and the
    claim went on passing. That one asks for everything.
    """
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
                if scoped and not VAR_RE.fullmatch(name):
                    continue
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


# Words that turn a mention into a denial. A bare substring test counted
# "the SDK never looks them up" as documentation, so a variable an SDK started
# reading could ship undocumented behind a sentence saying it does not.
# "reserved", "planned" and friends belong here rather than in some separate
# category: saying a variable is reserved for future use is saying the SDK does
# not read it today, which is a denial wearing optimistic clothes.
NEGATION_RE = re.compile(
    r"\b(never|not|no|none|nothing|cannot|n't|reserved|unused|ignored|planned)\b",
    re.I,
)

# ...and a mention only documents a read when the sentence actually claims one.
# Absence of a negation was never enough on its own: "`BASECAMP_FOO` appears in
# the table." denies nothing and asserts nothing either.
#
# The verbs are invariant 5's, so the two checks agree about what a claim looks
# like -- including the bare "use" that the sentence which prompted this whole
# gate was built on ("which Go and Ruby use to site their cache directories").
AFFIRMATIVE_RE = re.compile(
    r"\b(reads?|uses?|consults?|honou?rs?|checks?|respects?|sets?|configures?"
    r"|overrides?|enables?|disables?|sites?|looks? up|falls? back to)\b",
    re.I,
)


def affirmative_mentions(readme: Path) -> set[str]:
    """Variables this README positively documents.

    A table row counts. Prose counts only when the sentence carrying the
    variable is not a denial. Fenced code blocks never count -- every SDK's
    Quick Start shows the *caller* reading BASECAMP_TOKEN from its own
    environment, which says nothing about what the SDK reads.
    """
    tabled: set[str] = set()
    for _lineno, cells in table_rows(readme):
        tabled.update(VAR_RE.findall(cells[0]))
    mentioned: set[str] = set()
    denied: set[str] = set()
    for paragraph in prose_paragraphs(readme):
        text, _line_at = join_paragraph(paragraph)
        # Split on sentence enders only. A semicolon joins clauses of one
        # sentence, and splitting there detached "the SDK never looks them up"
        # from the variable it denies -- which is the exact sentence in
        # python/README.md this check exists to refuse.
        previous: list[str] = []
        for sentence in re.split(r"(?<=[.!?])\s+", text):
            named = VAR_RE.findall(sentence)
            if NEGATION_RE.search(sentence):
                # A denial that names nothing is denying what was just named:
                # "`BASECAMP_TOKEN` appears in examples. The SDK never reads
                # it." The pronoun is the whole trick, and taking the sentence
                # in isolation missed it.
                denied.update(named or previous)
            elif AFFIRMATIVE_RE.search(sentence):
                mentioned.update(named)
            if named:
                previous = named
    # Absence of a denial is not an affirmation. "`BASECAMP_TOKEN` appears in
    # examples. The SDK never reads it." passed on the strength of the first
    # sentence, which claims nothing, while the second says the opposite of what
    # documentation would -- so a denial anywhere in the prose withdraws a
    # merely neutral mention.
    #
    # A table row survives it. The row *is* the affirmative claim, and the
    # denial usually qualifies one code path rather than the SDK: go/README.md
    # says `StaticTokenProvider` does not read `BASECAMP_TOKEN`, which is true,
    # while the table documents the variable the SDK really does read.
    return tabled | (mentioned - denied)


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
            # ...and the other direction: a reader the column leaves out. Without
            # this, a second SDK could start reading a variable that already has
            # a row and every check would still pass.
            for sdk in SDKS:
                if var in reads[sdk] and sdk not in claimed:
                    failures.append(
                        f"{ROOT_README}:{lineno}: {sdk} reads {var} "
                        f"({reads[sdk][var][0]}) but the 'Read by' column omits it"
                    )

    # 2b. Reverse, root README: the row-by-row checks above only ever see
    # variables that already have a row, so an SDK could start reading
    # BASECAMP_NEW, document it in its own README, and leave the root
    # inventory silently short. Named anywhere affirmative counts -- the XDG
    # pair is carried in prose rather than the table on purpose, and invariant 5
    # is what holds that sentence honest.
    if root.is_file():
        root_documented = affirmative_mentions(root)
        for sdk in SDKS:
            for var, sites in sorted(reads[sdk].items()):
                if var not in root_documented:
                    failures.append(
                        f"{ROOT_README}: {sdk} reads {var} ({sites[0]}) but the root "
                        f"README never names it"
                    )

    # 3. Reverse, per SDK: a real read must be documented *affirmatively*.
    for sdk, spec in SDKS.items():
        readme = REPO / spec["readme"]
        if not readme.is_file():
            continue
        documented = affirmative_mentions(readme)
        for var, sites in sorted(reads[sdk].items()):
            if var not in documented:
                failures.append(
                    f"{spec['readme']}: {sdk} reads {var} ({sites[0]}) but the README "
                    f"does not document it (a mention inside a code example, or in a "
                    f"sentence saying it is *not* read, does not count)"
                )

    # 4. The root README's "read no environment variables at all" sentence.
    # "At all" means at all, so this one is unscoped: `process.env.HOME` breaks
    # the claim as surely as a BASECAMP_ variable would, and the scoped
    # inventory could not see it.
    for sdk in NO_ENV_SDKS:
        every = real_reads(SDKS[sdk], scoped=False)
        if every:
            detail = ", ".join(f"{v} ({s[0]})" for v, s in sorted(every.items()))
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
