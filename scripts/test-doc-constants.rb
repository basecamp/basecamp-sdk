#!/usr/bin/env ruby
# frozen_string_literal: true

# Negative-case self-test for scripts/sync-doc-constants.rb.
#
# The gate's own `make check` run only ever exercises the VALID repo, which
# proves it can say yes and nothing else. This test crafts each failure mode
# the gate claims to catch and asserts it rejects that input (non-zero exit +
# an expected message fragment), driving the gate through its
# DOC_CONSTANTS_ROOT override against a tiny throwaway git repo. It also
# confirms the real repo still passes (positive control).
#
# Same shape and reason as scripts/test-check-fixture-coverage.rb.
#
# Run directly (`ruby scripts/test-doc-constants.rb`) or via
# `make doc-constants-check` (which runs it after the live check).

require "json"
require "tmpdir"
require "fileutils"
require "open3"

ROOT = File.expand_path("..", __dir__)
GATE = File.join(__dir__, "sync-doc-constants.rb")

REVISION = "d0edc1283b231c58b7c88b014df5f8d231b1f7c8"
SHORT    = REVISION[0, 8]
PIN_DATE = "2026-07-31"
API_VER  = "2026-08-15"

failures = []

def expect_pass(failures, label, out, status)
  return if status.success?

  failures << "#{label}: expected PASS, gate exited #{status.exitstatus}:\n#{out}"
end

def expect_fail(failures, label, out, status, fragment)
  if status.success?
    failures << "#{label}: expected FAILURE, gate exited 0:\n#{out}"
  elsif !out.include?(fragment)
    failures << "#{label}: failed as expected but message missing #{fragment.inspect}:\n#{out}"
  end
end

def expect_absent(failures, label, out, fragment)
  return unless out.include?(fragment)

  failures << "#{label}: output should not mention #{fragment.inspect}:\n#{out}"
end

# --- the crafted repo ----------------------------------------------------------
#
# Small enough to read in one screen, and every marked value is deliberately
# correct so that a test can make exactly one thing wrong. Files are written,
# `git init`ed and `git add`ed, because the gate discovers Markdown through
# `git ls-files` (so the gitignored internal docs/ tree is never scanned) and
# would see an empty repo otherwise.

# The in-repo spec carries a version NOTHING in the crafted prose matches.
# The real spec is handed to the gate as --openapi from outside the repo, so
# every case in this file only passes if that argument is honored — a gate that
# quietly fell back to ./openapi.json (the bug fixed in 23d39eec2) fails the
# whole suite rather than one case that could rot into a tautology.
DECOY_API_VER = "2000-01-01"

def default_files
  {
    "openapi.json" => JSON.pretty_generate("info" => { "version" => DECOY_API_VER }),
    "conformance/schema.json" => JSON.pretty_generate(
      "properties" => { "assertions" => { "items" => { "properties" => {
        "type" => { "enum" => %w[status header jsonPath] },
      } } } }
    ),
    "spec/api-provenance.json" => JSON.pretty_generate(
      "bc3" => { "revision" => REVISION, "date" => PIN_DATE }
    ),
    "spec/doc-constants.json" => JSON.pretty_generate(
      "markerCounts" => {
        "api-version" => { "SPEC.md" => 1 },
        "bc3-pin" => { "COORDINATION.md" => 1 },
        "assertion-types" => { "SPEC.md" => 1 },
      }
    ),
    "COORDINATION.md" => <<~MD,
      # Coordination

      The pin is `#{SHORT}` as of the #{PIN_DATE} sync. <!-- @bc3-pin -->
    MD
    "SPEC.md" => <<~MD,
      # Spec

      API_VERSION is `#{API_VER}`. <!-- @api-version -->

      <!-- @assertion-types:begin -->
      | Type | Meaning |
      |------|---------|
      | `status` | HTTP status |
      | `header` | a header |
      | `jsonPath` | a JSON path |
      <!-- @assertion-types:end -->
    MD
    "spec/api-gaps/entry.md" => <<~MD,
      # An entry

      Verified against `abc1234` — the pin when this was written.
    MD
  }
end

