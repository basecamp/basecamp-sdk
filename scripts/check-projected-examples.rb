#!/usr/bin/env ruby
# frozen_string_literal: true

# Projected-example guard.
#
# Validates every example published in the generated `openapi.json` against the
# schema it sits under, in the PROJECTION — which is the one place nothing was
# checking.
#
# --- The seam this exists for --------------------------------------------------
#
# `spec/smithy-build.json`'s `jsonAdd` can append to a schema's `required` array
# in the OpenAPI projection:
#
#     "/components/schemas/Todolist/required/-": "color"
#
# That route exists because `Todolist` carries `@examples` and Smithy cannot put
# a `null` into an example for a String shape, so native `@required` on a
# required-AND-nullable member fails Smithy validation (#630, #637).
#
# `smithy validate` checks `@examples` against the SMITHY model, where the member
# is still natively optional. The projection adds the requiredness afterwards.
# So a projection-added required field can be missing from a published example
# and every gate stays green. That is not hypothetical: #637 shipped both
# `GetTodolistOrGroup` response examples without `color` while the schema
# declared it required, and a bot reviewer — not CI — caught it (fixed in
# 12b02cda8 by injecting the example values through `jsonAdd` too). See #638.
#
# --- What is checked -----------------------------------------------------------
#
# Every `examples` map entry and every singular `example` under:
#   * responses[code].content[mediaType]
#   * requestBody.content[mediaType]
#   * parameters[] (operation-level and path-item-level)
# is validated against the sibling `schema`, using the same composition-aware
# walk the fixture guard uses (scripts/schema_instance_validator.rb): required
# presence, declared type, nullability, arrays, `$ref`/`allOf` conjunction,
# `anyOf`/`oneOf` as at-least-one. Example objects given as
# `$ref: '#/components/examples/X'` are resolved first.
#
# --- The response envelope, and why there is nothing to do about it -------------
#
# The mappers in spec/smithy-bare-arrays rewrite a single-property
# `*ResponseContent` schema into the bare payload, because BC3 returns bare
# bodies while Smithy's restJson1 requires a wrapper structure. Until #644 they
# rewrote the SCHEMA only, so the published example kept the wrapper member
# Smithy emits from the `@examples` `output` node:
#
#     schema:  {"$ref": ".../Todolist"}
#     example: {"result": { ...todolist... }}   <- before #644
#
# #648's `BareResponseExampleMapper` mirrors that unwrapping onto the examples,
# so the two now agree and this gate compares them directly. No unwrapping here,
# and deliberately none: a gate that ALSO accepted the wrapped shape could not
# tell a correct example from a regression in that mapper. Comparing exactly what
# is published against exactly the schema it is published under is the whole
# claim, and it means this gate now guards `BareResponseExampleMapper` too — put
# the wrapper back and the run goes red.
#
# --- Known blind spot ----------------------------------------------------------
#
# This validates the examples that EXIST. A `required/-` pointer aimed at a
# schema that no example reaches is still unvalidated — the gate cannot fail on
# an example nobody wrote. The liveness floor below keeps that from silently
# becoming true of the whole gate, but it is not example coverage. Adding
# examples is the spec's job; this is the gate that makes them mean something.
#
# --- Running -------------------------------------------------------------------
#
# `ruby scripts/check-projected-examples.rb`, or `make check-projected-examples`
# (which also runs the self-test). `PROJECTED_EXAMPLES_OPENAPI` overrides the
# input document so scripts/test-check-projected-examples.rb can drive crafted
# specs through this exact checker.

require "json"

require_relative "schema_instance_validator"

PROJECT_ROOT = File.expand_path("..", __dir__)
OPENAPI_FILE = ENV.fetch("PROJECTED_EXAMPLES_OPENAPI", File.join(PROJECT_ROOT, "openapi.json"))

# HTTP methods an OpenAPI path item may carry. Anything else at that level
# (`parameters`, `summary`, `$ref`, `x-*`) is not an operation.
HTTP_METHODS = %w[get put post delete options head patch trace].freeze

# Read text as UTF-8 regardless of the process locale (LC_ALL=C would otherwise
# read as US-ASCII and choke on the spec's UTF-8 bytes).
def read_utf8(path)
  File.read(path, encoding: "UTF-8")
end

# Every example a media-type / parameter node publishes, as
# [[label, status, payload], ...] where status is one of:
#
#   :value    payload is the example value, ready to validate
#   :skip     payload is the reason there is nothing to validate
#   :error    payload is the reason this example cannot be checked at all
#
# OpenAPI allows two spellings and they mean different things. `examples` is a
# map of Example OBJECTS — the value lives under `value` (or is remote, under
# `externalValue`) — while the singular `example` IS the value. Conflating them
# would silently unwrap any example payload that happens to have a field named
# `value`. The generator emits only `examples` today; handling both means a
# future singular `example` cannot slip in unchecked.
def example_entries(node, doc)
  entries = []
  examples = node["examples"]
  if examples.is_a?(Hash)
    examples.each { |name, example| entries << [name, *example_object_value(example, doc)] }
  end
  entries << ["(example)", :value, node["example"]] if node.key?("example")
  entries
end

