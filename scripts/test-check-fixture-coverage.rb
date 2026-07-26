#!/usr/bin/env ruby
# frozen_string_literal: true

# Negative-case self-test for scripts/check-fixture-coverage.rb.
#
# The guard's own `make check` run only exercises the VALID manifest. This test
# crafts each failure mode and asserts the guard rejects it (non-zero exit + an
# expected message fragment), driving the checker through its FIXTURE_MANIFEST /
# FIXTURE_OPENAPI / FIXTURE_DIR env overrides against tmp inputs. It also
# confirms the real manifest still passes (positive control).
#
# Run directly (`ruby scripts/test-check-fixture-coverage.rb`) or via
# `make check-fixture-coverage` (which runs it after the live check).

require "json"
require "yaml"
require "tmpdir"
require "fileutils"
require "open3"
require "timeout"

ROOT = File.expand_path("..", __dir__)
CHECKER = File.join(__dir__, "check-fixture-coverage.rb")
REAL_OPENAPI = File.join(ROOT, "openapi.json")
REAL_FIXTURES = File.join(ROOT, "spec/fixtures")
REAL_MANIFEST = File.join(REAL_FIXTURES, "manifest.yaml")

def read_utf8(path) = File.read(path, encoding: "UTF-8")

# Run the checker with env overrides; returns [combined_output, status].
def run_checker(manifest:, openapi: REAL_OPENAPI, fixtures: REAL_FIXTURES)
  env = {
    "FIXTURE_MANIFEST" => manifest,
    "FIXTURE_OPENAPI" => openapi,
    "FIXTURE_DIR" => fixtures,
  }
  Open3.capture2e(env, "ruby", CHECKER)
end

# Like run_checker but spawns with a retained PID so a hung child can actually be
# killed on timeout (Open3.capture2e's cleanup would block waiting for the
# child). Raises Timeout::Error after `seconds`, having KILLed and reaped the
# process. Returns [combined_output, Process::Status].
def run_checker_killable(manifest:, openapi:, fixtures:, seconds: 30)
  env = { "FIXTURE_MANIFEST" => manifest, "FIXTURE_OPENAPI" => openapi, "FIXTURE_DIR" => fixtures }
  reader, writer = IO.pipe
  pid = Process.spawn(env, "ruby", CHECKER, out: writer, err: writer)
  writer.close
  out = +""
  pump = Thread.new { out = reader.read }
  status = nil
  begin
    Timeout.timeout(seconds) { _, status = Process.wait2(pid) }
  rescue Timeout::Error
    # Timeout can fire just after wait2 already reaped the child, so the kill/
    # reap may hit an already-gone process — tolerate ESRCH/ECHILD.
    begin
      Process.kill("KILL", pid)
      Process.wait(pid)
    rescue Errno::ESRCH, Errno::ECHILD
      # already exited/reaped
    end
    raise
  ensure
    pump.join
    reader.close
  end
  [out, status]
end

failures = []

def expect_pass(failures, label, out, status)
  return if status.success?

  failures << "#{label}: expected PASS but checker failed:\n#{out}"
end

def expect_fail(failures, label, out, status, fragment)
  if status.success?
    failures << "#{label}: expected FAILURE but checker passed:\n#{out}"
  elsif !out.include?(fragment)
    failures << "#{label}: failed as expected but message missing #{fragment.inspect}:\n#{out}"
  end
end

# A tmp copy of the real fixtures tree, with one fixture file mutated in place.
def with_mutated_fixture(rel)
  Dir.mktmpdir("fixture-cov-test") do |dir|
    fixtures = File.join(dir, "fixtures")
    FileUtils.cp_r(REAL_FIXTURES, fixtures)
    path = File.join(fixtures, rel)
    data = JSON.parse(read_utf8(path))
    yield data # mutate in place
    File.write(path, JSON.generate(data))
    # Point the manifest's fixture lookups at the mutated tree; reuse the real
    # manifest (paths are relative to FIXTURE_DIR).
    out, status = run_checker(manifest: File.join(fixtures, "manifest.yaml"), fixtures: fixtures)
    [out, status]
  end
end