# Materialise the crafted repo (after `mutate` has had a chance to break one
# thing), run the gate in `mode`, and hand the result to `inspect_result` while
# the tmp dir is still alive. Returns [combined_output, Process::Status].
#
# Layout: base/repo is the git checkout the gate scans; base/openapi-source.json
# is the --openapi argument, deliberately OUTSIDE the checkout.
def run_gate(mode: "--check", mutate: nil, inspect_result: nil, openapi_version: API_VER)
  base = Dir.mktmpdir("doc-constants-test")
  begin
    dir = File.join(base, "repo")
    FileUtils.mkdir_p(dir)

    source = File.join(base, "openapi-source.json")
    File.write(source, JSON.pretty_generate("info" => { "version" => openapi_version }),
               encoding: "UTF-8")

    files = default_files
    mutate&.call(files)

    files.each do |rel, body|
      path = File.join(dir, rel)
      FileUtils.mkdir_p(File.dirname(path))
      File.write(path, body, encoding: "UTF-8")
    end

    # The gate discovers Markdown through `git ls-files`, so an un-added tree
    # would look empty and every case would vacuously "pass".
    Open3.capture2e("git", "-C", dir, "init", "-q")
    Open3.capture2e("git", "-C", dir, "add", "-A")

    out, status = Open3.capture2e({ "DOC_CONSTANTS_ROOT" => dir }, "ruby", GATE, mode,
                                  "--openapi", source, chdir: dir)
    inspect_result&.call(out, status, dir)
    [out, status]
  ensure
    # git leaves the odd lock/objects file behind on macOS and mktmpdir's own
    # cleanup raises ENOTEMPTY when it loses that race. A leaked tmp dir is a
    # far better outcome than a gate that flakes inside `make check`.
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

def gate(mutate = nil, openapi_version: API_VER)
  run_gate(mutate: mutate, openapi_version: openapi_version)
end

def writer(mutate = nil, &inspect_result)
  run_gate(mode: "--write", mutate: mutate, inspect_result: inspect_result)
end

def read_in(dir, rel)
  File.read(File.join(dir, rel), encoding: "UTF-8")
end

# --- positive controls ---------------------------------------------------------

out, status = Open3.capture2e("ruby", GATE, "--check", chdir: ROOT)
expect_pass(failures, "real repo passes", out, status)

out, status = gate
expect_pass(failures, "crafted valid repo passes", out, status)

# --- @api-version --------------------------------------------------------------

out, status = gate ->(f) { f["SPEC.md"] = f["SPEC.md"].sub(API_VER, "2020-01-01") }
expect_fail(failures, "api-version drifted", out, status,
            "@api-version says 2020-01-01")

out, status = gate ->(f) { f["SPEC.md"] = f["SPEC.md"].sub("is `#{API_VER}`", "is unknown") }
expect_fail(failures, "api-version span has no date", out, status,
            "states no YYYY-MM-DD version")

# --openapi is the source of record, positively: move the PASSED file's version
# and the prose that used to match must now be reported as drifted. Paired with
# the decoy ./openapi.json every case already runs against, this pins the
# forwarding in both directions.
out, status = gate(nil, openapi_version: "2031-03-03")
expect_fail(failures, "--openapi is the source of record", out, status,
            "@api-version says #{API_VER}")
expect_absent(failures, "--openapi is the source of record", out, DECOY_API_VER)

# --- @bc3-pin ------------------------------------------------------------------

out, status = gate ->(f) { f["COORDINATION.md"] = f["COORDINATION.md"].sub(SHORT, "beefbeef") }
expect_fail(failures, "pin SHA drifted", out, status, "@bc3-pin says `beefbeef`")

out, status = gate ->(f) { f["COORDINATION.md"] = f["COORDINATION.md"].sub("`#{SHORT}` ", "") }
expect_fail(failures, "pin span has no SHA", out, status, "states no backticked bc3 SHA")

out, status = gate ->(f) { f["COORDINATION.md"] = f["COORDINATION.md"].sub(" as of the #{PIN_DATE} sync", "") }
expect_fail(failures, "pin span has no date", out, status, "states no YYYY-MM-DD sync date")

out, status = gate ->(f) { f["COORDINATION.md"] = f["COORDINATION.md"].sub(PIN_DATE, "2001-01-01") }
expect_fail(failures, "pin date drifted", out, status, "@bc3-pin says 2001-01-01")

