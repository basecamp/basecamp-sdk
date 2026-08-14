#!/usr/bin/env ruby
# frozen_string_literal: true

# check-fixture-execution — is any conformance fixture case executed by NOTHING?
#
# This is #602. Each runner's own case census (#742) answers a narrower
# question: "did THIS runner account for every case it should have". A case that
# every runner deliberately excludes leaves all six censuses green, because each
# one counted its own skip. Only a comparison ACROSS runners can see it, and
# that is what this gate does.
#
# It reads the execution manifests the runners write (conformance/manifests/
# <runner>.json, one per runner, holding the cases that runner did not execute
# and why) and fails when a case appears in every one of them.
#
# ## Why manifests rather than parsed output
#
# Five runners print `SKIP: <name>`; TypeScript does not, because a skip there
# is `it.skip` and vitest reports it in its own format. A gate scraping stdout
# would be blind to exactly one runner, in the silent direction: TypeScript
# would contribute an empty exclusion set and no case could ever reach all-six.
# The manifests are written from the counters each run loop increments.
#
# ## The absence rule, which is the whole design problem
#
# A gate over six inputs is only as good as its behaviour when an input is
# missing, and "missing" is the normal state here: `conformance-swift` is
# macOS-only, so a Linux run produces five manifests and never a sixth.
#
# The rule is therefore split, and neither half can go vacuous:
#
#   FULL mode (default) requires ALL SIX manifests and fails if any is absent.
#   A missing manifest is never "assume that runner ran everything" — that
#   assumption is exactly what makes an all-six case invisible. This is the mode
#   CI runs, after collecting the Linux five and the macOS one.
#
#   PARTIAL mode (--partial) is for a run that cannot produce all six. It
#   reports a case excluded by every VISIBLE runner as a WARNING and exits 0.
#   That is deliberately not a failure: five-of-six is not the all-six claim,
#   and Swift may well execute the case. A warning cannot produce a false
#   failure, which is the only reason it is allowed to run on partial input.
#
# Both modes fail on ZERO manifests. An empty directory means the runners did
# not run, and a gate that reports success over no input certifies nothing —
# the vacuous pass this whole family of checks exists to refuse.
#
# ## Green on arrival, which is why the self-test matters
#
# Maximum overlap today is 2 of 6 (#596 narrowed it), so this gate passes on the
# current tree and a live run only ever proves it can say yes.
# scripts/test-check-fixture-execution.rb crafts the all-six state and asserts
# it says no.
#
# Exit non-zero on any violation.

require "json"
require "optparse"
require "set"

# Overridable so the self-test can point the whole gate at a synthetic manifest
# set and prove each rejection fires. Same mechanism as DOC_CONSTANTS_ROOT: this
# check is green on the real tree by construction (maximum overlap is 2 of 6),
# so a live run only ever proves it can say yes.
ROOT = File.expand_path(ENV.fetch("FIXTURE_EXECUTION_ROOT", File.expand_path("..", __dir__)))

# The six runners that must report. Hardcoded ON PURPOSE: deriving this from
# whatever files happen to be present is precisely the absence bug — a runner
# that stopped writing its manifest would silently shrink the expected set and
# the gate would go on reporting success over five, then four.
EXPECTED_RUNNERS = %w[go kotlin python ruby swift typescript].freeze

MANIFEST_DIR = File.join(ROOT, "conformance", "manifests")

Manifest = Struct.new(:runner, :total, :executed, :excluded, keyword_init: true)

class Failure < StandardError; end

