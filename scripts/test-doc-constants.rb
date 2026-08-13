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

# Captured gate output, retagged UTF-8. Under a non-UTF-8 locale (LC_ALL=C) Open3
# tags it US-ASCII, and both the expected fragments and the gate's own messages
# contain UTF-8 punctuation — so `out.include?(fragment)` raises
# Encoding::CompatibilityError before comparing anything, and every case dies for
# a reason unrelated to what it tests. The bytes are UTF-8 either way; only the
# tag is wrong. Applied here rather than at each of the five capture sites so a
# sixth cannot reintroduce it.
# THREE operations across two paths, in the --openapi source. `parameters` is a
# path-item key, not an operation, and is here so a counter that walked every
# key would report 4 and fail the positive control.
OP_COUNT = 3
SOURCE_PATHS = {
  "/todos" => { "get" => {}, "post" => {}, "parameters" => [] },
  "/todos/{id}" => { "get" => {} },
}.freeze

# The in-repo openapi.json is a DECOY with a different operation count, the same
# device DECOY_API_VER uses: if the gate ever read the checkout's copy instead of
# the --openapi argument, every operation-count case would report 5, not 3.
DECOY_PATHS = {
  "/decoy" => { "get" => {}, "post" => {}, "put" => {}, "delete" => {}, "patch" => {} },
}.freeze

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
    "openapi.json" => JSON.pretty_generate(
      "info" => { "version" => DECOY_API_VER },
      "paths" => DECOY_PATHS
    ),
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
        "operation-count" => { "SPEC.md" => 1 },
        "fixture-categories" => { "SPEC.md" => 1 },
        "fixture-section-map" => { "SPEC.md" => 1 },
      }
    ),
    # Two tracked conformance fixtures, which is the whole source of truth for
    # the two roster checks — their CONTENT is never read, only their tracked
    # filenames. `beta_write` earns its underscore: the category slug rule
    # rewrites it to `beta-write`, so a check that compared basenames verbatim
    # would pass the positive control and reject the real repo.
    "conformance/tests/alpha.json" => "[]\n",
    "conformance/tests/beta_write.json" => "[]\n",
    "COORDINATION.md" => <<~MD,
      # Coordination

      The pin is `#{SHORT}` as of the #{PIN_DATE} sync. <!-- @bc3-pin -->
    MD
    "SPEC.md" => <<~MD,
      # Spec

      ## §1. Something

      ## §2. Something Else

      API_VERSION is `#{API_VER}`. <!-- @api-version -->

      The surface is `#{OP_COUNT}` operations across 2 paths. <!-- @operation-count -->

      <!-- @assertion-types:begin -->
      | Type | Meaning |
      |------|---------|
      | `status` | HTTP status |
      | `header` | a header |
      | `jsonPath` | a JSON path |
      <!-- @assertion-types:end -->

      <!-- @fixture-categories:begin -->
      | Category | Files | Owning Spec Section(s) |
      |----------|-------|----------------------|
      | alpha | `alpha.json` | §1 Something |
      | beta-write | `beta_write.json` | §2 Something Else |
      <!-- @fixture-categories:end -->

      <!-- @fixture-section-map:begin -->
      | Test file | Test name | Primary section |
      |-----------|----------|----------------|
      | `alpha.json` | does a thing | §1 |
      | `beta_write.json` | does another thing | §2 |
      <!-- @fixture-section-map:end -->
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
def run_gate(mode: "--check", mutate: nil, inspect_result: nil, openapi_version: API_VER,
             openapi_paths: SOURCE_PATHS)
  base = Dir.mktmpdir("doc-constants-test")
  begin
    dir = File.join(base, "repo")
    FileUtils.mkdir_p(dir)

    source = File.join(base, "openapi-source.json")
    File.write(source,
               JSON.pretty_generate("info" => { "version" => openapi_version },
                                    "paths" => openapi_paths),
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

def gate(mutate = nil, openapi_version: API_VER, openapi_paths: SOURCE_PATHS)
  run_gate(mutate: mutate, openapi_version: openapi_version, openapi_paths: openapi_paths)
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

# --- @operation-count ----------------------------------------------------------
#
# The count is one jq away from openapi.json and was restated in prose six times
# across four files. A single new operation left five of them stale and took
# three review rounds to reconcile, which is the failure this marker retires.

out, status = gate ->(f) { f["SPEC.md"] = f["SPEC.md"].sub("`#{OP_COUNT}` operations", "`2` operations") }
expect_fail(failures, "operation-count drifted", out, status,
            "@operation-count says `2`")

out, status = gate ->(f) { f["SPEC.md"] = f["SPEC.md"].sub("`#{OP_COUNT}` operations", "several operations") }
expect_fail(failures, "operation-count span states no backticked integer", out, status,
            "states no backticked integer")

# Backticks are what tell the writer WHICH integer is the claim, so a span with
# two of them is ambiguous rather than merely redundant — it would silently
# rewrite both. SECURITY.md's real sentence names 125 GETs and 83 mutations
# beside the total, so this is the shape that would break it.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("across 2 paths", "across `2` paths")
}
expect_fail(failures, "operation-count span has two backticked integers", out, status,
            "exactly one is required")

