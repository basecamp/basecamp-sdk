#!/usr/bin/env ruby
# frozen_string_literal: true

# Negative-case self-test for scripts/check-fixture-execution.
#
# MANDATORY, not optional. The gate is green on this repo and will stay green:
# maximum overlap across the six runners is 2 of 6, so a live run can only ever
# prove the gate says yes. Its central case — one fixture case skipped by ALL
# SIX — cannot occur in this checkout, so the only way to know the gate can say
# no is to craft that state.
#
# Same shape and reason as scripts/test-doc-constants.rb: a throwaway git repo
# holding stub runners at the real paths, driven through the gate's
# FIXTURE_EXECUTION_ROOT override, with each case breaking exactly one thing.
#
# The crafted repo is GENERATED from one `{runner => [case name]}` description,
# stubs and SPEC roster together. Written by hand they would drift apart, and a
# mutation meant to test the all-six check would instead trip the roster check
# — proving the wrong thing, or nothing.
#
# The stubs deliberately use all three empty-literal spellings the real runners
# use — Python's `set()`, Kotlin's `emptyMap()`, Swift's `[:]` — because those
# are what a "non-empty extraction" fail-closed rule would choke on, and two of
# them have no bracketed block to bound at all.
#
# Run directly (`ruby scripts/test-check-fixture-execution.rb`) or via
# `make test-check-fixture-execution`.

require "json"
require "tmpdir"
require "fileutils"
require "open3"

ROOT = File.expand_path("..", __dir__)
GATE = File.join(__dir__, "check-fixture-execution")

GO_MAIN     = "conformance/runner/go/main.go"
PY_RUNNER   = "conformance/runner/python/runner.py"
RB_RUNNER   = "conformance/runner/ruby/runner.rb"
TS_RUNNER   = "conformance/runner/typescript/runner.test.ts"
KT_MAIN     = "kotlin/conformance/src/main/kotlin/com/basecamp/sdk/conformance/Main.kt"
SWIFT_RUNNER = "conformance/runner/swift/Sources/ConformanceRunner/Runner.swift"

failures = []

def utf8(out) = out.dup.force_encoding("UTF-8")

def expect_pass(failures, label, out, status)
  return if status.success?

  failures << "#{label}: expected PASS, gate exited #{status.exitstatus}:\n#{utf8(out)}"
end

def expect_fail(failures, label, out, status, fragment)
  if status.success?
    failures << "#{label}: expected FAILURE, gate exited 0:\n#{utf8(out)}"
  elsif !utf8(out).include?(fragment)
    failures << "#{label}: failed as expected but message missing #{fragment.inspect}:\n#{utf8(out)}"
  end
end

# --- the crafted repo ----------------------------------------------------------
#
# Four cases, chosen so the baseline is not merely "nothing is skipped":
#
#   runs everywhere  no runner excludes it.
#   five of six      named by go, ruby and typescript AND tagged `link-header`,
#                    so kotlin and swift skip the whole case too — FIVE of six,
#                    with python still running it. This must PASS. It is the
#                    near-miss the gate has to distinguish from the real thing,
#                    and a gate that fired on "is skipped anywhere" would reject
#                    the real repo, which carries 14 skips across three runners.
#   link-header only excluded by kotlin and swift only, exactly like the real
#                    pagination.json case.
#   live case        mode "live": excluded by all six mock runners BY DESIGN and
#                    executed by the TypeScript live canary. Out of scope, not a
#                    violation.

BASE_SKIPS = {
  "go" => ["five of six"],
  "python" => [],
  "ruby" => ["five of six"],
  "typescript" => ["five of six"],
  "kotlin" => [],
  "swift" => [],
}.freeze

ROSTER_HEADINGS = {
  "Go" => "go",
  "Python" => "python",
  "Ruby" => "ruby",
  "TypeScript" => "typescript",
  "Kotlin" => "kotlin",
  "Swift" => "swift",
}.freeze

def fixture_cases
  [
    { "name" => "runs everywhere", "operation" => "Op" },
    { "name" => "five of six", "operation" => "Op", "tags" => %w[pagination link-header] },
    { "name" => "link-header only", "operation" => "Op", "tags" => %w[pagination link-header] },
    { "name" => "live case", "operation" => "Op", "mode" => "live" },
  ]