# A tmp manifest (built from the real one, then mutated) against real fixtures.
def with_mutated_manifest
  manifest = YAML.safe_load(read_utf8(REAL_MANIFEST))
  yield manifest
  Dir.mktmpdir("fixture-cov-test") do |dir|
    path = File.join(dir, "manifest.yaml")
    File.write(path, YAML.dump(manifest))
    run_checker(manifest: path)
  end
end

# --- Positive control ----------------------------------------------------------

out, status = run_checker(manifest: REAL_MANIFEST)
expect_pass(failures, "real manifest passes", out, status)

# --- Fixture-content mutations -------------------------------------------------

out, status = with_mutated_fixture("comments/get.json") { |d| d["id"] = "not-a-number" }
expect_fail(failures, "required field wrong type", out, status, "expected integer, got string")

out, status = with_mutated_fixture("comments/get.json") { |d| d["id"] = nil }
expect_fail(failures, "required field null", out, status, "required field is null")

out, status = with_mutated_fixture("comments/get.json") { |d| d["id"] = 1.5 }
expect_fail(failures, "non-integral float for integer", out, status, "expected integer, got number")

out, status = with_mutated_fixture("comments/get.json") { |d| d["content_attachments"] = [nil] }
expect_fail(failures, "null array element", out, status, "null array element")

# --- Manifest mutations --------------------------------------------------------

out, status = with_mutated_manifest do |m|
  t = m["targets"].find { |e| e["id"] == "richtext-comment-0" }
  t["pointer"] = "/content_attachments/01"
end
expect_fail(failures, "RFC 6901 leading-zero pointer", out, status, "does not resolve")

out, status = with_mutated_manifest do |m|
  t = m["targets"].find { |e| e["id"] == "richtext-comment-0" }
  t["pointer"] = "/content_attachments/~2"
end
expect_fail(failures, "RFC 6901 bad escape", out, status, "does not resolve")

out, status = with_mutated_manifest do |m|
  m["covered_schemas"].delete("Todo")
  m["targets"].reject! { |e| e["id"] == "todo-get" }
end
expect_fail(failures, "dropped emitter (self-erasing)", out, status, "neither in covered_schemas")

out, status = with_mutated_manifest do |m|
  t = m["targets"].find { |e| e["id"] == "todo-get" }
  t.delete("schema")
  t["operation"] = "GetTodo"
  t["pointer"] = "/id"
end
expect_fail(failures, "operation entry with non-root pointer", out, status, "non-root")

# --- Synthetic emitter-discovery cases -----------------------------------------
#
# Exercise indirect rich-text-emitter shapes against a crafted OpenAPI (no
# targets, so no fixtures are read): the completeness check must still discover
# emitters declared through component-level composition and aliased array items,
# and must traverse reference/composition cycles without hanging.

