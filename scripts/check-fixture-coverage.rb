#!/usr/bin/env ruby
# frozen_string_literal: true

# Fixture-completeness guard.
#
# Validates spec/fixtures/manifest.yaml:
#   1. Every `targets` entry resolves — fixture exists, JSON pointer selects a
#      concrete non-null instance, the named schema is a real component — and
#      the selected instance carries every required field of its schema (the
#      recursive $ref/array/required walk, reused from the conformance walker).
#   2. The coverage invariant: every schema in `covered_schemas` has >= 1 active
#      target that resolves to a concrete, non-null instance whose declared root
#      schema is that component (the concrete-instance rule — transitive
#      reachability and empty arrays do not count).
#   3. `excluded_schemas` entries each carry a reason + tracking issue, name real
#      components, and do not overlap `covered_schemas`.
#
# Reuses conformance/runner/ruby/schema-walker.rb for validation — the required
# walk is not reimplemented here. Wired into `make check` in the
# scripts/validate-api-gaps.rb style (stdlib only).

require "json"
require "yaml"

require_relative "../conformance/runner/ruby/schema-walker"

PROJECT_ROOT = File.expand_path("..", __dir__)
MANIFEST_FILE = File.join(PROJECT_ROOT, "spec/fixtures/manifest.yaml")
FIXTURES_DIR = File.join(PROJECT_ROOT, "spec/fixtures")
OPENAPI_FILE = File.join(PROJECT_ROOT, "openapi.json")

errors = []

def fail_with(errors, message)
  errors << message
end

# RFC 6901 JSON pointer resolution. Returns [value, found]. "" selects the whole
# document. Missing keys / out-of-range indices / non-container traversal ->
# [nil, false].
def resolve_pointer(doc, pointer)
  return [doc, true] if pointer.nil? || pointer.empty?
  return [nil, false] unless pointer.start_with?("/")

  current = doc
  pointer.split("/", -1).drop(1).each do |raw|
    token = raw.gsub("~1", "/").gsub("~0", "~")
    case current
    when Hash
      return [nil, false] unless current.key?(token)

      current = current[token]
    when Array
      return [nil, false] unless token.match?(/\A\d+\z/)

      idx = token.to_i
      return [nil, false] if idx >= current.length

      current = current[idx]
    else
      return [nil, false]
    end
  end
  [current, true]
end

unless File.file?(MANIFEST_FILE)
  warn "ERROR: manifest not found at #{MANIFEST_FILE}"
  exit 2
end
unless File.file?(OPENAPI_FILE)
  warn "ERROR: openapi.json not found at #{OPENAPI_FILE}"
  exit 2
end

manifest = YAML.safe_load(File.read(MANIFEST_FILE))
walker = Basecamp::Conformance::SchemaWalker.new(OPENAPI_FILE)
openapi = JSON.parse(File.read(OPENAPI_FILE))
components = openapi.dig("components", "schemas") || {}

targets = manifest["targets"] || []
covered = manifest["covered_schemas"] || {}
excluded = manifest["excluded_schemas"] || {}

# Fixture cache so a fixture referenced by several targets is read once.
fixture_cache = {}
load_fixture = lambda do |rel|
  fixture_cache[rel] ||= begin
    path = File.join(FIXTURES_DIR, rel)
    File.file?(path) ? JSON.parse(File.read(path)) : :missing
  end
end

# --- 1. Targets ----------------------------------------------------------------

by_id = {}
# Records, per target id, the concrete root schema it resolved to (or nil) so
# the coverage pass can apply the concrete-instance rule without re-resolving.
resolved_root = {}

