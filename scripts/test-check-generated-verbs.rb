#!/usr/bin/env ruby
# frozen_string_literal: true

# Self-test for scripts/check-generated-verbs.rb, the ONE validator of
# spec/generated-verbs.json.
#
# WHY IT MATTERS MORE THAN MOST SELF-TESTS. The six generators read that file
# without validating it, so this gate is the only thing between a hand-edit and
# six loaders parsing whatever it says. Its live run only ever sees the committed
# declaration, which passes — so nothing there proves it rejects anything.
#
# The negative cases are not invented. Each is a shape that ACTUALLY told the six
# loaders apart while they each validated, over four review rounds:
#
#   ["get", 1]           Kotlin's JsonPrimitive.content rendered it "1"; Rust's
#                        filter_map dropped it and accepted the rest.
#   ["get", " "]         Kotlin and Rust rejected it, the other four accepted it.
#   ["get", " "]    Ruby's ASCII String#strip kept it; Python's str.strip
#                        removed it. Java's Character.isWhitespace excludes it,
#                        so Kotlin took it too.
#   ["get", " "]    Same split, a different character.
#   ["get", ""]          Swift's String.allSatisfy is vacuously true for "".
#   ["get", "get\n"]     JavaScript's $ matches before a final line terminator.
#   ["get", "GET"]       Case: an OpenAPI path item spells its methods lowercase.
#   ["get", "get"]       A duplicate emits the same operation twice.
#
# Every one of those now dies here instead, once, in one language — which is the
# whole argument for the restructure, so it is a test rather than a paragraph.
#
# Stdlib only. Wired into `make check`.

require "json"
require "tmpdir"

ROOT = File.expand_path("..", __dir__)
CHECKER = File.join(ROOT, "scripts", "check-generated-verbs.rb")

FAILURES = []
PASSES = []

def run(declaration)
  Dir.mktmpdir("generated-verbs") do |dir|
    path = File.join(dir, "generated-verbs.json")
    File.write(path, declaration.is_a?(String) ? declaration : JSON.pretty_generate(declaration))
    output = IO.popen({ "GENERATED_VERBS_FILE" => path }, ["ruby", CHECKER], err: %i[child out], &:read)
    [$?.exitstatus, output]
  end
end

def check(name, declaration, expect_pass:, expect_fragment: nil)
  status, output = run(declaration)
  passed = status.zero?
  ok = passed == expect_pass
  ok &&= output.include?(expect_fragment) if expect_fragment && !expect_pass
  if ok
    PASSES << name
    puts "  PASS  #{name}"
  else
    FAILURES << name
    puts "  FAIL  #{name} — exit #{status}, expected #{expect_pass ? 'pass' : 'fail'}#{expect_fragment ? " naming #{expect_fragment.inspect}" : ''}"
    output.lines.each { |l| puts "          #{l}" }
  end
end

VALID = { "verbs" => %w[get post put delete] }.freeze

puts "==> generated-verb declaration gate self-test"
puts
puts "Positive control"
check("the committed shape passes", VALID, expect_pass: true)
check("prose keys alongside `verbs` pass", VALID.merge(
  "$schema" => "https://basecamp.com/schemas/generated-verbs.json",
  "version" => "1.0.0",
  "what" => "words",
  "why" => ["words"],
  "adding_a_verb" => "words",
  "not_patch" => "words"
), expect_pass: true)
check("a single verb passes", { "verbs" => ["get"] }, expect_pass: true)
check("a non-alphabetical order passes — the array order IS emission order",
      { "verbs" => %w[delete get post put] }, expect_pass: true)

puts
puts "The shapes that told the six loaders apart"
[
  ["a non-string entry (Kotlin rendered it as text, Rust dropped it)", 1, "Integer"],
  ["an ASCII-blank entry", " ", "\" \""],
  ["a U+00A0 entry (Ruby's strip keeps it, Python's removes it)", " ", "verbs[1]"],
  ["a U+2003 entry (same split, different character)", " ", "verbs[1]"],
  ["an empty entry (Swift's allSatisfy is vacuously true)", "", "verbs[1]"],
  ["a trailing newline (JavaScript's $ matches before it)", "get\n", "verbs[1]"],
  ["an uppercase entry", "GET", "verbs[1]"],
].each do |name, entry, fragment|
  check(name, { "verbs" => ["get", entry] }, expect_pass: false, expect_fragment: fragment)
end

check("a duplicate entry", { "verbs" => %w[get post get] },
      expect_pass: false, expect_fragment: "duplicate")

puts
puts "Shapes that would leave the loaders with nothing to read"
check("no `verbs` key at all", { "what" => "words" }, expect_pass: false, expect_fragment: "NilClass")
check("`verbs` is not an array", { "verbs" => "get" }, expect_pass: false, expect_fragment: "not an array")
check("`verbs` is empty", { "verbs" => [] }, expect_pass: false, expect_fragment: "empty")
check("the top level is not an object", ["get"], expect_pass: false, expect_fragment: "not a JSON object")
check("the file is not JSON", "{ nope", expect_pass: false, expect_fragment: "not valid JSON")

puts
puts "A typo that would otherwise sit there being ignored"
check("a misspelled `verb` key is named, not skipped", { "verb" => %w[get] },
      expect_pass: false, expect_fragment: "unrecognised top-level key")

puts
if FAILURES.empty?
  puts "==> generated-verb declaration gate self-test: all #{PASSES.length} cases passed"
  exit 0
end
warn "==> generated-verb declaration gate self-test: #{FAILURES.length} of #{PASSES.length + FAILURES.length} cases failed"
FAILURES.each { |f| warn "  - #{f}" }
exit 1