# A count that is right for the wrong reason: a path item's non-operation keys
# must not be counted. Adding `parameters` to the second path keeps the real
# count at 3, so a walker that counted every key would now say 5 and fail.
out, status = gate(openapi_paths: {
  "/todos" => { "get" => {}, "post" => {}, "parameters" => [] },
  "/todos/{id}" => { "get" => {}, "parameters" => [], "summary" => "a summary", "servers" => [] },
})
expect_pass(failures, "path-item keys that are not operations are not counted", out, status)

# The decoy proves the count is read from --openapi, not the checkout: the
# in-repo openapi.json declares five operations, so a gate reading it would
# report 5 against a span that says 3.
out, status = gate ->(f) { f["openapi.json"] = JSON.pretty_generate("info" => { "version" => DECOY_API_VER }, "paths" => {}) }
expect_pass(failures, "operation count comes from --openapi, not the checkout", out, status)

# The writer fixes a drifted count in place, which is the whole point: nobody
# should be hand-editing six restatements again.
writer ->(f) { f["SPEC.md"] = f["SPEC.md"].sub("`#{OP_COUNT}` operations", "`999` operations") } do |out, status, dir|
  expect_pass(failures, "writer rewrites a drifted operation count", out, status)
  written = read_in(dir, "SPEC.md")
  unless written.include?("`#{OP_COUNT}` operations")
    failures << "writer: expected the operation count restored to #{OP_COUNT}, got:\n#{written[/^.*operations.*$/]}"
  end
  # The unticked integer in the same sentence is prose, not the claim, and the
  # writer must leave it exactly where it was.
  unless written.include?("across 2 paths")
    failures << "writer: rewrote an unticked integer it has no source for:\n#{written[/^.*operations.*$/]}"
  end
end

# The writer must REFUSE an ambiguous span, not rewrite every integer on it.
# This is the sharp edge: --write returns before the per-kind checkers run, so a
# blanket gsub corrupts the file first and the later check, comparing values that
# are now all identical, certifies the damage. Reproduced on the real SECURITY.md
# sentence before the guard existed: `125` GETs and `83` mutations both became
# `250`, and the check went green.
writer lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("across 2 paths", "across `2` paths")
} do |out, status, dir|
  expect_pass(failures, "writer exits 0 on an ambiguous operation-count span", out, status)
  written = read_in(dir, "SPEC.md")
  unless written.include?("across `2` paths")
    failures << "writer: rewrote an integer on an ambiguous @operation-count span:\n#{written[/^.*operations.*$/]}"
  end
  # And having declined, the span must still be rejected rather than left to rot.
  out2, status2 = Open3.capture2e({ "DOC_CONSTANTS_ROOT" => dir }, "ruby", GATE, "--check",
                                  "--openapi", File.join(File.dirname(dir), "openapi-source.json"),
                                  chdir: dir)
  expect_fail(failures, "check rejects the span the writer declined", out2, status2,
              "exactly one is required")