targets.each_with_index do |entry, i|
  id = entry["id"]
  if id.nil? || id.to_s.empty?
    fail_with(errors, "targets[#{i}]: missing `id`")
    next
  end
  if by_id.key?(id)
    fail_with(errors, "target `#{id}`: duplicate id")
    next
  end
  by_id[id] = entry

  operation = entry["operation"]
  fixture_rel = entry["fixture"]
  pointer = entry["pointer"] || ""
  schema_name = entry["schema"]

  if operation && (schema_name || !fixture_rel)
    # operation entry
    if schema_name
      fail_with(errors, "target `#{id}`: `operation` and `schema` are mutually exclusive")
      next
    end
  end

  # Locate the schema to validate against.
  schema =
    if operation
      s = walker.find_response_schema(operation)
      unless s
        fail_with(errors, "target `#{id}`: no 2xx/default response schema for operation `#{operation}`")
        next
      end
      s
    else
      unless schema_name
        fail_with(errors, "target `#{id}`: pointer entry needs an explicit `schema`")
        next
      end
      unless components.key?(schema_name)
        fail_with(errors, "target `#{id}`: unknown component schema `#{schema_name}`")
        next
      end
      { "$ref" => "#/components/schemas/#{schema_name}" }
    end

  # Load fixture (operation entries default pointer "").
  unless fixture_rel
    fail_with(errors, "target `#{id}`: missing `fixture`")
    next
  end
  doc = load_fixture.call(fixture_rel)
  if doc == :missing
    fail_with(errors, "target `#{id}`: fixture not found: spec/fixtures/#{fixture_rel}")
    next
  end

  instance, found = resolve_pointer(doc, pointer)
  unless found
    fail_with(errors, "target `#{id}`: JSON pointer `#{pointer}` does not resolve in #{fixture_rel}")
    next
  end
  if instance.nil?
    fail_with(errors, "target `#{id}`: pointer `#{pointer}` resolves to null (not a concrete instance) in #{fixture_rel}")
    next
  end

  # Record the declared root schema for the coverage pass. For pointer entries
  # it is the named component; for operation entries, the top-level $ref name.
  resolved_root[id] =
    if schema_name
      { name: schema_name, concrete: instance.is_a?(Hash) }
    else
      root = schema.is_a?(Hash) ? schema["$ref"] : nil
      m = root&.match(%r{/components/schemas/(.+)\z})
      { name: m && m[1], concrete: instance.is_a?(Hash) }
    end

  missing = walker.missing_required(instance, schema)
  next if missing.empty?

  where = pointer.empty? ? fixture_rel : "#{fixture_rel} at pointer `#{pointer}`"
  missing.each do |path|
    fail_with(errors, "target `#{id}`: #{where} missing required field `#{path}`")
  end
end

# --- 2. Coverage invariant -----------------------------------------------------

covered.each do |schema_name, ids|
  unless components.key?(schema_name)
    fail_with(errors, "covered_schemas: unknown component schema `#{schema_name}`")
    next
  end
  ids = Array(ids)
  if ids.empty?
    fail_with(errors, "covered_schemas[`#{schema_name}`]: no target ids listed")
    next
  end

  concrete_rep = false
  ids.each do |id|
    unless by_id.key?(id)
      fail_with(errors, "covered_schemas[`#{schema_name}`]: references unknown target `#{id}`")
      next
    end
    root = resolved_root[id]
    next unless root # target failed to resolve; already reported above

    if root[:name] == schema_name && root[:concrete]
      concrete_rep = true
    end
  end

  unless concrete_rep
    fail_with(errors,
      "covered_schemas[`#{schema_name}`]: no active target resolves to a concrete, " \
      "non-null instance declared as `#{schema_name}` (concrete-instance rule)")
  end
end

# --- 3. Exclusions -------------------------------------------------------------

excluded.each do |schema_name, meta|
  unless components.key?(schema_name)
    fail_with(errors, "excluded_schemas: unknown component schema `#{schema_name}`")
  end
  if covered.key?(schema_name)
    fail_with(errors, "excluded_schemas[`#{schema_name}`]: schema is also in covered_schemas")
  end
  meta ||= {}
  reason = meta["reason"]
  if reason.nil? || reason.to_s.strip.empty?
    fail_with(errors, "excluded_schemas[`#{schema_name}`]: missing `reason`")
  end
  issue = meta["issue"]
  if issue.nil? || issue.to_s.strip.empty?
    fail_with(errors, "excluded_schemas[`#{schema_name}`]: missing tracking `issue`")
  end
end

# --- Report --------------------------------------------------------------------

if errors.empty?
  covered_n = covered.size
  target_n = targets.size
  excluded_n = excluded.size
  puts "==> Fixture coverage clean — #{covered_n} covered schemas, " \
       "#{target_n} manifest targets, #{excluded_n} tracked exclusions"
  exit 0
else
  warn "Fixture-coverage check failed:"
  errors.sort.each { |e| warn "  - #{e}" }
  exit 1
end
