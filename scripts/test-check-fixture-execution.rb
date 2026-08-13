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

# Carries the `swiftSkips` WIRE, which the real runner has and the first draft
# of this stub did not. That omission was structural, not cosmetic: the run loop
# consults `swiftSkips` while the registry parses `temporarySkips`, so a stub
# without the wire made the suite unable to notice that the parser reads one
# variable and the runner obeys another. A generator is a source of truth for
# everything the suite can conclude, and a shape it never emits is a shape
# nothing can test — which is why the registry now pins the else-branch.
def swift_source(names)
  literal = if names.empty?
              "[:]"
            else
              "[#{names.map { |n| "#{n.inspect}: \"swift reason\"" }.join(', ')}]"
            end
  "private let temporarySkips: [String: String] = #{literal}\n\n" \
    "private let swiftSkips: [String: String] =\n" \
    "    ProcessInfo.processInfo.environment[\"SWIFT_CONFORMANCE_NO_SKIPS\"] == \"1\" " \
    "? [:] : temporarySkips\n\n" \
    "func run() {\n" \
    "    for tc in testCases {\n" \
    "        if tc.allTags.contains(\"link-header\") {\n" \
    "            skipped += 1\n" \
    "            continue\n" \
    "        }\n" \
    "        if let reason = swiftSkips[tc.name] {\n" \
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
        names.map { |n| "- \"#{n}\" — some justification only a human can write.\n" }.join +
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

def run_gate(skips: BASE_SKIPS, mutate: nil, git_init: true)
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
    # would look empty and every case would vacuously "pass". `git_init: false`
    # is the one case that wants exactly that, to reach the ls-files failure.
    if git_init
      Open3.capture2e("git", "-C", base, "init", "-q")
      Open3.capture2e("git", "-C", base, "add", "-A")
    end

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

# Two cases share a name; only one carries the tag. The TAGGED one is genuinely
# skipped by all six — four by name, Kotlin and Swift by tag — and must be
# reported. Collapsing tag exclusions to names got this wrong twice in
# opposite directions: crediting the untagged twin (false positive), then
# requiring every twin to agree (false negative, and quiet). Only per-case
# identity can answer differently for two cases that differ in the tag.
out, status = run_gate(skips: with_skips(go: ["shared name"], python: ["shared name"],
                                         ruby: ["shared name"], typescript: ["shared name"]),
                       mutate: lambda { |f|
                         cases = fixture_cases +
                                 [{ "name" => "shared name", "operation" => "Op",
                                    "tags" => %w[link-header] },
                                  { "name" => "shared name", "operation" => "Op" }]
                         f["conformance/tests/alpha.json"] = JSON.pretty_generate(cases)
                       })
expect_fail(failures, "tagged twin skipped by all six is reported", out, status,
            '"shared name" is skipped by all 6 runners')

# ...and its UNTAGGED twin must not be, since Kotlin and Swift run it. Same
# fixture, same name, opposite answer — which is the property that makes this
# per-case rather than per-name.
unless utf8(out).scan(/is skipped by all 6 runners/).length == 1
  failures << "only the tagged twin should be reported:\n#{utf8(out)}"
end

# A LIVE case excluded by all six must not be reported, and reaching that state
# now takes a name shared with a mock case — otherwise the live-only-waiver
# check fires first and the scope guard is never consulted. Both identities are
# excluded by all six here; only the mock one is a finding.
#
# This case exists because the orphan matrix said nothing detected the
# live-mode scope guard any more: the case that used to cover it became the
# live-only-waiver case two commits ago, and its old property went uncovered
# without anything going red. Deleting the guard now breaks this.
out, status = run_gate(skips: with_skips(go: ["shared mode"], python: ["shared mode"],
                                         ruby: ["shared mode"], typescript: ["shared mode"],
                                         kotlin: ["shared mode"], swift: ["shared mode"]),
                       mutate: lambda { |f|
                         cases = fixture_cases +
                                 [{ "name" => "shared mode", "operation" => "Op" },
                                  { "name" => "shared mode", "operation" => "Op", "mode" => "live" }]
                         f["conformance/tests/alpha.json"] = JSON.pretty_generate(cases)
                       })
