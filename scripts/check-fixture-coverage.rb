#!/usr/bin/env ruby
# frozen_string_literal: true

# Fixture-completeness guard.
#
# Validates spec/fixtures/manifest.yaml:
#   1. Every `targets` entry resolves — fixture exists, JSON pointer selects a
#      concrete instance, the named schema is a real component — and the selected
#      instance validates against its schema: every required field present and
#      non-null (unless the field schema is nullable), and every present value's
#      JSON type matches the declared type (a float is accepted for an integer
#      field only when mathematically integral; a null array element fails a
#      non-nullable item schema). Validation is composition-aware — it merges
#      `$ref` (including 3.1 `$ref`-with-siblings) and `allOf`, so a composed
#      target whose required fields live in a branch is still enforced.
#   2. The coverage invariant: every schema in `covered_schemas` has >= 1 active
#      target that resolves to a concrete instance whose declared root schema is
#      that component (the concrete-instance rule — transitive reachability and
#      empty arrays do not count; an object component wants a Hash, an array
#      component a non-empty Array).
#   3. Inventory completeness: every RichTextAttachment-emitting component schema
#      (the #408 class this guard exists to protect) must be either covered or
#      excluded — so the inventory can't silently shrink by dropping a schema.
#      Emitter discovery follows `$ref` (with siblings), traverses component-level
#      `allOf`/`anyOf`/`oneOf`, and resolves aliased array items, guarding cycles.
#   4. `excluded_schemas` entries each carry a reason + tracking issue, name real
#      components, and do not overlap `covered_schemas`.
#
# Uses conformance/runner/ruby/schema-walker.rb only for `find_response_schema`
# (operation-entry response-schema lookup). The required/type/nullability walk is
# implemented here because it must be composition-aware, which the walker is not.
# Wired into `make check` in the scripts/validate-api-gaps.rb style (stdlib only).
#
# Paths default to the repo layout but honour FIXTURE_MANIFEST / FIXTURE_OPENAPI
# / FIXTURE_DIR env overrides so the negative-case self-test
# (scripts/test-check-fixture-coverage.rb) can point it at crafted inputs.

require "json"
require "yaml"
require "set"

require_relative "../conformance/runner/ruby/schema-walker"

PROJECT_ROOT = File.expand_path("..", __dir__)
MANIFEST_FILE = ENV.fetch("FIXTURE_MANIFEST", File.join(PROJECT_ROOT, "spec/fixtures/manifest.yaml"))
FIXTURES_DIR = ENV.fetch("FIXTURE_DIR", File.join(PROJECT_ROOT, "spec/fixtures"))
OPENAPI_FILE = ENV.fetch("FIXTURE_OPENAPI", File.join(PROJECT_ROOT, "openapi.json"))

# Read text as UTF-8 regardless of the process locale (LC_ALL=C would otherwise
# read as US-ASCII and choke on the spec's UTF-8 bytes).
def read_utf8(path)
  File.read(path, encoding: "UTF-8")
end

errors = []

def fail_with(errors, message)
  errors << message
end

# RFC 6901 JSON pointer resolution. Returns [value, found]. "" selects the whole
# document. Escapes: `~0`->`~`, `~1`->`/`; a `~` not followed by 0/1 is invalid.
# Array indices must be "0" or a non-zero-leading run of digits (§4) — "01" is
# malformed and does NOT silently resolve element 1. Missing keys / out-of-range
# indices / non-container traversal / bad escapes -> [nil, false].
def resolve_pointer(doc, pointer)
  return [doc, true] if pointer.nil? || pointer.empty?
  return [nil, false] unless pointer.start_with?("/")

  current = doc
  pointer.split("/", -1).drop(1).each do |raw|
    return [nil, false] if raw.match?(/~(?![01])/) # invalid escape (e.g. ~2, trailing ~)

    token = raw.gsub("~1", "/").gsub("~0", "~")
    case current
    when Hash
      return [nil, false] unless current.key?(token)

      current = current[token]
    when Array
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

# --- Schema helpers ------------------------------------------------------------

# Ruby name of a `$ref`, or nil.
def ref_name(schema)
  return nil unless schema.is_a?(Hash) && schema["$ref"].is_a?(String)

  schema["$ref"].match(%r{/components/schemas/(.+)\z})&.captures&.first
end

# Returns [Set(declared non-null json-type strings), nullable?] for a single
# schema node (no traversal). Handles OpenAPI 3.1 null-union (`type:[X,"null"]`)
# and 3.0 `nullable:true`. An empty set means the node declares no type.
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

# True when `value`'s JSON type satisfies the declared `types`. integer/number
# interchange with one guard: a float supplied for an integer-only field passes
# only when it is mathematically integral (so FlexInt `1024.0` passes but `1.5`
# fails).
def type_matches?(types, value)
  return true if types.empty?

  actual = json_type(value)
  return true if types.include?(actual)

  if actual == "number" && types.include?("integer") && !types.include?("number")
    return value.is_a?(Float) && value.finite? && value == value.truncate
  end
  return true if actual == "integer" && types.include?("number")

  false
