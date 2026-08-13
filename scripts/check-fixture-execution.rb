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

  excluded = entries.map do |entry|
    unless entry.is_a?(Hash)
      raise Failure, "#{File.basename(path)}: an `excluded` entry is not an object"
    end

    [entry.fetch("name"), entry["reason"].to_s]
  end.to_h

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
    m.excluded.each_key { |name| by_case[name] << m.runner }
  end

  everywhere = by_case.select { |_, runners| runners.sort == present }

  if partial
    puts "==> Fixture execution (partial: #{present.length} of #{EXPECTED_RUNNERS.length} runners — " \
         "#{missing.join(', ')} did not report)"
    everywhere.each do |name, runners|
      warn "WARN: #{name.inspect} is excluded by all #{runners.length} reporting runner(s) " \
           "(#{runners.join(', ')}); #{missing.join(', ')} unknown. Not a failure — " \
           "#{missing.length == 1 ? 'that runner' : 'those runners'} may execute it. Re-run on a " \
           "host that can produce every manifest to settle it."
    end
    puts "    #{manifests.first.total} non-live case(s); #{everywhere.length} excluded by every " \
         "reporting runner"
    return 0
  end

  unless everywhere.empty?
    everywhere.each do |name, runners|
      warn "FAIL: #{name.inspect} is excluded by ALL #{runners.length} runners — it is a fixture " \
           "case executed by nothing, which no single runner's census can see."
      manifests.each do |m|
        reason = m.excluded[name]
        warn "        #{m.runner}: #{reason.empty? ? '(no reason recorded)' : reason}"
      end
    end
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
