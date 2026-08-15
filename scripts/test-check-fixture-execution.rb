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
require "yaml"
require "tmpdir"
require "fileutils"
require "open3"

GATE = File.join(__dir__, "check-fixture-execution.rb")
RUNNERS = %w[go kotlin python ruby swift typescript].freeze
ROSTER_FILE = "spec/zero-skip-roster.yml"

failures = []

def utf8(out) = out.dup.force_encoding("UTF-8")

# A manifest set where every runner ran all 3 cases and excluded nothing.
def default_manifests
  RUNNERS.to_h do |runner|
    [runner, { "runner" => runner, "total_non_live" => 3, "executed" => 3, "excluded" => [] }]
  end
end

# A roster that agrees with whatever the manifests say, so roster drift is only
# ever exercised by a case that asks for it. Built as a Ruby structure and
# dumped, so a case that wants to break the roster breaks one field rather than
# hand-editing YAML text — and a case that wants to break the SYNTAX passes a
# raw string instead (see `roster_text:`).
def default_roster(manifests)
  runners = RUNNERS.to_h do |runner|
    # Defensive: some cases replace a manifest with a non-Hash or nil on
    # purpose, and this helper only exists to keep the roster in step with the
    # ones that are real.
    entry = manifests[runner]
    excluded = entry.is_a?(Hash) ? (entry["excluded"] || []) : []
    [runner, {
      "source" => "`runner.x` `SKIPS`",
      "note" => "a note, which an empty section needs",
      "skips" => excluded.map { |e| { "case" => e["name"], "reason" => "because." } },
    }]
  end
  { "runners" => runners }
end

# A valid roster in which nobody skips anything — the starting point for every
# case that wants a specific disagreement. Rebuilt on each call, because the
# cases mutate what they are handed.
def roster_without_skips
  default_roster(default_manifests)
end

# ...with one runner's skips replaced wholesale.
def roster_with(**by_runner)
  doc = roster_without_skips
  by_runner.each { |runner, skips| doc["runners"].fetch(runner.to_s)["skips"] = skips }
  doc
end

CLEAN_SECTION = %(    source: "`x`"\n    note: "a note"\n    skips: []\n)

# A valid roster as TEXT, with Go's body substitutable and arbitrary text
# appendable inside `runners`. The duplicate-key cases have no other way in: a
# Ruby Hash cannot hold one key twice, so `YAML.dump` can never emit the defect
# they exist to catch, and `roster:` goes through `YAML.dump`.
def raw_roster(go: CLEAN_SECTION, tail: "")
  sections = RUNNERS.map { |runner| "  #{runner}:\n#{runner == 'go' ? go : CLEAN_SECTION}" }
  "runners:\n#{sections.join}#{tail}"
end

# Materialise the manifests (after `mutate` has had a chance to break one) and
# run the gate against them.
#
# `roster:` replaces the roster STRUCTURE (a Hash, dumped to YAML, or nil to
# write no file at all); `roster_text:` replaces the file's bytes outright, for
# the cases about YAML the loader cannot parse.
def gate(mutate = nil, partial: false, roster: :default, roster_text: nil, env: {})
  manifests = default_manifests
  mutate&.call(manifests)

  Dir.mktmpdir do |dir|
    FileUtils.mkdir_p(File.join(dir, "conformance", "manifests"))
    FileUtils.mkdir_p(File.join(dir, "spec"))

    doc = roster == :default ? default_roster(manifests) : roster
    body = roster_text || (doc.nil? ? nil : YAML.dump(doc))
    File.write(File.join(dir, ROSTER_FILE), body) unless body.nil?

    manifests.each do |runner, body|
      next if body.nil? # a nil entry means "this runner did not report"

      File.write(File.join(dir, "conformance", "manifests", "#{runner}.json"),
                 "#{JSON.pretty_generate(body)}\n")
    end

    cmd = ["ruby", GATE]
    cmd << "--partial" if partial
    out, status = Open3.capture2e({ "FIXTURE_EXECUTION_ROOT" => dir }.merge(env), *cmd)
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
# up while the comparison saw a single entry. The roster is handed the case ONCE
# — a roster built from this manifest would carry the duplicate too, and fail in
# the loader for a different reason than the one this case is about.
out, status = gate(lambda { |m|
  2.times { m["go"]["excluded"] << { "file" => "alpha.json", "name" => "dup", "reason" => "x" } }
  m["go"]["executed"] -= 2
}, roster: roster_with(go: [{ "case" => "dup", "reason" => "x." }]))
expect_fail(failures, "one case excluded twice in one manifest", out, status, "is excluded twice")