out, status = gate ->(f) { f["COORDINATION.md"] = f["COORDINATION.md"].sub("`#{SHORT}`", SHORT) }
expect_fail(failures, "pin SHA lost its backticks", out, status, "bare SHA #{SHORT}")

# The bare-SHA scan must not fire on a long decimal run — an issue or recording
# id in a pin sentence is not a SHA. Regression test for the 7-digit false
# positive in the first cut of this gate.
out, status = gate lambda { |f|
  f["COORDINATION.md"] = f["COORDINATION.md"].sub("sync.", "sync; see recording 1069479523.")
}
expect_pass(failures, "decimal id in a pin span is not a bare SHA", out, status)
expect_absent(failures, "decimal id in a pin span is not a bare SHA", out, "bare SHA")

# ...but an all-decimal abbreviation of the ACTUAL pin still has to be caught,
# which is the case the letter requirement alone would miss.
DECIMAL_REVISION = "1234567890123456789012345678901234567890"
out, status = gate lambda { |f|
  f["spec/api-provenance.json"] =
    JSON.pretty_generate("bc3" => { "revision" => DECIMAL_REVISION, "date" => PIN_DATE })
  f["COORDINATION.md"] =
    "# Coordination\n\nThe pin is #{DECIMAL_REVISION[0, 8]} as of the #{PIN_DATE} sync. <!-- @bc3-pin -->\n"
}
expect_fail(failures, "all-decimal pin abbreviation is still a bare SHA", out, status,
            "bare SHA #{DECIMAL_REVISION[0, 8]}")

# --- unmarked restatement of the CURRENT pin -----------------------------------

out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = "# An entry\n\nThe provenance pin is `#{SHORT}`.\n"
}
expect_fail(failures, "unmarked current-pin restatement", out, status,
            "is the current provenance pin, restated outside a @bc3-pin span")

# Backticks must not be the way around the rule.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = "# An entry\n\nThe provenance pin is #{SHORT} today.\n"
}
expect_fail(failures, "unmarked BARE current-pin restatement", out, status,
            "is the current provenance pin, restated outside a @bc3-pin span")

# ...and neither must a COMPOUND code span. `A..B` is not a lone SHA, so the
# backticked pattern skips it; blanking code spans before the bare scan used to
# hide it from that pass too, which put the blind spot precisely on range
# notation — the most common way these files name a revision.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = "# An entry\n\nThe `dffa7e11b3..#{SHORT}` range is current.\n"
}
expect_fail(failures, "current pin inside a compound code span", out, status,
            "is the current provenance pin, restated outside a @bc3-pin span")

# The other endpoint of that range is NOT the current pin and must stay silent —
# reading inside compound spans must not turn settled history into a finding.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = "# An entry\n\nThe `dffa7e11b3..e83b2733` range was triaged.\n"
}
expect_pass(failures, "historical range is left alone", out, status)

grant = lambda { |f, entry|
  cfg = JSON.parse(f["spec/doc-constants.json"])
  cfg["unmarkedPinCitations"] = { "spec/api-gaps/entry.md" => entry }
  f["spec/doc-constants.json"] = JSON.pretty_generate(cfg)
}

out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = "# An entry\n\nSDK #528 repinned to `#{SHORT}`.\n"
  grant.call(f, "count" => 1, "reason" => "as-of fact about SDK #528")
}
expect_pass(failures, "granted pin citation passes", out, status)

# The grant is bounded. A SECOND unmarked citation appearing in an already-
# granted file is the hole an unbounded file grant leaves open, and it is
# widest in exactly the files that need granting.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] =
    "# An entry\n\nSDK #528 repinned to `#{SHORT}`.\n\nThe provenance pin is `#{SHORT}`.\n"
  grant.call(f, "count" => 1, "reason" => "as-of fact about SDK #528")
}
expect_fail(failures, "granted file grows an extra citation", out, status,
            "grants 1 unmarked citation(s) of the current pin, found 2 (lines 3, 5)")

