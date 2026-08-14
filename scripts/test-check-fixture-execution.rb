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
ROSTER_LABELS = { "go" => "Go", "kotlin" => "Kotlin", "python" => "Python",
                  "ruby" => "Ruby", "swift" => "Swift", "typescript" => "TypeScript" }.freeze

failures = []

def utf8(out) = out.dup.force_encoding("UTF-8")

# A manifest set where every runner ran all 3 cases and excluded nothing.
def default_manifests
  RUNNERS.to_h do |runner|
    [runner, { "runner" => runner, "total_non_live" => 3, "executed" => 3, "excluded" => [] }]
  end
end

# A SPEC roster that agrees with whatever the manifests say, so roster drift is
# only ever exercised by a case that asks for it.
def default_spec(manifests)
  labels = ROSTER_LABELS
  body = RUNNERS.map do |runner|
    lines = ["**#{labels.fetch(runner)}** (`x`):"]
    # Defensive: some cases replace a manifest with a non-Hash or nil on
    # purpose, and this helper only exists to keep the roster in step with the
    # ones that are real.
    entry = manifests[runner]
    excluded = entry.is_a?(Hash) ? (entry["excluded"] || []) : []
    excluded.each do |e|
      lines << %(- "#{e['name']}" — because.)
    end
    lines.join("\n")
  end.join("\n\n")

  "# Spec\n\n<!-- zero-skip-roster:begin -->\n#{body}\n<!-- zero-skip-roster:end -->\n"
end

# Materialise the manifests (after `mutate` has had a chance to break one) and
# run the gate against them.
def gate(mutate = nil, partial: false, spec: nil)
  manifests = default_manifests
  mutate&.call(manifests)

  Dir.mktmpdir do |dir|
    FileUtils.mkdir_p(File.join(dir, "conformance", "manifests"))
    File.write(File.join(dir, "SPEC.md"), spec || default_spec(manifests))
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

# Adds a case to the exclusion set of the named runners. Identity is
# [file, name]: case names are not unique across fixtures.
def exclude(manifests, name, runners, file: "alpha.json")
  runners.each do |runner|
    m = manifests.fetch(runner)
    m["excluded"] << { "file" => file, "name" => name, "reason" => "#{runner} cannot do this" }
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

# `--partial` describes the INPUT, not a softer verdict. With every runner
# reporting anyway — a macOS developer passing the flag out of habit — this IS
# the all-six claim, and warning here would let the one state the gate exists to
# reject exit 0.
out, status = gate(lambda { |m| exclude(m, "dead fixture case", RUNNERS) }, partial: true)
expect_fail(failures, "partial flag does not soften a complete six-runner set", out, status,
            "is excluded by ALL 6 runners")

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

# --- case identity is [file, name], not name ---------------------------------

# The same NAME in two different fixtures is two different cases. Three files
# share "replace-omission-clears: sparse replace sends the request verbatim with
# no GET" and two share the non-idempotent POST retry name, so a name-keyed
# comparison would read "excluded by three runners in one file and three in
# another" as excluded by all six — a false failure on cases that all run.
out, status = gate lambda { |m|
  exclude(m, "shared name", RUNNERS.first(3), file: "alpha.json")
  exclude(m, "shared name", RUNNERS.last(3), file: "beta.json")
}
expect_pass(failures, "one name in two files is two cases, not an all-six overlap", out, status)

# ...and the converse: the SAME case in the same file, excluded everywhere,
# still fails. Without this the case above could be "fixed" by never matching.
out, status = gate lambda { |m| exclude(m, "shared name", RUNNERS, file: "alpha.json") }
expect_fail(failures, "one name in one file excluded everywhere still fails", out, status,
            "is excluded by ALL 6 runners")

# A manifest whose entries lack `file` cannot be compared by identity at all.
# This is also what a manifest written by a pre-#602 runner looks like.
out, status = gate lambda { |m|
  m["ruby"]["excluded"] << { "name" => "no file key", "reason" => "..." }
  m["ruby"]["executed"] -= 1
}
expect_fail(failures, "an exclusion without its fixture file", out, status, "missing required key")

# One case excluded twice by one runner: its own integrity count would still add
# up while the comparison saw a single entry.
out, status = gate lambda { |m|
  2.times { m["go"]["excluded"] << { "file" => "alpha.json", "name" => "dup", "reason" => "x" } }
  m["go"]["executed"] -= 2
}
expect_fail(failures, "one case excluded twice in one manifest", out, status, "is excluded twice")

# --- SPEC roster set-equality (#736) -----------------------------------------

# A runner skip the roster does not list. This is the drift that was ALREADY in
# SPEC when the check was written: Kotlin and Swift excluded the link-header
# case via their tag branch while the roster described it in prose.
out, status = gate(lambda { |m| exclude(m, "unlisted skip", ["ruby"]) },
                   spec: "<!-- zero-skip-roster:begin -->\n" +
                         RUNNERS.map { |r| "**#{ ROSTER_LABELS.fetch(r) }** (`x`):" }.join("\n\n") +
                         "\n<!-- zero-skip-roster:end -->\n")
expect_fail(failures, "a runner skip missing from the roster", out, status,
            "the SPEC roster does not list it")

# The opposite drift: a roster line for a skip that has been closed. "A PR that
# closes a gap deletes exactly its own lines" is the roster's own rule, and it
# was enforced by nothing.
out, status = gate(nil,
                   spec: "<!-- zero-skip-roster:begin -->\n" +
                         RUNNERS.map { |r|
                           head = "**#{ ROSTER_LABELS.fetch(r) }** (`x`):"
                           r == "go" ? "#{head}\n- \"a closed gap\" — stale." : head
                         }.join("\n\n") +
                         "\n<!-- zero-skip-roster:end -->\n")