end

# Merges a schema's effective constraints across `$ref` (including 3.1
# `$ref`-with-siblings) and `allOf`, returning
# [required(Array), properties(Hash), types(Set), nullable(bool), items(schema),
#  alt_groups(Array of branch-arrays)].
# `allOf` is a conjunction (merged into required/properties). `anyOf`/`oneOf` are
# alternatives: their branches are NOT merged (that would over-require) but each
# group is collected so validation can require at least one branch to match —
# including groups inherited through `$ref` and `allOf`. `visited` (component
# names) + depth guard terminate reference/composition cycles.
def merged_constraints(schema, components, visited = Set.new, depth = 0)
  req = []
  props = {}
  types = Set.new
  nullable = false
  items = nil
  alt_groups = []
  return [req, props, types, nullable, items, alt_groups] if depth > 40 || !schema.is_a?(Hash)

  absorb = lambda do |sub|
    r2, p2, t2, n2, i2, a2 = merged_constraints(sub, components, visited, depth + 1)
    req.concat(r2)
    p2.each { |k, v| props[k] ||= v }
    types.merge(t2)
    nullable ||= n2
    items ||= i2
    alt_groups.concat(a2)
  end

  name = ref_name(schema)
  if name && !visited.include?(name)
    visited << name
    absorb.call(components[name])
    # fall through to local keywords (OpenAPI 3.1 permits $ref siblings)
  end

  t, nn = allowed_types(schema)
  types.merge(t)
  nullable ||= nn
  (schema["properties"] || {}).each { |k, v| props[k] ||= v }
  (schema["required"] || []).each { |r| req << r }
  items ||= schema["items"]
  (schema["allOf"] || []).each { |sub| absorb.call(sub) }
  %w[anyOf oneOf].each do |key|
    alt_groups << schema[key] if schema[key].is_a?(Array) && !schema[key].empty?
  end

  [req, props, types, nullable, items, alt_groups]
end

# Composition-aware validation of `value` against `schema`. Reports
# (path-tagged) errors for a missing required field, a required field present as
# null against a non-nullable schema, a null array element against a non-nullable
# item schema, and a present value whose JSON type contradicts the declared type.
# Optional object-field nulls are tolerated: the Smithy-derived OpenAPI
# under-marks some nullable optionals (e.g. Person.bio/location are `type:string`
# yet the wire sends null), so flagging them would be a false positive.
def instance_errors(prefix, value, schema, components, depth = 0)
  return [] if depth > 60
  return [] if value.nil? # optional-null tolerated; required-/element-null handled in context

  req, props, types, _nullable, items, alt_groups = merged_constraints(schema, components)

  unless type_matches?(types, value)
    label = prefix.empty? ? "(root)" : prefix
    return ["#{label}: expected #{types.to_a.sort.join('|')}, got #{json_type(value)}"]
  end

  errs = []

  # anyOf/oneOf: the value must satisfy at least one branch of each group
  # (oneOf is validated as "at least one" — enforcing exactly-one would need full
  # discriminator/enum/const validation to avoid false positives).
  alt_groups.each do |branches|
    next if branches.any? { |branch| instance_errors(prefix, value, branch, components, depth + 1).empty? }

    label = prefix.empty? ? "(root)" : prefix
    errs << "#{label}: value matches none of the #{branches.length} allowed alternatives (anyOf/oneOf)"
  end

  if value.is_a?(Hash)
    req.uniq.each do |rk|
      field = prefix.empty? ? rk : "#{prefix}/#{rk}"
      if !value.key?(rk)
        errs << "missing required field `#{field}`"
      elsif value[rk].nil?
        _, _, _, field_nullable, = merged_constraints(props[rk] || {}, components)
        errs << "#{field}: required field is null but its schema is not nullable" unless field_nullable
      end
    end
    value.each do |k, v|
      next unless props.key?(k)

      child = prefix.empty? ? k : "#{prefix}/#{k}"
      errs.concat(instance_errors(child, v, props[k], components, depth + 1))
    end
  elsif value.is_a?(Array) && items
    _, _, _, item_nullable, = merged_constraints(items, components)
    value.each_with_index do |item, i|
      ip = "#{prefix}[#{i}]"
      if item.nil?
        errs << "#{ip}: null array element but the item schema is not nullable" unless item_nullable
        next
      end
      errs.concat(instance_errors(ip, item, items, components, depth + 1))
    end
  end

  errs
end

def concrete_for?(schema_name, instance, components)
  _, _, types, = merged_constraints({ "$ref" => "#/components/schemas/#{schema_name}" }, components)
  if types.include?("array")
    instance.is_a?(Array) && !instance.empty?
  else
    instance.is_a?(Hash)
  end
