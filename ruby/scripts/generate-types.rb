#!/usr/bin/env ruby
# frozen_string_literal: true

# Generates Ruby type classes from OpenAPI schemas.
# Creates classes with JSON serialization support.
#
# Usage: ruby scripts/ruby/generate-types.rb > lib/basecamp/generated/types.rb

require 'json'
require 'time'

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

  schemas = JSON.parse(File.read(openapi_path))['components']['schemas'] || {}
  sorted = schemas.keys.sort

  sorted.each do |name|
    next if SKIP_PATTERNS.any? { |p| name.match?(p) }

    schema = schemas[name]
    next unless schema['type'] == 'object'

    properties = schema['properties'] || {}
    next if properties.empty?

    puts ''
    puts "    # #{name}"
    puts "    class #{name}"
    puts '      include TypeHelpers'

    attr_names = properties.keys.map { |k| k.gsub(/([A-Z])/, '_\1').downcase.gsub(/^_/, '') }
    puts "      attr_accessor #{attr_names.map { |n| ":#{n}" }.join(", ")}"
    puts ''

    puts '      def initialize(data = {})'
    properties.each do |prop_name, prop_schema|
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
      elsif prop_schema['x-go-type'] == 'time.Time'
                    "parse_datetime(data[\"#{prop_name}\"])"
      else
                    "data[\"#{prop_name}\"]"
      end

      puts "        @#{attr_name} = #{converter}"
    end
    puts '      end'
    puts ''

    puts '      def to_h'
    puts '        {'
    properties.each_key do |prop_name|
      attr_name = prop_name.gsub(/([A-Z])/, '_\1').downcase.gsub(/^_/, '')
      puts "          \"#{prop_name}\" => @#{attr_name},"
    end
    puts '        }.compact'
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
