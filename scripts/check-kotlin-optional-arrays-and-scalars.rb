#!/usr/bin/env ruby
# frozen_string_literal: true

# Presence-preservation guard for the Kotlin generator's ARRAY and PRIMITIVE
# SCALAR properties.
#
# SPEC.md's Optional Fields rule: a zero value is not a substitute for absence.
# Kotlin must therefore encode presence in the static type, so an absent field
# stays distinguishable from a present-but-zero / present-but-empty one.
#
# --- Scope (read this before renaming or widening) -----------------------------
#
# This guard covers exactly two property classes:
#
#   * ARRAYS            — `List<T>` / `List<T>?`
#   * PRIMITIVE SCALARS — String, Int, Long, Short, Byte, Boolean, Double,
#                         Float, Char
#
# It deliberately does NOT cover OBJECT / `$ref` / enum / JsonElement-typed
# properties. Those are reference types in Kotlin, so absence is already
# representable and there is no zero-value sentinel to guard against. A checker
# named for "optional fields" in general would overstate what is actually
# verified — hence arrays-and-scalars in the name. Keep the name honest if the
# coverage changes.
#
# --- The invariant -------------------------------------------------------------
#
# Three wire states must stay distinct, so there are three encodings:
#
#   required, non-nullable  ->  `T`           (no `?`, no default)
#   required, nullable      ->  `T?`          (nullable, NO default — the key
#                                              must still be present;
#                                              kotlinx.serialization only
#                                              tolerates an absent key when the
#                                              property has a default)
#   optional                ->  `T? = null`   (absence decodes to null)
#
# The required-and-nullable row is not a carve-out to silence false positives:
# it is a real third state the spec models as `type: [T, "null"]` on a required
# member (SearchType.key, TimelineEventData.starts_at / ends_at, Wormhole.color
# / destination_url). Emitting it as `T? = null` would silently accept a
# response that omits the key entirely, so the *absent* default is load-bearing
# and is asserted here rather than skipped.
#
# No property may carry a zero-value sentinel default (`= emptyList()`,
# `= false`, `= 0`, `= 0L`, `= ""`, `= 0.0`) — that is the whole point.
#
# --- Two levels of enforcement -------------------------------------------------
#
#   * Schema-checked — each property is cross-checked against its component's
#     `required` set and declared nullability, so BOTH halves of the guarantee
#     are proven: a required member mistakenly emitted as `T? = null` is caught,
#     not silently accepted. This covers two kinds of class:
#       - generated/models/<Component>.kt, whose basename is the component;
#       - generated/services request bodies, where `<OperationId>Body` resolves
#         through the operation's `requestBody` reference in the spec — NOT a
#         `<OperationId>RequestContent` name guess, which would silently miss
#         the aliased bodies (CreateAnswer -> QuestionAnswerPayload). Request
#         bodies are where a wrong requiredness actually drops data on a write,
#         so they are resolved rather than skipped.
#     A member of a schema-backed class that resolves to no property is an
#     ERROR (renamed member, typo'd @SerialName), not a silent downgrade —
#     except for the narrow SYNTHESIZED_MEMBERS allowlist below.
#   * Structural — only the query-parameter `*Options` classes and supporting
#     non-component models remain here. They have no body schema, so
#     requiredness cannot be derived and only a defaulted property is
#     constrained: it must be nullable and default to `null`.
#
# Pins the Kotlin scalar fix (#424) and the optional-array fix (#433) against
# regression. Wired into `make check` as kt-check-optional-arrays-and-scalars.

require "json"

PROJECT_ROOT = File.expand_path("..", __dir__)
GENERATED_DIR = File.join(
  PROJECT_ROOT,
  "kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated"
)
MODELS_DIR = File.join(GENERATED_DIR, "models")
OPENAPI_FILE = File.join(PROJECT_ROOT, "openapi.json")

# Kotlin primitive scalars. Reference types (objects, $refs, enums, JsonElement,
# Map) are intentionally excluded — see the Scope note above.
PRIMITIVE_SCALARS = %w[String Int Long Short Byte Boolean Double Float Char].freeze

unless File.file?(OPENAPI_FILE)
  warn "ERROR: openapi.json not found at #{OPENAPI_FILE}"
  exit 2
end

