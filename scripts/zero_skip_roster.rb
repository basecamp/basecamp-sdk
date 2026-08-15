# frozen_string_literal: true

# SPEC §19's Zero-Skip roster: loader, validator, renderer.
#
# One roster, two gates, and they need different halves of it:
#
#   scripts/sync-doc-constants.rb renders it and requires SPEC.md's
#   <!-- @zero-skip-roster:begin/end --> block to be byte-identical.
#   scripts/check-fixture-execution.rb compares its case names, per runner,
#   against the execution manifests a conformance run produces.
#
# Shared here rather than duplicated because the two gates must agree on what
# the file MEANS. A second copy of "which keys are required, and what counts as
# a skip" is a second thing to keep in step, and the one that drifts is whichever
# gate the author was not editing.
#
# ## Why this file exists at all, which is the point of the whole change
#
# The roster used to live in SPEC.md as prose and be READ BACK OUT of it. The
# justification was that every misreading surfaces as a set mismatch and never as
# a passing comparison. That invariant was breached five times, each fix a new
# selector: only `- ` bullets recognised; a duplicate block ignored; a duplicate
# entry invisible to `Array#-`; blockquote, table and HTML-comment payloads; and
# finally `include?('"')` being ASCII-only, so a curly-quoted or backticked
# entry slipped past — as does a second quoted name on an otherwise canonical
# bullet, which the live Kotlin line already carries (`yields "no response"`).
#
# The fifth variation of one bypass is evidence about the instrument. So the
# direction reversed: prose is now an OUTPUT. Nothing parses the block, so there
# is no character class to widen and no line shape to recognise — a byte SPEC
# carries that this file does not produce is a diff, whatever it is made of.
#
# What that does NOT buy, stated plainly because the byte comparison is easy to
# oversell: it makes SPEC's block faithful to this file. It says nothing about
# whether this file is faithful to the runners — that is the manifest comparison
# in check-fixture-execution.rb, and it is the half that needs a conformance run.
# Nor does it check the classifications or the reasons; those stay judgement,
# which is why §19 keeps its `[manual]` tag.

require "yaml"