end

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

# A grant covers as-of FACTS, never the class-A grammar. Swapping one granted
# citation for "the provenance pin is X" leaves the count untouched, so the
# count alone would wave a current-value claim through under a reason that does
# not describe it.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = "# An entry\n\nThe provenance pin is `#{SHORT}`.\n"
  grant.call(f, "count" => 1, "reason" => "as-of fact about SDK #528")
}
expect_fail(failures, "a grant does not cover a class-A claim", out, status,
            "names the current pin in the grammar of a current-value claim")

out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = "# An entry\n\nVerified at the pinned #{SHORT} today.\n"
  grant.call(f, "count" => 1, "reason" => "as-of fact about SDK #528")
}
expect_fail(failures, "a grant does not cover \"at the pinned X\"", out, status,
            "names the current pin in the grammar of a current-value claim")

# The grammar check only fires on lines that actually name today's pin, so
# an as-of citation in a granted file is still fine.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = "# An entry\n\nThe pin is `dffa7e11b3` in that old note.\n"
  grant.call(f, "count" => 1, "reason" => "as-of fact about SDK #528")
  cfg = JSON.parse(f["spec/doc-constants.json"])
  cfg["unmarkedPinCitations"] = {}
  f["spec/doc-constants.json"] = JSON.pretty_generate(cfg)
}
expect_pass(failures, "class-A grammar about a HISTORICAL sha is not flagged", out, status)

# The grant is bounded. A SECOND unmarked citation appearing in an already-
# granted file is the hole an unbounded file grant leaves open, and it is
# widest in exactly the files that need granting.
# Both sentences are as-of facts, so only the COUNT is under test here — the
# class-A grammar check below is a separate floor and must not be what fires.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] =
    "# An entry\n\nSDK #528 repinned to `#{SHORT}`.\n\nSDK #530 verified against `#{SHORT}`.\n"
  grant.call(f, "count" => 1, "reason" => "as-of fact about SDK #528")
}
expect_fail(failures, "granted file grows an extra citation", out, status,
            "grants 1 unmarked citation(s) of the current pin, found 2 (lines 3, 5)")

# Claims are counted, not lines. A second claim appended to a line that already
# carries one must not hide behind the first — with a first-match-only scan the
# count still read 1 here and the new claim sailed through.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] =
    "# An entry\n\nSDK #528 repinned to `#{SHORT}`, and #530 verified against `#{SHORT}`.\n"
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

# ...but the fence must CLOSE. A ````-fence may quote a ```-fence, which is how
# you write a Markdown example about Markdown. Toggling on every fence line left
# an odd number of inner fences flipping the flag on for good, so everything
# after the block read as code — a restatement there was never even looked at,
# and the gate reported success. Fail-open, so this case must FAIL.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = <<~MD
    # An entry

    ````
    ```
    ````

    The provenance pin is `#{SHORT}`.
  MD
}
expect_fail(failures, "prose after a nested fence is still prose", out, status,
            "is the current provenance pin, restated outside a @bc3-pin span")

# The same, one delimiter swapped: a ``` line inside a ~~~ block is content,
# not a close, because closing requires the same character.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = <<~MD
    # An entry

    ~~~
    ```
    ~~~

    The provenance pin is `#{SHORT}`.
  MD
}
expect_fail(failures, "a mismatched delimiter does not close a fence", out, status,
            "is the current provenance pin, restated outside a @bc3-pin span")