# Read as UTF-8 regardless of process locale (LC_ALL=C would otherwise read as
# US-ASCII and choke on the spec's / generated code's UTF-8 bytes).
spec = JSON.parse(File.read(OPENAPI_FILE, encoding: "UTF-8"))
components = spec.dig("components", "schemas") || {}

errors = []

files = Dir.glob(File.join(GENERATED_DIR, "**", "*.kt")).sort
if files.empty?
  warn "ERROR: no generated Kotlin files found under #{GENERATED_DIR}"
  exit 2
end

# Parse one property line into [wire_name, type_part, default_part, kind], or
# nil when the line isn't a guarded `val`. `wire_name` is the @SerialName value
# when present, else the Kotlin field name (the generator only adds @SerialName
# when the camelCase name differs from the wire name, so this recovers the wire
# name). `kind` is :array or :scalar; anything else yields nil (out of scope).
def parse_property(line)
  field = line[/\bval\s+(\w+)\s*:/, 1]
  return nil unless field

  decl = line[/\bval\s+\w+\s*:\s*(.*)$/, 1]
  return nil unless decl

  decl = decl.strip.sub(/[,)]\s*$/, "").strip

  type_part, default_part =
    if decl.include?("=")
      left, right = decl.split("=", 2)
      [ left.strip, right.strip ]
    else
      [ decl, nil ]
    end

  bare = type_part.sub(/\?\z/, "")
  kind =
    if bare.start_with?("List<")
      :array
    elsif PRIMITIVE_SCALARS.include?(bare)
      :scalar
    end
  return nil unless kind

  serial = line[/@SerialName\("([^"]+)"\)/, 1]
  [ serial, field, type_part, default_part, kind ]
end

# camelCase -> snake_case, for the request-body classes under generated/services.
# Those are plain (non-@Serializable) body holders, so unlike the models they
# carry no @SerialName to recover the wire name from; the service method does
# the snake_case mapping when it builds the JSON.
def snake_case(name)
  name.gsub(/([a-z\d])([A-Z])/, '\1_\2').downcase
end

# The schema key a generated member corresponds to: an explicit @SerialName wins;
# otherwise prefer the field name as-is, falling back to its snake_case form.
def wire_key(serial, field, known_props)
  return serial if serial
  return field if known_props&.key?(field)

  snake = snake_case(field)
  known_props&.key?(snake) ? snake : field
end

# Members a generator synthesizes onto a schema-backed class, which therefore
# have no counterpart in the component's `properties`. Keep this list minimal
# and justified — everything else absent from `properties` is an error.
#
#   Person.system_label — companion emitted alongside flexible-integer id
#   fields; carried on the wire but not modeled as a schema property.
SYNTHESIZED_MEMBERS = {
  "Person" => %w[system_label].freeze
}.freeze

# operationId => request-body component name, read from the spec's paths rather
# than guessed from a naming convention. Most operations use
# `<OperationId>RequestContent`, but some reference a shared payload component
# instead (CreateAnswer -> QuestionAnswerPayload, UpdateAnswer ->
# QuestionAnswerUpdatePayload). Deriving the map means an alias can never
# silently drop a request body onto the weaker structural path.
def build_request_body_map(spec)
  map = {}
  (spec["paths"] || {}).each_value do |ops|
    ops.each do |verb, op|
      next unless %w[get post put patch delete].include?(verb)
      next unless op.is_a?(Hash)

      oid = op["operationId"]
      ref = op.dig("requestBody", "content", "application/json", "schema", "$ref")
      map[oid] = ref.split("/").last if oid && ref
    end
  end
  map
end

# Resolve the OpenAPI component backing a generated Kotlin class.
#
# Model classes are named for their component directly. Service request-body
# classes are named `<OperationId>Body`; their component comes from the
# operation's `requestBody` reference, so requiredness for request bodies —
# where a wrong answer drops data on a write — is resolved rather than skipped,
# aliases included. Query-parameter `*Options` classes have no body schema and
# correctly resolve to nil, leaving them on the structural path.
def resolve_component(class_name, components, request_bodies)
  return nil unless class_name
  return class_name if components.key?(class_name)

  if class_name.end_with?("Body")
    operation = class_name.delete_suffix("Body")
    from_spec = request_bodies[operation]
    return from_spec if from_spec && components.key?(from_spec)
  end

  nil
end