# --- roster set-equality (#736) ----------------------------------------------
#
# The roster is spec/zero-skip-roster.yml. It used to be SPEC.md's prose, read
# back out by a parser, and the cases below used to craft Markdown: a bullet with
# an unrecognised marker, a blockquoted entry, a second roster block, a claim
# riding a heading. Each of those was a bypass found after the fact, and each fix
# was one more selector.
#
# They are gone because the class is gone, not because it was ruled out of
# scope. A stale entry now has to be an entry — a `case:` under a runner — and
# whatever it is spelled with, it is compared to the manifest byte for byte. The
# spelling cases that remain are here as evidence of exactly that (see "the old
# bypasses are just strings now", below).
#
# What SPEC's block says is checked by `make doc-constants-check`, which renders
# it from this file and requires byte equality. Nothing here reads SPEC.md at
# all.

# A runner skip the roster does not list. This is the drift that was ALREADY in
# SPEC when the check was written: Kotlin and Swift excluded the link-header
# case via their tag branch while the roster described it in prose.
out, status = gate(lambda { |m| exclude(m, "unlisted skip", ["ruby"]) },
                   roster: roster_without_skips)
expect_fail(failures, "a runner skip missing from the roster", out, status,
            "does not list it")

# The opposite drift: an entry for a skip that has been closed. "A PR that
# closes a gap deletes exactly its own entry" is the roster's own rule, and it
# was enforced by nothing.
out, status = gate(nil, roster: roster_with(go: [{ "case" => "a closed gap", "reason" => "stale." }]))
expect_fail(failures, "a roster entry for a skip that no longer exists", out, status,
            "which no longer excludes it")

# A runner with no section contributes nothing, so its skips would go unchecked.
# An absent key and an empty list are not the same statement, and only one of
# them was written on purpose.
out, status = gate(nil, roster: begin
  doc = roster_without_skips
  doc["runners"].delete("go")
  doc
end)
expect_fail(failures, "a runner with no roster section", out, status, "no section for go")

# No roster file at all must not read as "the roster agrees".
out, status = gate(nil, roster: nil)
expect_fail(failures, "no roster file", out, status, "is missing")

# An EMPTY roster is the vacuity case: both sides empty must not be a pass.
# Every runner is required, so an empty `runners` map fails as six missing
# sections rather than as agreement with six empty manifests.
out, status = gate(nil, roster: { "runners" => {} })
expect_fail(failures, "an empty roster is not agreement", out, status, "no section for")

# ...including a file that parses to nothing at all.
out, status = gate(nil, roster_text: "\n")
expect_fail(failures, "a roster that parses to nothing", out, status, "expected a mapping")

# YAML the loader cannot parse is not a roster that lists nothing. It has to
# fail, not fall back to an empty set that agrees with clean manifests.
out, status = gate(nil, roster_text: "runners:\n  go:\n   - [oops\n")
expect_fail(failures, "a roster that is not valid YAML", out, status, "not valid YAML")

# A section for a runner nothing compares is dead weight that reads as coverage.
out, status = gate(nil, roster: begin
  doc = roster_without_skips
  # A DEEP copy: dumping one object twice emits a YAML alias — and a shallow
  # `dup` still shares the `skips` array, which is enough to emit one. This case
  # would then be pinning the alias refusal below instead of the unknown-runner
  # rule.
  doc["runners"]["haskell"] = Marshal.load(Marshal.dump(doc["runners"]["go"]))
  doc
end)
expect_fail(failures, "a roster section for an unknown runner", out, status, "unknown runner")

# Anchors and aliases are refused rather than resolved: two sections sharing one
# anchor edit as one and read as two. Left to Psych they raise a class that is
# not a syntax error, so an unguarded loader exits with a backtrace naming the
# loader rather than a message naming the file.
out, status = gate(nil, roster_text: <<~YAML)
  runners:
    go: &section
      source: "`x`"
      note: "a note"
      skips: []
    kotlin: *section
YAML
expect_fail(failures, "a roster using a YAML alias", out, status, "aliases are refused")