expect_fail(failures, "live twin of an all-six mock case is not reported", out, status,
            '"shared mode" is skipped by all 6 runners')
unless utf8(out).scan(/is skipped by all 6 runners/).length == 1
  failures << "the live twin must not be reported as well:\n#{utf8(out)}"
end

# A renamed table left COMMENTED OUT above the live one used to match the
# anchor, and the scan read an empty table off the dead line. The active table
# here holds the sixth exclusion, so a parser fooled by the comment reports a
# passing five-of-six — the quiet direction — while the fixed one still sees it.
out, status = run_gate(skips: with_skips(python: ["five of six"]),
                       mutate: lambda { |f|
                         edit(f, PY_RUNNER, "class ConformanceRunner:",
                              "class ConformanceRunner:\n    # SKIPS: set[str] = set()  # renamed")
                       })
expect_fail(failures, "commented-out declaration is not the active one", out, status,
            '"five of six" is skipped by all 6 runners')

# Array subtraction removes every copy, so a name rostered twice leaves both
# sides of the comparison empty and passes.
out, status = run_gate(mutate: lambda { |f|
  edit(f, "SPEC.md", "- \"five of six\" — some justification only a human can write.\n",
       "- \"five of six\" — some justification only a human can write.\n" \
       "- \"five of six\" — a second, contradictory waiver.\n")
})
expect_fail(failures, "case rostered more than once", out, status, "rostered more than once")

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
expect_fail(failures, "skip naming a live-only case is a dead waiver", out, status,
            "exist only in live mode")

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

# A TYPE ANNOTATION's brackets must never reach the depth counter. Both PR bots
# found this independently: `SKIPS: set[str] =` with its literal on the next
# line opens and closes `[str]` on the declaration line, so a scan that started
# at the anchor saw depth return to zero at end of line and read the table as
# EMPTY while it held every skip it ever had. Silent, and in the direction that
# turns a real all-six exclusion into a passing five-of-six — which is why both
# of these craft the all-six state and demand it still be seen.
out, status = run_gate(skips: with_skips(python: ["five of six"]),
                       mutate: lambda { |f|
                         edit(f, PY_RUNNER, 'SKIPS: set[str] = {"five of six"}',
                              "SKIPS: set[str] = (\n        {\"five of six\"}\n    )")
                       })
expect_fail(failures, "python type brackets before a wrapped literal", out, status,
            '"five of six" is skipped by all 6 runners')

# "runs everywhere" is UNTAGGED, so all six exclusions have to come from the
# name tables and Swift's parse is load-bearing: read its table as empty and the
# count drops to five, which passes. Using a link-header-tagged case here would
# have let the tag branch supply Swift's exclusion and the test would hold
# whether the parse worked or not.
all_six = %w[go python ruby typescript kotlin swift]
          .to_h { |runner| [runner.to_sym, ["runs everywhere"]] }

out, status = run_gate(skips: with_skips(**all_six),
                       mutate: lambda { |f|
                         edit(f, SWIFT_RUNNER,
                              'private let temporarySkips: [String: String] = ["runs everywhere": "swift reason"]',
                              "private let temporarySkips: [String: String] =\n    [\"runs everywhere\": \"swift reason\"]")
                       })
expect_fail(failures, "swift type brackets before a wrapped literal", out, status,
            '"runs everywhere" is skipped by all 6 runners')

# An UNREGISTERED whole-case tag branch skips cases this module never accounts
# for. The anchor proves the branch the registry knows about still exists and
# says nothing about a second one added beside it.
out, status = run_gate(mutate: lambda { |f|
  edit(f, KT_MAIN, '        if ("link-header" in tc.tags) {',
       "        if (\"slow\" in tc.tags) {\n            continue\n        }\n" \
       "        if (\"link-header\" in tc.tags) {")
})
expect_fail(failures, "unregistered tag branch", out, status,
            "ConformanceSkips::RUNNERS registers 1 whole-case tag branch(es) there")

