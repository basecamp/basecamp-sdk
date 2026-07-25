#!/usr/bin/env ruby
# frozen_string_literal: true

# D-invariant guard for the Kotlin generator's optional-array contract.
#
# Every OPTIONAL generated array must be `List<T>? = null` so an absent array
# stays distinct from a present-but-empty one (SPEC.md Optional Fields rule),
# aligning Kotlin with the other five SDKs. Every REQUIRED array stays a plain
# non-null `List<T>`. No generated array may default to the `= emptyList()`
# sentinel that this fix removed.
#
# Two levels of enforcement:
#
#   * Schema-checked (generated/models/<Component>.kt whose basename is an
#     OpenAPI component): each array property is cross-checked against the
#     component's `required` set — a REQUIRED array MUST be non-null `List<T>`
#     (no `?`, no default); an OPTIONAL array MUST be `List<T>? = null`. This
#     proves both halves of the guarantee, so a required array mistakenly
#     emitted as `List<T>?` is caught, not silently accepted.
#   * Structural (everything else — request/param types in generated/services,
#     supporting model types not present as components): a property with a
#     default MUST be `List<T>? = null` (so `= emptyList()` or any non-null
#     default fails). Requiredness can't be derived here, so a bare `List<T>`
#     or nullable-without-default is left alone.
#
# Pins the D fix against regression. Wired into `make check`.

require "json"

PROJECT_ROOT = File.expand_path("..", __dir__)
GENERATED_DIR = File.join(
  PROJECT_ROOT,
  "kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated"
)
MODELS_DIR = File.join(GENERATED_DIR, "models")
OPENAPI_FILE = File.join(PROJECT_ROOT, "openapi.json")

unless File.file?(OPENAPI_FILE)
  warn "ERROR: openapi.json not found at #{OPENAPI_FILE}"
  exit 2
end

components = JSON.parse(File.read(OPENAPI_FILE)).dig("components", "schemas") || {}

errors = []

files = Dir.glob(File.join(GENERATED_DIR, "**", "*.kt")).sort
if files.empty?
  warn "ERROR: no generated Kotlin files found under #{GENERATED_DIR}"
  exit 2
end

# Parse one property line into [wire_name, type_part, default_part] or nil when
# the line isn't an array-typed `val`. `wire_name` is the @SerialName value when
# present, else the Kotlin field name (the generator only adds @SerialName when
# the camelCase name differs from the wire name, so this recovers the wire name).
def parse_array_property(line)
  return nil unless line =~ /:\s*(List<.*)$/

  decl = Regexp.last_match(1).strip.sub(/[,)]\s*$/, "").strip
  field = line[/\bval\s+(\w+)\s*:/, 1]
  return nil unless field

  serial = line[/@SerialName\("([^"]+)"\)/, 1]
  wire = serial || field

  type_part, default_part =
    if decl.include?("=")
      left, right = decl.split("=", 2)
      [left.strip, right.strip]
    else
      [decl, nil]
    end

  [wire, type_part, default_part]
end

files.each do |path|
  rel = path.sub("#{PROJECT_ROOT}/", "")
  component_name =
    if path.start_with?("#{MODELS_DIR}/")
      base = File.basename(path, ".kt")
      base if components.key?(base)
    end
  component = component_name ? components[component_name] : nil
  required_fields = component ? (component["required"] || []) : nil
  known_props = component ? (component["properties"] || {}) : nil

  File.foreach(path).with_index(1) do |line, lineno|
    parsed = parse_array_property(line)
    next unless parsed

    wire, type_part, default_part = parsed
    nullable = type_part.end_with?("?")
    has_default = !default_part.nil?

    # Schema-checked path: the field is a known property of an OpenAPI component.
    if required_fields && known_props&.key?(wire)
      if required_fields.include?(wire)
        if nullable || has_default
          errors << "#{rel}:#{lineno}: `#{wire}` is REQUIRED in schema " \
                    "`#{component_name}` but is emitted as `#{type_part}" \
                    "#{has_default ? " = #{default_part}" : ''}` — a required " \
                    "array must be non-null `List<T>` with no default"
        end
      elsif !nullable || default_part != "null"
        errors << "#{rel}:#{lineno}: `#{wire}` is OPTIONAL in schema " \
                  "`#{component_name}` but is emitted as `#{type_part}" \
                  "#{has_default ? " = #{default_part}" : ''}` — an optional " \
                  "array must be `List<T>? = null`"
      end
      next
    end

    # Structural path: requiredness unknown (request/param types, supporting
    # non-component models). Only a defaulted array is constrained.
    next unless has_default

    if default_part == "emptyList()"
      errors << "#{rel}:#{lineno}: optional array uses the forbidden " \
                "`= emptyList()` sentinel — must be `List<T>? = null`"
    elsif !nullable
      errors << "#{rel}:#{lineno}: non-null array `#{type_part}` carries a " \
                "default (`= #{default_part}`) — an optional array must be " \
                "nullable (`List<T>? = null`)"
    elsif default_part != "null"
      errors << "#{rel}:#{lineno}: nullable array defaults to " \
                "`#{default_part}` — must default to `null`"
    end
  end
end

if errors.empty?
  puts "==> Kotlin optional-array invariant clean — #{files.size} generated files scanned"
  exit 0
else
  warn "Kotlin optional-array invariant failed:"
  errors.each { |e| warn "  - #{e}" }
  exit 1
end