# A schema member is nullable when the spec spells `type: [T, "null"]` or sets
# `nullable: true` (the enhance pass uses both forms).
def schema_nullable?(prop_schema)
  return false unless prop_schema.is_a?(Hash)
  return true if prop_schema["nullable"] == true

  type = prop_schema["type"]
  type.is_a?(Array) && type.include?("null")
end

request_bodies = build_request_body_map(spec)

files.each do |path|
  rel = path.sub("#{PROJECT_ROOT}/", "")
  in_models = path.start_with?("#{MODELS_DIR}/")
  # In models/ the file basename IS the component. In services/ a single file
  # holds several data classes, so the enclosing class is tracked as we scan.
  file_component = in_models ? File.basename(path, ".kt") : nil
  current_class = nil

  File.foreach(path, encoding: "UTF-8").with_index(1) do |line, lineno|
    if (m = line.match(/\bdata class (\w+)\(/))
      current_class = m[1]
    end

    parsed = parse_property(line)
    next unless parsed

    component_name = resolve_component(file_component || current_class, components, request_bodies)
    component = component_name ? components[component_name] : nil
    required_fields = component ? (component["required"] || []) : nil
    known_props = component ? (component["properties"] || {}) : nil

    serial, field, type_part, default_part, kind = parsed
    wire = wire_key(serial, field, known_props)
    nullable = type_part.end_with?("?")
    has_default = !default_part.nil?
    noun = kind == :array ? "array" : "scalar"
    shape = "`#{type_part}#{has_default ? " = #{default_part}" : ''}`"

    # Schema-checked path: the field is a known property of an OpenAPI component.
    if required_fields && known_props&.key?(wire)
      if required_fields.include?(wire)
        if schema_nullable?(known_props[wire])
          # Required AND nullable: present-but-null. Nullable type, NO default —
          # a default would let an omitted key decode silently.
          unless nullable && !has_default
            errors << "#{rel}:#{lineno}: `#{wire}` is REQUIRED and NULLABLE in " \
                      "schema `#{component_name}` but is emitted as #{shape} — " \
                      "must be `T?` with no default, so an omitted key still fails"
          end
        elsif nullable || has_default
          errors << "#{rel}:#{lineno}: `#{wire}` is REQUIRED in schema " \
                    "`#{component_name}` but is emitted as #{shape} — a required " \
                    "#{noun} must be non-null `T` with no default"
        end
      elsif !nullable || default_part != "null"
        errors << "#{rel}:#{lineno}: `#{wire}` is OPTIONAL in schema " \
                  "`#{component_name}` but is emitted as #{shape} — an optional " \
                  "#{noun} must be `T? = null`"
      end
      next
    end

    # A member of a schema-backed class that is absent from the component's
    # `properties` is either a generator-synthesized companion (allowlisted) or
    # a real mismatch — a renamed/dropped property, or a typo'd @SerialName —
    # which would otherwise slip silently onto the weaker structural path and
    # quietly void the schema-backed guarantee for that member.
    if component && !known_props&.key?(wire)
      unless SYNTHESIZED_MEMBERS[component_name]&.include?(wire)
        errors << "#{rel}:#{lineno}: `#{wire}` is emitted on schema-backed class " \
                  "`#{component_name}` but is not a property of that component — " \
                  "requiredness cannot be checked. Fix the name, or add it to " \
                  "SYNTHESIZED_MEMBERS if the generator synthesizes it."
      end
      next
    end

    # Structural path: requiredness unknown (query-param `*Options` classes and
    # supporting non-component models). Only a defaulted property is constrained.
    next unless has_default

    if default_part == "emptyList()"
      errors << "#{rel}:#{lineno}: optional array uses the forbidden " \
                "`= emptyList()` sentinel — must be `List<T>? = null`"
    elsif !nullable
      errors << "#{rel}:#{lineno}: non-null #{noun} `#{type_part}` carries a " \
                "default (`= #{default_part}`) — an optional #{noun} must be " \
                "nullable (`T? = null`)"
    elsif default_part != "null"
      errors << "#{rel}:#{lineno}: nullable #{noun} defaults to " \
                "`#{default_part}` — must default to `null`"
    end
  end
end

if errors.empty?
  puts "==> Kotlin optional array/scalar invariant clean — #{files.size} generated files scanned"
  exit 0
else
  warn "Kotlin optional array/scalar invariant failed:"
  errors.each { |e| warn "  - #{e}" }
  exit 1
end