# And the inner fence really is inside: a restatement between ```` and ```` is
# code even though a bare ``` sits between them.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = <<~MD
    # An entry

    ````
    ```
    The provenance pin is `#{SHORT}`.
    ```
    ````
  MD
}
expect_pass(failures, "a nested fence keeps its contents fenced", out, status)

# Four spaces is an INDENTED code block, not a fence opener. Showing a fence by
# indenting it is common, the quoted example rarely has a matching close, and a
# permissive indent took it as an opening that then swallowed the rest of the
# file — the fail-open hole again, through the indentation door. So this must
# FAIL: the restatement below is still prose.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = <<~MD
    # An entry

    To open a code block, write:

        ```ruby

    The provenance pin is `#{SHORT}`.
  MD
}
expect_fail(failures, "a 4-space-indented fence does not open one", out, status,
            "is the current provenance pin, restated outside a @bc3-pin span")

# Three spaces is still a fence (CONTRIBUTING.md indents fences inside numbered
# list items exactly this way), so its contents stay code.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = <<~MD
    # An entry

    1. Step one:

       ```
       The provenance pin is `#{SHORT}`.
       ```
  MD
}
expect_pass(failures, "a 3-space-indented fence still fences", out, status)

# A backtick fence's info string may not contain a backtick, so "```code``` is
# inline" is a paragraph with an inline code span, not an opening. Taking it as
# one opened a fence the line never closes and the rest of the file went
# unscanned — so this must FAIL.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = <<~MD
    # An entry

    ```code``` is how you write it inline.

    The provenance pin is `#{SHORT}`.
  MD
}
expect_fail(failures, "a backtick in a backtick fence's info string means no fence", out, status,
            "is the current provenance pin, restated outside a @bc3-pin span")

# A normal info string still opens a fence, so its contents stay code.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = <<~MD
    # An entry

    ```ruby
    The provenance pin is `#{SHORT}`.
    ```
  MD
}
expect_pass(failures, "a plain info string still opens a fence", out, status)

# Tilde fences carry no such restriction — backticks in their info are fine.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = <<~MD
    # An entry

    ~~~`weird`
    The provenance pin is `#{SHORT}`.
    ~~~
  MD
}
expect_pass(failures, "backticks in a tilde fence's info are allowed", out, status)

# Uppercase hex is the same revision. A lowercase-only scan waved it through.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = "# An entry\n\nThe provenance pin is #{SHORT.upcase}.\n"
}
expect_fail(failures, "an uppercase spelling of the pin still counts", out, status,
            "is the current provenance pin, restated outside a @bc3-pin span")