# --- duplicate mapping keys ---------------------------------------------------
#
# The alias above is one way YAML lets a roster mean something other than what
# it looks like. A repeated KEY is the other, and the worse one: it is legal,
# Psych accepts it, and it keeps only the LAST — so the earlier claim is
# discarded in silence. That is exactly the duplicate-section false green this
# whole change exists to retire, reappearing inside the instrument that replaced
# the parser. The loader therefore reads the parsed AST, where a key is a key,
# rather than the Hash the duplicate has already been erased from.
#
# Each case below is written so it would PASS with the guard removed: what
# survives the duplicate agrees with the manifests, and only the discarded half
# is a ghost. A case that failed either way would be pinning some other rule.

# A whole runner section, twice. The first claims a skip nothing excludes.
out, status = gate(nil, roster_text: raw_roster(
  go: %(    source: "`x`"\n    note: "a note"\n    skips:\n) +
      %(      - case: "a discarded ghost"\n        reason: "never compared."\n),
  tail: %(  go:\n    source: "`x`"\n    note: "the section that wins"\n    skips: []\n)
))
expect_fail(failures, "a runner section written twice", out, status,
            "`runners.go` is written more than once")

# One FIELD of a section, twice, and `skips` is where it costs most: the
# surviving list is the one compared against the manifests.
out, status = gate(nil, roster_text: raw_roster(go: %(    source: "`x`"\n    note: "a note"\n) +
  %(    skips:\n      - case: "a discarded ghost"\n        reason: "never compared."\n) +
  %(    skips: []\n)))
expect_fail(failures, "a section field written twice", out, status,
            "`runners.go.skips` is written more than once")

# One field of a SKIP ENTRY, twice — the deepest level, and the one an
# enumeration of levels rather than a walk would have missed.
out, status = gate(lambda { |m| exclude(m, "the case that wins", ["go"]) },
                   roster_text: raw_roster(go: %(    source: "`x`"\n    note: "a note"\n) +
                     %(    skips:\n      - case: "a discarded ghost"\n) +
                     %(        case: "the case that wins"\n        reason: "never compared."\n)))
expect_fail(failures, "a skip entry field written twice", out, status,
            "`runners.go.skips[0].case` is written more than once")

# A second YAML DOCUMENT, which is the duplicate one level out and is breach 2
# of the old reader — "a second complete roster block silently ignored" —
# reproduced in the file format. `safe_load` returns the first document and
# drops the rest without a word. The first document here is the one that agrees
# with the manifests, so with the guard removed this passes green while a whole
# second roster sits in the file unread.
out, status = gate(nil, roster_text: "#{raw_roster}---\n#{raw_roster(
  go: %(    source: "`x`"\n    note: "a note"\n    skips:\n) +
      %(      - case: "an unread second roster"\n        reason: "never compared."\n)
)}")
expect_fail(failures, "a second YAML document", out, status, "holds 2 YAML documents")

# An extra TOP-LEVEL key. The loader used to read `doc["runners"]` and ignore
# whatever else the document held, so a misspelled or duplicated top-level
# mapping could carry a whole roster that is visible in the file and read by
# nothing — the discarded claim the nested unknown-key rules already refuse one
# level down, at the level a reader is likeliest to paste one.
out, status = gate(nil, roster: begin
  doc = roster_without_skips
  doc["runnners"] = { "go" => { "source" => "`x`", "note" => "unread", "skips" => [
    { "case" => "an unread entry", "reason" => "never compared." }
  ] } }
  doc
end)
expect_fail(failures, "an unknown top-level key", out, status, "unknown top-level key(s) runnners")

# A YAML MERGE key, which is the duplicate-key defect wearing a different key.
# Psych resolves `<<` before the Hash exists and any field the section also
# states explicitly wins, so the merged entry vanishes — and the duplicate rule
# cannot see it, because `<<` and `skips` are two distinct keys in the AST. The
# manifests are clean here, so the surviving `skips: []` agrees with them and
# this is green without the rule.
out, status = gate(nil, roster_text: raw_roster(go: %(    source: "`x`"\n    note: "a note"\n) +
  %(    <<: {skips: [{case: "a merged ghost", reason: "discarded."}]}\n) +
  %(    skips: []\n)))
expect_fail(failures, "a section using a YAML merge key", out, status, "uses a YAML merge key")

