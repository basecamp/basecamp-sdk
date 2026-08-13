#!/usr/bin/env ruby
# frozen_string_literal: true

# What each conformance runner refuses to execute, read out of the runners
# themselves.
#
# Six runners exclude fixture cases through two unrelated mechanisms, and both
# have to be read to answer "does anything actually run this case?":
#
#   1. A name-keyed literal skip table — goSDKSkips, RUBY_SKIPS, TS_SDK_SKIPS,
#      SKIPS, KOTLIN_SKIPS, temporarySkips.
#   2. A whole-case tag branch in the run loop. Kotlin and Swift skip any case
#      tagged `link-header` outright.
#
# The `link-header` literal means two DIFFERENT things across the six, and a
# gate that read it as one thing would be wrong in both directions. Go, Python,
# Ruby and TypeScript use the tag to suppress that case's `requestCount`
# ASSERTION — the case still runs, its `statusCode` and `noError` assertions
# still fire, and it is not an exclusion. Kotlin and Swift use it to skip the
# whole CASE, because both derive a response's status from the last mock
# response the SDK consumed and an auto-paginating SDK walks past the end of a
# one-response queue, so `statusCode: 200` cannot pass there. That asymmetry is
# deliberate and documented (SPEC §19, #573 narrowed the four; #596 kept the
# two), so only Kotlin and Swift carry `excluded_tags` below.
#
# Extracted here rather than in the gate so scripts/check-fixture-execution and
# any later consumer read one parser. Precedent: scripts/schema_instance_validator.rb,
# required by scripts/check-projected-examples.rb.
#
# WHY NOT THE BASH TEMPLATE. scripts/check-replay-decoder-parity solves a
# similar-looking problem with awk and sed, and neither of its two load-bearing
# devices transfers:
#
#   - Its fail-closed rule is "the extracted set is non-empty", which is right
#     for a decoder table and would fail this build on arrival: three of the six
#     skip tables are LEGITIMATELY empty, and two of those (`emptyMap()`, `[:]`)
#     have no bracketed block for its `block()` helper to bound at all. The
#     predicate here is instead "the declaration anchor matched exactly one
#     line" — a missing anchor is the internal error; an anchor with zero keys
#     is a valid empty table.
#   - Its keys are identifiers. These keys are prose sentences — "Mixed-case
#     host and explicit default port stay on the mocked origin" — with spaces,
#     commas and em dashes, so every `[[:alnum:]]+` capture in that template
#     matches nothing.

require "json"
require "set"

