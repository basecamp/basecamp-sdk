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
  # A Path Item Object is read by EXCLUSION: its non-operation fields are a
  # closed, spec-defined set, its extensions are `x-` prefixed, and every other
  # field is an operation. A verb list here dropped an operation's metadata in
  # silence for any method outside the five it knew (#925). Nothing in this
  # file is verb-specific — metadata is keyed by operationId — so the walk is
  # total and no verb bound belongs here.
  NON_OPERATION_FIELDS = %w[summary description servers parameters].freeze

  # Membership is decided above, by exclusion. The shared declaration every
  # generator reads (spec/generated-verbs.json) is ORDERED, and that order is
  # reused here to keep metadata.json byte-stable. Order is not membership: a
  # verb absent from the declaration sorts deterministically to the end by name
  # and is still extracted, because nothing in this file is verb-specific.
  # The self-test points this at a crafted declaration to prove the bound is
  # sourced from the shared file rather than a private literal; production runs
  # never set it.
  GENERATED_VERBS_FILE = ENV.fetch(
    'BASECAMP_GENERATED_VERBS', File.expand_path('../../spec/generated-verbs.json', __dir__)
  )

  # Read VERBATIM. scripts/check-generated-verbs.rb is the only thing that
  # rejects a malformed declaration, and it is a prerequisite of every
  # *-generate target and a member of `make check`, so nothing gets here without
  # passing it. This loader deliberately performs NO validation: six loaders that
  # each validated disagreed five times in four review rounds, every one of them
  # on invalid input, and each surviving predicate is another chance to disagree.
  METHOD_ORDER = begin
    JSON.parse(File.read(GENERATED_VERBS_FILE, encoding: 'UTF-8')).fetch('verbs').freeze
  rescue Errno::ENOENT, JSON::ParserError, KeyError => e
    abort "Error: cannot read #{GENERATED_VERBS_FILE}: #{e.message}. Run " \
          "'make check-generated-verbs' — it is a prerequisite of every generate target."
  end

  def initialize(openapi_path)
    # Read as UTF-8 regardless of process locale (LC_ALL=C would otherwise read
    # as US-ASCII and JSON.parse dies on the spec's multibyte characters)
    @openapi = JSON.parse(File.read(openapi_path, encoding: 'UTF-8'))
  end

  def extract
    operations = {}

    (@openapi['paths'] || {}).each do |path, path_item|
      each_operation(path, path_item) do |_method, operation|
        operation_id = operation['operationId']

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
    (@openapi['paths'] || {}).each do |path, path_item|
      each_operation(path, path_item) do |_method, operation|
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

  # Yields [verb, operation] for every operation in a path item. A field that is
  # neither a known non-operation field nor a readable operation object stops
  # the run by name rather than being stepped over.
  def each_operation(path, path_item)
    unless path_item.is_a?(Hash)
      abort "Error: openapi.json path #{path} is a #{path_item.class}, not a path item object."
    end

    if path_item.key?('$ref')
      abort "Error: openapi.json path #{path} is a $ref to #{path_item['$ref'].inspect}. " \
            'This extractor cannot resolve a path-item reference, and skipping it would leave ' \
            'every operation behind it without runtime metadata.'
    end

    # OpenAPI 3.2's `additionalOperations` is a MAP of method to Operation, not an
    # operation. Read as one it carries no operationId, so it would drop every
    # operation inside it without saying so. Refuse by name until the walk learns
    # the map shape.
    if path_item.key?('additionalOperations')
      abort "Error: openapi.json path #{path} declares `additionalOperations`, which OpenAPI 3.2 " \
            'defines as a map of method to Operation. This walk reads a path-item field as a ' \
            'single operation, so it would drop every operation inside it. Teach the walk the map ' \
            'shape, or take the field out of the spec.'
    end

    fields = path_item.keys.reject { |f| NON_OPERATION_FIELDS.include?(f) || f.start_with?('x-') }
    fields.sort_by! { |f| [ METHOD_ORDER.index(f) || METHOD_ORDER.length, f ] }

    fields.each do |field|
      operation = path_item[field]

      unless operation.is_a?(Hash)
        abort "Error: openapi.json path #{path} field #{field.inspect} is a #{operation.class}, " \
              'which is neither a known non-operation field nor an operation object. If a later ' \
              'OpenAPI version added it, add it to NON_OPERATION_FIELDS with a reason.'
      end

      # An operation has to be IDENTIFIABLE. OpenAPI lets operationId be omitted,
      # and every walker here used to step over one that was — a silent drop of a
      # real operation, which is #925 wearing a different field.
      op_id = operation['operationId']
      unless op_id.is_a?(String) && !op_id.empty?
        abort "Error: openapi.json declares #{field.upcase} #{path} with no operationId. " \
              'Everything downstream is keyed by it, and skipping the operation would drop it ' \
              'from the SDK in silence.'
      end

      yield field, operation
    end
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
