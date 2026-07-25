#!/usr/bin/env ruby
# frozen_string_literal: true

# Fixture-completeness guard.
#
# Validates spec/fixtures/manifest.yaml:
#   1. Every `targets` entry resolves — fixture exists, JSON pointer selects a
#      concrete instance, the named schema is a real component — and the selected
#      instance both (a) carries every required field of its schema (the
#      recursive $ref/array/required walk, reused from the conformance walker)
#      and (b) matches the schema's declared types and nullability (no required
#      field null against a non-nullable schema, no wrong container/scalar type).
#   2. The coverage invariant: every schema in `covered_schemas` has >= 1 active
#      target that resolves to a concrete instance whose declared root schema is
#      that component (the concrete-instance rule — transitive reachability and
#      empty arrays do not count; an object component wants a Hash, an array
#      component a non-empty Array).
#   3. Inventory completeness: every RichTextAttachment-emitting component schema
#      (the #408 class this guard exists to protect) must be either covered or
#      excluded — so the inventory can't silently shrink by dropping a schema.
#   4. `excluded_schemas` entries each carry a reason + tracking issue, name real
#      components, and do not overlap `covered_schemas`.
#
# Reuses conformance/runner/ruby/schema-walker.rb for the required-field walk.
# The type/nullability walk is additional (the walker checks presence only).
# Wired into `make check` in the scripts/validate-api-gaps.rb style (stdlib only).

require "json"
require "yaml"
require "set"

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
# document. Array indices must be "0" or a non-zero-leading run of digits (RFC
# 6901 §4) — "01" is malformed and does NOT silently resolve element 1. Missing
# keys / out-of-range indices / non-container traversal -> [nil, false].
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
      # RFC 6901 array index: "0" or [1-9][0-9]* — reject leading zeroes.
      return [nil, false] unless token == "0" || token.match?(/\A[1-9]\d*\z/)

      idx = token.to_i
      return [nil, false] if idx >= current.length

      current = current[idx]
    else
      return [nil, false]
    end
  end
  [current, true]
end

# --- Schema helpers (local; the walker keeps its resolver private) -------------