# A skip naming no fixture case is a waiver for nothing — it arrives by rename
# or deletion, and keeps a line alive in SPEC §19's roster that reads as an
# accepted divergence nobody can find.
out, status = run_gate(skips: with_skips(go: ["a case that was renamed away"]))
expect_fail(failures, "skip names no fixture case", out, status,
            "names 1 case(s) no fixture defines")

# --- string and comment syntax the scanner must not mis-read --------------------
#
# Every one of these fails QUIET if unhandled — a key that never becomes a token
# is a lost exclusion, and a brace inside an unread literal moves the depth
# counter and truncates the table. All four craft the all-six state so a silent
# under-count drops the count to five and passes.

# Nothing in this repo enforces double quotes — there is no prettier or eslint
# config, only .editorconfig — so a single-quoted TypeScript key is ordinary
# authoring, not an exotic case.
out, status = run_gate(skips: with_skips(**all_six),
                       mutate: lambda { |f|
                         f[TS_RUNNER] = f[TS_RUNNER].gsub('"runs everywhere"', "'runs everywhere'")
                       })
expect_fail(failures, "single-quoted TypeScript key", out, status,
            '"runs everywhere" is skipped by all 6 runners')

# A Go raw string holding a brace moves the depth counter and ends the scan
# early, losing every entry AFTER it. So the raw string has to sit on the first
# of two entries: with it on the last, truncation costs nothing and the case
# holds whether backticks are handled or not — which is how the first draft of
# this case passed against a parser that did not read them.
out, status = run_gate(skips: with_skips(**all_six.merge(go: ["five of six", "runs everywhere"])),
                       mutate: lambda { |f|
                         edit(f, GO_MAIN, '"five of six": "go reason",',
                              '"five of six": `a reason with a } brace`,')
                       })
expect_fail(failures, "go raw string containing a brace", out, status,
            '"runs everywhere" is skipped by all 6 runners')

# Four of six runners are `/* … */` languages. A block comment with an
# unbalanced brace is the case that empties an entire table.
out, status = run_gate(skips: with_skips(**all_six),
                       mutate: lambda { |f|
                         edit(f, GO_MAIN, "var goSDKSkips = map[string]string{\n",
                              "var goSDKSkips = map[string]string{\n\t/* dropped in #123 because the } branch went away */\n")
                       })
expect_fail(failures, "go block comment containing a brace", out, status,
            '"runs everywhere" is skipped by all 6 runners')

# An unrecognized-spelling entry placed FIRST has no predecessor at all, so a
# pairwise adjacency rule structurally cannot see it. This is the ordering the
# earlier `Pair(...)` case did not cover.
out, status = run_gate(skips: with_skips(kotlin: ["five of six"]),
                       mutate: lambda { |f|
                         edit(f, KT_MAIN, 'mapOf("five of six" to "kotlin reason")',
                              'mapOf(Pair("also skipped", REASON), "five of six" to "kotlin reason")')
                       })
expect_fail(failures, "unrecognized spelling in the first entry", out, status,
            "entry has no key this parser can place")

# The ordering that defeated the NEIGHBOUR heuristic, which inferred key-ness
# from the previous string and so assumed values are strings. With a CONSTANT
# value the value slot holds no string, so `mapOf("A" to REASON, Pair("B",
# REASON))` read as A(keyed), B(unkeyed) and B — a real skip — was accepted as
# A's value and dropped. Reading entry structure rather than adjacency is what
# closes it, and this is the case that proves the difference.
out, status = run_gate(skips: with_skips(kotlin: ["five of six"]),
                       mutate: lambda { |f|
                         edit(f, KT_MAIN, 'mapOf("five of six" to "kotlin reason")',
                              'mapOf("five of six" to REASON, Pair("also skipped", REASON))')
                       })
expect_fail(failures, "unrecognized entry after one with a constant value", out, status,
            "entry has no key this parser can place")

# An entry holding NO string literal — a spread — must be rejected rather than
# skipped as if it were a trailing comma. Entries are therefore recognized by
# CONTENT: a trailing comma leaves an entry with nothing in it, a spread leaves
# one with tokens the parser cannot read.
out, status = run_gate(mutate: lambda { |f|
  edit(f, TS_RUNNER, "};", "  ...COMMON_SKIPS,\n};")
})
expect_fail(failures, "skip table composed from a spread", out, status,
            "it holds no string literal at all")

