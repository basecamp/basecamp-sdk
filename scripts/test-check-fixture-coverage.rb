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

def run_synthetic(openapi:, manifest:)
  Dir.mktmpdir("fixture-cov-synthetic") do |dir|
    op = File.join(dir, "openapi.json")
    mf = File.join(dir, "manifest.yaml")
    File.write(op, JSON.generate(openapi))
    File.write(mf, YAML.dump(manifest))
    Timeout.timeout(30) { run_checker(manifest: mf, openapi: op, fixtures: dir) }
  end
end

begin
  out, status = run_synthetic(openapi: SYNTHETIC_OPENAPI, manifest: EMPTY_MANIFEST)
  # (1) + (2): both indirect emitters must be discovered and flagged as unaccounted.
  expect_fail(failures, "component-level allOf emitter discovered", out, status, "`AllOfEmitter`")
  expect_fail(failures, "aliased RichTextAttachment item discovered", out, status, "`AliasedItemEmitter`")
  # (3): the cycle components must NOT be misclassified as emitters, and — proven
  # by the run returning at all under Timeout — discovery terminated safely.
  if out.include?("`CycleA`") || out.include?("`SelfRefItem`") || out.include?("`CycleItemHolder`")
    failures << "cycle components should not be flagged as emitters:\n#{out}"
  end
rescue Timeout::Error
  failures << "emitter discovery did not terminate on a reference/composition cycle (hung)"
end

# --- Report --------------------------------------------------------------------

if failures.empty?
  puts "==> Fixture-coverage self-test passed — 1 positive + 8 negative + 3 synthetic emitter cases"
  exit 0
else
  warn "Fixture-coverage self-test FAILED:"
  failures.each { |f| warn "  - #{f}" }
  exit 1
end