end

def go_source(names)
  +"package main\n\n" \
   "// Tests where the Go SDK's behavior intentionally differs.\n" \
   "var goSDKSkips = map[string]string{\n" +
    names.map { |n| "\t#{n.inspect}: \"go reason\",\n" }.join +
    "}\n"
end

# `set()` / `{}` when empty, exercising the spelling that has no bracketed block
# for an awk-style delimiter search to bound.
def python_source(names)
  skips = names.empty? ? "set()" : "{#{names.map(&:inspect).join(', ')}}"
  reasons = "{#{names.map { |n| "#{n.inspect}: \"python reason\"" }.join(', ')}}"
  "class ConformanceRunner:\n" \
    "    SKIPS: set[str] = #{skips}\n" \
    "    SKIP_REASONS: dict[str, str] = #{reasons}\n"
end

def ruby_source(names)
  +"RUBY_SKIPS = Set.new([\n" +
    names.map { |n| "  #{n.inspect},\n" }.join +
    "].freeze)\n\nRUBY_SKIP_REASONS = {\n" +
    names.map { |n| "  #{n.inspect} => \"ruby reason\",\n" }.join +
    "}.freeze\n"
end

# The value on its own line, as the real table writes it: a key/value pair split
# across lines is what breaks a same-line regex capture.
def ts_source(names)
  +"const TS_SDK_SKIPS: Record<string, string> = {\n" +
    names.map { |n| "  #{n.inspect}:\n    \"typescript reason\",\n" }.join +
    "};\n"
end

def kotlin_source(names)
  literal = if names.empty?
              "emptyMap()"
            else
              "mapOf(#{names.map { |n| "#{n.inspect} to \"kotlin reason\"" }.join(', ')})"
            end
  "private val KOTLIN_SKIPS: Map<String, String> = #{literal}\n\n" \
    "fun run() {\n" \
    "    for (tc in cases) {\n" \
    "        if (\"link-header\" in tc.tags) {\n" \
    "            skipped++\n" \
    "            continue\n" \
    "        }\n" \
    "    }\n" \
    "}\n"
end

def swift_source(names)
  literal = if names.empty?
              "[:]"
            else
              "[#{names.map { |n| "#{n.inspect}: \"swift reason\"" }.join(', ')}]"
            end
  "private let temporarySkips: [String: String] = #{literal}\n\n" \
    "func run() {\n" \
    "    for tc in testCases {\n" \
    "        if tc.allTags.contains(\"link-header\") {\n" \
    "            skipped += 1\n" \
    "            continue\n" \
    "        }\n" \
    "    }\n" \
    "}\n"
end

# The SPEC roster, generated from the same description as the stubs. A "none"
# block still needs its heading: an absent heading is how a runner's skips stop
# being restated at all, which is what the gate rejects.
def spec_source(skips)
  blocks = ROSTER_HEADINGS.map do |heading, runner|
    names = skips.fetch(runner)
    if names.empty?
      "**#{heading}** (`table`) — none.\n\n"
    else
      "**#{heading}** (`table`):\n" +
        names.map { |n| "- #{n.inspect} — some justification only a human can write.\n" }.join +
        "\n"
    end
  end.join

  "# Spec\n\n### Zero-Skip Target `[manual]`\n\n" \
    "One line per runner x test, verbatim from the runners' skip mechanisms.\n\n" \
    "#{blocks}---\n\n## Next Section\n"
end

def default_files(skips = BASE_SKIPS)
  {
    "conformance/tests/alpha.json" => JSON.pretty_generate(fixture_cases),
    "SPEC.md" => spec_source(skips),
    GO_MAIN => go_source(skips.fetch("go")),
    PY_RUNNER => python_source(skips.fetch("python")),
    RB_RUNNER => ruby_source(skips.fetch("ruby")),
    TS_RUNNER => ts_source(skips.fetch("typescript")),
    KT_MAIN => kotlin_source(skips.fetch("kotlin")),
    SWIFT_RUNNER => swift_source(skips.fetch("swift")),
  }