# Follows $ref chains within components until a non-ref schema or a cycle.
def resolve_ref(schema, components)
  seen = {}
  current = schema
  while current.is_a?(Hash) && current["$ref"].is_a?(String)
    ref = current["$ref"]
    break if seen[ref]

    seen[ref] = true
    m = ref.match(%r{\A(?:openapi\.json)?#/components/schemas/(.+)\z})
    break unless m

    nxt = components[m[1]]
    break unless nxt

    current = nxt
  end
  current
end

# Returns [Set(declared non-null json-type strings), nullable?]. Handles OpenAPI
# 3.1 null-union (`type: [X, "null"]`) and 3.0 `nullable: true`. An empty set
# means the type is unconstrained/compositional (oneOf/allOf) — skip type checks.
def allowed_types(schema)
  return [Set.new, false] unless schema.is_a?(Hash)

  t = schema["type"]
  case t
  when Array
    members = t.compact
    [Set.new(members - ["null"]), members.include?("null")]
  when String
    [Set.new([t]), schema["nullable"] == true]
  else
    [Set.new, schema["nullable"] == true]
  end
end

def json_type(value)
  case value
  when Hash then "object"
  when Array then "array"
  when String then "string"
  when true, false then "boolean"
  when Integer then "integer"
  when Float then "number"
  else "null"
  end
end

# Accepts an actual json-type against the schema's declared set. integer/number
# are interchangeable so float-spelled integers (e.g. FlexInt `1024.0`) pass.
def type_ok?(types, actual)
  return true if types.empty?
  return true if types.include?(actual)

  numeric = %w[integer number]
  numeric.include?(actual) && types.any? { |t| numeric.include?(t) }
end

# Recursive type/nullability validation. Reports (prefix-tagged) errors for:
#   - a present value whose JSON type contradicts the schema's declared type;
#   - a REQUIRED field present as null against a non-nullable schema.
# Optional-null and unknown/compositional schemas are left alone (forward-compat,
# matching the walker's extras philosophy).
def type_errors(prefix, value, schema, components, depth = 0)
  return [] if depth > 20

  resolved = resolve_ref(schema, components)
  return [] unless resolved.is_a?(Hash)

  return [] if value.nil? # optional-null tolerated here; required-null caught below

  errs = []
  types, = allowed_types(resolved)
  actual = json_type(value)
  unless type_ok?(types, actual)
    label = prefix.empty? ? "(root)" : prefix
    return ["#{label}: expected #{types.to_a.sort.join('|')}, got #{actual}"]
  end

  if value.is_a?(Hash) && resolved["properties"].is_a?(Hash)
    props = resolved["properties"]
    (resolved["required"] || []).each do |rk|
      next unless value.key?(rk) && value[rk].nil?

      _, nullable = allowed_types(resolve_ref(props[rk] || {}, components))
      next if nullable

      field = prefix.empty? ? rk : "#{prefix}/#{rk}"
      errs << "#{field}: required field is null but its schema is not nullable"
    end
    value.each do |k, v|
      next unless props.key?(k)

      child = prefix.empty? ? k : "#{prefix}/#{k}"
      errs.concat(type_errors(child, v, props[k], components, depth + 1))
    end
  elsif value.is_a?(Array) && resolved["items"]
    value.each_with_index do |item, i|
      errs.concat(type_errors("#{prefix}[#{i}]", item, resolved["items"], components, depth + 1))
    end
  end

  errs
end

# True when the instance is a concrete representative of `schema_name`: an
# object component wants a non-null Hash; an array component a non-empty Array.
def concrete_for?(schema_name, instance, components)
  types, = allowed_types(components[schema_name] || {})
  if types.include?("array")
    instance.is_a?(Array) && !instance.empty?
  else
    instance.is_a?(Hash)
  end
end

# Component schemas that declare at least one array-of-RichTextAttachment member
# — the rich-text emitters the guard exists to keep represented.
def rich_text_emitters(components)
  components.select do |_name, schema|
    props = schema.is_a?(Hash) ? schema["properties"] : nil
    next false unless props.is_a?(Hash)

    props.any? do |_pn, ps|
      ps.is_a?(Hash) && ps["type"] == "array" &&
        ps.dig("items", "$ref").to_s.end_with?("/RichTextAttachment")
    end
  end.keys
end

# --- Load -----------------------------------------------------------------------

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
# Records, per target id, the concrete root schema it resolved to (or nil) so the
# coverage pass can apply the concrete-instance rule without re-resolving.
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

  if operation && schema_name
    fail_with(errors, "target `#{id}`: `operation` and `schema` are mutually exclusive")
    next
  end
  if operation && !pointer.empty?
    fail_with(errors,
      "target `#{id}`: operation entries validate the whole response body — " \
      "a non-root `pointer` (#{pointer}) is not allowed (use a pointer entry with an explicit schema)")
    next
  end

  # Locate the schema to validate against, and the schema NAME for coverage.
  schema, root_schema_name =
    if operation
      s = walker.find_response_schema(operation)
      unless s
        fail_with(errors, "target `#{id}`: no 2xx/default response schema for operation `#{operation}`")
        next
      end
      m = s.is_a?(Hash) ? s["$ref"].to_s.match(%r{/components/schemas/(.+)\z}) : nil
      [s, m && m[1]]
    else
      unless schema_name
        fail_with(errors, "target `#{id}`: pointer entry needs an explicit `schema`")
        next
      end
      unless components.key?(schema_name)
        fail_with(errors, "target `#{id}`: unknown component schema `#{schema_name}`")
        next
      end
      [{ "$ref" => "#/components/schemas/#{schema_name}" }, schema_name]
    end

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

  # Record for the coverage pass: concreteness is schema-type-aware.
  concrete = root_schema_name ? concrete_for?(root_schema_name, instance, components) : instance.is_a?(Hash)
  resolved_root[id] = { name: root_schema_name, concrete: concrete }

  where = pointer.empty? ? fixture_rel : "#{fixture_rel} at pointer `#{pointer}`"
  walker.missing_required(instance, schema).each do |path|
    fail_with(errors, "target `#{id}`: #{where} missing required field `#{path}`")
  end
  type_errors("", instance, schema, components).each do |msg|
    fail_with(errors, "target `#{id}`: #{where} #{msg}")
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

    concrete_rep = true if root[:name] == schema_name && root[:concrete]
  end

  unless concrete_rep
    fail_with(errors,
      "covered_schemas[`#{schema_name}`]: no active target resolves to a concrete " \
      "instance declared as `#{schema_name}` (concrete-instance rule)")
  end
end

# --- 3. Inventory completeness (non-self-erasing) ------------------------------

accounted = Set.new(covered.keys) | Set.new(excluded.keys)
rich_text_emitters(components).sort.each do |emitter|
  next if accounted.include?(emitter)

  fail_with(errors,
    "schema `#{emitter}` declares a RichTextAttachment array member but is neither " \
    "in covered_schemas nor excluded_schemas — the coverage inventory must account " \
    "for every rich-text emitter (add a covered target or an exclusion with a reason)")
end

# --- 4. Exclusions -------------------------------------------------------------

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
  puts "==> Fixture coverage clean — #{covered.size} covered schemas, " \
       "#{targets.size} manifest targets, #{excluded.size} tracked exclusions, " \
       "#{rich_text_emitters(components).size} rich-text emitters accounted for"
  exit 0
else
  warn "Fixture-coverage check failed:"
  errors.sort.each { |e| warn "  - #{e}" }
  exit 1
end
