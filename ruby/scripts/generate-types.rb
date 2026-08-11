#!/usr/bin/env ruby
# frozen_string_literal: true

# Generates Ruby type classes from OpenAPI schemas.
# Creates classes with JSON serialization support.
#
# Usage: ruby scripts/ruby/generate-types.rb > lib/basecamp/generated/types.rb

require 'json'
require 'set'
require 'time'
require_relative 'go_type_spellings'

# Schemas to skip (internal/generated response wrappers)
SKIP_PATTERNS = [
  /ResponseContent$/,
  /RequestContent$/,
  /InputPayload$/,
  /ErrorResponseContent$/
].freeze

def header
  <<~HEADER
    # frozen_string_literal: true

    # Auto-generated from OpenAPI spec. Do not edit manually.
    # Generated: #{Time.now.utc.iso8601}

    require "json"
    require "time"
  HEADER
end

def generate_helpers
  <<~HELPERS

    # Type conversion helpers
    module TypeHelpers
      module_function

      def identity(value)
        value
      end

      def parse_integer(value)
        return nil if value.nil?
        value.to_i
      end

      def parse_float(value)
        return nil if value.nil?
        value.to_f
      end

      def parse_boolean(value)
        return nil if value.nil?
        !!value
      end

      def parse_datetime(value)
        return nil if value.nil?
        return value if value.is_a?(Time)
        Time.parse(value.to_s)
      rescue ArgumentError
        nil
      end

      def parse_type(value, type_name)
        return nil if value.nil?
        return value unless value.is_a?(Hash)

        type_class = Basecamp::Types.const_get(type_name)
        type_class.new(value)
      rescue NameError
        value
      end

      def parse_array(value, type_name)
        return nil if value.nil?
        return value unless value.is_a?(Array)

        type_class = Basecamp::Types.const_get(type_name)
        value.map { |item| item.is_a?(Hash) ? type_class.new(item) : item }
      rescue NameError
        value
      end
    end
  HELPERS
end

# Renders a deprecation +reason+ as one or more Ruby comment lines (#406).
#
# A reason can be sourced from a multi-line OpenAPI description. Interpolating
# one into a single "# @deprecated ..." line would leave every continuation on a
# bare, un-commented source line, which is a syntax error in the generated
# types.rb — Ruby is the only owned generator exposed this way (Python escapes
# the reason, Kotlin/TypeScript emit block comments). Splitting on line
# boundaries and prefixing each line with the comment leader keeps the output
# valid for any reason.
#
# +indent+ is the leading source indentation; +tag_prefix+ is inserted between
# the "# " leader and the YARD tag (the per-attribute site nests its tag under
# an @!attribute directive). Continuation lines are indented two columns past
# the tag so YARD folds them into the same tag's text.
#
# Single-line reasons render byte-identically to the previous single-line
# interpolation, so this is output-neutral for the current spec.
def deprecation_doc_lines(reason, indent:, tag_prefix: '')
  lines = reason.to_s.split(/\r?\n/, -1)
  out = [ "#{indent}# #{tag_prefix}@deprecated #{lines.first}" ]
  continuation = "#{indent}# #{tag_prefix}  "
  lines.drop(1).each do |line|
    out << (line.empty? ? "#{indent}#" : "#{continuation}#{line}")
  end
  out
end