end

# True when `schema` denotes (through `$ref` chains + siblings and
# allOf/anyOf/oneOf composition) the RichTextAttachment component — so an alias
# (`{$ref: SomeAlias}` -> RichTextAttachment) or a composed item schema is still
# recognized. `visited` (component names) terminates ref cycles.
def rich_text_attachment?(schema, components, visited = Set.new, depth = 0)
  return false if depth > 40 || !schema.is_a?(Hash)

  name = ref_name(schema)
  if name
    return true if name == "RichTextAttachment"

    unless visited.include?(name)
      visited << name
      return true if rich_text_attachment?(components[name], components, visited, depth + 1)
    end
    # fall through to local keywords ($ref siblings)
  end

  %w[allOf anyOf oneOf].each do |key|
    (schema[key] || []).each do |sub|
      return true if rich_text_attachment?(sub, components, visited, depth + 1)
    end
  end
  false
end

# True when `schema` is (or composes/refs) an array whose items resolve to the
# RichTextAttachment component. Follows `$ref` (with siblings) and
# allOf/anyOf/oneOf on both the array schema and its items; `visited` guards
# cycles.
def references_rich_text_array?(schema, components, visited = Set.new, depth = 0)
  return false if depth > 40 || !schema.is_a?(Hash)

  name = ref_name(schema)
  if name && !visited.include?(name)
    visited << name
    return true if references_rich_text_array?(components[name], components, visited, depth + 1)
    # fall through to local keywords ($ref siblings)
  end

  t = schema["type"]
  if (t == "array" || (t.is_a?(Array) && t.include?("array"))) && schema["items"]
    return true if rich_text_attachment?(schema["items"], components)
  end

  %w[allOf anyOf oneOf].each do |key|
    (schema[key] || []).each do |sub|
      return true if references_rich_text_array?(sub, components, visited, depth + 1)
    end
  end
  false
end

# Collects every property schema of a component, following `$ref` (with siblings)
# and traversing component-level allOf/anyOf/oneOf so composed/inherited
# properties are seen. `visited` (component names) guards ref cycles.
def collect_property_schemas(schema, components, out, visited = Set.new, depth = 0)
  return if depth > 40 || !schema.is_a?(Hash)

  name = ref_name(schema)
  if name && !visited.include?(name)
    visited << name
    collect_property_schemas(components[name], components, out, visited, depth + 1)
    # fall through to local keywords ($ref siblings)
  end

  (schema["properties"] || {}).each_value { |ps| out << ps }
  %w[allOf anyOf oneOf].each do |key|
    (schema[key] || []).each do |sub|
      collect_property_schemas(sub, components, out, visited, depth + 1)
    end
  end
end

# A whole-component alias — a schema that is only a `$ref` to another component
# (bare annotations aside). It introduces no decode surface of its own, so it is
# NOT an independent emitter: covering its target covers it, and if that target
# is itself an emitter it is caught directly. Skipping these keeps the emitter
# set to the concrete decode types (the ~40 `*ResponseContent` response
# envelopes are pure aliases of already-covered components). A `$ref` WITH local
# structure (3.1 siblings) is not a pure alias and is still examined.
def pure_ref_alias?(schema)
  schema.is_a?(Hash) && schema.key?("$ref") &&
    (schema.keys - %w[$ref description title]).empty?
end

def rich_text_emitters(components)
  components.select do |_name, schema|
    next false unless schema.is_a?(Hash)
    next false if pure_ref_alias?(schema)

    props = []
    collect_property_schemas(schema, components, props)
    props.any? { |ps| references_rich_text_array?(ps, components) }
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

manifest = YAML.safe_load(read_utf8(MANIFEST_FILE))
walker = Basecamp::Conformance::SchemaWalker.new(OPENAPI_FILE)
openapi = JSON.parse(read_utf8(OPENAPI_FILE))
components = openapi.dig("components", "schemas") || {}

targets = manifest["targets"] || []
covered = manifest["covered_schemas"] || {}
excluded = manifest["excluded_schemas"] || {}

fixture_cache = {}
load_fixture = lambda do |rel|
  fixture_cache[rel] ||= begin
    path = File.join(FIXTURES_DIR, rel)
    File.file?(path) ? JSON.parse(read_utf8(path)) : :missing
  end
end

# --- 1. Targets ----------------------------------------------------------------

by_id = {}
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

  concrete = root_schema_name ? concrete_for?(root_schema_name, instance, components) : instance.is_a?(Hash)
  resolved_root[id] = { name: root_schema_name, concrete: concrete }

  where = pointer.empty? ? fixture_rel : "#{fixture_rel} at pointer `#{pointer}`"
  instance_errors("", instance, schema, components).each do |msg|
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
    next unless root

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