def load_manifest(path)
  raw = JSON.parse(File.read(path))
  # Shape-check before reaching for keys. A manifest that is a JSON string or
  # array still parses, and `fetch` on it raises NoMethodError — which exits
  # non-zero, so the gate is fail-closed either way, but reports a Ruby
  # backtrace instead of naming the file. Found by the self-test.
  unless raw.is_a?(Hash)
    raise Failure, "#{File.basename(path)}: manifest is not a JSON object (got #{raw.class})"
  end

  runner = raw.fetch("runner")
  entries = raw.fetch("excluded")
  unless entries.is_a?(Array)
    raise Failure, "#{File.basename(path)}: `excluded` is not an array (got #{entries.class})"
  end

  # Keyed on [file, name], NOT name alone. Case names are not unique across
  # fixtures — "replace-omission-clears: sparse replace sends the request
  # verbatim with no GET" appears in three files and the non-idempotent POST
  # retry name in two — while names ARE unique within a file. Keyed on name
  # alone, a runner excluding one of those collapses two entries into one: its
  # own `executed + excluded` integrity check then fails spuriously, and two
  # genuinely different cases become indistinguishable in the intersection
  # below, where a name excluded by three runners in one file and three in
  # another would read as excluded by all six.
  excluded = entries.map do |entry|
    unless entry.is_a?(Hash)
      raise Failure, "#{File.basename(path)}: an `excluded` entry is not an object"
    end

    [[entry.fetch("file"), entry.fetch("name")], entry["reason"].to_s]
  end

  duplicated = excluded.map(&:first).tally.select { |_, n| n > 1 }.keys
  unless duplicated.empty?
    raise Failure, "#{File.basename(path)}: #{duplicated.first.join(' / ')} is excluded twice; " \
                   "one case, one exclusion"
  end

  excluded = excluded.to_h

  manifest = Manifest.new(
    runner: runner,
    total: raw.fetch("total_non_live"),
    executed: raw.fetch("executed"),
    excluded: excluded
  )

  # The same invariant each runner asserts before writing, re-checked here
  # because this gate is downstream of a file anyone could hand-edit — and
  # because "absent from the exclusion set" and "ran fine" are indistinguishable
  # to the comparison below. If they can drift, a dropped case reads as covered.
  actual = manifest.executed + manifest.excluded.length
  if actual != manifest.total
    raise Failure, "#{File.basename(path)}: #{manifest.executed} executed + " \
                   "#{manifest.excluded.length} excluded = #{actual}, but the runner reports " \
                   "#{manifest.total} non-live case(s). A case went unrecorded as either."
  end

  manifest
rescue JSON::ParserError => e
  raise Failure, "#{File.basename(path)}: not valid JSON (#{e.message})"
rescue KeyError => e
  raise Failure, "#{File.basename(path)}: missing required key (#{e.message})"
end

def load_all
  unless Dir.exist?(MANIFEST_DIR)
    raise Failure, "no manifest directory at conformance/manifests — run `make conformance` first"
  end

  paths = Dir.glob(File.join(MANIFEST_DIR, "*.json")).sort
  raise Failure, "conformance/manifests holds no manifests — the runners did not report" if paths.empty?

  manifests = paths.map { |p| load_manifest(p) }

  by_runner = manifests.group_by(&:runner)
  by_runner.select { |_, v| v.length > 1 }.each_key do |runner|
    raise Failure, "two manifests both claim to be `#{runner}`"
  end

  unknown = by_runner.keys - EXPECTED_RUNNERS
  unless unknown.empty?
    raise Failure, "manifest(s) from unknown runner(s): #{unknown.sort.join(', ')}. " \
                   "Add them to EXPECTED_RUNNERS or the comparison silently ignores them."
  end

  # Every runner censuses the same fixture tree, so a disagreement means one of
  # them read a different tree — which makes the exclusion sets incomparable
  # long before it makes them wrong.
  totals = manifests.map(&:total).uniq
  if totals.length > 1
    detail = manifests.map { |m| "#{m.runner}=#{m.total}" }.sort.join(", ")
    raise Failure, "runners disagree on how many non-live cases exist (#{detail}); " \
                   "their exclusion sets are not comparable"
  end

  manifests
end