module ConformanceSkips
  class ExtractionError < StandardError; end

  # One literal declaration to read out of a runner.
  #
  # `anchor` names the declaration and nothing else — no type annotation, no
  # opening bracket — so ordinary reformatting does not break the gate while a
  # RENAME does, which is the deliberate act a reviewer should have to answer
  # for. It must match exactly one line; see declaration_keys.
  #
  # `comment` is the line-comment syntax, needed so a `#`/`//` that mentions a
  # bracket or a quote cannot move the depth counter.
  Table = Struct.new(:label, :file, :anchor, :comment, keyword_init: true) do
    def id(runner) = "#{runner}/#{label}"
  end

  # A branch in a runner's run loop that skips a whole case by tag. There is no
  # literal to parse, so the registry states the tag and this pins the branch
  # that implements it: if the line goes or changes shape, the gate fails loudly
  # rather than carrying a claim about code that no longer exists.
  # `accessor` is how the run loop reaches a case's tags in that language. Every
  # occurrence of it in the file must belong to a registered branch: the anchor
  # proves the branch this registry KNOWS about still exists, and says nothing
  # about a SECOND branch added later, which would skip whole cases this module
  # never accounts for. Counting the accessor bounds that — not by recognizing
  # arbitrary new syntax, but by refusing to vouch for a file that inspects tags
  # somewhere the registry does not name.
  #
  # A REGEXP, not a string, and the difference is the whole guarantee. As one
  # literal it was one SPELLING wide: Kotlin registered `tc.tags`, so
  # `.filter { "slow" !in it.tags }` sailed past it — and `it.tags` is not an
  # exotic alternative, it is the idiom the surrounding code teaches, seven
  # lines above at `.filter { it.mode == "mock" }`. The registered spelling only
  # reads `tc.tags` because that branch happens to sit inside
  # `for (tc in testCases)`. Swift was the same: `allTags` is a computed
  # convenience (`var allTags: [String] { tags ?? [] }`) over a stored `tags`
  # sitting right beside it.
  #
  # Neither evasion needs anyone to be evading anything — an ordinary refactor
  # reaches both — and the roster gives NO backstop here, because Kotlin's and
  # Swift's tag exclusions are deliberately prose-only, so those bullet lists are
  # empty and the comparison is `[] == []` whichever way it goes.
  TagBranch = Struct.new(:tag, :file, :anchor, :accessor, keyword_init: true)

  # A line the registry asserts still exists, where there is no literal to parse
  # and no tag to count — a wire between two declarations, say. Same fail-closed
  # predicate as every other anchor: exactly one matching line, or the claim the
  # registry makes about this file is no longer backed by anything.
  PinnedLine = Struct.new(:file, :anchor, :claim, keyword_init: true)

  Runner = Struct.new(:name, :tables, :tag_branches, :pins, keyword_init: true) do
    # The authoritative exclusion table is the FIRST; any others are companions
    # whose key sets must match it (see check_companion_tables).
    def skip_table = tables.first
  end

  RUNNERS = [
    Runner.new(
      name: "go",
      tables: [
        Table.new(label: "goSDKSkips", file: "conformance/runner/go/main.go",
                  anchor: "var goSDKSkips", comment: :slash),
      ],
      tag_branches: [], pins: []
    ),
    Runner.new(
      name: "python",
      tables: [
        # A class attribute, not a module constant — the anchor is indented, so
        # it is matched anywhere on the line rather than at column 1.
        Table.new(label: "SKIPS", file: "conformance/runner/python/runner.py",
                  anchor: "SKIPS: set[str]", comment: :hash),
        Table.new(label: "SKIP_REASONS", file: "conformance/runner/python/runner.py",
                  anchor: "SKIP_REASONS: dict[str, str]", comment: :hash),
      ],
      tag_branches: [], pins: []
    ),
    Runner.new(
      name: "ruby",
      tables: [
        Table.new(label: "RUBY_SKIPS", file: "conformance/runner/ruby/runner.rb",
                  anchor: "RUBY_SKIPS =", comment: :hash),
        Table.new(label: "RUBY_SKIP_REASONS", file: "conformance/runner/ruby/runner.rb",
                  anchor: "RUBY_SKIP_REASONS =", comment: :hash),
      ],
      tag_branches: [], pins: []
    ),
    Runner.new(
      name: "typescript",
      tables: [
        Table.new(label: "TS_SDK_SKIPS", file: "conformance/runner/typescript/runner.test.ts",
                  anchor: "const TS_SDK_SKIPS", comment: :slash),
      ],
      tag_branches: [], pins: []
    ),
    Runner.new(
      name: "kotlin",
      tables: [
        Table.new(label: "KOTLIN_SKIPS",
                  file: "kotlin/conformance/src/main/kotlin/com/basecamp/sdk/conformance/Main.kt",
                  anchor: "val KOTLIN_SKIPS", comment: :slash),
      ],
      tag_branches: [
        TagBranch.new(tag: "link-header",
                      file: "kotlin/conformance/src/main/kotlin/com/basecamp/sdk/conformance/Main.kt",
                      anchor: 'if ("link-header" in tc.tags) {',
                      accessor: /\.tags\b/),
      ],
      pins: []
    ),
    Runner.new(
      name: "swift",
      tables: [
        # `temporarySkips`, not `swiftSkips`. #602's issue text names the
        # latter, which is an env-gated ternary (SWIFT_CONFORMANCE_NO_SKIPS)
        # holding no skip literals of its own. Parsing it does not read an
        # empty table — it reads ["SWIFT_CONFORMANCE_NO_SKIPS", "1"], two junk
        # keys that check_stale_skips would report loudly — but it is still the
        # wrong declaration to read, and the `pins` entry below is what makes
        # reading `temporarySkips` equivalent to obeying `swiftSkips`.
        # SPEC.md §19's roster has this right and the issue is stale.
        Table.new(label: "temporarySkips",
                  file: "conformance/runner/swift/Sources/ConformanceRunner/Runner.swift",
                  anchor: "let temporarySkips", comment: :slash),
      ],
      tag_branches: [
        TagBranch.new(tag: "link-header",
                      file: "conformance/runner/swift/Sources/ConformanceRunner/Runner.swift",
                      anchor: 'if tc.allTags.contains("link-header") {',
                      accessor: /\.tags\b|\ballTags\b/),
      ],
      pins: [
        PinnedLine.new(
          file: "conformance/runner/swift/Sources/ConformanceRunner/Runner.swift",
          anchor: /\?\s*\[:\]\s*:\s*temporarySkips\s*\z/,
          claim: "the run loop consults `swiftSkips`, which this registry does not parse; " \
                 "pinning its else-branch is what makes reading `temporarySkips` equivalent"
        ),
      ]
    ),
  ].freeze

  # All six exclude any case whose mode is not "mock" at load time, and all six
  # do it by treating an UNRECOGNIZED mode as non-mock — `(mode ?? "mock") ==
  # "mock"` and its five equivalents. So a typo'd mode is a silent all-six
  # exclusion, which is why the gate validates modes rather than merely
  # filtering on them.
  # UNMODELLED BY DECISION: Kotlin and Swift both count a `TestResult.skipped`
  # flag as a skip at runtime. Both default false and neither has a writer in
  # the mock path today (Kotlin's only producer is ReplayRunner.kt, a separate
  # entry point; Swift declares the field with no assignment anywhere), so
  # there is nothing to extract and nothing to anchor. If either ever gains a
  # writer it becomes a whole-case exclusion this module cannot see — register
  # it here then.
  KNOWN_MODES = %w[mock live].freeze
  DEFAULT_MODE = "mock"

  OPENERS = "{[(".freeze
  CLOSERS = "}])".freeze
  ESCAPES = { "n" => "\n", "t" => "\t", "r" => "\r", "0" => "\0" }.freeze

  # The indices of the lines an anchor matches.
  #
  # ONE matching mechanism for every anchor this module looks for. The
  # "exactly one line" PREDICATE beside each call is deliberately left repeated
  # — it is `== 1` with no substance that could grow, and the substance at those
  # sites is three different human-facing messages that unifying would
  # parameterize away.
  #
  # But how the matches are FOUND is teachable, and it was written four
  # different ways: `select` with `include?`, `select` with `start_with?`, and
  # `count` with `include?` twice. That is a duplicated concept wearing the
  # predicate's clothes — the day someone decides an anchor inside a commented-
  # out line should not count, three of the four would keep the old rule, which
  # is exactly how `comment_end` came to exist. `anchored:` keeps the one real
  # difference in intent: a roster heading must START its line, or prose merely
  # mentioning it would match.
  # `anchor` is a String matched as a substring, or a Regexp matched against the
  # line — the latter for notions that have more than one legitimate spelling,
  # like how a run loop reaches a case's tags.
  def self.anchor_lines(source, anchor, anchored: false)
    lines = source.is_a?(Array) ? source : source.lines
    lines.each_index.select do |index|
      line = lines[index]
      if anchor.is_a?(Regexp) then line.match?(anchor)
      elsif anchored then line.start_with?(anchor)
      else line.include?(anchor)
      end
    end
  end

  # The keys of one literal declaration.
  #
  # Fail-closed on the ANCHOR, not on the key count: an anchor that matches no
  # line (renamed, reformatted past recognition) or more than one (ambiguous) is
  # an extraction failure and must stop the build. An anchor that matches
  # exactly one line and yields zero keys is a genuinely empty table, which
  # three of the six are today.
  def self.declaration_keys(source, table)
    lines = source.lines
    hits = anchor_lines(lines, table.anchor)

    unless hits.length == 1
      raise ExtractionError,
            "#{table.file}: the declaration anchor #{table.anchor.inspect} matched " \
            "#{hits.length} line(s), expected exactly 1. The table was renamed or reshaped — " \
            "read it, then update ConformanceSkips::RUNNERS in the same commit. This is an " \
            "extraction failure, not an empty table: an empty table is a valid answer here and " \
            "a broken parser is not."
    end

    # Scanning starts at the ASSIGNMENT, not at the anchor, so that a TYPE
    # ANNOTATION's brackets never reach the depth counter.
    #
    # Starting at the anchor was wrong in the quiet direction, and both PR bots
    # found it independently. `SKIPS: set[str] =` with its literal on the next
    # line — what a formatter produces — opens and closes `[str]` on the anchor
    # line, so depth returns to zero at end of line with `entered` set, the scan
    # reports :closed, and the table reads EMPTY while holding every skip it
    # ever had. Swift's `[String: String]` is the same shape. A silently empty
    # skip table is one fewer exclusion, so a case skipped by all six reads as a
    # passing five-of-six and this gate reports success.
    #
    # Anchoring on `=` removes the class rather than patching the two spellings:
    # every one of these declarations is an assignment, nothing before the `=`
    # is ever part of the literal, and Go's `= map[string]string{` — where the
    # brackets sit AFTER the `=` — still balances on the same counter.
    # From the anchor's START, not its end: two anchors are the assignment
    # themselves (`RUBY_SKIPS =`), so searching past them finds nothing.
    anchor_start = lines[hits.first].index(table.anchor)
    assignment = lines[hits.first].index("=", anchor_start)
    if assignment.nil?
      raise ExtractionError,
            "#{table.file}: the declaration at #{table.anchor.inspect} has no `=` on its line, " \
            "so this parser cannot tell where the declaration header ends and the literal begins"
    end

    offset = lines[0, hits.first].sum(&:length) + assignment
    strings, state = scan(source, offset, table.comment)

    if state == :unclosed
      raise ExtractionError,
            "#{table.file}: the declaration at #{table.anchor.inspect} never closes — its " \
            "brackets are unbalanced through end of file"
    end

    # Map or set, decided by the text rather than by a per-runner setting: if any
    # top-level string is followed by a key separator the declaration is a map
    # and only those strings are keys; otherwise every string is an element.
    # This is what lets one parser read Go's `"k": "v"`, Ruby's `"k" => "v"`,
    # Kotlin's `"k" to "v"`, Swift's `["k": "v"]`, Ruby's `Set.new(["k"])` and
    # Python's `{"k"}` without six spellings of the same rule.
    keyed = strings.select { |_, separator| separator }
    return strings.map(&:first) if keyed.empty?

    # Map mode drops every string that is not a key, which is correct for the
    # VALUES and silently wrong for an entry written in a spelling
    # KEY_SEPARATOR_RE does not know. `mapOf("a" to "b", Pair("c", "d"))` is the
    # concrete case: "a" is keyed, so the parser commits to map mode, and "c" —
    # a real skip — is discarded as if it were a value.
    #
    # That fails QUIET, in the direction that matters. A dropped key is one
    # fewer exclusion, so a case genuinely skipped by all six reads as a passing
    # five-of-six and the gate reports success. A gate that silently
    # under-reports is worse than no gate, because it also stops people looking.
    #
    # The invariant that separates the two: in a map, every non-key string is a
    # VALUE, and a value is always immediately preceded by its key. So an unkeyed
    # string must have a keyed predecessor — anything else is an entry this
    # parser did not understand, and anything not positively interpreted is
    # reported, never credited.
    #
    # Stated per-string rather than over adjacent PAIRS, which was
    # order-dependent and missed half the cases. `mapOf(Pair("B", CONST), "A" to
    # "inline")` reads as B(unkeyed), A(keyed), inline(unkeyed): no two unkeyed
    # strings are adjacent, so a pairwise rule saw nothing and dropped "B" — a
    # real skip — silently. Reversing the two entries made the same construct
    # raise, which is the order the self-test happened to use. A first string
    # that is unkeyed has no predecessor at all and is the case a pairwise rule
    # structurally cannot see.
    #
    # Deliberately NOT strict alternation: `{"a": REASON, "b": OTHER}` with
    # constant values yields two adjacent KEYED strings and is perfectly
    # readable, so requiring key/value/key/value would reject a table this
    # parser handles correctly.
    strings.each_with_index do |(value, value_keyed), position|
      next if value_keyed

      predecessor = position.positive? ? strings[position - 1] : nil
      next if predecessor && predecessor[1]

      context = predecessor ? "follows #{predecessor[0].inspect}" : "opens the literal"
      raise ExtractionError,
            "a skip table has a string this parser cannot place: #{value.inspect} #{context} " \
            "with neither in key position. Two causes, and the message cannot tell them apart: " \
            "an entry whose key uses a spelling KEY_SEPARATOR_RE does not know (`:`, `=>` and " \
            "`to` are the ones it does), or a VALUE split across string literals — " \
            "`\"reason one\" +\\n \"reason two\"` — which this parser does not join. For the " \
            "second, put the reason on one literal. Reading either as a value would drop a real " \
            "skip and turn an all-six exclusion into a passing five-of-six, so it is reported " \
            "rather than guessed at."
    end

    keyed.map(&:first)
  end

  # Walk from `offset`, tracking bracket depth outside strings and comments.
  #
  # Returns [[value, followed_by_separator], ...] and :closed / :unclosed. The
  # declaration ends at the END OF THE LINE on which depth returns to zero, not
  # the instant it does. That is a BOUND, not a guarantee: a declaration that
  # returns to depth zero mid-statement and continues (`emptyMap() +\n
  # mapOf(...)`, Swift `.merging`, a second `const`) is truncated there. The
  # roster comparison catches every such shape tried except truncate-to-empty
  # paired with a "none" block, which is why the unread-bullet guard exists.
  # The reason it terminates here at all: `map[string]string{` closes its `[...]` and reopens
  # with `{` on one line, and stopping at the first zero would read the type
  # parameter and call the table empty.
  def self.scan(source, offset, comment)
    strings = []
    depth = 0
    entered = false
    index = offset
    length = source.length

    while index < length
      char = source[index]

      if char == "\n"
        return [strings, :closed] if entered && depth.zero?

        index += 1
        next
      end

      if (skip = comment_end(source, index, comment))
        index = skip
        next
      end

      # Raw and multiline string literals this parser deliberately does not
      # read: Kotlin/Python `"""`, Python `'''`, Swift `#"…"#`. Mis-tokenizing
      # one silently shifts every string after it, so it is reported rather than
      # guessed at — the same rule as an unrecognized key spelling.
      if source[index, 3] == '"""' || source[index, 3] == "'''" ||
         (comment == :slash && char == "#" && source[index + 1] == '"')
        raise ExtractionError,
              "a skip table uses a raw or multiline string literal (#{source[index, 3].inspect}); " \
              "this parser reads only ordinary quoted strings, and guessing at the delimiter " \
              "would shift every key after it"
      end

      # Single quotes are string delimiters EVERYWHERE, not only in the `#`
      # comment languages. The narrower rule was wrong in the quiet direction:
      # nothing in this repo enforces double quotes — there is no prettier or
      # eslint config, only .editorconfig — so a single-quoted TS_SDK_SKIPS key
      # was invisible, and a mixed table dropped just the single-quoted entries
      # with no raise, because an unread string never becomes a token for the
      # key-position check to see.
      #
      # The cost is that Go's and Kotlin's CHARACTER literals now read as
      # strings. That is the right trade twice over: for depth tracking it is
      # exactly correct, and for extraction it can only ADD a token, which the
      # key-position check turns into a loud failure rather than a silent drop.
      #
      # Backticks are Go raw strings and TypeScript template literals — both
      # strings, both able to hold a brace that would otherwise move depth.
      if char == '"' || char == "'" || (comment == :slash && char == "`")
        value, index = read_string(source, index)
        strings << [value, separator_follows?(source, index, comment)]
        next
      end

      if OPENERS.include?(char)
        depth += 1
        entered = true
      elsif CLOSERS.include?(char)
        depth -= 1
        raise ExtractionError, "unbalanced closing bracket while reading a skip table" if depth.negative?
      end

      index += 1
    end

    [strings, entered && depth.zero? ? :closed : :unclosed]
  end

  # The string literal starting at `index`; returns [value, index_after_quote].
  def self.read_string(source, index)
    quote = source[index]
    value = +""
    cursor = index + 1

    while cursor < source.length
      char = source[cursor]
      if char == "\\"
        escaped = source[cursor + 1]
        value << (ESCAPES[escaped] || escaped || "")
        cursor += 2
        next
      end
      break if char == quote

      value << char
      cursor += 1
    end

    raise ExtractionError, "unterminated string literal while reading a skip table" if cursor >= source.length

    [value, cursor + 1]
  end

  # Where a comment starting at `index` ends, or nil if none starts there.
  #
  # ONE definition, two callers, and it is one definition because it was two.
  # `scan` was taught about `/* … */`; `separator_follows?` kept the older
  # line-comments-only rule of the SAME idea, and nothing connected them — so
  # `"key" /* why */ : "value"` found no separator, no string landed in key
  # position, and the whole table fell to set mode with every reason string read
  # as a skip.
  #
  # That is a DUPLICATED CONCEPT, not a default surviving in an unvisited branch,
  # and the distinction picks the remedy: a stale default wants a conservative
  # value, one idea with two implementations wants one definition with a name,
  # so that teaching it once teaches it everywhere. The rest of this parser was
  # audited for the same shape when this was found — string delimiters, bracket
  # depth, key position and the separator each have exactly one definition. The
  # "anchor matched exactly one line" predicate appears at three call sites and
  # is deliberately left alone: it is a repeated one-line idiom whose substance
  # is three different human-facing messages, with no shared definition that
  # could be taught something and drift.
  def self.comment_end(source, index, comment)
    char = source[index]
    return nil unless ["#", "/"].include?(char)

    if (comment == :hash && char == "#") ||
       (comment == :slash && char == "/" && source[index + 1] == "/")
      return source.index("\n", index) || source.length
    end

    # Four of the six runners are `/* … */` languages, and an unhandled block
    # comment truncates or empties a table: one holding an unbalanced brace
    # ("dropped in #123 because the } branch went away") moves the depth counter
    # and ends the scan early. A comment with balanced brackets happens to work,
    # which is what made the failure hard to anticipate rather than obvious.
    if comment == :slash && char == "/" && source[index + 1] == "*"
      close = source.index("*/", index + 2)
      raise ExtractionError, "unterminated block comment while reading a skip table" if close.nil?

      return close + 2
    end

    nil
  end

  KEY_SEPARATOR_RE = /\A(?::|=>|to\b)/

  # Whether the next meaningful token after a string is a key separator, which
  # is what distinguishes `"k": "v"` (k is a key, v is not) from `"k",`.
  def self.separator_follows?(source, index, comment)
    cursor = index
    while cursor < source.length
      if source[cursor] =~ /\s/
        cursor += 1
      elsif (skip = comment_end(source, cursor, comment))
        cursor = skip
      else
        break
      end
    end

    source[cursor..].to_s.match?(KEY_SEPARATOR_RE)
  end

  # SPEC §19's Zero-Skip Target roster, as { runner => [case name, ...] }.
  #
  # Read for COMPARISON only, never as an input to exclusions: a wrong
  # extraction and a stale roster could otherwise agree and both stay green,
  # which is the failure the gate exists to prevent.
  #
  # The roster holds two separable things and only the first is derivable. The
  # ENUMERATION — "one line per runner × test, verbatim from the runners' skip
  # mechanisms", in the section's own words — is what this returns. The
  # CLASSIFICATION beside it (waiver-backed / architectural / unwaivered, plus
  # the reasoning) is judgement no gate can derive, and nothing here reads it.
  #
  # Only the NAME-KEYED tables are compared. Kotlin's and Swift's whole-case
  # `link-header` exclusion is documented in the roster's prose rather than as a
  # bullet, deliberately, and their bullet lists are correspondingly empty.
  #
  # The TypeScript live canary's placeholder skip is likewise absent from both
  # sides and needs no special case: it lives in live-runner.test.ts, which is
  # not a skip table this module reads. The roster says outright that it "is
  # deliberately not rostered" — do not teach the parser about it.
  ROSTER_ANCHOR = "### Zero-Skip Target"
  ROSTER_HEADINGS = {
    "Go" => "go",
    "Python" => "python",
    "Ruby" => "ruby",
    "TypeScript" => "typescript",
    "Kotlin" => "kotlin",
    "Swift" => "swift",
  }.freeze

  def self.roster(root, relative = "SPEC.md")
    source = read(root, relative)
    lines = source.lines
    hits = anchor_lines(lines, ROSTER_ANCHOR, anchored: true)
    unless hits.length == 1
      raise ExtractionError,
            "#{relative}: the roster heading #{ROSTER_ANCHOR.inspect} matched #{hits.length} " \
            "line(s), expected exactly 1"
    end

    body = lines[(hits.first + 1)..].take_while { |line| !line.start_with?("## ", "---") }.join
    split = body.split(/^\*\*([A-Za-z]+)\*\*/)[1..].to_a.each_slice(2).to_a
    blocks = split.to_h

    # to_h keeps the last of a repeated key, so a second **Ruby** block would
    # silently replace the first and half the roster would stop being compared.
    repeated = split.map(&:first).tally.select { |_, n| n > 1 }.keys & ROSTER_HEADINGS.keys
    unless repeated.empty?
      raise ExtractionError,
            "#{relative}: the Zero-Skip roster has more than one **#{repeated.first}** block"
    end

    missing = ROSTER_HEADINGS.keys - blocks.keys
    unless missing.empty?
      raise ExtractionError,
            "#{relative}: the Zero-Skip roster has no **#{missing.first}** block. Every runner " \
            "needs one, including the ones with nothing to list — an absent heading is how a " \
            "runner's skips stop being restated at all."
    end

    ROSTER_HEADINGS.to_h do |heading, runner|
      # Each block runs to the next bolded runner heading; a bullet is a line
      # whose first token is a quoted case name, which is what separates the
      # enumeration from the surrounding prose.
      block = blocks.fetch(heading)
      # Greedy to the LAST quote before the em dash, because case names contain
      # inner quotes: cards_write.json already carries `…goes on the wire as
      # "", with no GET`. A lazy `[^"]+` truncates that at the first inner quote,
      # so the day that case needs a skip, `make check` could not be made green
      # without renaming the fixture.
      bullets = block.scan(/^- "(.+)"\s+—/).flatten

      # A block that yields no bullets must SAY it has none.
      #
      # Otherwise absence-of-parse reads as absence-of-claim, structurally:
      # three of these six comparisons are `[] == []` today, and any parser
      # failure makes a fourth vacuous at exactly the moment it would have
      # mattered — the skip vanishes from the exclusion sets AND the roster
      # omission the gate exists to force goes unreported, both from one cause.
      # The three genuinely-empty blocks already write "none", so requiring the
      # word costs nothing and restates no list, which is what keeps this from
      # reopening the pinned-key-count question.
      if bullets.empty?
        # A bullet line the parser could not read is NOT an empty block, and the
        # "none" token cannot tell them apart. That matters most exactly where
        # it is weakest: Kotlin's and Swift's blocks already say "none BEYOND
        # the whole-case tag branch", so the two runners likeliest to gain a
        # first skip are the two whose prose permanently pre-satisfies the
        # token. Checking for unread bullets first closes that, and unlike
        # requiring "none" on the heading line it survives Swift's block, which
        # wraps its prose so the token lands on the second line.
        unread = block.lines.select { |line| line.start_with?("- ") }
        unless unread.empty?
          raise ExtractionError,
                "#{relative}: the Zero-Skip roster's **#{heading}** block has " \
                "#{unread.length} bullet line(s) this parser could not read, e.g. " \
                "#{unread.first.strip.inspect}. A bullet is `- \"case name\" — justification`; " \
                "an unreadable one is a skip that silently stops being rostered."
        end

        unless block.match?(/\bnone\b/i)
          raise ExtractionError,
                "#{relative}: the Zero-Skip roster's **#{heading}** block lists no skips and does " \
                "not say \"none\". An unparsed block is indistinguishable from an empty one, and " \
                "both halves of this gate then compare nothing against nothing."
        end
      end

      [runner, bullets]
    end
  end

  def self.read(root, relative)
    path = File.join(root, relative)
    raise ExtractionError, "missing #{relative}" unless File.exist?(path)

    File.read(path, encoding: "UTF-8")
  end

  # Every table's keys, as { "runner/label" => [key, ...] }.
  def self.tables(root)
    RUNNERS.each_with_object({}) do |runner, out|
      runner.tables.each do |table|
        out[table.id(runner.name)] = declaration_keys(read(root, table.file), table)
      end
    end
  end

  # The tag branches, verified to still exist. Returns { runner => [tag, ...] }.
  def self.excluded_tags(root)
    RUNNERS.each_with_object({}) do |runner, out|
      out[runner.name] = runner.tag_branches.map do |branch|
        source = read(root, branch.file)
        hits = anchor_lines(source, branch.anchor).length
        unless hits == 1
          raise ExtractionError,
                "#{branch.file}: the whole-case tag branch #{branch.anchor.inspect} matched " \
                "#{hits} line(s), expected exactly 1. ConformanceSkips::RUNNERS claims this " \
                "runner skips every case tagged #{branch.tag.inspect}; if that stopped being " \
                "true, drop the tag from the registry — do not widen the anchor until it matches."
        end
        branch.tag
      end

      # The anchors above prove the registered branches still exist and say
      # nothing about an UNREGISTERED one. A second `if (... in tc.tags)` would
      # skip whole cases this module never accounts for, and the miss is quiet
      # in the usual direction: fewer known exclusions, so an all-six skip reads
      # as a passing five-of-six.
      #
      # Bounded syntactically rather than by recognizing arbitrary new branches:
      # every mention of the language's tag accessor in the run-loop file must
      # belong to a registered branch. That is a real guarantee over the file it
      # reads and no guarantee at all over a branch written elsewhere, which is
      # why the two files it reads are the two run loops.
      runner.tag_branches.group_by(&:file).each do |file, branches|
        accessor = branches.first.accessor
        mentions = anchor_lines(read(root, file), accessor).length
        next if mentions == branches.length

        raise ExtractionError,
              "#{file}: #{mentions} line(s) inspect #{accessor.inspect}, but " \
              "ConformanceSkips::RUNNERS registers #{branches.length} whole-case tag branch(es) " \
              "there. An unregistered branch skips cases this gate cannot see — register it with " \
              "its tag, or if it is not a whole-case skip, say so here."
      end

      # Lines the registry asserts still exist where there is nothing to parse
      # and nothing to count. Swift's is the one that matters: the run loop
      # consults `swiftSkips`, an env-gated view, while this module parses
      # `temporarySkips`. Pinning the else-branch is what makes reading the
      # latter equivalent to obeying the former — without it, a skip added as
      # `: temporarySkips.merging(["solo": "…"]) { a, _ in a }` is honoured by
      # Swift and invisible here.
      runner.pins.each do |pin|
        hits = anchor_lines(read(root, pin.file), pin.anchor).length
        next if hits == 1

        raise ExtractionError,
              "#{pin.file}: the pinned line #{pin.anchor.inspect} matched #{hits} line(s), " \
              "expected exactly 1 — #{pin.claim}"
      end
    end
  end

  # What each runner will not execute: { runner => Set(case names) }, given the
  # loaded fixture cases (each a Hash with "name" and optional "tags").
  def self.exclusions(root, cases)
    table_keys = tables(root)
    tags = excluded_tags(root)

    RUNNERS.each_with_object({}) do |runner, out|
      excluded = Set.new(table_keys.fetch(runner.skip_table.id(runner.name)))
      runner_tags = tags.fetch(runner.name)
      unless runner_tags.empty?
        cases.each do |test_case|
          case_tags = test_case["tags"] || []
          # A non-array `tags` would answer include? in whatever way its own
          # class defines — a Hash by key, a String by substring — and the wrong
          # answer here is silent under-exclusion, which reads as a case some
          # runner still executes. conformance-fixtures-check validates the
          # fixture schema, but a gate that credits malformed input because
          # another gate usually catches it is crediting what it did not read.
          unless case_tags.is_a?(Array)
            raise ExtractionError,
                  "#{test_case['name'].inspect} has a non-array \"tags\" " \
                  "(#{case_tags.class}); this gate cannot tell which cases the tag branches skip"
          end

          excluded << test_case["name"] if runner_tags.any? { |tag| case_tags.include?(tag) }
        end
      end
      out[runner.name] = excluded
    end
  end
end