# Claims are counted, not lines. A second claim appended to a line that already
# carries one must not hide behind the first — with a first-match-only scan the
# count still read 1 here and the new claim sailed through.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] =
    "# An entry\n\nSDK #528 repinned to `#{SHORT}`, and the provenance pin is `#{SHORT}`.\n"
  grant.call(f, "count" => 1, "reason" => "as-of fact about SDK #528")
}
expect_fail(failures, "two citations on one line both count", out, status,
            "grants 1 unmarked citation(s) of the current pin, found 2 (line 3 (x2))")

# An entry that no longer describes anything is a standing permission nobody
# reviewed — it would silently cover the next unmarked restatement in the file.
out, status = gate ->(f) { grant.call(f, "count" => 1, "reason" => "stale grant") }
expect_fail(failures, "dead pin-citation grant", out, status,
            "found 0 — the entry no longer describes the file")

# ...including when the granted file is gone entirely: a grant that outlives
# its path springs back unreviewed the day someone recreates it.
out, status = gate lambda { |f|
  f.delete("spec/api-gaps/entry.md")
  grant.call(f, "count" => 1, "reason" => "grant for a deleted file")
}
expect_fail(failures, "grant for a deleted file", out, status,
            "spec/api-gaps/entry.md: spec/doc-constants.json grants 1 unmarked citation(s)")

# The grant SHAPE is load-bearing. The pre-count spelling was a bare string;
# accepting one now would read as "grant everything" in the one list whose
# whole job is to be bounded.
out, status = gate ->(f) { grant.call(f, "as-of fact about SDK #528") }
expect_fail(failures, "bare-string grant rejected", out, status,
            "must be an object with \"count\" and \"reason\"")

out, status = gate ->(f) { grant.call(f, "count" => 0, "reason" => "zero") }
expect_fail(failures, "zero count rejected", out, status,
            ".count must be a positive integer, got 0")

out, status = gate ->(f) { grant.call(f, "count" => "1", "reason" => "stringly typed") }
expect_fail(failures, "non-integer count rejected", out, status,
            ".count must be a positive integer, got \"1\"")

out, status = gate ->(f) { grant.call(f, "count" => 1, "reason" => "   ") }
expect_fail(failures, "blank reason rejected", out, status,
            "needs a non-empty \"reason\"")

# A historical SHA must NOT trip it: that is the whole point of the marker
# convention, and a gate that flagged settled triage would be unusable.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = "# An entry\n\nVerified at `dffa7e11b3`, the pin at the time.\n"
}
expect_pass(failures, "historical SHA in prose is left alone", out, status)

# Fenced code is not prose. A pin sentence quoted in an example must not fire.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = "# An entry\n\n```\nThe provenance pin is `#{SHORT}`.\n```\n"
}
expect_pass(failures, "fenced pin restatement is not prose", out, status)

# --- @assertion-types ----------------------------------------------------------

out, status = gate ->(f) { f["SPEC.md"] = f["SPEC.md"].sub("| `jsonPath` | a JSON path |\n", "") }
expect_fail(failures, "assertion type missing from table", out, status, "missing: `jsonPath`")

out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("<!-- @assertion-types:end -->",
                                  "| `invented` | nope |\n<!-- @assertion-types:end -->")
}
expect_fail(failures, "table documents an absent type", out, status,
            "absent from conformance/schema.json: `invented`")

out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("<!-- @assertion-types:end -->",
                                  "| `status` | again |\n<!-- @assertion-types:end -->")
}
expect_fail(failures, "assertion type tabulated twice", out, status, "is tabulated twice")

# --- marker structure ----------------------------------------------------------

out, status = gate ->(f) { f["SPEC.md"] += "\nSomething. <!-- @nope -->\n" }
expect_fail(failures, "unknown marker kind", out, status, "unknown doc-constant marker @nope")

out, status = gate ->(f) { f["SPEC.md"] += "\nSomething. <!-- @assertion-types -->\n" }
expect_fail(failures, "block marker used as a line marker", out, status, "is a block marker")

out, status = gate ->(f) { f["SPEC.md"] += "\nSomething. <!-- @bc3-pin:begin -->\n" }
expect_fail(failures, "line marker given a :begin", out, status, "is a line marker; drop the :begin")

out, status = gate ->(f) { f["SPEC.md"] = f["SPEC.md"].sub("<!-- @assertion-types:end -->", "") }
expect_fail(failures, "unclosed block", out, status, ":begin never closed")