# An info string closes nothing — ```ruby inside a ``` block is content.
out, status = gate lambda { |f|
  f["spec/api-gaps/entry.md"] = <<~MD
    # An entry

    ```
    ```ruby
    The provenance pin is `#{SHORT}`.
    ```
  MD
}
expect_pass(failures, "a fence with an info string does not close", out, status)

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

# --- @fixture-categories (SPEC §19's Test Categories table) ---------------------
#
# The drift this catches happened three times in three shapes before the gate
# existed, so each shape gets a case. The gate is green on the repo as it
# stands only because Part 1 of this change backfilled the four missing rows;
# against the tree it landed on, both checks fail.

out, status = gate ->(f) { f["SPEC.md"] = f["SPEC.md"].sub("| beta-write | `beta_write.json` | §2 Something Else |\n", "") }
expect_fail(failures, "fixture with no category row", out, status, "missing: `beta_write.json`")

out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("<!-- @fixture-categories:end -->",
                                  "| gamma | `gamma.json` | §3 |\n<!-- @fixture-categories:end -->")
}
expect_fail(failures, "category row for an untracked file", out, status,
            "files git does not track under conformance/tests/: `gamma.json`")

out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("<!-- @fixture-categories:end -->",
                                  "| alpha | `alpha.json` | §1 again |\n<!-- @fixture-categories:end -->")
}
expect_fail(failures, "fixture categorised twice", out, status,
            "`alpha.json` is tabulated on more than one row")

# The slug rule is the half of the bijection a set comparison cannot see: both
# directions still match here, and only the category name is wrong.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("| beta-write | `beta_write.json` |", "| beta_write | `beta_write.json` |")
}
expect_fail(failures, "category slug disagrees with its filename", out, status,
            "`beta_write.json` dictates the slug `beta-write`")

# A row that satisfies PRESENCE and states NOTHING. Both PR bots flagged this
# on both tables: the claim is that every fixture has an owning spec section,
# and a blank attribution cell would let a new fixture through with exactly the
# thing the row exists to carry left empty — the gate reporting the claim kept
# while it is vacuous.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("| beta-write | `beta_write.json` | §2 Something Else |",
                                  "| beta-write | `beta_write.json` |  |")
}
expect_fail(failures, "category row with a blank owning section", out, status,
            "names no owning spec section")

# A raw pipe in the attribution shifts the real section into a FOURTH cell and
# leaves the fragment before it in cells[2] — where non-`§` attributions are
# legitimately allowed, so the gate would validate the wrong cell and never see
# the `§99` sitting in the actual section position. Accepting "at least three"
# is what made that silent; the row must fail on its shape.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("| beta-write | `beta_write.json` | §2 Something Else |",
                                  "| beta-write | `beta_write.json` | see A | B §99 |")
}
expect_fail(failures, "category row with a fourth cell from a raw pipe", out, status,
            "cell(s), not 3")

# Same shape on Appendix D, whose free-form test summaries are the likeliest
# place in the repo for someone to write `supports A | B`.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("| `beta_write.json` | does another thing | §2 |",
                                  "| `beta_write.json` | supports A | B | §99 |")
}
expect_fail(failures, "mapping row with a fourth cell from a raw pipe", out, status,
            "cell(s), not 3")

# Two DISTINCT fixtures deriving ONE category. `_` and `-` collapse to the same
# slug, so both rows satisfy the per-row slug rule and the file tally sees two
# different files — the table passes every check that looks at filenames while
# not being the bijection its own heading asserts. Adding the file to both
# rosters keeps the set comparisons satisfied, so the collision is the only
# thing left for the gate to catch.
out, status = gate lambda { |f|
  f["conformance/tests/beta-write.json"] = "[]\n"
  f["SPEC.md"] = f["SPEC.md"]
                 .sub("| beta-write | `beta_write.json` | §2 Something Else |\n",
                      "| beta-write | `beta_write.json` | §2 Something Else |\n" \
                      "| beta-write | `beta-write.json` | §2 Something Else |\n")
                 .sub("| `beta_write.json` | does another thing | §2 |\n",
                      "| `beta_write.json` | does another thing | §2 |\n" \
                      "| `beta-write.json` | does a third thing | §2 |\n")
}
expect_fail(failures, "two fixtures deriving one category slug", out, status,
            "category `beta-write` is dictated by")

# A pin restatement INSIDE a block span. The writer rewrites line spans only and
# the block checkers read nothing but the `|` rows, so an ordinary sentence
# parked in a roster block survives both untouched. If block bodies were dropped
# from the prose pool, this unmarked current-pin claim would be invisible to
# check_unmarked_pin and silently stale at the next repin — the gate's own span
# bookkeeping hiding the claim class it exists to catch.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("<!-- @fixture-categories:end -->",
                                  "\nRosters verified against `#{SHORT}`.\n" \
                                  "<!-- @fixture-categories:end -->")
}
expect_fail(failures, "unmarked pin citation inside a block span", out, status,
            SHORT)

out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("| `beta_write.json` | does another thing | §2 |",
                                  "| `beta_write.json` |  |  |")
}
expect_fail(failures, "Appendix D row with blank cells", out, status,
            "leaves Test name and Primary section empty")

# A section reference that resolves to no heading. Catches a typo, and more
# usefully a reference that resolved when written and stopped resolving when a
# section was renumbered — the case a reviewer of the same PR cannot see.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("| beta-write | `beta_write.json` | §2 Something Else |",
                                  "| beta-write | `beta_write.json` | §99 Renumbered Away |")
}
expect_fail(failures, "category row cites a section that does not exist", out, status,
            "owning section(s) §99 do not resolve")

out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("| `beta_write.json` | does another thing | §2 |",
                                  "| `beta_write.json` | does another thing | §99 |")
}
expect_fail(failures, "Appendix D row cites a section that does not exist", out, status,
            "primary section(s) §99 do not resolve")