# A raw or multiline literal is reported rather than guessed at: mis-tokenizing
# one shifts every string after it.
out, status = run_gate(mutate: lambda { |f|
  edit(f, KT_MAIN, "emptyMap()", 'mapOf("""triple quoted""" to "reason")')
})
expect_fail(failures, "raw/multiline string literal", out, status,
            "raw or multiline string literal")

# Zero MOCK cases is the third way to have nothing to check, beside no files and
# no cases: the six mock runners execute nothing, reported as success.
out, status = run_gate(mutate: lambda { |f|
  cases = fixture_cases.map { |c| c.merge("mode" => "live") }
  f["conformance/tests/alpha.json"] = JSON.pretty_generate(cases)
})
expect_fail(failures, "no mock cases at all", out, status, "not one runs in mock mode")

# Git's pathspec `*` matches across `/`, but all six runners glob fixtures
# NON-RECURSIVELY. A nested fixture is executed by nothing, so counting its
# cases here would have this gate commit the exact defect it exists to catch:
# they appear in no skip table, so they read as executed everywhere.
out, status = run_gate(mutate: lambda { |f|
  f["conformance/tests/nested/deep.json"] =
    JSON.pretty_generate([{ "name" => "nested case nothing runs", "operation" => "Op" }])
})
expect_fail(failures, "nested fixture is rejected, not silently excluded", out, status,
            "executed by NO runner")

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
            "entry has no key this parser can place")

# A non-array `tags` answers include? by its own class's rules — a Hash by key,
# a String by substring — and the wrong answer is silent under-exclusion.
out, status = run_gate(mutate: lambda { |f|
  cases = fixture_cases
  cases[1]["tags"] = { "link-header" => true }
  f["conformance/tests/alpha.json"] = JSON.pretty_generate(cases)
})
expect_fail(failures, "non-array tags", out, status, 'has a non-array "tags"')

# The tag branch has no literal to parse, so the registry asserts it by naming
# the line that implements it. If the registry can no longer find that line, its
# claim that kotlin skips every link-header case is backed by nothing.
#
# The branch is REFORMATTED rather than deleted, and that is the whole point of
# the case. Deleting it also removes the only mention of `tc.tags`, so the
# accessor-count guard fires too — two independent mechanisms reaching the same
# verdict, which means the case proves nothing about EITHER. It showed up in the
# per-case mutation matrix as an orphan: no single mutation could kill it,
# because disabling one guard left the other. Keeping `tc.tags` on the line
# leaves exactly one mechanism able to answer.
out, status = run_gate(mutate: lambda { |f|
  edit(f, KT_MAIN, '        if ("link-header" in tc.tags) {',
       "        if (\"link-header\" in tc.tags)\n        {")
})
expect_fail(failures, "kotlin tag branch reformatted past its anchor", out, status,
            "whole-case tag branch")

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

# Ruby carries TWO skips here so that deleting one bullet leaves the block
# non-empty. Deleting the only bullet would empty the block instead, which the
# parsed-not-merely-present guard below rejects for a different reason — and
# this case is about the set comparison, not about that.
out, status = run_gate(skips: with_skips(ruby: ["five of six", "runs everywhere"]),
                       mutate: lambda { |f|
                         edit(f, "SPEC.md",
                              "- \"runs everywhere\" — some justification only a human can write.\n", "")
                       })
expect_fail(failures, "skip missing from the roster", out, status, "skipped but not rostered")

# A roster block that lists nothing must SAY "none". Otherwise absence-of-parse
# reads as absence-of-claim: three of these six comparisons are `[] == []`
# today, and any parser failure makes a fourth vacuous at exactly the moment it
# would have mattered — the skip vanishes from the exclusion sets AND the roster
# omission goes unreported, both from one cause.
out, status = run_gate(mutate: lambda { |f|
  edit(f, "SPEC.md", "- \"five of six\" — some justification only a human can write.\n", "")
})
expect_fail(failures, "roster block emptied without saying none", out, status,
            'lists no skips and does not say "none"')

