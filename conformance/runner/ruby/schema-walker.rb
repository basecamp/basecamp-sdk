# frozen_string_literal: true

require "json"

# Pure-Ruby port of conformance/runner/typescript/schema-validator.ts.
#
# Walks parsed JSON against the OpenAPI response schema, surfacing:
#   - missing_required: slash-separated paths for required fields absent
#     from the body (e.g. "owner/id")
#   - extras_seen: dotted paths for fields present on the wire but not
#     declared in the schema (e.g. "unreads[].new_field")
#
# Conventions match the TS walker exactly so cross-language replay output is
# directly comparable:
#   - Required-walk object paths use "/" (e.g. "owner/id"); extras-walk
#     object paths use "." (e.g. "owner.new_field"). The two streams use
#     distinct separators so they're visually distinguishable in tooling.
#   - Array element paths use "[i]" for required walk, "[]" for extras walk
#     (mirrors how the TS validator's collectExtras tags item-level extras and
#     how a per-index required-field check identifies the offending element).
#   - "$ref" chains resolve until a non-ref schema or a cycle. Both
#     "#/components/schemas/X" and "openapi.json#/components/schemas/X" are
#     accepted.
#   - additionalProperties:false is intentionally ignored — extras are reported
#     but do not fail validation (forward-compat).
#   - Bounded recursion depth (12) as a cycle guard.
module Basecamp
  module Conformance
    class SchemaWalker
      MAX_DEPTH = 12

      def initialize(openapi_path)
        @doc = JSON.parse(File.read(openapi_path))
      end

      # Returns the response schema for operation_id, or nil when none exists.
      # Prefers 200, then any explicit 2xx, then "default".
      def find_response_schema(operation_id)
        candidates = %w[200 201 202 203 204 default]
        @doc.fetch("paths", {}).each_value do |path_item|
          path_item.each_value do |op|
            next unless op.is_a?(Hash) && op["operationId"] == operation_id

            responses = op["responses"] || {}
            candidates.each do |code|
              schema = responses.dig(code, "content", "application/json", "schema")
              return schema if schema
            end
            responses.each do |code, response|
              next unless code.is_a?(String) && code =~ /\A2\d\d\z/

              schema = response.dig("content", "application/json", "schema")
              return schema if schema
            end
          end
        end
        nil
      end

      # Returns array of slash-separated path strings for required fields
      # absent from body (e.g. "owner/id").
      def missing_required(body, schema)
        walk_required("", body, schema, 0)
      end

      # Returns array of dotted-path strings for fields present on the wire but
      # not declared in the schema. Recurses through known properties so item-
      # level extras on lists surface (e.g. "unreads[].new_field").
      def extras_seen(body, schema)
        walk_extras("", body, schema, 0)
      end

      private

      def walk_required(prefix, body, schema, depth)
        return [] if depth > MAX_DEPTH

        resolved = resolve_ref(schema)
        return [] unless resolved.is_a?(Hash)

        if body.is_a?(Array)
          return [] unless resolved["type"] == "array" && resolved["items"]

          missing = []
          body.each_with_index do |item, i|
            child_prefix = prefix.empty? ? "[#{i}]" : "#{prefix}[#{i}]"
            missing.concat(walk_required(child_prefix, item, resolved["items"], depth + 1))
          end
          return missing
        end

        return [] unless body.is_a?(Hash)
        return [] if resolved["type"] && resolved["type"] != "object"

        props = resolved["properties"] || {}
        required = resolved["required"] || []
        missing = []

        # Required-field paths use `/` as the separator (extras_seen uses `.`)
        # so the two streams are visually distinct in tooling and consistent
        # across Ruby/Python/Go/Kotlin walkers.
        required.each do |name|
          unless body.key?(name)
            field_path = prefix.empty? ? name : "#{prefix}/#{name}"
            missing << field_path
          end
        end

        body.each do |key, value|
          next unless props.key?(key)

          field_path = prefix.empty? ? key : "#{prefix}/#{key}"
          missing.concat(walk_required(field_path, value, props[key], depth + 1))
        end

        missing
      end

      def walk_extras(prefix, body, schema, depth)
        return [] if depth > MAX_DEPTH
        return [] if body.nil?

        resolved = resolve_ref(schema)
        return [] unless resolved.is_a?(Hash)

        if body.is_a?(Array)
          return [] unless resolved["type"] == "array" && resolved["items"]

          seen = {}
          child_prefix = prefix.empty? ? "[]" : "#{prefix}[]"
          body.each do |item|
            walk_extras(child_prefix, item, resolved["items"], depth + 1).each { |e| seen[e] = true }
          end
          return seen.keys
        end

        return [] unless body.is_a?(Hash)
        return [] if resolved["type"] && resolved["type"] != "object"

        props = resolved["properties"] || {}
        extras = []
        body.each do |key, value|
          field_path = prefix.empty? ? key : "#{prefix}.#{key}"
          if props.key?(key)
            extras.concat(walk_extras(field_path, value, props[key], depth + 1))
          else
            extras << field_path
          end
        end
        extras
      end

      # Follows $ref chains until a non-ref schema or a cycle.
      # Accepts "#/components/schemas/X" and "openapi.json#/components/schemas/X".
      def resolve_ref(schema)
        seen = {}
        current = schema
        while current.is_a?(Hash) && current["$ref"].is_a?(String)
          ref = current["$ref"]
          break if seen[ref]

          seen[ref] = true
          match = ref.match(%r{\A(?:openapi\.json)?#/components/schemas/(.+)\z})
          break unless match

          next_schema = @doc.dig("components", "schemas", match[1])
          break unless next_schema

          current = next_schema
        end
        current
      end
    end
  end
end