SYNTHETIC_OPENAPI = {
  "components" => { "schemas" => {
    "RichTextAttachment" => { "type" => "object", "properties" => { "id" => { "type" => "integer" } } },
    # A whole-component alias of RichTextAttachment (an aliased item target).
    "RTAAlias" => { "$ref" => "#/components/schemas/RichTextAttachment" },
    "Base" => { "type" => "object", "properties" => { "title" => { "type" => "string" } } },
    # (1) Emitter whose companion array arrives via COMPONENT-LEVEL allOf.
    "AllOfEmitter" => { "allOf" => [
      { "$ref" => "#/components/schemas/Base" },
      { "type" => "object",
        "properties" => { "content_attachments" => { "type" => "array",
                                                     "items" => { "$ref" => "#/components/schemas/RichTextAttachment" } } } },
    ] },
    # (2) Emitter whose array ITEMS reference RichTextAttachment through an alias.
    "AliasedItemEmitter" => { "type" => "object",
                              "properties" => { "description_attachments" => { "type" => "array",
                                                                              "items" => { "$ref" => "#/components/schemas/RTAAlias" } } } },
    # (2b) Emitter declared as a $ref WITH sibling local properties (OpenAPI 3.1)
    # — the local content_attachments must be seen despite the $ref.
    "SiblingEmitter" => { "$ref" => "#/components/schemas/Base",
                          "properties" => { "content_attachments" => { "type" => "array",
                                                                       "items" => { "$ref" => "#/components/schemas/RichTextAttachment" } } } },
    # (2c) Emitter whose array `type` and `items` are split across allOf
    # conjuncts — only the EFFECTIVE (merged) view sees the array-of-RTA.
    "SplitEmitter" => { "type" => "object",
                        "properties" => { "atts" => { "allOf" => [
                          { "type" => "array" },
                          { "items" => { "$ref" => "#/components/schemas/RichTextAttachment" } },
                        ] } } },
    # (2d) Enclosing conjunction supplies `type: array`; an anyOf branch supplies
    # the RTA items. Only preserving the enclosing type while evaluating each
    # alternative sees the array-of-RTA.
    "EnclosingArrayEmitter" => { "type" => "object", "properties" => { "atts" => { "allOf" => [
      { "type" => "array" },
      { "anyOf" => [{ "items" => { "$ref" => "#/components/schemas/RichTextAttachment" } },
                    { "items" => { "type" => "string" } }] },
    ] } } },
    # (2e) Symmetric: enclosing conjunction supplies the RTA items; an anyOf
    # branch supplies `type: array`.
    "EnclosingItemsEmitter" => { "type" => "object", "properties" => { "atts" => { "allOf" => [
      { "items" => { "$ref" => "#/components/schemas/RichTextAttachment" } },
      { "anyOf" => [{ "type" => "array" }, { "type" => "object" }] },
    ] } } },
    # (3a) Component-level composition cycle (no rich text) — must terminate.
    "CycleA" => { "allOf" => [{ "$ref" => "#/components/schemas/CycleB" }] },
    "CycleB" => { "allOf" => [{ "$ref" => "#/components/schemas/CycleA" }] },
    # (3b) A self-referential item alias reached from an array — must terminate.
    "SelfRefItem" => { "allOf" => [{ "$ref" => "#/components/schemas/SelfRefItem" }] },
    "CycleItemHolder" => { "type" => "object",
                           "properties" => { "things" => { "type" => "array",
                                                          "items" => { "$ref" => "#/components/schemas/SelfRefItem" } } } },
  } },
}.freeze

EMPTY_MANIFEST = { "targets" => [], "covered_schemas" => {}, "excluded_schemas" => {} }.freeze

# A composed COVERED target: ComposedTarget's required fields (base_field, extra)
# live in $ref + allOf branches, and it is a rich-text emitter. An empty-object
# fixture must fail composition-aware required validation.
COMPOSED_OPENAPI = {
  "components" => { "schemas" => {
    "RichTextAttachment" => { "type" => "object", "properties" => { "id" => { "type" => "integer" } } },
    "Base" => { "type" => "object", "required" => ["base_field"],
                "properties" => { "base_field" => { "type" => "string" } } },
    "ComposedTarget" => { "allOf" => [
      { "$ref" => "#/components/schemas/Base" },
      { "type" => "object", "required" => ["extra"],
        "properties" => { "extra" => { "type" => "string" },
                          "content_attachments" => { "type" => "array",
                                                    "items" => { "$ref" => "#/components/schemas/RichTextAttachment" } } } },
    ] },
  } },
}.freeze

COMPOSED_MANIFEST = {
  "targets" => [{ "id" => "composed", "fixture" => "composed.json", "pointer" => "", "schema" => "ComposedTarget" }],
  "covered_schemas" => { "ComposedTarget" => ["composed"] },
  "excluded_schemas" => {},
}.freeze

# Writes openapi + manifest (+ optional fixture files) to a tmp tree and runs the
# checker there, killably (so a cycle that hangs discovery is terminated, not
# left blocking the suite).
def run_synthetic(openapi:, manifest:, fixtures: {})
  Dir.mktmpdir("fixture-cov-synthetic") do |dir|
    op = File.join(dir, "openapi.json")
    mf = File.join(dir, "manifest.yaml")
    fx = File.join(dir, "fixtures")
    FileUtils.mkdir_p(fx)
    File.write(op, JSON.generate(openapi))
    File.write(mf, YAML.dump(manifest))
    fixtures.each do |rel, data|
      p = File.join(fx, rel)
      FileUtils.mkdir_p(File.dirname(p))
      File.write(p, JSON.generate(data))
    end
    run_checker_killable(manifest: mf, openapi: op, fixtures: fx)
  end