# A document whose `## §N.` headings cannot be found gives the check nothing to
# resolve against, and "no sections defined" must not read as "every reference
# resolves" — the both-sides-empty shape, one table over.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("## §1. Something\n\n", "").sub("## §2. Something Else\n\n", "")
}
expect_fail(failures, "no section headings to resolve against", out, status,
            "no `## §N.` headings found")

# A row with NO section reference is deliberately still accepted — the real
# table has one (`live-my-surface.json`, attributed to external governance), and
# rejecting it would need a carve-out list rather than a rule.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("| `beta_write.json` | does another thing | §2 |",
                                  "| `beta_write.json` | does another thing | External governance |")
}
expect_pass(failures, "a row attributed to external governance is accepted", out, status)

# Git's pathspec `*` matches across `/`, but all six runners glob fixtures
# NON-RECURSIVELY. Demanding a roster row for a fixture nothing executes would
# be this gate requiring documentation for a claim that is not true — and the
# basename collapse would make `nested/alpha.json` and `alpha.json`
# indistinguishable, so a nested file could silently satisfy the row for a real
# one.
out, status = gate ->(f) { f["conformance/tests/nested/deep.json"] = "[]\n" }
expect_pass(failures, "nested fixtures are out of scope", out, status)
unless utf8(out).include?("fixtures         2 tracked")
  failures << "nested fixture must not be counted as tracked:\n#{utf8(out)}"
end

# A row the parser cannot read must be REPORTED, not filter_mapped away — the
# fail-open shape the fence handling exists to avoid, one table out.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("| alpha | `alpha.json` |", "| alpha | alpha.json |")
}
expect_fail(failures, "unparseable Files cell", out, status,
            "is not a single backticked fixture filename")

# Marker pair around the wrong lines: a span with no table at all reads as
# "nothing to check" unless the absence of a separator row is itself an error.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub(/<!-- @fixture-categories:begin -->.*<!-- @fixture-categories:end -->/m,
                                  "<!-- @fixture-categories:begin -->\nProse, no table.\n<!-- @fixture-categories:end -->")
}
expect_fail(failures, "marked span holds no table", out, status, "holds no Markdown table")

# Vacuity, both sides. These two are what make the pinned per-table count the
# bash template carries genuinely redundant rather than merely argued away.
#
# The reasoning for dropping it runs: markerCounts already guarantees the marked
# span EXISTS, and a bidirectional set comparison catches a deleted row, a bogus
# row, and a wholesale emptied table. That holds only if the comparison actually
# RUNS when one side parses to nothing. If an empty parse short-circuits —
# nothing to compare, so nothing to fail — then gutting the table between intact
# markers is silent, and the count would have caught precisely that. It is the
# same silent-empty failure as a scan that reads `map[string]string{` as an
# empty table, one level up: absence of parsed CONTENT must never be readable as
# absence of a CLAIM.

# Documented side emptied, markers and header left intact.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"]
                 .sub("| alpha | `alpha.json` | §1 Something |\n", "")
                 .sub("| beta-write | `beta_write.json` | §2 Something Else |\n", "")
}
expect_fail(failures, "categories table gutted between intact markers", out, status,
            "the @fixture-categories table has no data rows")

out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"]
                 .sub("| `alpha.json` | does a thing | §1 |\n", "")
                 .sub("| `beta_write.json` | does another thing | §2 |\n", "")
}
expect_fail(failures, "Appendix D gutted between intact markers", out, status,
            "the @fixture-section-map table has no data rows")

# Derived side emptied: `git ls-files conformance/tests/*.json` matching nothing
# is an extraction failure, not 22 pieces of drift — and emphatically not a
# vacuously satisfied comparison. Both fixtures are deleted, so every row would
# otherwise be reported as `extra` and the cause would be buried under symptoms.
out, status = gate lambda { |f|
  f.delete("conformance/tests/alpha.json")
  f.delete("conformance/tests/beta_write.json")
}
expect_fail(failures, "no tracked fixtures at all", out, status,
            "matched nothing, so this check has no source of truth")