expect_fail(failures, "a roster line for a skip that no longer exists", out, status,
            "which no longer excludes it")

# A runner with no heading contributes nothing, so its skips would go unchecked.
out, status = gate(nil, spec: "<!-- zero-skip-roster:begin -->\n**Go** (`x`):\n<!-- zero-skip-roster:end -->\n")
expect_fail(failures, "a runner with no roster section", out, status, "has no section for")

# No roster at all must not read as "the roster agrees".
out, status = gate(nil, spec: "# Spec\n\nNo roster here.\n")
expect_fail(failures, "SPEC with no roster block", out, status, "holds no")

# A bullet the extractor cannot read is a name missing from the roster set —
# loud, never a silent pass. This is the property that makes the parser
# acceptable at all.
out, status = gate(nil,
                   spec: "<!-- zero-skip-roster:begin -->\n**Go** (`x`):\n- unquoted case name\n" +
                         (RUNNERS - ["go"]).map { |r| "**#{ ROSTER_LABELS.fetch(r) }** (`x`):" }.join("\n\n") +
                         "\n<!-- zero-skip-roster:end -->\n")
expect_fail(failures, "a roster bullet without a quoted name", out, status,
            "list-shaped but not a canonical bullet")

# Roster drift must be caught in PARTIAL mode too. The normal Linux `make
# check` path always passes --partial (Swift's manifest is macOS-only), so a
# roster check that ran only in full mode never ran locally at all — a stale Go
# or Ruby line would reach CI untouched. Partial input relaxes the all-six
# overlap verdict and nothing else.
out, status = gate(lambda { |m|
  exclude(m, "unlisted skip", ["ruby"])
  m["swift"] = nil
}, partial: true,
   spec: "<!-- zero-skip-roster:begin -->\n" +
         RUNNERS.map { |r| "**#{ROSTER_LABELS.fetch(r)}** (`x`):" }.join("\n\n") +
         "\n<!-- zero-skip-roster:end -->\n")
expect_fail(failures, "roster drift is caught in partial mode", out, status,
            "the SPEC roster does not list it")

# A SECOND complete roster block is not compared, so a stale line inside it
# would pass unnoticed — a silent pass, the one failure mode this extractor is
# not allowed to have.
one_block = RUNNERS.map { |r| "**#{ROSTER_LABELS.fetch(r)}** (`x`):" }.join("\n\n")
out, status = gate(nil,
                   spec: "<!-- zero-skip-roster:begin -->\n#{one_block}\n<!-- zero-skip-roster:end -->\n" \
                         "\n<!-- zero-skip-roster:begin -->\n#{one_block}\n<!-- zero-skip-roster:end -->\n")
expect_fail(failures, "two roster blocks in SPEC", out, status, "exactly one")

# One case listed twice under a runner. Array#- removes every matching
# occurrence, so both diffs come back empty and the duplicate passes — carrying
# two possibly conflicting classifications for one case.
out, status = gate(lambda { |m| exclude(m, "listed twice", ["go"]) },
                   spec: "<!-- zero-skip-roster:begin -->\n" +
                         RUNNERS.map { |r|
                           head = "**#{ROSTER_LABELS.fetch(r)}** (`x`):"
                           r == "go" ? "#{head}\n- \"listed twice\" — a.\n- \"listed twice\" — b." : head
                         }.join("\n\n") +
                         "\n<!-- zero-skip-roster:end -->\n")
expect_fail(failures, "one case listed twice under a runner", out, status, "twice under go")

# Two sections for one runner splits its lines, so neither reads as the whole
# set and the contract is broken before any comparison runs.
out, status = gate(nil,
                   spec: "<!-- zero-skip-roster:begin -->\n" +
                         (RUNNERS.map { |r| "**#{ROSTER_LABELS.fetch(r)}** (`x`):" } +
                          ["**Go** (`x`):"]).join("\n\n") +
                         "\n<!-- zero-skip-roster:end -->\n")
expect_fail(failures, "two roster sections for one runner", out, status, "more than one section")

# A STALE entry written with any non-canonical list marker. `start_with?("- ")`
# skipped these, so the entry never entered the roster set, never contradicted a
# manifest, and the gate passed — a false green, the one outcome this extractor
# may not produce. Fixed by inverting the default: list-shaped means canonical
# or error, which closes every spelling at once rather than one at a time.
["  - \"indented stale\" — x.", "* \"asterisk stale\" — x.",
 "+ \"plus stale\" — x.", "1. \"ordered stale\" — x."].each do |bullet|
  out, status = gate(nil,
                     spec: "<!-- zero-skip-roster:begin -->\n" +
                           RUNNERS.map { |r|
                             head = "**#{ROSTER_LABELS.fetch(r)}** (`x`):"
                             r == "go" ? "#{head}\n#{bullet}" : head
                           }.join("\n\n") +
                           "\n<!-- zero-skip-roster:end -->\n")
  expect_fail(failures, "stale roster entry as #{bullet.strip[0, 12]}", out, status,
              "list-shaped but not a canonical bullet")
end

# --- report ------------------------------------------------------------------

if failures.empty?
  puts "check-fixture-execution self-test: all cases passed."
  exit 0
end

warn "check-fixture-execution self-test: #{failures.length} case(s) failed"
failures.each { |f| warn "\n#{f}" }
exit 1