end

def run_gate(skips: BASE_SKIPS, mutate: nil)
  base = Dir.mktmpdir("fixture-execution-test")
  begin
    files = default_files(skips)
    mutate&.call(files)

    files.each do |rel, body|
      path = File.join(base, rel)
      FileUtils.mkdir_p(File.dirname(path))
      File.write(path, body, encoding: "UTF-8")
    end

    # The gate discovers fixtures through `git ls-files`, so an un-added tree
    # would look empty and every case would vacuously "pass".
    Open3.capture2e("git", "-C", base, "init", "-q")
    Open3.capture2e("git", "-C", base, "add", "-A")

    Open3.capture2e({ "FIXTURE_EXECUTION_ROOT" => base }, "ruby", GATE, chdir: base)
  ensure
    # git leaves the odd lock/objects file behind on macOS and mktmpdir's own
    # cleanup raises ENOTEMPTY when it loses that race. A leaked tmp dir beats a
    # gate that flakes inside `make check`.
    begin
      FileUtils.remove_entry(base)
    rescue SystemCallError
      begin
        FileUtils.remove_entry(base)
      rescue SystemCallError
        nil
      end
    end
  end
end

def edit(files, key, from, to)
  updated = files.fetch(key).sub(from, to)
  raise "self-test bug: #{from.inspect} not found in #{key}" if updated == files.fetch(key)

  files[key] = updated
end

def with_skips(**overrides)
  BASE_SKIPS.merge(overrides.transform_keys(&:to_s))
end

# --- positive controls ---------------------------------------------------------

out, status = Open3.capture2e("ruby", GATE, chdir: ROOT)
expect_pass(failures, "real repo passes", out, status)

out, status = run_gate
expect_pass(failures, "crafted valid repo passes", out, status)

# The near-miss must pass, or the gate is just "no skips allowed" and the real
# repo — 2 Go skips, 11 Ruby, 1 TypeScript — could never satisfy it.
unless utf8(out).include?("no case is excluded by every runner")
  failures << "crafted valid repo: expected the ok line, got:\n#{utf8(out)}"
end

# --- the central case: skipped by all six ---------------------------------------
#
# The state #573 shipped and #596 undid, and the state this repo cannot reach
# today. Python is the one runner still executing "five of six" in the baseline,
# so adding it there — and only there — is the whole difference between a
# legitimate five-of-six and the defect.

out, status = run_gate(skips: with_skips(python: ["five of six"]))
expect_fail(failures, "case skipped by all six runners", out, status,
            '"five of six" is skipped by all 6 runners')

# ...and it must name every runner, so the reader can see which one to narrow.
ROSTER_HEADINGS.each_value do |runner|
  failures << "all-six failure must name #{runner}:\n#{utf8(out)}" unless utf8(out).include?(runner)
end

# The same defect reached through the tag branch instead of the name tables:
# widening the whole-case link-header skip to the four runners that today only
# suppress its requestCount assertion is precisely what #573 did.
out, status = run_gate(skips: with_skips(go: ["link-header only"], python: ["link-header only"],
                                         ruby: ["link-header only"],
                                         typescript: ["link-header only"]))
expect_fail(failures, "tag-excluded case skipped by the other four", out, status,
            '"link-header only" is skipped by all 6 runners')

# --- scope ----------------------------------------------------------------------

# A live case is excluded by all six mock runners by construction — named in the
# four, tagged for the other two — and is still not a violation, because the
# TypeScript live canary is what runs it. Reporting it would fire on all 31
# cases in live-my-surface.json the day this landed.
out, status = run_gate(
  skips: with_skips(go: ["live case"], python: ["live case"], ruby: ["live case"],
                    typescript: ["live case"]),
  mutate: lambda { |f|
    cases = fixture_cases
    cases[3]["tags"] = %w[link-header]
    f["conformance/tests/alpha.json"] = JSON.pretty_generate(cases)
  }
)
expect_pass(failures, "live cases are out of scope", out, status)