# ...and the three genuinely-empty blocks must still pass, or the guard would
# reject the state the roster is aiming at.
out, status = run_gate
expect_pass(failures, "empty blocks saying none are accepted", out, status)

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

# The roster heading must START its line. Prose that MENTIONS the heading is
# ordinary — SPEC.md's own §19 narrative refers to the roster by name — and a
# substring match would see two "headings" and fail the exactly-one predicate on
# a document that is perfectly correct. This is the one place the shared anchor
# matcher takes `anchored:`, and without a case the flag was unproven: dropping
# the distinction entirely broke nothing.
out, status = run_gate(mutate: lambda { |f|
  edit(f, "SPEC.md", "One line per runner x test",
       "The ### Zero-Skip Target roster below is one line per runner x test")
})
expect_pass(failures, "prose mentioning the roster heading is not a heading", out, status)

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

# --- tag accessors, the swiftSkips wire, and roster bullets ----------------------
#
# All four are quiet, and three were reachable by an ordinary refactor in the
# idiom the surrounding code already uses. The roster gives no backstop for the
# first two: Kotlin's and Swift's tag exclusions are prose-only, so those bullet
# lists are empty and the comparison is `[] == []` either way.

# `it.tags` is not an exotic spelling — `Main.kt` writes `.filter { it.mode ==
# "mock" }` seven lines above the tag branch, so it is the idiom the file
# teaches. Against a single-literal accessor this branch is invisible.
out, status = run_gate(mutate: lambda { |f|
  edit(f, KT_MAIN, '        if ("link-header" in tc.tags) {',
       "        if (cases.filter { \"slow\" !in it.tags }.isEmpty()) { return }\n" \
       "        if (\"link-header\" in tc.tags) {")
})
expect_fail(failures, "second kotlin tag branch spelled it.tags", out, status,
            "ConformanceSkips::RUNNERS registers 1 whole-case tag branch(es) there")

# Swift's `allTags` is a computed convenience over a stored `tags` sitting right
# beside it, so the raw spelling is equally natural.
out, status = run_gate(mutate: lambda { |f|
  edit(f, SWIFT_RUNNER, '        if tc.allTags.contains("link-header") {',
       "        if tc.tags?.contains(\"slow\") == true { continue }\n" \
       "        if tc.allTags.contains(\"link-header\") {")
})
expect_fail(failures, "second swift tag branch spelled tags", out, status,
            "ConformanceSkips::RUNNERS registers 1 whole-case tag branch(es) there")

# The run loop consults `swiftSkips`, not the `temporarySkips` this module
# parses. Wrapping the else-branch honours a skip in Swift and hides it here.
out, status = run_gate(mutate: lambda { |f|
  edit(f, SWIFT_RUNNER, "? [:] : temporarySkips",
       '? [:] : temporarySkips.merging(["solo case": "not yet supported"]) { a, _ in a }')
})
expect_fail(failures, "swiftSkips wire rewired around temporarySkips", out, status,
            "the run loop consults `swiftSkips`")

# A case name with inner quotes is not hypothetical: cards_write.json already
# carries one. A lazy bullet capture truncates it, so the roster could never be
# made to match the runner.
out, status = run_gate(skips: with_skips(go: ['a name with "inner" quotes']),
                       mutate: lambda { |f|
                         cases = fixture_cases + [{ "name" => 'a name with "inner" quotes',
                                                    "operation" => "Op" }]
                         f["conformance/tests/alpha.json"] = JSON.pretty_generate(cases)
                       })
expect_pass(failures, "roster bullet with inner quotes round-trips", out, status)

# A bullet the parser cannot read is not an empty block, and "none" cannot tell
# them apart — worst exactly where Kotlin's and Swift's blocks already say
# "none BEYOND the tag branch", pre-satisfying the token forever.
out, status = run_gate(skips: with_skips(kotlin: ["five of six"]),
                       mutate: lambda { |f|
                         f["SPEC.md"] = f["SPEC.md"].sub(
                           "- \"five of six\" — some justification only a human can write.",
                           "- “five of six” — curly quotes the parser cannot read."
                         )
                         f["SPEC.md"] = f["SPEC.md"].sub("**Kotlin** (`table`):",
                                                         "**Kotlin** (`table`) — none beyond the tag branch:")
                       })