# Unpacks one Example Object, resolving `$ref: '#/components/examples/X'` first.
def example_object_value(example, doc)
  if example.is_a?(Hash) && example["$ref"].is_a?(String)
    name = example["$ref"].match(%r{\A#/components/examples/(.+)\z})&.captures&.first
    example = name && doc.dig("components", "examples", name)
    return [:error, "`$ref` does not resolve to an entry in components/examples"] unless example
  end

  return [:error, "example is not an Example Object"] unless example.is_a?(Hash)
  return [:value, example["value"]] if example.key?("value")
  # An `externalValue` is a URL. Fetching it would make an offline gate depend on
  # the network, which is how a gate learns to skip.
  return [:skip, "example is an `externalValue` URL, which this gate does not fetch"] if example.key?("externalValue")

  [:error, "Example Object declares neither `value` nor `externalValue`"]
end

# --- Load ----------------------------------------------------------------------

unless File.file?(OPENAPI_FILE)
  warn "ERROR: openapi.json not found at #{OPENAPI_FILE}"
  exit 2
end

doc = JSON.parse(read_utf8(OPENAPI_FILE))
components = doc.dig("components", "schemas") || {}

errors = []
skipped = []
counts = Hash.new(0)

# --- Walk ----------------------------------------------------------------------

# Validates one example value against `schema`, tagging every message with
# `where` so a failure names the operation, the example and the field.
validate = lambda do |where, kind, value, schema|
  if schema.nil?
    skipped << "#{where}: no schema declared alongside the example — nothing to validate against"
    return
  end

  counts[kind] += 1
  SchemaInstanceValidator.instance_errors("", value, schema, components).each do |msg|
    errors << "#{where}: #{msg}"
  end
end

# Records a non-:value example entry. Returns true when the caller should stop.
triaged = lambda do |where, status, payload|
  case status
  when :error then errors << "#{where}: #{payload}"
  when :skip then skipped << "#{where}: #{payload}"
  end
  status != :value
end

# Media-type nodes (responses and requestBody bodies).
walk_content = lambda do |where_prefix, kind, content|
  (content || {}).each do |media_type, media|
    next unless media.is_a?(Hash)

    example_entries(media, doc).each do |name, status, payload|
      where = "#{where_prefix}/#{media_type}/examples/#{name}"
      next if triaged.call(where, status, payload)

      validate.call(where, kind, payload, media["schema"])
    end
  end
end

# Parameter nodes carry the example beside their own `schema`.
walk_parameters = lambda do |where_prefix, parameters|
  (parameters || []).each do |param|
    next unless param.is_a?(Hash)

    example_entries(param, doc).each do |name, status, payload|
      where = "#{where_prefix} parameters/#{param['name']}/examples/#{name}"
      next if triaged.call(where, status, payload)

      validate.call(where, :parameter, payload, param["schema"])
    end
  end
end

(doc["paths"] || {}).each do |path, path_item|
  next unless path_item.is_a?(Hash)

  # Path-level parameters are shared by every method, so they are walked once
  # per path rather than once per operation — otherwise one example would be
  # counted (and any failure reported) once for each method under it. None
  # exist in the generated document today; handling them keeps that from
  # becoming a silent blind spot if the generator starts hoisting them.
  walk_parameters.call(path, path_item["parameters"])

  path_item.each do |method, op|
    next unless HTTP_METHODS.include?(method) && op.is_a?(Hash)

    label = "#{method.upcase} #{path}"
    label += " (#{op['operationId']})" if op["operationId"]

    (op["responses"] || {}).each do |code, response|
      next unless response.is_a?(Hash)

      walk_content.call("#{label} responses/#{code}", :response, response["content"])
    end

    walk_content.call("#{label} requestBody", :request_body, (op["requestBody"] || {})["content"])

    walk_parameters.call(label, op["parameters"])
  end
end

# --- Liveness floor ------------------------------------------------------------
#
# The seam lives in RESPONSE examples: that is where a projection-added `required`
# entry meets a value Smithy never checked. A run that validates none of them
# passes for the wrong reason — the walk broke, or the examples were deleted. A
# gate that no-ops is the failure mode this gate exists to prevent, so say so
# instead of exiting green.

if counts[:response].zero?
  errors << "no response example was validated — either openapi.json stopped publishing response " \
            "examples or this walk stopped finding them; a vacuous pass is not a pass"
end

# --- Report --------------------------------------------------------------------

total = counts.values.sum

if errors.empty?
  puts "==> Projected examples validate — #{total} checked " \
       "(#{counts[:response]} response, #{counts[:request_body]} request-body, " \
       "#{counts[:parameter]} parameter)"
  skipped.sort.each { |s| puts "    skipped: #{s}" }
  exit 0
else
  warn "Projected-example validation failed:"
  errors.sort.each { |e| warn "  - #{e}" }
  warn ""
  warn "These examples are published in openapi.json but contradict the schema they sit under."
  warn "A projection-added `required` entry (spec/smithy-build.json `jsonAdd` .../required/-) is not"
  warn "checked by `smithy validate`, which only sees the pre-projection Smithy model — so the example"
  warn "has to be updated in the same place the requiredness came from. See #638."
  exit 1
end