# Both sides empty at once — the case where a naive set comparison is trivially
# satisfied and BOTH halves are wrong together.
out, status = gate lambda { |f|
  f.delete("conformance/tests/alpha.json")
  f.delete("conformance/tests/beta_write.json")
  f["SPEC.md"] = f["SPEC.md"]
                 .sub("| alpha | `alpha.json` | §1 Something |\n", "")
                 .sub("| beta-write | `beta_write.json` | §2 Something Else |\n", "")
                 .sub("| `alpha.json` | does a thing | §1 |\n", "")
                 .sub("| `beta_write.json` | does another thing | §2 |\n", "")
}
expect_fail(failures, "both sides empty is not agreement", out, status,
            "matched nothing, so this check has no source of truth")

# --- @fixture-section-map (SPEC Appendix D) -------------------------------------
#
# Coverage only, deliberately: Appendix D's rows bundle several cases each, so
# "one row per fixture" is not true of it and is not asserted. What IS asserted
# is that no fixture is absent and no row names a file that is not one.

out, status = gate ->(f) { f["SPEC.md"] = f["SPEC.md"].sub("| `alpha.json` | does a thing | §1 |\n", "") }
expect_fail(failures, "fixture absent from Appendix D", out, status,
            "no row maps these tracked fixtures to a primary section: `alpha.json`")

out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("<!-- @fixture-section-map:end -->",
                                  "| `gamma.json` | invented | §3 |\n<!-- @fixture-section-map:end -->")
}
expect_fail(failures, "Appendix D row for an untracked file", out, status,
            "rows name files git does not track under conformance/tests/: `gamma.json`")

# Two rows for one fixture is CORRECT here — uploads_write.json legitimately has
# four. A check that copied the categories table's bijection would reject the
# real repo.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"].sub("<!-- @fixture-section-map:end -->",
                                  "| `alpha.json` | a second theme | §1 |\n<!-- @fixture-section-map:end -->")
}
expect_pass(failures, "Appendix D allows several rows per fixture", out, status)

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

# Unmarking a roster is the cheapest way to silence it, so the inventory has to
# cover the two new kinds like every other. Without the count, deleting the
# marker pair leaves a table nothing checks and a gate that reports success.
out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"]
                 .sub("<!-- @fixture-categories:begin -->\n", "")
                 .sub("<!-- @fixture-categories:end -->\n", "")
}
expect_fail(failures, "fixture-categories markers deleted", out, status,
            "SPEC.md: spec/doc-constants.json declares 1 @fixture-categories marker(s), found 0")

out, status = gate lambda { |f|
  f["SPEC.md"] = f["SPEC.md"]
                 .sub("<!-- @fixture-section-map:begin -->\n", "")
                 .sub("<!-- @fixture-section-map:end -->\n", "")
}
expect_fail(failures, "fixture-section-map markers deleted", out, status,
            "SPEC.md: spec/doc-constants.json declares 1 @fixture-section-map marker(s), found 0")

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

# Same for the two fixture rosters: a row's owning-section attribution and case
# summary are human-authored, so the writer must leave a stale table alone and
# let `--check` fail. Repinning gives the writer real work to do in the same
# file, which is what makes "it left the tables alone" mean something.
writer lambda { |f|
  repin_to.call("c" * 40, "2026-09-09").call(f)
  f.delete("conformance/tests/beta_write.json")
} do |out, status, dir|
  expect_pass(failures, "writer ignores fixture-roster drift", out, status)
  written = read_in(dir, "SPEC.md")
  unless written.include?("| beta-write | `beta_write.json` | §2 Something Else |")
    failures << "writer: must not edit the fixture-categories table:\n#{written}"
  end
  unless written.include?("| `beta_write.json` | does another thing | §2 |")
    failures << "writer: must not edit the fixture-section-map table:\n#{written}"
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
