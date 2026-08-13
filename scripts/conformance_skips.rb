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
  TagBranch = Struct.new(:tag, :file, :anchor, :accessor, keyword_init: true)

  Runner = Struct.new(:name, :tables, :tag_branches, keyword_init: true) do
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
      tag_branches: []
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
      tag_branches: []
    ),
    Runner.new(
      name: "ruby",
      tables: [
        Table.new(label: "RUBY_SKIPS", file: "conformance/runner/ruby/runner.rb",
                  anchor: "RUBY_SKIPS =", comment: :hash),
        Table.new(label: "RUBY_SKIP_REASONS", file: "conformance/runner/ruby/runner.rb",
                  anchor: "RUBY_SKIP_REASONS =", comment: :hash),
      ],
      tag_branches: []
    ),
    Runner.new(
      name: "typescript",
      tables: [
        Table.new(label: "TS_SDK_SKIPS", file: "conformance/runner/typescript/runner.test.ts",
                  anchor: "const TS_SDK_SKIPS", comment: :slash),
      ],
      tag_branches: []
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
                      accessor: "tc.tags"),
      ]
    ),
    Runner.new(
      name: "swift",
      tables: [
        # `temporarySkips`, not `swiftSkips`. #602's issue text names the
        # latter; the latter is an env-gated ternary
        # (SWIFT_CONFORMANCE_NO_SKIPS) holding no literals of its own, so
        # parsing it would read an empty table no matter what was skipped.
        # SPEC.md §19's roster has this right and the issue is stale.
        Table.new(label: "temporarySkips",
                  file: "conformance/runner/swift/Sources/ConformanceRunner/Runner.swift",
                  anchor: "let temporarySkips", comment: :slash),
      ],
      tag_branches: [
        TagBranch.new(tag: "link-header",
                      file: "conformance/runner/swift/Sources/ConformanceRunner/Runner.swift",
                      anchor: 'if tc.allTags.contains("link-header") {',
                      accessor: "allTags"),
      ]
    ),
  ].freeze

  # All six exclude any case whose mode is not "mock" at load time, and all six
  # do it by treating an UNRECOGNIZED mode as non-mock — `(mode ?? "mock") ==
  # "mock"` and its five equivalents. So a typo'd mode is a silent all-six
  # exclusion, which is why the gate validates modes rather than merely
  # filtering on them.
  KNOWN_MODES = %w[mock live].freeze
  DEFAULT_MODE = "mock"

  OPENERS = "{[(".freeze
  CLOSERS = "}])".freeze
  ESCAPES = { "n" => "\n", "t" => "\t", "r" => "\r", "0" => "\0" }.freeze

  # The keys of one literal declaration.
  #
  # Fail-closed on the ANCHOR, not on the key count: an anchor that matches no
  # line (renamed, reformatted past recognition) or more than one (ambiguous) is
  # an extraction failure and must stop the build. An anchor that matches
  # exactly one line and yields zero keys is a genuinely empty table, which
  # three of the six are today.
  def self.declaration_keys(source, table)
    lines = source.lines
    hits = lines.each_index.select { |index| lines[index].include?(table.anchor) }

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
    # VALUE, and a value is always immediately preceded by its key. Two
    # unkeyed strings in a row means an entry this parser did not understand.
    # Anything not positively interpreted is reported, never credited.
    strings.each_cons(2) do |(previous, previous_keyed), (value, value_keyed)|
      next if value_keyed || previous_keyed

      raise ExtractionError,
            "a skip table mixes key spellings this parser does not recognize: #{value.inspect} " \
            "follows #{previous.inspect} with neither in key position. Every entry must spell " \
            "its key with one of `:`, `=>` or `to`, or ConformanceSkips::KEY_SEPARATOR_RE has to " \
            "learn the new spelling — silently reading it as a value would drop a real skip and " \
            "turn an all-six exclusion into a passing five-of-six."
    end

    keyed.map(&:first)
  end

  # Walk from `offset`, tracking bracket depth outside strings and comments.
  #
  # Returns [[value, followed_by_separator], ...] and :closed / :unclosed. The
  # declaration ends at the END OF THE LINE on which depth returns to zero, not
  # the instant it does: `map[string]string{` closes its `[...]` and reopens
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

      if (comment == :hash && char == "#") ||
         (comment == :slash && char == "/" && source[index + 1] == "/")
        index = source.index("\n", index) || length
        next
      end

      # Single quotes are string delimiters only where the language has no
      # character literal: Go, Kotlin and Swift all spell a char with them, and
      # treating `'{'` as a string there would be as wrong as the reverse.
      if char == '"' || (comment == :hash && char == "'")
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

  KEY_SEPARATOR_RE = /\A(?::|=>|to\b)/

  # Whether the next meaningful token after a string is a key separator, which
  # is what distinguishes `"k": "v"` (k is a key, v is not) from `"k",`.
  def self.separator_follows?(source, index, comment)
    cursor = index
    while cursor < source.length
      char = source[cursor]
      if char =~ /\s/
        cursor += 1
      elsif (comment == :hash && char == "#") ||
            (comment == :slash && char == "/" && source[cursor + 1] == "/")
        cursor = (source.index("\n", cursor) || source.length)
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
    hits = lines.each_index.select { |index| lines[index].start_with?(ROSTER_ANCHOR) }
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
      [runner, blocks.fetch(heading).scan(/^- "([^"]+)"/).flatten]
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
        hits = source.lines.count { |line| line.include?(branch.anchor) }
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
        mentions = read(root, file).lines.count { |line| line.include?(accessor) }
        next if mentions == branches.length

        raise ExtractionError,
              "#{file}: #{mentions} line(s) inspect #{accessor.inspect}, but " \
              "ConformanceSkips::RUNNERS registers #{branches.length} whole-case tag branch(es) " \
              "there. An unregistered branch skips cases this gate cannot see — register it with " \
              "its tag, or if it is not a whole-case skip, say so here."
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