# --- SPEC section 19 Zero-Skip roster (#736) ---------------------------------
#
# The roster claims, in its own text, to enumerate "every skip a default
# (mock-mode) conformance run reports, one line per runner x test, verbatim
# from the runners' skip mechanisms". Nothing checked that, and it was already
# untrue when this was written: Kotlin and Swift each exclude the `link-header`
# case wholesale via their tag branch, and the roster described that in prose
# instead of enumerating it -- two of six runners wrong, in a roster nobody
# re-derives by hand.
#
# The manifests make the ENUMERATION derivable, so it is now checked for set
# equality. The classification and reasoning on each line stay judgement, and
# nothing asserts them; that half is why the section keeps its `[manual]` tag.
#
# WHY THIS PARSER IS ACCEPTABLE WHERE THE ROSTER-TABLE ONE WAS NOT. #740
# declined to keep teaching `sync-doc-constants.rb` more GFM -- separator
# widths, backslash parity -- because a mis-parse there is SILENT: it validates
# the wrong cell and reports success. This extraction fails LOUD in both
# directions. A bullet it cannot read is a name missing from the roster set,
# which is a mismatch; a name it invents is an extra, also a mismatch. There is
# no reading of a malformed line that produces a passing comparison, so the
# failure mode is a false alarm the author fixes, never a false green.
#
# The delimiters are deliberately NOT `@`-markers. sync-doc-constants owns
# those, and it runs in spec-gates where no conformance run has happened, so it
# has no manifests to compare against. Registering a kind there whose real
# enforcement lives here would split one check across two gates.
ROSTER_BEGIN = "<!-- zero-skip-roster:begin -->"
ROSTER_END   = "<!-- zero-skip-roster:end -->"

# Heading label => runner id. Hardcoded for the same reason EXPECTED_RUNNERS is:
# deriving it from whatever headings appear lets a renamed section silently drop
# a runner out of the comparison.
ROSTER_HEADINGS = {
  "Go" => "go", "Python" => "python", "Ruby" => "ruby",
  "TypeScript" => "typescript", "Kotlin" => "kotlin", "Swift" => "swift"
}.freeze

# Returns { runner => [case name] } as the roster states it.
def parse_roster(spec_path)
  text = File.read(spec_path, encoding: "UTF-8")
  i = text.index(ROSTER_BEGIN)
  j = text.index(ROSTER_END)
  raise Failure, "SPEC.md holds no #{ROSTER_BEGIN} / #{ROSTER_END} pair" if i.nil? || j.nil? || j < i

  body = text[(i + ROSTER_BEGIN.length)...j]
  roster = Hash.new { |h, k| h[k] = [] }
  runner = nil
  seen = []

  body.each_line do |line|
    if (m = line.match(/\A\*\*([A-Za-z]+)\*\*/))
      label = m[1]
      runner = ROSTER_HEADINGS[label]
      raise Failure, "SPEC roster has a section for unknown runner #{label.inspect}" if runner.nil?

      seen << runner
      roster[runner]
      next
    end

    next unless line.start_with?("- ")
    raise Failure, "SPEC roster bullet before any runner heading: #{line.strip[0, 60]}" if runner.nil?

    name = line[/\A-\s+"([^"]+)"/, 1]
    if name.nil?
      raise Failure, "SPEC roster bullet does not open with a quoted case name: #{line.strip[0, 80]}"
    end

    roster[runner] << name
  end

  missing = EXPECTED_RUNNERS - seen
  unless missing.empty?
    raise Failure, "SPEC roster has no section for: #{missing.join(', ')} - a runner without a " \
                   "heading contributes nothing and its skips go unrecorded"
  end

  roster
end

# Compares the roster's enumeration against what the runners reported.
def check_roster(manifests, spec_path)
  roster = parse_roster(spec_path)
  errors = []

  manifests.each do |m|
    actual = m.excluded.keys.map(&:last)

    # The roster identifies a case by NAME alone, so it cannot express two
    # same-named cases from different fixtures. No runner excludes such a pair
    # today; if one ever does, the roster needs file qualifiers and this says so
    # rather than silently comparing an ambiguous set.
    dupes = actual.tally.select { |_, n| n > 1 }.keys
    unless dupes.empty?
      errors << "#{m.runner} excludes #{dupes.first.inspect} in more than one fixture; the SPEC " \
                "roster identifies cases by name alone and cannot express that. Add the fixture " \
                "to those roster lines and teach this check to read it."
      next
    end

    stated = roster[m.runner]
    (actual - stated).each do |name|
      errors << "#{m.runner} excludes #{name.inspect} and the SPEC roster does not list it"
    end
    (stated - actual).each do |name|
      errors << "the SPEC roster lists #{name.inspect} for #{m.runner}, which no longer excludes it"
    end
  end

  errors
