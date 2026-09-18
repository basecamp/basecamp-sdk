#!/usr/bin/env ruby
# frozen_string_literal: true

# Negative + synthetic self-test for scripts/check-required-tags.rb.
#
# The live gate (`make check-required-tags`) only ever runs against the real,
# all-tagged openapi.json, so its green run proves the check ACCEPTS a clean
# spec but nothing about whether it REJECTS anything. This drives the check from
# outside with crafted openapi documents in a tmpdir (via REQUIRED_TAGS_OPENAPI)
# and asserts the pass/fail verdict for each shape. The tracked openapi.json is
# never written to. Stdlib only.

require "json"
require "tmpdir"

CHECK = File.join(__dir__, "check-required-tags.rb")

def run_check(spec, allowlist: nil)
  Dir.mktmpdir do |dir|
    path = File.join(dir, "openapi.json")
    File.write(path, JSON.generate(spec))
    env = { "REQUIRED_TAGS_OPENAPI" => path }
    env["REQUIRED_TAGS_ALLOWLIST"] = allowlist if allowlist
    output = IO.popen(env, ["ruby", CHECK], err: [:child, :out], &:read)
    [$?.exitstatus, output]
  end
end

def op(tags)
  operation = { "operationId" => "SomeOp" }
  operation["tags"] = tags unless tags.nil?
  operation
end

def spec_with(operation)
  { "openapi" => "3.1.0", "paths" => { "/{accountId}/thing" => { "get" => operation } } }
end

FAILURES = []

def expect(desc, condition)
  if condition
    puts "  ok   #{desc}"
  else
    puts "  FAIL #{desc}"
    FAILURES << desc
  end
end

# 1. A clean, single-tag spec passes.
status, = run_check(spec_with(op(["Recordings"])))
expect("single-tag operation passes", status.zero?)

# 2. An operation with no tags key fails, and the message names it.
status, out = run_check(spec_with(op(nil)))
expect("missing tags key fails", status == 1)
expect("missing-tags failure names the operation", out.include?("SomeOp"))

# 3. An empty tags array fails (absent and empty are the same drift).
status, = run_check(spec_with(op([])))
expect("empty tags array fails", status == 1)

# 4. Two tags fail — catalog.Load requires exactly one.
status, out = run_check(spec_with(op(%w[Recordings Automation])))
expect("multi-tag operation fails", status == 1)
expect("multi-tag failure names both tags", out.include?("Recordings") && out.include?("Automation"))

# 5. An allowlisted untagged operation passes (the allowlist path works).
status, = run_check(spec_with(op(nil)), allowlist: "SomeOp")
expect("allowlisted untagged operation passes", status.zero?)

# 6. Fail-closed on a spec with no operations — a truncated openapi.json must
#    not pass vacuously.
status, = run_check({ "openapi" => "3.1.0", "paths" => { "/{accountId}/thing" => {} } })
expect("spec with no operations fails closed", status == 1)

# 7. Fail-closed on a spec with no paths at all.
status, = run_check({ "openapi" => "3.1.0" })
expect("spec with no paths fails closed", status == 1)

# 8. Non-HTTP keys under a path (e.g. shared parameters) are ignored, not
#    counted as untagged operations.
mixed = {
  "openapi" => "3.1.0",
  "paths" => {
    "/{accountId}/thing" => {
      "parameters" => [{ "name" => "accountId", "in" => "path" }],
      "get" => op(["Recordings"])
    }
  }
}
status, = run_check(mixed)
expect("path-level parameters are ignored", status.zero?)

# 9. The regression both reviewers found: a tagged GET beside an untagged HEAD.
#    A gate that enumerates verbs and misses HEAD never visits the second
#    operation, so this document passes and reports one operation.
tagged_get_untagged_head = {
  "openapi" => "3.1.0",
  "paths" => {
    "/{accountId}/thing" => {
      "get" => { "operationId" => "GetThing", "tags" => ["Recordings"] },
      "head" => { "operationId" => "HeadThing" }
    }
  }
}
status, out = run_check(tagged_get_untagged_head)
expect("untagged HEAD beside a tagged GET fails", status == 1)
expect("untagged-HEAD failure names the HEAD operation", out.include?("HeadThing"))