# --- a value that is markup rather than text ----------------------------------
#
# Values render VERBATIM into a Markdown document, so an HTML comment delimiter
# in one is not text. `<!--` opens a comment that swallows the rest of the block
# and whatever follows it in SPEC; a complete `<!-- @zero-skip-roster:end -->`
# injects a doc-constant MARKER, which the writer emits and exits 0 over, leaving
# a document the next check refuses as structurally malformed.
#
# Reported as THREE findings across two rounds — a bare `<!--`, an injected
# marker, and then `<details>`, which needs no comment at all and collapses
# everything after it on GitHub while both gates stay green. The first two were
# closed with `/<!--|-->/`; the third is what showed that was a selector rather
# than a rule.
#
# Raw HTML has exactly two openers: `<` begins every tag and every comment, `&`
# begins every entity. Refusing both closes the class including the spellings
# nobody has written, and REPLACES the comment rule rather than joining it. The
# `<details>`, `<script>` and entity rows are here so it cannot narrow back. The
# entities sit MID-VALUE on purpose: `&lt;` ends in a semicolon, so at the end
# of a value the clause rule would refuse it first and the case would be pinning
# that guard instead of this one.
#
# A bare `-->` is deliberately NO LONGER refused, and this is a real narrowing
# rather than an oversight. The old rule took it because it was half of a
# delimiter pair; with `<` refused, no value can open a comment for it to close,
# and the renderer emits none either — so it is literal text in a Markdown
# document, like any other punctuation. Refusing a closer that cannot close
# anything would be the fortress this PR keeps arguing against.
["<!--", %(a note <!-- @zero-skip-roster:end -->),
 %(<!-- @fixture-categories:begin -->), "<details>", "<script>",
 "an &amp; entity", "an &lt;b&gt; entity"].each do |markup|
  out, status = gate(nil, roster: begin
    doc = roster_without_skips
    doc["runners"]["go"]["note"] = "a note #{markup}".strip
    doc
  end)
  expect_fail(failures, "a note carrying #{markup.inspect}", out, status,
              "Values render verbatim into Markdown")
end

# ...but NOT `case`, which this file copies rather than composes. It must match
# the fixture's name byte for byte, so a rule about how a value ought to be
# WRITTEN cannot apply to it: refusing one would make a legitimately-named skip
# unrepresentable and this gate permanently red, with no edit to the roster that
# helps, since rephrasing changes the very key being compared. Asserted as a
# PASS, because "the roster can state what the runner actually skipped" is the
# property, and a guard that swallows a valid state fails silently otherwise.
["a case with a <bracket>", "a case with an &amp", "a trailing space case "].each do |name|
  out, status = gate(lambda { |m| exclude(m, name, ["go"]) },
                     roster: roster_with(go: [{ "case" => name, "reason" => "because." }]))
  expect_pass(failures, "a copied case name spelled #{name.inspect}", out, status)
end

# The exemption is for COPIED keys only: the same spelling in composed prose is
# still refused, which is what keeps it an exemption rather than a hole.
out, status = gate(nil, roster: begin
  doc = roster_without_skips
  doc["runners"]["go"]["note"] = "a note with a <bracket> in it"
  doc
end)
expect_fail(failures, "the same spelling in a composed note", out, status,
            "Values render verbatim into Markdown")

# The rule that is not about authoring still reaches `case`: a control character
# cannot render onto the one line the roster gives it, whoever wrote it.
out, status = gate(lambda { |m| exclude(m, "a case", ["go"]) }, roster: roster_with(
  go: [{ "case" => "a case\u{2028}- \"smuggled\"", "reason" => "because." }]
))
expect_fail(failures, "a copied case name carrying a line separator", out, status,
            "a control character or line separator")

# The same rule reaches every field, not just the qualifiers — a `source` and a
# `reason` render into the document exactly as verbatim.
[["source", "`x` <!-- @zero-skip-roster:end -->"],
 ["reason", "because. <!-- oops -->"]].each do |key, markup|
  out, status = gate(lambda { |m| exclude(m, "a real case", ["go"]) }, roster: begin
    doc = roster_with(go: [{ "case" => "a real case", "reason" => "because." }])
    if key == "reason"
      doc["runners"]["go"]["skips"][0]["reason"] = markup
    else
      doc["runners"]["go"][key] = markup
    end
    doc
  end)
  expect_fail(failures, "a #{key} carrying an HTML comment", out, status,
              "Values render verbatim into Markdown")
end