# Main execution
if __FILE__ == $PROGRAM_NAME
  openapi_path = ARGV[0] || File.expand_path('../../openapi.json', __dir__)

  unless File.exist?(openapi_path)
    warn "Error: OpenAPI file not found: #{openapi_path}"
    exit 1
  end

  puts header
  puts generate_helpers
  puts ''
  puts 'module Basecamp'
  puts '  module Types'
  puts '    include TypeHelpers'

  # UTF-8 regardless of process locale — see generate-metadata.rb
  schemas = JSON.parse(File.read(openapi_path, encoding: 'UTF-8'))['components']['schemas'] || {}
  sorted = schemas.keys.sort

  sorted.each do |name|
    next if SKIP_PATTERNS.any? { |p| name.match?(p) }

    schema = schemas[name]
    next unless schema['type'] == 'object'

    properties = schema['properties'] || {}
    next if properties.empty?

    required_fields = schema['required'] || []
    required_set = required_fields.to_set

    required_props = properties.keys.select { |k| required_set.include?(k) }.sort
    optional_props = properties.keys.reject { |k| required_set.include?(k) }.sort
    ordered_props = required_props + optional_props

    puts ''
    puts "    # #{name}"
    # Documentation-only deprecation (see #406): YARD marks a whole class/method,
    # not individual params, so a class-level @deprecated tag documents a wholly
    # deprecated type.
    if schema['deprecated']
      puts deprecation_doc_lines(schema['x-deprecated-reason'] || 'deprecated', indent: '    ')
    end
    puts "    class #{name}"
    puts '      include TypeHelpers'

    attr_names = ordered_props.map { |k| k.gsub(/([A-Z])/, '_\1').downcase.gsub(/^_/, '') }
    # Add system_label for schemas with flexible integer fields
    has_flexible = ordered_props.any? { |k| properties[k]['x-go-type']&.include?('FlexibleInt64') }
    attr_names << 'system_label' if has_flexible

    # Per-attribute deprecation. The accessors are declared in one grouped
    # attr_accessor, so a bare comment would wrongly document every attribute;
    # a YARD @!attribute directive scopes the @deprecated tag to just this one.
    ordered_props.each do |k|
      ps = properties[k]
      next unless ps['deprecated']

      ruby_name = k.gsub(/([A-Z])/, '_\1').downcase.gsub(/^_/, '')
      puts "      # @!attribute [rw] #{ruby_name}"
      puts deprecation_doc_lines(ps['x-deprecated-reason'] || 'deprecated', indent: '      ', tag_prefix: '  ')
    end

    puts "      attr_accessor #{attr_names.map { |n| ":#{n}" }.join(", ")}"

    unless required_props.empty?
      required_symbols = required_props.map { |k| k.gsub(/([A-Z])/, '_\1').downcase.gsub(/^_/, '') }
      puts ''
      puts '      # @return [Array<Symbol>]'
      puts '      def self.required_fields'
      puts "        %i[#{required_symbols.join(' ')}].freeze"
      puts '      end'
    end

    puts ''

    puts '      def initialize(data = {})'
    ordered_props.each do |prop_name|
      prop_schema = properties[prop_name]
      attr_name = prop_name.gsub(/([A-Z])/, '_\1').downcase.gsub(/^_/, '')

      converter = if prop_schema['$ref']
                    ref_name = prop_schema['$ref'].split('/').last
                    "parse_type(data[\"#{prop_name}\"], \"#{ref_name}\")"
      elsif prop_schema['type'] == 'array' && prop_schema.dig('items', '$ref')
                    ref_name = prop_schema['items']['$ref'].split('/').last
                    "parse_array(data[\"#{prop_name}\"], \"#{ref_name}\")"
      elsif prop_schema['type'] == 'integer'
                    "parse_integer(data[\"#{prop_name}\"])"
      elsif prop_schema['type'] == 'number'
                    "parse_float(data[\"#{prop_name}\"])"
      elsif prop_schema['type'] == 'boolean'
                    "parse_boolean(data[\"#{prop_name}\"])"
      elsif GoTypeSpellings.timestamp_go_type?(prop_schema['x-go-type'])
                    "parse_datetime(data[\"#{prop_name}\"])"
      else
                    "data[\"#{prop_name}\"]"
      end

      puts "        @#{attr_name} = #{converter}"
      # Add system_label after flexible integer id fields
      if prop_schema['x-go-type']&.include?('FlexibleInt64')
        puts '        @system_label = data["system_label"]'
      end
    end
    puts '      end'
    puts ''

    puts '      def to_h'
    puts '        {'
    ordered_props.each do |prop_name|
      attr_name = prop_name.gsub(/([A-Z])/, '_\1').downcase.gsub(/^_/, '')
      puts "          \"#{prop_name}\" => @#{attr_name},"
    end
    # A required-and-nullable field (OpenAPI 3.1 `type: [..., "null"]`) carries an
    # explicit null that must survive to_h; plain .compact would drop it. Keep
    # nil for exactly those keys — required-but-non-nullable fields still get
    # dropped when nil, like any other nil. Everything else keeps .compact.
    required_nullable = required_fields.select do |k|
      t = properties[k] && properties[k]['type']
      t.is_a?(Array) && t.include?('null')
    end
    if required_nullable.any?
      keep_list = required_nullable.map { |k| "\"#{k}\"" }.join(', ')
      puts "        }.reject { |k, v| v.nil? && ![#{keep_list}].include?(k) }"
    else
      puts '        }.compact'
    end
    puts '      end'
    puts ''

    puts '      def to_json(*args)'
    puts '        to_h.to_json(*args)'
    puts '      end'

    puts '    end'
  end

  puts '  end'
  puts 'end'
end