end

def run(partial:)
  manifests = load_all
  present = manifests.map(&:runner).sort
  missing = EXPECTED_RUNNERS - present

  if !missing.empty? && !partial
    raise Failure, "missing manifest(s) for: #{missing.join(', ')}. Every runner must report " \
                   "before an all-six claim can be made; a missing manifest is not a runner that " \
                   "executed everything. (Swift is macOS-only — use --partial for a run that " \
                   "cannot produce all six.)"
  end

  # name => the runners that did NOT execute it
  by_case = Hash.new { |h, k| h[k] = [] }
  manifests.each do |m|
    m.excluded.each_key { |id| by_case[id] << m.runner }
  end

  everywhere = by_case.select { |_, runners| runners.sort == present }

  # `--partial` is a statement about the INPUT, not a licence to soften the
  # verdict. When every expected runner reported anyway — a macOS developer
  # passing the flag out of habit, or a CI step that keeps it for safety —
  # "excluded by all present runners" IS the all-six claim, and downgrading it
  # to a warning would let the one state this gate exists to reject exit 0.
  # Partial handling applies only when a manifest is genuinely absent.
  partial &&= !missing.empty?

  if partial
    puts "==> Fixture execution (partial: #{present.length} of #{EXPECTED_RUNNERS.length} runners — " \
         "#{missing.join(', ')} did not report)"
    everywhere.each do |id, runners|
      warn "WARN: #{id.join(' / ').inspect} is excluded by all #{runners.length} reporting runner(s) " \
           "(#{runners.join(', ')}); #{missing.join(', ')} unknown. Not a failure — " \
           "#{missing.length == 1 ? 'that runner' : 'those runners'} may execute it. Re-run on a " \
           "host that can produce every manifest to settle it."
    end
    puts "    #{manifests.first.total} non-live case(s); #{everywhere.length} excluded by every " \
         "reporting runner"
    return 0
  end

  unless everywhere.empty?
    everywhere.each do |id, runners|
      warn "FAIL: #{id.join(' / ').inspect} is excluded by ALL #{runners.length} runners — it is a " \
           "fixture case executed by nothing, which no single runner's census can see."
      manifests.each do |m|
        reason = m.excluded[id]
        warn "        #{m.runner}: #{reason.empty? ? '(no reason recorded)' : reason}"
      end
    end
    return 1
  end

  roster_errors = check_roster(manifests, File.join(ROOT, "SPEC.md"))
  unless roster_errors.empty?
    roster_errors.each { |e| warn "FAIL: #{e}" }
    warn "      SPEC section 19's Zero-Skip roster claims to enumerate every skip verbatim from " \
         "the runners' skip mechanisms (#736). It is restated by hand, so it drifts."
    return 1
  end

  overlap = by_case.values.map(&:length).max || 0
  puts "==> Fixture execution: #{manifests.first.total} non-live case(s) across " \
       "#{present.length} runners; none excluded everywhere (maximum overlap #{overlap} of " \
       "#{EXPECTED_RUNNERS.length})"
  0
end

if __FILE__ == $PROGRAM_NAME
  partial = false
  OptionParser.new do |o|
    o.banner = "Usage: check-fixture-execution.rb [--partial]"
    o.on("--partial", "Accept fewer than six manifests; report all-visible overlap as a warning") do
      partial = true
    end
  end.parse!

  begin
    exit run(partial: partial)
  rescue Failure => e
    warn "FAIL: #{e.message}"
    exit 1
  end
end