# --- a value that can end a line ----------------------------------------------
#
# Every scalar renders onto ONE line. A value carrying a line ending spells a
# second heading or bullet in SPEC while the block still matches byte for byte,
# and — for a `case` — while the manifest projection still compares the whole
# scalar. Same false green, sourced from a character instead of from a parser.
#
# The rule is a CLASS (no control character, no line/paragraph separator), not a
# list, and these cases are evidence for that shape rather than for one
# character. Review reported `\r`; it is only the first of the five below, and
# the other four are spellings nobody has written. Narrowing the guard to the
# reported character leaves all four green, which is the mutation that would
# otherwise have to be argued about instead of demonstrated.
#
# The smuggled payload deliberately ends in a letter, not a period. Ending it in
# one would make the CLAUSE rule refuse the note first, and these cases would be
# pinning that guard instead of this one — the substring trap, one field over.
{ "CR" => "\u{000D}", "U+2028 LINE SEPARATOR" => "\u{2028}",
  "U+2029 PARAGRAPH SEPARATOR" => "\u{2029}", "U+0085 NEL" => "\u{0085}",
  "VT" => "\u{000B}" }.each do |label, char|
  out, status = gate(nil, roster: begin
    doc = roster_without_skips
    doc["runners"]["go"]["note"] = %(a note#{char}- "a smuggled stale entry" — see above)
    doc
  end)
  expect_fail(failures, "a note carrying #{label}", out, status,
              "a control character or line separator")
end

# NOT tested here: the same separator in a `case`. The guard is one rule in
# `scalar` and covers every field, but a case name is compared byte-exact
# against the manifests, so a separator in one is a LOUD mismatch rather than a
# false green — and a case asserting the refusal would pass off the mismatch
# instead. Writing it revealed something else, which is below.

# --- the gate reads its manifests as UTF-8 ------------------------------------
#
# Fallout from the case above: the manifest read was UNPINNED, so under LC_ALL=C
# — which is how CI runs — a non-ASCII byte in any case name reached JSON as
# US-ASCII and died as Encoding::InvalidByteSequenceError with a Ruby backtrace
# instead of a verdict. Never fired only because every tracked fixture case name
# is ASCII. It was also the file's one unpinned read, beside a roster reader
# that pinned its own and that this change deletes.
#
# The roster AGREES with the manifest here, so the assertion is a pass: the
# gate must reach its verdict at all, which is what an unpinned read denies it.
accented = "Bracketed IPv6 loopback \u{2014} caf\u{00E9} na\u{00EF}ve"
out, status = gate(lambda { |m| exclude(m, accented, ["go"]) },
                   roster: roster_with(go: [{ "case" => accented, "reason" => "r." }]),
                   env: { "LC_ALL" => "C", "LANG" => "C" })
expect_pass(failures, "a non-ASCII case name under LC_ALL=C", out, status)

# An absent `skips` key reads as "skips nothing" to any comparison, which is a
# stale roster that passes. Refused, so the empty case has to be written down.
out, status = gate(nil, roster: begin
  doc = roster_without_skips
  doc["runners"]["go"].delete("skips")
  doc
end)
expect_fail(failures, "a section with no skips key", out, status, "has no `skips` key")

# "None" with no reason is indistinguishable from a section somebody forgot to
# fill in — which is the state Python's section would be in if its note were
# dropped, and the note is the whole content of that section.
out, status = gate(nil, roster: begin
  doc = roster_without_skips
  doc["runners"]["go"].delete("note")
  doc
end)
expect_fail(failures, "an empty section that says nothing", out, status, "says nothing")

# A qualifier that punctuates itself. The renderer supplies every terminator, so
# `note: "nothing to skip."` renders `— none; nothing to skip..` and
# `classification: "architectural."` renders `— architectural.; a note` — and
# neither gate can object afterwards, because both compare SPEC against exactly
# what the renderer produced. This is the one rule the YAML stated only in a
# comment; a comment is not a gate.
#
# The trailing-space rows are that rule's bypass, found in review:
# CLAUSE_TERMINATOR_RE anchors at the end, so a single space after the period
# hid it and `nothing to skip. ` rendered `nothing to skip. .`. Closed by
# refusing surrounding whitespace on every value rather than by teaching the
# terminator regex to look past it — the space is content in its own right, and
# a trailing one on ANY value lands invisibly at the end of a rendered line.
#
# The NBSP and ideographic-space rows are the SECOND round of that same lesson,
# and the reason the rule is a \p{Space} match rather than `value.strip`:
# String#strip is ASCII-only, so of the eight whitespace code points worth
# testing it removes two, and U+00A0 walked through the fix for U+0020 and hid
# the period all over again. They are here so the rule cannot quietly narrow
# back to ASCII.
[["note", "nothing to skip.", "ends in terminating punctuation"],
 ["classification", "architectural.", "ends in terminating punctuation"],
 ["note", "nothing to skip. ", "begins or ends with whitespace"],
 ["classification", "architectural. ", "begins or ends with whitespace"],
 ["note", "nothing to skip.\u{00A0}", "begins or ends with whitespace"],
 ["source", "`x`\u{3000}", "begins or ends with whitespace"]]
  .each do |key, value, want|
  out, status = gate(nil, roster: begin
    doc = roster_without_skips
    doc["runners"]["go"][key] = value
    doc
  end)
  expect_fail(failures, "a #{key} of #{value.inspect}", out, status, want)
end

# ...and the same rule from the other end, where there is no punctuation
# involved at all: a leading space is content that shifts the render.
out, status = gate(nil, roster: begin
  doc = roster_without_skips
  doc["runners"]["go"]["source"] = " `x`"
  doc
end)
expect_fail(failures, "a source with a leading space", out, status,
            "which begins or ends with whitespace")

# A misspelled key contributes nothing and looks like a filled-in section.
out, status = gate(nil, roster: begin
  doc = roster_without_skips
  doc["runners"]["go"]["skip"] = doc["runners"]["go"].delete("skips")
  doc
end)
expect_fail(failures, "a section with a misspelled key", out, status, "unknown key(s)")

out, status = gate(nil, roster: begin
  doc = roster_without_skips
  doc["runners"]["go"]["skips"] = "not a list"
  doc
end)
expect_fail(failures, "skips that is not a list", out, status, "not a list")

out, status = gate(lambda { |m| exclude(m, "nameless", ["go"]) }, roster: begin
  roster_with(go: [{ "reason" => "no case key." }])
end)
expect_fail(failures, "a skip entry with no case name", out, status, "`case` is required")

# One case listed twice under a runner. Array#- removes every matching
# occurrence, so both diffs come back empty and the duplicate passes — carrying
# two possibly conflicting reasons for one case.
out, status = gate(lambda { |m| exclude(m, "listed twice", ["go"]) },
                   roster: roster_with(go: [{ "case" => "listed twice", "reason" => "a." },
                                            { "case" => "listed twice", "reason" => "b." }]))
expect_fail(failures, "one case listed twice under a runner", out, status, "more than once")

# Roster drift must be caught in PARTIAL mode too. The normal Linux `make
# check` path always passes --partial (Swift's manifest is macOS-only), so a
# roster check that ran only in full mode never ran locally at all — a stale Go
# or Ruby entry would reach CI untouched. Partial input relaxes the all-six
# overlap verdict and nothing else.
out, status = gate(lambda { |m|
  exclude(m, "unlisted skip", ["ruby"])
  m["swift"] = nil
}, partial: true, roster: roster_without_skips)
expect_fail(failures, "roster drift is caught in partial mode", out, status, "does not list it")

# --- the old bypasses are just strings now -----------------------------------
#
# Five times, a stale entry hid from the prose reader by being spelled in a way
# the reader did not recognise: a `*` marker instead of `- `, a blockquote, a
# table row, an HTML comment, curly quotes or backticks around the name (the
# quote test was ASCII-only), and a second quoted name riding an otherwise
# canonical bullet — which the live Kotlin entry actually carries, in its
# `yields "no response"`. Each was invisible to the comparison, so it never
# contradicted a manifest and the gate passed.
#
# None of those is a shape any more. A roster entry is a `case:` string, and the
# comparison is byte-exact, so every spelling below is a stale entry the gate
# reports rather than skips. This case is the evidence for that claim: it is the
# same five payloads, and they all fail.
["“curly quoted stale”",
 "`backticked stale`",
 "> - \"blockquoted stale\" — x.",
 "| Go | \"table-row stale\" | x |",
 "real name\" — also supersedes \"stale ghost"].each do |name|
  out, status = gate(nil, roster: roster_with(go: [{ "case" => name, "reason" => "stale." }]))
  expect_fail(failures, "a stale entry spelled #{name[0, 24].inspect}", out, status,
              "which no longer excludes it")
end

# --- report ------------------------------------------------------------------

if failures.empty?
  puts "check-fixture-execution self-test: all cases passed."
  exit 0
end

warn "check-fixture-execution self-test: #{failures.length} case(s) failed"
failures.each { |f| warn "\n#{f}" }
exit 1