expect_fail(failures, "unreadable bullet masked by a none token", out, status,
            "bullet line(s) this parser could not read")

# --- guard sites the per-case matrix showed nothing was reaching -----------------
#
# The matrix measures cases against MUTATIONS, so a guard nobody mutated
# produces no row and is invisible by construction — which is how one orphan
# here turned out to be a short mutation list rather than a weak case. Walking
# every raise and early return in the parser instead, against the code, found
# eleven guard sites with no case behind them. These are the seven worth having.
#
# Deliberately still untested, and why: an unterminated block comment, an
# unterminated string literal, and an unbalanced closing bracket are all
# malformed source that would fail its own language's compiler long before this
# gate ran, so a case would only assert that Ruby's own parsing works.

# A line comment holding a brace, the `#`-language twin of the block-comment
# case. Same failure: the brace moves the depth counter and truncates the table
# after that entry, so the comment goes on the FIRST of two.
out, status = run_gate(skips: with_skips(**all_six.merge(ruby: ["five of six", "runs everywhere"])),
                       mutate: lambda { |f|
                         edit(f, RB_RUNNER, "  \"five of six\",\n",
                              "  \"five of six\", # dropped when the } branch went away\n")
                       })
expect_fail(failures, "ruby line comment containing a brace", out, status,
            '"runs everywhere" is skipped by all 6 runners')

# (An escaped quote in a runner key is covered by "roster bullet with inner
# quotes round-trips" below, which exercises both halves at once: the runner
# side must unescape `\"` and the roster side must capture past the inner
# quotes. This case previously asserted a MISMATCH between them, which turned
# out to be a bug in the crafted roster generator rather than a property —
# it escaped quotes the way Ruby's `inspect` does, where a human writing prose
# would not.)

# The separator may be separated from its key by whitespace, a newline or a
# comment. If that scan stopped recognizing it, NO string would be in key
# position, the table would fall to set mode, and every reason string would be
# read as a skip — so this passes only because the separator is still found.
out, status = run_gate(mutate: lambda { |f|
  edit(f, GO_MAIN, '"five of six": "go reason",', "\"five of six\" /* why */ :\n\t\t\"go reason\",")
})
expect_pass(failures, "separator reached across a comment and a newline", out, status)

# A declaration whose `=` is not on the anchor's line: the parser cannot tell
# where the header ends and the literal begins, so it says so.
#
# Reached through GO, not Ruby. Two of the eight anchors (`RUBY_SKIPS =`) end in
# the assignment themselves, so moving the `=` also moves the anchor and the
# anchor-not-found error fires first — the guard is unreachable from those two
# by construction, which is worth knowing rather than working around.
out, status = run_gate(mutate: lambda { |f|
  edit(f, GO_MAIN, "var goSDKSkips = map[string]string{", "var goSDKSkips\nvar _ = map[string]string{")
})
expect_fail(failures, "no assignment on the declaration line", out, status,
            "has no `=` on its line")

# Fixture-shape guards. conformance-fixtures-check validates the schema, but a
# gate that credits malformed input because another gate usually catches it is
# crediting what it did not read — the same argument as the non-array `tags`.
out, status = run_gate(mutate: ->(f) { f["conformance/tests/alpha.json"] = "{}\n" })
expect_fail(failures, "fixture is not a JSON array", out, status,
            "expected a JSON array of test cases")

out, status = run_gate(mutate: lambda { |f|
  f["conformance/tests/alpha.json"] = JSON.pretty_generate([{ "operation" => "Op" }])
})
expect_fail(failures, "case has no name", out, status, 'test case has no string "name"')

out, status = run_gate(mutate: ->(f) { f["conformance/tests/alpha.json"] = "[ not json\n" })
expect_fail(failures, "fixture is not valid JSON", out, status, "is not valid JSON")

# Discovery itself failing must be loud: an un-inited tree returns no files, and
# reading that as "nothing to check" is the vacuity this gate exists to reject.
out, status = run_gate(git_init: false)
expect_fail(failures, "root is not a git checkout", out, status, "git ls-files")

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