# An unrecognized mode is an all-six exclusion one step earlier, and a silent
# one: every runner spells the filter as "mock unless told otherwise".
out, status = run_gate(mutate: lambda { |f|
  cases = fixture_cases
  cases[0]["mode"] = "moc"
  f["conformance/tests/alpha.json"] = JSON.pretty_generate(cases)
})
expect_fail(failures, "unrecognized mode", out, status,
            'declares mode "moc", which no runner recognizes')

# --- extraction fail-closed -----------------------------------------------------
#
# The predicate is "the anchor matched exactly one line", NOT "the extracted set
# is non-empty" — three of the six real tables are legitimately empty, and the
# baseline above keeps all three of those spellings live.

out, status = run_gate(mutate: ->(f) { edit(f, GO_MAIN, "var goSDKSkips", "var goSkips") })
expect_fail(failures, "skip table renamed", out, status,
            'the declaration anchor "var goSDKSkips" matched 0 line(s)')

out, status = run_gate(mutate: ->(f) { f[GO_MAIN] += "\nvar goSDKSkips = map[string]string{}\n" })
expect_fail(failures, "skip table declared twice", out, status,
            "matched 2 line(s), expected exactly 1")

out, status = run_gate(mutate: ->(f) { edit(f, RB_RUNNER, "].freeze)", "") })
expect_fail(failures, "declaration never closes", out, status, "never closes")

# A declaration whose literal begins on the line AFTER the name — what a rubocop
# reformat produces, and the reason the scan ends at the end of the line where
# depth returns to zero rather than the instant it does. A parser that stopped
# at the first zero would read an EMPTY ruby table here, which is the dangerous
# direction: fewer exclusions, so the all-six defect turns back into a passing
# five-of-six and the gate reports success.
out, status = run_gate(skips: with_skips(python: ["five of six"]),
                       mutate: lambda { |f|
                         edit(f, RB_RUNNER, "RUBY_SKIPS = Set.new([", "RUBY_SKIPS =\n  Set.new([")
                       })
expect_fail(failures, "literal on the line after the name", out, status,
            '"five of six" is skipped by all 6 runners')

out, status = run_gate(mutate: ->(f) { f.delete(SWIFT_RUNNER) })
expect_fail(failures, "runner source missing", out, status, "missing conformance/runner/swift")

# An entry whose key spelling the parser does not recognize must be REPORTED,
# never read as a value. Kotlin's `Pair(...)` form is the concrete case, and it
# fails quiet: the dropped key is one fewer exclusion, so a case skipped by all
# six reads as a passing five-of-six. Anything not positively interpreted is
# reported, never credited.
out, status = run_gate(skips: with_skips(kotlin: ["five of six"]),
                       mutate: lambda { |f|
                         edit(f, KT_MAIN, '"five of six" to "kotlin reason"',
                              '"five of six" to "kotlin reason", Pair("also skipped", "why")')
                       })
expect_fail(failures, "unrecognized key spelling in a map", out, status,
            "mixes key spellings this parser does not recognize")

# A non-array `tags` answers include? by its own class's rules — a Hash by key,
# a String by substring — and the wrong answer is silent under-exclusion.
out, status = run_gate(mutate: lambda { |f|
  cases = fixture_cases
  cases[1]["tags"] = { "link-header" => true }
  f["conformance/tests/alpha.json"] = JSON.pretty_generate(cases)
})
expect_fail(failures, "non-array tags", out, status, 'has a non-array "tags"')

# The tag branch has no literal to parse, so the registry asserts it by naming
# the line that implements it. If that line goes, the registry's claim that
# kotlin skips every link-header case is no longer backed by anything.
out, status = run_gate(mutate: lambda { |f|
  edit(f, KT_MAIN, 'if ("link-header" in tc.tags) {', "if (false) {")
})
expect_fail(failures, "kotlin tag branch removed", out, status, "whole-case tag branch")

# Vacuity: no tracked fixtures is an extraction failure, not "nothing to check".
out, status = run_gate(mutate: ->(f) { f.delete("conformance/tests/alpha.json") })
expect_fail(failures, "no tracked fixtures", out, status,
            "matched nothing, so this gate has no cases to check")