out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("<!-- @assertion-types:begin -->",
                                  "<!-- @assertion-types:begin -->\n<!-- @assertion-types:begin -->")
}
expect_fail(failures, "nested block begin", out, status, "inside an unclosed")

out, status = gate ->(f) { f["SPEC.md"] += "\n<!-- @assertion-types:end -->\n" }
expect_fail(failures, ":end without :begin", out, status, ":end without a matching :begin")

# --- marker inventory (exact counts) -------------------------------------------

out, status = gate ->(f) { f["COORDINATION.md"] = f["COORDINATION.md"].sub(" <!-- @bc3-pin -->", "") }
expect_fail(failures, "marker deleted", out, status,
            "COORDINATION.md: spec/doc-constants.json declares 1 @bc3-pin marker(s), found 0")

# The reason this is an exact count and not a floor: a claim ADDED to a file
# that already satisfies its number would otherwise be unprotected forever —
# delete it again later and the count still passes.
out, status = gate lambda { |f|
  f["SPEC.md"] += "\nAlso API_VERSION is `#{API_VER}`. <!-- @api-version -->\n"
}
expect_fail(failures, "marker added without recording it", out, status,
            "A marked claim was added without recording it — set the count to 2")

# ...and once recorded, deleting THAT marker fails too — the property a floor
# could not give.
out, status = gate lambda { |f|
  cfg = JSON.parse(f["spec/doc-constants.json"])
  cfg["markerCounts"]["api-version"]["SPEC.md"] = 2
  f["spec/doc-constants.json"] = JSON.pretty_generate(cfg)
}
expect_fail(failures, "recorded marker later deleted", out, status,
            "declares 2 @api-version marker(s), found 1")

# A marker in a file the inventory never mentions is equally unprotected.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] += "\nThe pin is `#{SHORT}` as of the #{PIN_DATE} sync. <!-- @bc3-pin -->\n"
}
expect_fail(failures, "marker in an undeclared file", out, status,
            "spec/api-gaps/entry.md: carries 1 @bc3-pin marker(s) that spec/doc-constants.json does not declare")

# A marker moved to another file leaves the original short — the count is
# per-file precisely so a relocation is not silent.
out, status = gate lambda { |f|
  f["COORDINATION.md"] = f["COORDINATION.md"].sub(" <!-- @bc3-pin -->", "")
  f["spec/api-gaps/entry.md"] += "\nThe pin is `#{SHORT}` as of the #{PIN_DATE} sync. <!-- @bc3-pin -->\n"
}
expect_fail(failures, "marker relocated without moving the count", out, status,
            "COORDINATION.md: spec/doc-constants.json declares 1 @bc3-pin marker(s), found 0")

# Backticked markers are documentation OF the convention, not uses of it — they
# must not count toward the inventory.
out, status = gate lambda { |f|
  f["COORDINATION.md"] = "# Coordination\n\nMark the line with `<!-- @bc3-pin -->` to gate it.\n"
}
expect_fail(failures, "backticked marker does not satisfy the count", out, status,
            "declares 1 @bc3-pin marker(s), found 0")

out, status = gate lambda { |f|
  cfg = JSON.parse(f["spec/doc-constants.json"])
  cfg["markerCounts"]["not-a-kind"] = { "SPEC.md" => 1 }
  f["spec/doc-constants.json"] = JSON.pretty_generate(cfg)
}
expect_fail(failures, "count declared for an unknown kind", out, status,
            "count declared for unknown marker @not-a-kind")

# --- source-of-truth failures --------------------------------------------------

out, status = gate ->(f) { f.delete("spec/api-provenance.json") }
expect_fail(failures, "missing provenance", out, status, "missing spec/api-provenance.json")

out, status = gate ->(f) { f["spec/api-provenance.json"] = "{not json" }
expect_fail(failures, "unparseable provenance", out, status, "is not valid JSON")

out, status = gate lambda { |f|
  f["spec/api-provenance.json"] = JSON.pretty_generate("bc3" => { "revision" => "d0edc12", "date" => PIN_DATE })
}
expect_fail(failures, "abbreviated revision in provenance", out, status,
            "is not a full 40-char SHA")

