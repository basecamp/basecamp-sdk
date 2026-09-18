#!/usr/bin/env ruby
# frozen_string_literal: true

# The ONE validator of spec/generated-verbs.json.
#
# WHY THERE IS EXACTLY ONE. The declaration names the HTTP methods every SDK
# generator emits, and six generators read it — Ruby (twice), Python, TypeScript,
# Kotlin, Swift and Rust. When each of them also VALIDATED it, they disagreed
# about what it meant, five times in four review rounds, each in a different
# language for a different reason:
#
#   - Kotlin's JsonPrimitive.content renders a number as text, so ["get", 1] read
#     as get/1 where the others rejected it.
#   - Rust's filter_map dropped the non-string and accepted the rest.
#   - Ruby's String#strip is ASCII-only, so U+00A0 and U+2003 survived it while
#     Python's str.strip removed them.
#   - Swift's String.allSatisfy is vacuously true for "", so the empty string
#     passed a character-class check.
#   - JavaScript's $ can match before a final line terminator, so "get\n" passed
#     the same check.
#
# Two repairs were tried and neither held. Requiring a non-BLANK string moved the
# disagreement from "is this a string" to "what counts as blank". A positive
# [a-z]+ format rule was meant to make whitespace, case, BOMs and normalisation
# moot by defining the allowed set; it moved the disagreement again, to "what does
# this language's predicate actually match". The population was never bounded by
# the rule being expressed. It is bounded by the union of six standard libraries'
# string and regex semantics, which is why every round found another.
#
# So the loaders stopped validating. This file is the only thing that rejects a
# malformed declaration, and it is strict enough that VALID input has no
# interpretive room left: a non-empty array of distinct lowercase ASCII tokens,
# with no unexpected top-level keys. Every JSON parser in the six
# languages agrees about a file like that — the divergences all occurred on
# INVALID input, which after this gate cannot reach a loader.
#
# WHY IT IS A PREREQUISITE RATHER THAN A CI JOB. There is no single generation
# entry point: ts-generate, rb-generate, py-generate, swift-generate,
# kt-generate-services and rs-generate-services are each directly invokable, and
# `generate` only aggregates them. A CI-only validator is bypassed by anyone
# running `make rb-generate` locally, which is exactly where a hand-edit gets
# made. It is an order-only prerequisite of every one of those targets AND a
# member of `make check`, so a hand-edit that never regenerates is caught too.
#
# WHAT IT DOES NOT COVER. scripts/gen-catalog/main.go also reads the declaration
# and keeps a deliberately looser superset bound; that generator is owned
# elsewhere and is out of scope here. This gate only constrains the file, so a
# looser reader of a file this strict cannot be made wrong by it.
#
# Stdlib only. Exercised by scripts/test-check-generated-verbs.rb.

require "json"

PROJECT_ROOT = File.expand_path("..", __dir__)
VERBS_FILE = ENV.fetch("GENERATED_VERBS_FILE", File.join(PROJECT_ROOT, "spec", "generated-verbs.json"))

# An HTTP method is a token. Lowercase because an OpenAPI Path Item Object spells
# its operation fields in lowercase, and the loaders compare against path-item
# keys verbatim.
VERB = /\A[a-z]+\z/

# Everything else in the file is prose for the reader. Listed so a typo in a key
# name — `verb` for `verbs` — cannot sit there being ignored while the real key
# is absent, and so a key nobody recognises is questioned rather than carried.
KNOWN_KEYS = %w[$schema version what why adding_a_verb not_patch verbs].freeze

def die(problems)
  warn "ERROR: #{VERBS_FILE} is not a declaration the generators can read verbatim."
  problems.each { |p| warn "  - #{p}" }
  warn ""
  warn "  This file is the single source for the HTTP methods the SDK generators emit,"
  warn "  and the six loaders read it WITHOUT validating — that is what stops them"
  warn "  disagreeing about malformed content. So it has to be exactly right here."
  exit 1
end

def main
  unless File.file?(VERBS_FILE)
    die(["the file does not exist (every *-generate target depends on this check)"])
  end

  declaration =
    begin
      JSON.parse(File.read(VERBS_FILE, encoding: "UTF-8"))
    rescue JSON::ParserError => e
      die(["it is not valid JSON: #{e.message}"])
    end

  problems = []

  unless declaration.is_a?(Hash)
    die(["the top level is #{declaration.class}, not a JSON object"])
  end

  unknown = declaration.keys - KNOWN_KEYS
  unless unknown.empty?
    problems << "unrecognised top-level key(s): #{unknown.map(&:inspect).join(', ')} " \
                "(a typo here leaves the real key absent; add it to KNOWN_KEYS if it is intended)"
  end

  verbs = declaration["verbs"]
  if !verbs.is_a?(Array)
    problems << "`verbs` is #{verbs.class}, not an array"
  elsif verbs.empty?
    problems << "`verbs` is empty; the generators would emit nothing"
  else
    verbs.each_with_index do |verb, i|
      if !verb.is_a?(String)
        problems << "verbs[#{i}] is #{verb.class} (#{verb.inspect}); every entry must be a string"
      elsif !verb.match?(VERB)
        problems << "verbs[#{i}] is #{verb.inspect}; every entry must match #{VERB.inspect} — " \
                    "lowercase ASCII letters only, which is how an OpenAPI path item spells a method"
      end
    end

    duplicates = verbs.select { |v| v.is_a?(String) }.tally.select { |_, n| n > 1 }.keys
    unless duplicates.empty?
      problems << "duplicate entr#{duplicates.one? ? 'y' : 'ies'}: #{duplicates.map(&:inspect).join(', ')}"
    end

    # NO ORDER RULE, deliberately. The array is ordered and that order is
    # emission order in every generator, so it is CONTENT rather than formatting:
    # sorting it would reorder every generated method in seven SDKs, and there is
    # nothing to canonicalise because the file is declaring the order, not
    # spelling a set in some arbitrary sequence. Determinism — the thing a
    # canonical form usually buys — a JSON array already gives.
  end

  die(problems) unless problems.empty?

  puts "generated-verbs: #{verbs.length} verb(s) — #{verbs.join(', ')}"
end

main if $PROGRAM_NAME == __FILE__