out, status = run_gate(mutate: ->(f) { f["conformance/tests/alpha.json"] = "[]\n" })
expect_fail(failures, "fixtures hold no cases", out, status, "yielded no test cases")

# --- companion reason tables ----------------------------------------------------
#
# RUBY_SKIPS and RUBY_SKIP_REASONS enumerate one list twelve lines apart with
# nothing keeping them in sync. Both directions are real defects: a skip with no
# reason falls back to "Ruby SDK behavior differs" — an undocumented waiver,
# which is the one thing SPEC §19's roster exists to prevent — and a reason for a
# case that is not skipped is a claim about behavior that does not happen.

out, status = run_gate(mutate: lambda { |f|
  edit(f, RB_RUNNER, "  \"five of six\" => \"ruby reason\",\n", "")
})
expect_fail(failures, "skip with no recorded reason", out, status, "skipped with no recorded reason")

out, status = run_gate(mutate: lambda { |f|
  edit(f, RB_RUNNER, "}.freeze", "  \"nobody skips this\" => \"stale\",\n}.freeze")
})
expect_fail(failures, "reason for a case that is not skipped", out, status,
            "reason for a case that is not skipped")

# --- SPEC §19's Zero-Skip roster ------------------------------------------------
#
# Set equality on the ENUMERATION only. The roster is never an input to the
# exclusion sets: if it were, a wrong extraction and a stale roster could agree
# and both stay green, which is the failure the gate exists to prevent.

out, status = run_gate(mutate: lambda { |f|
  edit(f, "SPEC.md", "- \"five of six\" — some justification only a human can write.\n", "")
})
expect_fail(failures, "skip missing from the roster", out, status, "skipped but not rostered")

out, status = run_gate(mutate: lambda { |f|
  edit(f, "SPEC.md", "**Python** (`table`) — none.",
       "**Python** (`table`):\n- \"a skip python does not have\" — stale.")
})
expect_fail(failures, "roster lists a skip that was deleted", out, status,
            "rostered but not skipped")

out, status = run_gate(mutate: ->(f) { edit(f, "SPEC.md", "**Swift** (`table`) — none.\n\n", "") })
expect_fail(failures, "roster drops a runner block", out, status,
            "the Zero-Skip roster has no **Swift** block")

out, status = run_gate(mutate: ->(f) { edit(f, "SPEC.md", "### Zero-Skip Target", "### Skips") })
expect_fail(failures, "roster section renamed", out, status,
            'the roster heading "### Zero-Skip Target" matched 0 line(s)')

# A repeated heading would be silently collapsed by a hash build, dropping the
# first block's bullets out of the comparison entirely.
out, status = run_gate(mutate: lambda { |f|
  edit(f, "SPEC.md", "**Swift** (`table`) — none.",
       "**Ruby** (`table`) — none.\n\n**Swift** (`table`) — none.")
})
expect_fail(failures, "roster repeats a runner block", out, status,
            "more than one **Ruby** block")

# The justification is judgement, and nothing asserts it. A roster whose
# reasoning is rewritten wholesale must still pass, or the gate would be
# demanding text it has no source for — the reason the first pass ruled this
# roster out entirely.
out, status = run_gate(mutate: lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].gsub("— some justification only a human can write.",
                                   "— rewritten wholesale, waiver 9Z.9, architectural.")
})
expect_pass(failures, "roster justification text is not asserted", out, status)

# --- locale independence --------------------------------------------------------
#
# Case names carry em dashes and the gate's own messages carry UTF-8
# punctuation, so a gate that read in the locale's encoding would die on the
# first non-ASCII byte under CI's LC_ALL=C.

out, status = Open3.capture2e({ "LC_ALL" => "C", "LANG" => "C" }, "ruby", GATE, chdir: ROOT)
expect_pass(failures, "real repo passes under LC_ALL=C", out, status)

# --- report ---------------------------------------------------------------------

if failures.empty?
  puts "check-fixture-execution self-test: all cases passed."
  exit 0
else
  warn "check-fixture-execution self-test: #{failures.length} case(s) failed."
  failures.each { |f| warn "\n#{f}" }
  exit 1
end