out, status = gate lambda { |f|
  f["conformance/schema.json"] = JSON.pretty_generate("properties" => {})
}
expect_fail(failures, "assertion enum moved in the schema", out, status, "schema moved?")

# --- the writer ----------------------------------------------------------------

repin_to = lambda { |sha, date|
  lambda { |f|
    f["spec/api-provenance.json"] =
      JSON.pretty_generate("bc3" => { "revision" => sha, "date" => date })
  }
}

writer repin_to.call("a" * 40, "2026-09-09") do |out, status, dir|
  expect_pass(failures, "writer runs clean", out, status)
  written = read_in(dir, "COORDINATION.md")
  unless written.include?("`#{'a' * 8}` as of the 2026-09-09 sync.")
    failures << "writer: expected COORDINATION.md rewritten to the new pin, got:\n#{written}"
  end
  # Abbreviation length is preserved, not normalised to 40.
  if written.include?("a" * 9)
    failures << "writer: SHA abbreviation length was not preserved:\n#{written}"
  end
end

# The writer must never touch a .writerExcludes file, must say so, and must
# still exit 0 so `make generate` is not held hostage — `--check` is what fails.
writer lambda { |f|
  repin_to.call("b" * 40, "2026-09-09").call(f)
  cfg = JSON.parse(f["spec/doc-constants.json"])
  cfg["writerExcludes"] = { "COORDINATION.md" => "heads a narrative" }
  f["spec/doc-constants.json"] = JSON.pretty_generate(cfg)
} do |out, status, dir|
  expect_pass(failures, "writer exits 0 when it declines", out, status)
  unless out.include?("was NOT rewritten")
    failures << "writer: declining an excluded file must be announced, got:\n#{out}"
  end
  unless out.include?("heads a narrative")
    failures << "writer: the decline must quote the committed reason, got:\n#{out}"
  end
  written = read_in(dir, "COORDINATION.md")
  unless written.include?("`#{SHORT}`") && written.include?(PIN_DATE)
    failures << "writer: excluded file must be left untouched, got:\n#{written}"
  end
end

# ...and --check must then reject exactly that state, so declining is a
# deferral to a human and not a hole.
out, status = gate lambda { |f|
  repin_to.call("b" * 40, "2026-09-09").call(f)
  cfg = JSON.parse(f["spec/doc-constants.json"])
  cfg["writerExcludes"] = { "COORDINATION.md" => "heads a narrative" }
  f["spec/doc-constants.json"] = JSON.pretty_generate(cfg)
}
expect_fail(failures, "check rejects what the writer declined", out, status,
            "@bc3-pin says `#{SHORT}`")

# @assertion-types is out of the writer's reach by design: a new row needs a
# description only a human can write, and failing here would break every
# `make generate`.
writer lambda { |f|
  f["conformance/schema.json"] = JSON.pretty_generate(
    "properties" => { "assertions" => { "items" => { "properties" => {
      "type" => { "enum" => %w[status header jsonPath brandNew] },
    } } } }
  )
} do |out, status, dir|
  expect_pass(failures, "writer ignores assertion-type drift", out, status)
  written = read_in(dir, "SPEC.md")
  if written.include?("brandNew")
    failures << "writer: must not invent an assertion-type row:\n#{written}"
  end
end

# Structural marker damage IS fatal to the writer: it means a span it was
# supposed to maintain was invisible to it.
writer ->(f) { f["SPEC.md"] += "\nSomething. <!-- @nope -->\n" } do |out, status, _dir|
  if status.success?
    failures << "writer: malformed markers must be fatal, got exit 0:\n#{out}"
  end
end

# --- locale independence -------------------------------------------------------
#
# Pins the LC_ALL=C fix: the tracked Markdown and openapi.json both carry
# non-ASCII, and Ruby otherwise reads them in the locale's encoding.
out, status = Open3.capture2e({ "LC_ALL" => "C", "LANG" => "C" }, "ruby", GATE, "--check", chdir: ROOT)
expect_pass(failures, "real repo passes under LC_ALL=C", out, status)

# --- report --------------------------------------------------------------------

if failures.empty?
  puts "sync-doc-constants self-test: all cases passed."
  exit 0
else
  warn "sync-doc-constants self-test: #{failures.length} case(s) failed."
  failures.each { |f| warn "\n#{f}" }
  exit 1
end