end

begin
  out, status = run_synthetic(openapi: SYNTHETIC_OPENAPI, manifest: EMPTY_MANIFEST)
  # Indirect emitters must all be discovered and flagged as unaccounted.
  expect_fail(failures, "component-level allOf emitter discovered", out, status, "`AllOfEmitter`")
  expect_fail(failures, "aliased RichTextAttachment item discovered", out, status, "`AliasedItemEmitter`")
  expect_fail(failures, "$ref-with-siblings emitter discovered", out, status, "`SiblingEmitter`")
  expect_fail(failures, "split type/items allOf emitter discovered", out, status, "`SplitEmitter`")
  expect_fail(failures, "enclosing-array + anyOf-items emitter discovered", out, status, "`EnclosingArrayEmitter`")
  expect_fail(failures, "enclosing-items + anyOf-array emitter discovered", out, status, "`EnclosingItemsEmitter`")
  # The cycle components must NOT be misclassified as emitters, and — proven by
  # the run returning at all under the killable timeout — discovery terminated.
  if out.include?("`CycleA`") || out.include?("`SelfRefItem`") || out.include?("`CycleItemHolder`")
    failures << "cycle components should not be flagged as emitters:\n#{out}"
  end
rescue Timeout::Error
  failures << "emitter discovery did not terminate on a reference/composition cycle (hung)"
end

# Composition-aware required validation: an empty fixture for a composed covered
# target must fail on the required fields contributed by its $ref/allOf branches.
begin
  out, status = run_synthetic(openapi: COMPOSED_OPENAPI, manifest: COMPOSED_MANIFEST,
                              fixtures: { "composed.json" => {} })
  expect_fail(failures, "composed covered target with empty fixture fails", out, status, "missing required field")
  %w[base_field extra].each do |f|
    failures << "composed-target test did not report missing `#{f}`:\n#{out}" unless out.include?("`#{f}`")
  end
rescue Timeout::Error
  failures << "composed-target validation hung"
end

# Alternative-group (anyOf/oneOf) validation, with the group inherited through
# BOTH allOf and $ref (ComposedAlt -> allOf -> $ref AltBase -> oneOf). A value
# must satisfy at least one branch; an empty object satisfies neither.
ALT_OPENAPI = {
  "components" => { "schemas" => {
    "AltBase" => { "oneOf" => [
      { "type" => "object", "required" => ["a"], "properties" => { "a" => { "type" => "string" } } },
      { "type" => "object", "required" => ["b"], "properties" => { "b" => { "type" => "string" } } },
    ] },
    "ComposedAlt" => { "allOf" => [{ "$ref" => "#/components/schemas/AltBase" }] },
  } },
}.freeze

def alt_manifest
  { "targets" => [{ "id" => "alt", "fixture" => "alt.json", "pointer" => "", "schema" => "ComposedAlt" }],
    "covered_schemas" => { "ComposedAlt" => ["alt"] },
    "excluded_schemas" => {} }
end

begin
  out, status = run_synthetic(openapi: ALT_OPENAPI, manifest: alt_manifest, fixtures: { "alt.json" => {} })
  expect_fail(failures, "anyOf/oneOf: empty object matches no alternative", out, status, "matches none of")

  out, status = run_synthetic(openapi: ALT_OPENAPI, manifest: alt_manifest, fixtures: { "alt.json" => { "a" => "x" } })
  expect_pass(failures, "anyOf/oneOf: value matching a branch passes", out, status)
rescue Timeout::Error
  failures << "alternative-group validation hung"
end

# allOf nullability is a conjunction: a required field whose schema is
# allOf[nullable-branch, non-nullable-branch] must reject null (one branch
# forbids it), not accept it because a single branch is nullable.
NULLABLE_ALLOF_OPENAPI = {
  "components" => { "schemas" => {
    "NullAllOfTarget" => { "type" => "object", "required" => ["f"], "properties" => {
      "f" => { "allOf" => [
        { "type" => "string", "nullable" => true },
        { "type" => "string" },
      ] },
    } },
  } },
}.freeze