# 10. Every verb OpenAPI 3.1 names as a Path Item operation is checked.
%w[get put post delete options head patch trace].each do |verb|
  spec = {
    "openapi" => "3.1.0",
    "paths" => { "/{accountId}/thing" => { verb => { "operationId" => "Untagged#{verb.capitalize}" } } }
  }
  status, out = run_check(spec)
  expect("untagged #{verb.upcase} fails", status == 1 && out.include?("Untagged#{verb.capitalize}"))
end

# 11. The case a verb list cannot reach however carefully it is maintained: a
#     method nobody wrote down. `query` is real (OpenAPI 3.2 adds it) and
#     Smithy's @http trait would emit any string at all. This is why the check
#     identifies operations by what they are NOT.
%w[query purge notify].each do |verb|
  spec = {
    "openapi" => "3.1.0",
    "paths" => { "/{accountId}/thing" => { verb => { "operationId" => "Untagged#{verb.capitalize}" } } }
  }
  status, out = run_check(spec)
  expect("untagged #{verb.upcase} (verb not in any fixed list) fails", status == 1 && out.include?("Untagged#{verb.capitalize}"))
end

# 12. Each non-operation field the Path Item Object defines is skipped, not
#     mistaken for an untagged operation. All five together, beside a real one.
every_non_operation_field = {
  "openapi" => "3.1.0",
  "paths" => {
    "/{accountId}/thing" => {
      "$ref" => "#/components/pathItems/Thing",
      "summary" => "Thing",
      "description" => "A thing.",
      "servers" => [{ "url" => "https://example.com" }],
      "parameters" => [{ "name" => "accountId", "in" => "path" }],
      "x-vendor-note" => { "anything" => true },
      "get" => op(["Recordings"])
    }
  }
}
status, out = run_check(every_non_operation_field)
expect("non-operation path item fields are skipped", status.zero?)
expect("skipping them still counts the one real operation", out.include?("1 operations"))

# 13. A field that is neither a known non-operation field nor a readable
#     operation object is reported, not stepped over. This is the fail-closed
#     half of the inversion: the check says what it could not understand.
malformed = {
  "openapi" => "3.1.0",
  "paths" => { "/{accountId}/thing" => { "get" => op(["Recordings"]), "sideband" => "not an object" } }
}
status, out = run_check(malformed)
expect("an unreadable path item field fails", status == 1)
expect("unreadable-field failure names the field", out.include?("sideband"))

# 14. A path item that is not an object at all is reported rather than silently
#     contributing nothing.
status, out = run_check({ "openapi" => "3.1.0", "paths" => { "/{accountId}/thing" => "nonsense" } })
expect("a non-object path item fails", status == 1)
expect("non-object path item is named", out.include?("/{accountId}/thing"))

# 15. A tag has to name a domain. `[""]` satisfies "exactly one" while naming
#     nothing, and the generators do not even agree what it means: TypeScript
#     reads "" as falsy and files the operation under Miscellaneous, Ruby and
#     Python read it as truthy and derive a service from the empty string. So a
#     blank tag would not merely mis-file an operation, it would split the SDKs.
[[""], ["   "], ["\t"]].each do |tags|
  status, out = run_check(spec_with(op(tags)))
  expect("blank tag #{tags.inspect} fails", status == 1)
  expect("blank tag #{tags.inspect} failure names the operation", out.include?("SomeOp"))
end

# 16. A tag that is not a string names no domain either.
[[123], [nil], [{ "name" => "Recordings" }]].each do |tags|
  status, = run_check(spec_with(op(tags)))
  expect("non-string tag #{tags.inspect} fails", status == 1)
end

# 17. A tags value that is not an array at all is treated as untagged rather
#     than crashing the check.
status, = run_check(spec_with(op("Recordings")))
expect("a non-array tags value fails", status == 1)

# 18. Surrounding whitespace is not itself disqualifying — only the absence of
#     a name is. This pins the rule to "names a domain", not "is trimmed".
status, = run_check(spec_with(op([" Recordings "])))
expect("a tag with surrounding whitespace still passes", status.zero?)

if FAILURES.empty?
  puts "check-required-tags self-test: all cases passed"
else
  warn "check-required-tags self-test: #{FAILURES.length} case(s) failed"
  exit 1
end
