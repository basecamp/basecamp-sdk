#!/usr/bin/env ruby
# frozen_string_literal: true

# Negative-case self-test for scripts/check-fixture-execution.rb.
#
# The gate is green on the real repo BY CONSTRUCTION — maximum overlap is 2 of 6
# (#596 narrowed it), so `make check-fixture-execution` proves only that it can
# say yes. The one state it exists to reject cannot be produced by any committed
# fixture: a case every runner excludes. This crafts exactly that, plus every
# way the gate could report success over input it should refuse.
#
# Same shape and reason as scripts/test-doc-constants.rb, driving the gate
# through its FIXTURE_EXECUTION_ROOT override against a throwaway tree.
#
# Run directly, or via `make check-fixture-execution` (which runs it after the
# live check).

require "json"
require "tmpdir"
require "fileutils"
require "open3"

GATE = File.join(__dir__, "check-fixture-execution.rb")
RUNNERS = %w[go kotlin python ruby swift typescript].freeze

failures = []

def utf8(out) = out.dup.force_encoding("UTF-8")

# A manifest set where every runner ran all 3 cases and excluded nothing.
def default_manifests
  RUNNERS.to_h do |runner|
    [runner, { "runner" => runner, "total_non_live" => 3, "executed" => 3, "excluded" => [] }]
  end
end

# Materialise the manifests (after `mutate` has had a chance to break one) and
# run the gate against them.
def gate(mutate = nil, partial: false)
  manifests = default_manifests
  mutate&.call(manifests)

  Dir.mktmpdir do |dir|
    FileUtils.mkdir_p(File.join(dir, "conformance", "manifests"))
    manifests.each do |runner, body|
      next if body.nil? # a nil entry means "this runner did not report"

      File.write(File.join(dir, "conformance", "manifests", "#{runner}.json"),
                 "#{JSON.pretty_generate(body)}\n")
    end

    cmd = ["ruby", GATE]
    cmd << "--partial" if partial
    out, status = Open3.capture2e({ "FIXTURE_EXECUTION_ROOT" => dir }, *cmd)
    [utf8(out), status]
  end
end

def expect_pass(failures, label, out, status)
  return if status.success?

  failures << "#{label}: expected SUCCESS, gate exited #{status.exitstatus}:\n#{out}"
end

def expect_fail(failures, label, out, status, fragment)
  if status.success?
    failures << "#{label}: expected FAILURE, gate exited 0:\n#{out}"
  elsif !out.include?(fragment)
    failures << "#{label}: failed as expected but without #{fragment.inspect}:\n#{out}"
  end
end

# Adds `name` to the exclusion set of the named runners.
def exclude(manifests, name, runners)
  runners.each do |runner|
    m = manifests.fetch(runner)
    m["excluded"] << { "name" => name, "reason" => "#{runner} cannot do this" }
    m["executed"] -= 1
  end
end

# --- positive control --------------------------------------------------------

out, status = gate
expect_pass(failures, "positive control: six clean manifests", out, status)

# --- THE case the gate exists to reject --------------------------------------

# Excluded by every runner. No single runner's census can see this: each one
# counted its own skip and reported a matching total.
out, status = gate lambda { |m| exclude(m, "dead fixture case", RUNNERS) }
expect_fail(failures, "case excluded by all six runners", out, status,
            "is excluded by ALL 6 runners")

# The boundary. Five of six is NOT the all-six claim — the sixth runner does
# execute the case, so failing here would be a false alarm, and this is the case
# that would fire if the comparison were written as "excluded by most".
out, status = gate lambda { |m| exclude(m, "thin but covered", RUNNERS - ["swift"]) }
expect_pass(failures, "five of six is not all six", out, status)

# --- the absence rule --------------------------------------------------------

# A missing manifest must never read as "that runner executed everything" —
# that assumption is exactly what makes an all-six case invisible.
out, status = gate lambda { |m| m["swift"] = nil }
expect_fail(failures, "a missing manifest fails full mode", out, status,
            "missing manifest(s) for: swift")

# The same input in partial mode is the macOS-only Swift lane's normal state.
out, status = gate(lambda { |m| m["swift"] = nil }, partial: true)
expect_pass(failures, "partial mode accepts five manifests", out, status)

# Excluded by every VISIBLE runner, with one absent. This must WARN and pass:
# five-of-six is not the all-six claim, and a warning cannot false-fail — which
# is the only reason partial mode is allowed to run at all.
out, status = gate(lambda { |m|
  exclude(m, "unsettled case", RUNNERS - ["swift"])
  m["swift"] = nil
}, partial: true)
expect_pass(failures, "partial mode warns rather than fails", out, status)
unless out.include?("WARN:") && out.include?("unsettled case")
  failures << "partial mode should WARN about an all-visible exclusion:\n#{out}"
end

# Zero manifests is not agreement. An empty directory means the runners did not
# run, and a gate reporting success over no input certifies nothing.
out, status = gate lambda { |m| RUNNERS.each { |r| m[r] = nil } }
expect_fail(failures, "no manifests at all", out, status, "did not report")

# ...in partial mode too, which is the mode most likely to be pointed at an
# empty directory by accident.
out, status = gate(lambda { |m| RUNNERS.each { |r| m[r] = nil } }, partial: true)
expect_fail(failures, "no manifests at all, partial mode", out, status, "did not report")

# --- manifest integrity ------------------------------------------------------

# `executed + excluded` must equal the census total. Without this, a case a
# runner silently DROPPED is simply absent from its exclusion set — and absent
# reads identically to "ran fine", so the comparison would score the case as
# covered by that runner precisely when it was not.
out, status = gate lambda { |m| m["ruby"]["executed"] = 2 }
expect_fail(failures, "a manifest that does not add up", out, status,
            "A case went unrecorded as either")

# Runners censusing different trees produce incomparable exclusion sets, and the
# disagreement is the finding — long before any exclusion is compared.
out, status = gate lambda { |m|
  m["python"]["total_non_live"] = 4
  m["python"]["executed"] = 4
}
expect_fail(failures, "runners disagreeing on the case count", out, status,
            "disagree on how many non-live cases exist")

# A runner nobody expects is silently ignored by any set comparison keyed on the
# expected roster — so it is rejected instead of being quietly dropped.
out, status = gate lambda { |m|
  m["haskell"] = { "runner" => "haskell", "total_non_live" => 3, "executed" => 3, "excluded" => [] }
}
expect_fail(failures, "a manifest from an unknown runner", out, status,
            "unknown runner")

# Two files both claiming one runner: whichever loses the race would silently
# decide that runner's exclusion set.
out, status = gate lambda { |m| m["swift"] = m["go"].dup }
expect_fail(failures, "two manifests claiming one runner", out, status,
            "both claim to be `go`")

out, status = gate lambda { |m| m["ruby"] = "not an object" }
expect_fail(failures, "a manifest that is not valid JSON structure", out, status, "ruby.json")

out, status = gate lambda { |m| m["kotlin"].delete("executed") }
expect_fail(failures, "a manifest missing a required key", out, status, "missing required key")

# --- report ------------------------------------------------------------------

if failures.empty?
  puts "check-fixture-execution self-test: all cases passed."
  exit 0
end

warn "check-fixture-execution self-test: #{failures.length} case(s) failed"
failures.each { |f| warn "\n#{f}" }
exit 1