def null_allof_manifest
  { "targets" => [{ "id" => "nallof", "fixture" => "nallof.json", "pointer" => "", "schema" => "NullAllOfTarget" }],
    "covered_schemas" => { "NullAllOfTarget" => ["nallof"] },
    "excluded_schemas" => {} }
end

begin
  out, status = run_synthetic(openapi: NULLABLE_ALLOF_OPENAPI, manifest: null_allof_manifest,
                              fixtures: { "nallof.json" => { "f" => nil } })
  expect_fail(failures, "allOf nullability is conjunctive (null rejected)", out, status,
              "required field is null but its schema is not nullable")

  out, status = run_synthetic(openapi: NULLABLE_ALLOF_OPENAPI, manifest: null_allof_manifest,
                              fixtures: { "nallof.json" => { "f" => "x" } })
  expect_pass(failures, "allOf field with a concrete value passes", out, status)
rescue Timeout::Error
  failures << "allOf-nullability validation hung"
end

# anyOf/oneOf nullability: a required field typed only by non-null alternatives
# must REJECT null (no branch permits it); an explicitly-nullable alternative
# must ACCEPT it. This is the path {}-fails / matching-object-passes can't reach.
ALT_NULL_OPENAPI = {
  "components" => { "schemas" => {
    "AltNullReject" => { "type" => "object", "required" => ["f"], "properties" => {
      "f" => { "oneOf" => [{ "type" => "string" }, { "type" => "integer" }] },
    } },
    "AltNullAccept" => { "type" => "object", "required" => ["f"], "properties" => {
      "f" => { "oneOf" => [{ "type" => "string", "nullable" => true }, { "type" => "integer" }] },
    } },
  } },
}.freeze

def alt_null_manifest(schema, id)
  { "targets" => [{ "id" => id, "fixture" => "#{id}.json", "pointer" => "", "schema" => schema }],
    "covered_schemas" => { schema => [id] },
    "excluded_schemas" => {} }
end

begin
  out, status = run_synthetic(openapi: ALT_NULL_OPENAPI, manifest: alt_null_manifest("AltNullReject", "reject"),
                              fixtures: { "reject.json" => { "f" => nil } })
  expect_fail(failures, "anyOf/oneOf: non-null alternatives reject required null", out, status,
              "required field is null but its schema is not nullable")

  out, status = run_synthetic(openapi: ALT_NULL_OPENAPI, manifest: alt_null_manifest("AltNullAccept", "accept"),
                              fixtures: { "accept.json" => { "f" => nil } })
  expect_pass(failures, "anyOf/oneOf: an explicitly-nullable alternative accepts null", out, status)
rescue Timeout::Error
  failures << "alternative-nullability validation hung"
end

# allOf type constraints INTERSECT: a property declared in two allOf branches
# with conflicting types must satisfy both. A value matching only one branch
# fails (union would have wrongly accepted it).
INTERSECT_OPENAPI = {
  "components" => { "schemas" => {
    "IntersectTarget" => { "allOf" => [
      { "type" => "object", "required" => ["f"], "properties" => { "f" => { "type" => "string" } } },
      { "type" => "object", "properties" => { "f" => { "type" => "integer" } } },
    ] },
  } },
}.freeze

begin
  out, status = run_synthetic(
    openapi: INTERSECT_OPENAPI,
    manifest: { "targets" => [{ "id" => "isect", "fixture" => "isect.json", "pointer" => "", "schema" => "IntersectTarget" }],
                "covered_schemas" => { "IntersectTarget" => ["isect"] }, "excluded_schemas" => {} },
    fixtures: { "isect.json" => { "f" => "x" } },
  )
  expect_fail(failures, "allOf type constraints intersect (conflicting branch)", out, status, "expected integer, got string")
rescue Timeout::Error
  failures << "allOf type-intersection validation hung"
end

