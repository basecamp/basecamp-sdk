#!/usr/bin/env ruby
# frozen_string_literal: true

# Extracts x-basecamp-* extensions from OpenAPI spec into a runtime-accessible metadata file.
# This allows the Ruby SDK to read operation metadata at runtime for retry, pagination, etc.
# It also emits +personIdSites+, the per-operation table of response paths the
# generated read path decodes a person id at (see {Basecamp::PersonIdSites}).
#
# Usage: ruby scripts/ruby/generate-metadata.rb > lib/basecamp/generated/metadata.json

require 'json'
require 'time'

# Extract metadata from OpenAPI spec
class MetadataExtractor
  METHODS = %w[get post put patch delete].freeze

  def initialize(openapi_path)
    # Read as UTF-8 regardless of process locale (LC_ALL=C would otherwise read
    # as US-ASCII and JSON.parse dies on the spec's multibyte characters)
    @openapi = JSON.parse(File.read(openapi_path, encoding: 'UTF-8'))
  end

  def extract
    operations = {}

    (@openapi['paths'] || {}).each_value do |path_item|
      METHODS.each do |method|
        operation = path_item[method]
        next unless operation

        operation_id = operation['operationId']
        next unless operation_id

        metadata = extract_operation_metadata(operation)
        operations[operation_id] = metadata if metadata.any?
      end
    end

    {
      '$schema' => 'https://basecamp.com/schemas/sdk-metadata.json',
      'version' => '1.0.0',
      'generated' => Time.now.utc.iso8601,
      'operations' => operations,
      'personIdSites' => person_id_sites
    }
  end

  # The Go-type marker that makes a schema's +id+ a flexible person id. Go
  # generates +Person.Id+ as +types.FlexibleInt64+ from exactly this extension,
  # and it is the only field that carries it, so the table is selected on the
  # marker and never on a key name: +UpcomingSchedulePerson+,
  # +MyAssignmentAssignee+, +OutOfOfficePerson+ and
  # +TemplateLibraryConfirmationPerson+ sit under the same key names with a
  # plain int64 id, where a string is a decode error in the reference.
  FLEXIBLE_PERSON_ID = 'types.FlexibleInt64'

  # {operationId => [path, ...]} for every operation whose 2xx JSON response
  # reaches a schema whose +id+ is {FLEXIBLE_PERSON_ID}. A path is its
  # components joined by "."; "[]" is an array element, and a path that ENDS
  # in "[]" names an array of people. "$" is the body itself.
  def person_id_sites
    sites = {}
    (@openapi['paths'] || {}).each_value do |path_item|
      METHODS.each do |method|
        operation = path_item[method]
        next unless operation && operation['operationId']

        paths = (operation['responses'] || {}).flat_map do |code, response|
          next [] unless code.to_s.start_with?('2')

          response = resolve_response(response)
          (response['content'] || {}).values.flat_map do |media|
            found = []
            walk_person_sites(media['schema'] || {}, [], [], found)
            found
          end
        end.uniq.sort
        sites[operation['operationId']] = paths if paths.any?
      end
    end
    sites.sort.to_h
  end

  def resolve_response(response)
    return response unless response['$ref']

    @openapi.dig('components', 'responses', response['$ref'].split('/').last)
  end

  # Walks $ref, allOf/oneOf/anyOf, items and properties. A schema already on
  # the $ref stack is a cycle and is not re-entered. additionalProperties has
  # no runtime path syntax, so a person reached through one fails generation
  # rather than being silently left out of the table.
  def walk_person_sites(schema, path, stack, found)
    if (ref = schema['$ref'])
      name = ref.split('/').last
      return if stack.include?(name)

      target = @openapi.dig('components', 'schemas', name)
      found << (path.empty? ? '$' : path.join('.')) if target.dig('properties', 'id', 'x-go-type') == FLEXIBLE_PERSON_ID
      walk_person_sites(target, path, stack + [ name ], found)
      return
    end

    %w[allOf oneOf anyOf].each do |key|
      (schema[key] || []).each { |sub| walk_person_sites(sub, path, stack, found) }
    end
    walk_person_sites(schema['items'] || {}, path + [ '[]' ], stack, found) if schema['type'] == 'array' || schema['items']
    (schema['properties'] || {}).each { |key, sub| walk_person_sites(sub, path + [ key ], stack, found) }

    extra = schema['additionalProperties']
    return unless extra.is_a?(Hash)

    nested = []
    walk_person_sites(extra, path + [ '{}' ], stack, nested)
    raise "person id site under additionalProperties at #{nested.first} has no runtime path syntax" if nested.any?
  end

  private

  def extract_operation_metadata(operation)
    metadata = {}

    # Extract x-basecamp-retry
    if (retry_config = operation['x-basecamp-retry'])
      metadata['retry'] = {
        'maxAttempts' => retry_config['maxAttempts'],
        'baseDelayMs' => retry_config['baseDelayMs'],
        'backoff' => retry_config['backoff'],
        'retryOn' => retry_config['retryOn']
      }
    end

    # Extract x-basecamp-pagination
    if (pagination = operation['x-basecamp-pagination'])
      metadata['pagination'] = {
        'style' => pagination['style'],
        'totalCountHeader' => pagination['totalCountHeader'],
        'maxPageSize' => pagination['maxPageSize']
      }.compact
    end

    # Extract x-basecamp-idempotent
    if (idempotent = operation['x-basecamp-idempotent'])
      metadata['idempotent'] = {
        'keySupported' => idempotent['keySupported'],
        'keyHeader' => idempotent['keyHeader'],
        'natural' => idempotent['natural']
      }.compact
    end

    # Extract x-basecamp-sensitive
    if (sensitive = operation['x-basecamp-sensitive'])
      metadata['sensitive'] = sensitive.map do |s|
        {
          'field' => s['field'],
          'category' => s['category'],
          'redact' => s['redact']
        }.compact
      end
    end

    metadata
  end
end

# Main execution
if __FILE__ == $PROGRAM_NAME
  openapi_path = ARGV[0] || File.expand_path('../../openapi.json', __dir__)

  unless File.exist?(openapi_path)
    warn "Error: OpenAPI file not found: #{openapi_path}"
    exit 1
  end

  extractor = MetadataExtractor.new(openapi_path)
  metadata = extractor.extract

  puts JSON.pretty_generate(metadata)
end