module ZeroSkipRoster
  RELATIVE_PATH = "spec/zero-skip-roster.yml"

  # Hardcoded, and REQUIRED — all six, always. Deriving the set from whatever
  # keys the file happens to carry is the absence bug this repo keeps relearning:
  # a runner that quietly lost its section would shrink the expected set and both
  # gates would go on reporting success over five, then four. The order is the
  # render order, and it is the order SPEC has always used.
  RUNNERS = %w[go python ruby typescript kotlin swift].freeze

  LABELS = {
    "go" => "Go", "python" => "Python", "ruby" => "Ruby",
    "typescript" => "TypeScript", "kotlin" => "Kotlin", "swift" => "Swift"
  }.freeze

  SECTION_KEYS = %w[source classification note skips].freeze
  SKIP_KEYS    = %w[case reason].freeze

  # A qualifier (`classification`, `note`) is a CLAUSE, and the renderer owns
  # every terminator around it: qualifiers are joined with "; " and the heading
  # ends in ":" or ".". So a qualifier carrying its own renders `nothing to
  # skip..` or `architectural.; a note:` — malformed prose that neither gate can
  # object to afterwards, because both compare SPEC against exactly what the
  # renderer produced, doubled period and all. The YAML said "a clause, NOT a
  # sentence" in a comment; this is that sentence made enforceable.
  #
  # `reason` is deliberately NOT held to the mirror rule. It renders last on its
  # own line with nothing appended, so it has nothing to double up against, and
  # a reason ending in `)` or a backtick is ordinary rather than suspect.
  CLAUSE_TERMINATOR_RE = /[.!?;:,]\z/

  Section = Struct.new(:runner, :source, :classification, :note, :skips, keyword_init: true)
  Skip    = Struct.new(:name, :reason, keyword_init: true)

  # Raised for anything the file could be other than a valid roster. Callers turn
  # it into their own failure type; nothing recovers from it, because a roster
  # that cannot be read is not a roster that says nothing — it is a gate with no
  # source of truth, and both gates treat that as fatal rather than vacuous.
  class Malformed < StandardError; end

  class << self
    # Returns { runner => Section } for all six runners, or raises Malformed.
    def load(root)
      path = File.join(root, RELATIVE_PATH)
      raise Malformed, "#{RELATIVE_PATH} is missing" unless File.exist?(path)

      # Encoding pinned, not left to the locale: the reasons carry em dashes and
      # backticks, and under LC_ALL=C an unpinned read dies on the first
      # non-ASCII byte instead of checking anything.
      #
      # `safe_load` with its defaults: no aliases, no arbitrary classes. Aliases
      # are refused rather than merely unused — two runner sections sharing one
      # anchor would edit as one section and read as two, and a roster whose
      # entries change in pairs is worse than one that repeats itself.
      #
      # Rescuing Psych::Exception, not Psych::SyntaxError: the alias refusal is a
      # sibling class, and left uncaught it exits non-zero with a Psych backtrace
      # that names this loader instead of the file the reader has to fix.
      #
      # The AST pass runs FIRST, and it is the one thing safe_load cannot do for
      # us: duplicate mapping keys are legal YAML, and Psych keeps the LAST one
      # silently. `go:` written twice, `skips:` written twice, `case:` written
      # twice — each loads as a single value with the earlier claim discarded
      # without a word. That is precisely the duplicate-section false green this
      # change exists to retire, reappearing inside the instrument that replaced
      # the parser: the discarded entry would sit in the source of truth,
      # render nowhere, compare against nothing, and still read to a human as
      # rostered.
      text = File.read(path, encoding: "UTF-8")

      doc = begin
        reject_duplicate_keys(Psych.parse(text, filename: RELATIVE_PATH), "")
        YAML.safe_load(text, filename: RELATIVE_PATH)
      rescue Psych::Exception => e
        raise Malformed, "#{RELATIVE_PATH} is not valid YAML, or uses YAML this loader refuses " \
                         "(anchors and aliases are refused on purpose): #{e.message}"
      end

      unless doc.is_a?(Hash)
        raise Malformed, "#{RELATIVE_PATH}: expected a mapping at the top level, got #{describe(doc)}"
      end

      runners = doc["runners"]
      unless runners.is_a?(Hash)
        raise Malformed, "#{RELATIVE_PATH}: expected a `runners` mapping, got #{describe(runners)}"
      end

      missing = RUNNERS - runners.keys
      unless missing.empty?
        raise Malformed, "#{RELATIVE_PATH}: no section for #{missing.join(', ')}. All six runners " \
                         "are required — a runner that skips nothing carries an empty `skips` list " \
                         "and a note saying why. An absent section reads as unset and contributes " \
                         "nothing to either comparison, which is how a runner's skips go unrecorded."
      end

      unknown = runners.keys - RUNNERS
      unless unknown.empty?
        raise Malformed, "#{RELATIVE_PATH}: section(s) for unknown runner(s): " \
                         "#{unknown.map(&:to_s).sort.join(', ')}. Add them to RUNNERS in " \
                         "#{File.basename(__FILE__)} or nothing compares them."
      end

      RUNNERS.to_h { |runner| [runner, section(runner, runners.fetch(runner))] }
    end

    # { runner => [case name] }, which is the projection the manifest comparison
    # works in.
    def case_names(roster)
      roster.transform_values { |s| s.skips.map(&:name) }
    end

    # The exact lines SPEC.md must carry between the block markers — every
    # section, then one trailing blank line, so the closing marker is preceded by
    # a blank line the way the rest of the document's blocks are.
    #
    # Ordered by RUNNERS, never by the file's key order: the roster's claim is a
    # set, and rendering in canonical order means reordering the YAML cannot
    # produce a SPEC diff nobody meant to make.
    def render_lines(roster)
      RUNNERS.flat_map do |runner|
        s = roster.fetch(runner)

        # The qualifier list is what keeps the heading grammar to one rule
        # instead of one per section shape. `none` is DERIVED from `skips` being
        # empty rather than written down, so a section cannot claim to skip
        # nothing while listing skips, and one that skips nothing cannot forget
        # to say so.
        qualifiers = []
        qualifiers << "none" if s.skips.empty?
        qualifiers << s.classification if s.classification
        qualifiers << s.note if s.note

        head = +"**#{LABELS.fetch(runner)}** (#{s.source})"
        head << " — #{qualifiers.join('; ')}" unless qualifiers.empty?
        head << (s.skips.empty? ? "." : ":")

        [head, *s.skips.map { |skip| %(- "#{skip.name}" — #{skip.reason}) }, ""]
      end
    end

    private

    # Refuses a repeated mapping key anywhere in the document, reading the
    # PARSER'S OWN structure rather than the text. A regex over the raw YAML
    # would be the selector treadmill again — it would have to learn quoting,
    # indentation, flow mappings, comments and block scalars just to say where a
    # key is and whether it is one. The parser already knows all of that; the
    # AST is where it says so, and a spelling it accepts as a key is a key by
    # definition.
    #
    # Every level is covered by one walk, because "which levels matter" is the
    # question that gets answered wrongly later: a second `runners:`, a second
    # `go:`, a second `skips:` and a second `case:` are the same defect at four
    # depths, and enumerating them would leave the fifth.
    def reject_duplicate_keys(node, path)
      case node
      when Psych::Nodes::Stream, Psych::Nodes::Document
        node.children.each { |child| reject_duplicate_keys(child, path) }
      when Psych::Nodes::Sequence
        node.children.each_with_index { |child, i| reject_duplicate_keys(child, "#{path}[#{i}]") }
      when Psych::Nodes::Mapping
        seen = {}
        node.children.each_slice(2) do |key, value|
          # A non-scalar key (a list or mapping used as a key) has no name to
          # compare and cannot be one of ours; the unknown-key rules below
          # refuse it by its own path. Recurse into its value regardless, so a
          # duplicate nested under one is still found.
          name = key.is_a?(Psych::Nodes::Scalar) ? key.value : nil
          here = name.nil? ? "#{path}.?" : (path.empty? ? name : "#{path}.#{name}")

          if name && seen.key?(name)
            raise Malformed, "#{RELATIVE_PATH}: `#{here}` is written more than once " \
                             "(lines #{seen.fetch(name)} and #{key.start_line + 1}). YAML allows " \
                             "it and keeps only the LAST, so the earlier one would be discarded " \
                             "in silence — a claim written into the roster that renders nowhere " \
                             "and is compared against nothing."
          end

          seen[name] = key.start_line + 1 if name
          reject_duplicate_keys(value, here)
        end
      end
    end

    def section(runner, raw)
      unless raw.is_a?(Hash)
        raise Malformed, "#{RELATIVE_PATH}: `#{runner}` is #{describe(raw)}, not a mapping"
      end

      # Unknown keys are REFUSED, not ignored. A typo'd `skip:` or `cases:` would
      # otherwise leave the section reading as "skips nothing" — a stale roster
      # that passes, which is the failure this whole change exists to retire.
      extra = raw.keys - SECTION_KEYS
      unless extra.empty?
        raise Malformed, "#{RELATIVE_PATH}: `#{runner}` carries unknown key(s) " \
                         "#{extra.map(&:to_s).sort.join(', ')}; known keys are " \
                         "#{SECTION_KEYS.join(', ')}. A misspelled key would silently contribute " \
                         "nothing."
      end

      source = scalar(runner, raw, "source", required: true)
      classification = clause(runner, raw, "classification")
      note = clause(runner, raw, "note")

      unless raw.key?("skips")
        raise Malformed, "#{RELATIVE_PATH}: `#{runner}` has no `skips` key. Write `skips: []` and a " \
                         "`note` saying why — an absent list and an empty one read the same to a " \
                         "comparison, and only one of them was written on purpose."
      end

      skips = raw.fetch("skips")
      unless skips.is_a?(Array)
        raise Malformed, "#{RELATIVE_PATH}: `#{runner}.skips` is #{describe(skips)}, not a list"
      end

      entries = skips.each_with_index.map { |entry, i| skip(runner, entry, i) }

      # Array#- removes EVERY occurrence, so a case named twice is invisible to
      # the manifest comparison: both directions come back empty and the roster
      # passes while carrying two possibly conflicting reasons for one case.
      # Refused here rather than at the comparison so the render half cannot
      # emit it either.
      dupes = entries.map(&:name).tally.select { |_, n| n > 1 }.keys
      unless dupes.empty?
        raise Malformed, "#{RELATIVE_PATH}: `#{runner}` lists #{dupes.first.inspect} more than " \
                         "once; the roster promises one line per runner × test, and a repeat is " \
                         "invisible to the set comparison"
      end

      if entries.empty? && note.nil?
        raise Malformed, "#{RELATIVE_PATH}: `#{runner}` skips nothing and says nothing. An empty " \
                         "`skips` list needs a `note` explaining it — 'none' with no reason is " \
                         "indistinguishable from a section somebody forgot to fill in."
      end

      Section.new(runner: runner, source: source, classification: classification, note: note,
                  skips: entries)
    end

    def skip(runner, raw, index)
      where = "#{RELATIVE_PATH}: `#{runner}.skips[#{index}]`"
      raise Malformed, "#{where} is #{describe(raw)}, not a mapping" unless raw.is_a?(Hash)

      extra = raw.keys - SKIP_KEYS
      unless extra.empty?
        raise Malformed, "#{where} carries unknown key(s) #{extra.map(&:to_s).sort.join(', ')}; " \
                         "a skip is #{SKIP_KEYS.join(' + ')} and nothing else"
      end

      Skip.new(name: scalar(runner, raw, "case", required: true, where: "#{where} `case`"),
               reason: scalar(runner, raw, "reason", required: true, where: "#{where} `reason`"))
    end

    # A `scalar` that is also a clause: see CLAUSE_TERMINATOR_RE for why the
    # renderer cannot tolerate one that terminates itself.
    def clause(runner, raw, key)
      value = scalar(runner, raw, key)
      if value&.match?(CLAUSE_TERMINATOR_RE)
        raise Malformed, "#{RELATIVE_PATH}: `#{runner}.#{key}` ends in terminating punctuation " \
                         "(#{value[-1].inspect}). It is a clause the renderer punctuates: it is " \
                         "joined to the other qualifiers with `; ` and the heading is closed with " \
                         "`.` or `:`, so #{value.inspect} renders as #{"#{value}.".inspect}. Drop " \
                         "the final character."
      end

      value
    end

    # A present, non-empty, single-line string — or nil when the key is absent.
    #
    # An explicit `null` is refused rather than treated as absence: it is a key
    # somebody wrote and left empty, which is a different act from omitting it,
    # and the difference is exactly the one the empty-`skips` rule above turns
    # on.
    #
    # Newlines are refused because every value here is rendered into ONE line.
    # A multi-line note does not corrupt the byte comparison — SPEC would carry
    # the newline too — but it would let a value spell a second heading or
    # bullet, leaving a block that misdescribes its own shape while matching
    # perfectly.
    def scalar(runner, raw, key, required: false, where: nil)
      where ||= "#{RELATIVE_PATH}: `#{runner}.#{key}`"
      unless raw.key?(key)
        raise Malformed, "#{where} is required" if required

        return nil
      end

      value = raw.fetch(key)
      raise Malformed, "#{where} is #{describe(value)}, not a string" unless value.is_a?(String)
      raise Malformed, "#{where} is empty" if value.strip.empty?
      raise Malformed, "#{where} spans more than one line; every value renders onto one" if value.include?("\n")

      value
    end

    def describe(value)
      value.nil? ? "null" : "a #{value.class}"
    end
  end
end