# allOf conjoins array `items` from every branch: an element must satisfy all
# item schemas, not just the first branch's.
ITEMS_ALLOF_OPENAPI = {
  "components" => { "schemas" => {
    "ItemsTarget" => { "allOf" => [
      { "type" => "object", "required" => ["xs"],
        "properties" => { "xs" => { "type" => "array", "items" => { "type" => "string" } } } },
      { "type" => "object",
        "properties" => { "xs" => { "type" => "array", "items" => { "type" => "integer" } } } },
    ] },
  } },
}.freeze

begin
  out, status = run_synthetic(
    openapi: ITEMS_ALLOF_OPENAPI,
    manifest: { "targets" => [{ "id" => "items", "fixture" => "items.json", "pointer" => "", "schema" => "ItemsTarget" }],
                "covered_schemas" => { "ItemsTarget" => ["items"] }, "excluded_schemas" => {} },
    fixtures: { "items.json" => { "xs" => ["hello"] } },
  )
  expect_fail(failures, "allOf conjoins array items across branches", out, status, "expected integer, got string")
rescue Timeout::Error
  failures << "allOf items-conjoin validation hung"
end

# OpenAPI 3.1 scalar `type: "null"`: a required field so typed accepts null.
NULL_TYPE_OPENAPI = {
  "components" => { "schemas" => {
    "NullTypeTarget" => { "type" => "object", "required" => ["n"], "properties" => { "n" => { "type" => "null" } } },
  } },
}.freeze

begin
  out, status = run_synthetic(
    openapi: NULL_TYPE_OPENAPI,
    manifest: { "targets" => [{ "id" => "nt", "fixture" => "nt.json", "pointer" => "", "schema" => "NullTypeTarget" }],
                "covered_schemas" => { "NullTypeTarget" => ["nt"] }, "excluded_schemas" => {} },
    fixtures: { "nt.json" => { "n" => nil } },
  )
  expect_pass(failures, 'scalar type:"null" required field accepts null', out, status)

  out, status = run_synthetic(
    openapi: NULL_TYPE_OPENAPI,
    manifest: { "targets" => [{ "id" => "nt", "fixture" => "nt.json", "pointer" => "", "schema" => "NullTypeTarget" }],
                "covered_schemas" => { "NullTypeTarget" => ["nt"] }, "excluded_schemas" => {} },
    fixtures: { "nt.json" => { "n" => "not-null" } },
  )
  expect_fail(failures, 'scalar type:"null" rejects a non-null value', out, status, "expected null, got string")
rescue Timeout::Error
  failures << "null-type validation hung"
end

# OpenAPI 3.1 union `type: ["null"]` (array form with only null) must keep its
# null-only constraint too — a non-null value fails, null passes.
NULL_ARRAY_TYPE_OPENAPI = {
  "components" => { "schemas" => {
    "NullArrTarget" => { "type" => "object", "required" => ["n"], "properties" => { "n" => { "type" => ["null"] } } },
  } },
}.freeze

begin
  m = { "targets" => [{ "id" => "na", "fixture" => "na.json", "pointer" => "", "schema" => "NullArrTarget" }],
        "covered_schemas" => { "NullArrTarget" => ["na"] }, "excluded_schemas" => {} }
  out, status = run_synthetic(openapi: NULL_ARRAY_TYPE_OPENAPI, manifest: m, fixtures: { "na.json" => { "n" => nil } })
  expect_pass(failures, 'type:["null"] required field accepts null', out, status)

  out, status = run_synthetic(openapi: NULL_ARRAY_TYPE_OPENAPI, manifest: m, fixtures: { "na.json" => { "n" => "x" } })
  expect_fail(failures, 'type:["null"] rejects a non-null value', out, status, "expected null, got string")
rescue Timeout::Error
  failures << "null-array-type validation hung"
end

# --- Report --------------------------------------------------------------------

if failures.empty?
  puts "==> Fixture-coverage self-test passed — 1 positive + 8 negative + 20 synthetic cases"
  exit 0
else
  warn "Fixture-coverage self-test FAILED:"
  failures.each { |f| warn "  - #{f}" }
  exit 1
end
