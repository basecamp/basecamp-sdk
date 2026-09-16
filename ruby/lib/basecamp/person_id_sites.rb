# frozen_string_literal: true

require "json"

module Basecamp
  # Decodes a person's id at the response paths the reference reads it as
  # +types.FlexibleInt64+, keyed by operation id.
  #
  # Go decodes every generated read through +Parse<Op>Response+, and
  # +generated.Person.Id+ is +types.FlexibleInt64+
  # (go/pkg/types/flexible_int64.go:27): an untagged +{"creator":{"id":"7"}}+
  # is 7 there. Ruby returns the parsed Hash, and the normalizer converts only
  # a person carrying +personable_type+ (plus creator/participants on the gauge
  # and notification surfaces), so that id reached the caller as the string
  # "7" (SPEC §10 "Person Ids Off the Wire").
  #
  # The fix is NOT a wider normalizer walk. The same key names hold plain int64
  # ids in the reference (+UpcomingSchedulePerson+, +MyAssignmentAssignee+,
  # +OutOfOfficePerson+, +TemplateLibraryConfirmationPerson+), where a string
  # is a decode error. So the sites come from the generated table in
  # +generated/metadata.json+ ("personIdSites"), which +generate-metadata.rb+
  # selects on the +x-go-type+ marker of +Person.id+ and never on a key name:
  # an operation not in the table, or a path not under it, is never touched.
  #
  # Runs AFTER {Http.normalize_person_ids} on the same body. Idempotent: a
  # second pass finds only integers and leaves them.
  module PersonIdSites
    HINT = "A person id in this response is neither a 64-bit integer nor a string, " \
           "or is a string out of int64 range; the response cannot be read."

    class << self
      # Decodes the person ids at +operation+'s sites in +body+, in place.
      #
      # +body+ is what the table's paths are relative to: the whole response
      # for a single read, and each PAGE for a paginated one — the array for a
      # bare-array list, the wrapper object for a wrapped one.
      #
      # @param body [Object] parsed, normalized JSON
      # @param operation [String, nil] canonical operation id
      # @return [Object] +body+
      # @raise [ApiError] non-retryable, when a site's id cannot be decoded
      def decode!(body, operation)
        paths = operation && table[operation]
        return body unless paths

        paths.each { |path, array_site| visit(body, path, 0, array_site, operation) }
        body
      end

      # {operationId => [[components, array_site], ...]}, memoized. Benign race:
      # concurrent first loads compute identical values.
      def table
        @table ||= JSON.parse(
          File.read(File.join(__dir__, "generated", "metadata.json"), encoding: "UTF-8")
        ).fetch("personIdSites").transform_values do |paths|
          paths.map do |path|
            components = path == "$" ? [] : path.split(".")
            array_site = components.last == "[]"
            components = components[0...-1] if array_site
            [ components.freeze, array_site ].freeze
          end.freeze
        end.freeze
      end

      private

      # Follows the components; a value of another shape on the way ends the
      # walk with nothing done. Go zero-fills or refuses those as part of its
      # whole-body decode, which this lenient tier does for no field — that
      # residual is not the person id's to close.
      def visit(value, components, index, array_site, operation)
        if index == components.length
          if array_site
            value.each { |person| decode_person(person, components, operation) } if value.is_a?(Array)
          else
            decode_person(value, components, operation)
          end
          return
        end

        component = components[index]
        if component == "[]"
          value.each { |element| visit(element, components, index + 1, array_site, operation) } if value.is_a?(Array)
        elsif value.is_a?(Hash) && value.key?(component)
          visit(value[component], components, index + 1, array_site, operation)
        end
      end

      # FlexibleInt64.UnmarshalJSON on one person's "id", present only: an
      # absent key is the zero value with no call in Go, so it stays absent.
      def decode_person(person, components, operation)
        return unless person.is_a?(Hash) && person.key?("id")

        raw = person["id"]
        # One rule for a person id off the wire: {Ids.person_from_wire} is the
        # flexible decoder — an int64 passes, a string goes through ParseInt
        # (ErrSyntax reads 0, flexible_int64.go:46; ErrRange fails,
        # flexible_int64.go:43), and a float, null, boolean, array, object or
        # out-of-range number fails (json.Number.Int64, flexible_int64.go:57).
        # No system_label: the decoder writes none.
        id = Ids.person_from_wire(raw)
        raise refusal(raw, components, operation) if id.nil?

        person["id"] = id unless raw.is_a?(Integer)
      end

      # Names the operation, the site and the value's kind — not the value,
      # which is the response's and travels into logs from here.
      def refusal(raw, components, operation)
        site = components.empty? ? "the response body" : components.join(".")
        shape = case raw
        when String, Integer then "an out-of-range #{raw.class}"
        when nil then "null"
        else "a #{raw.class}"
        end
        ApiError.new("#{operation} returned a person id at #{site} that is #{shape}", hint: HINT, retryable: false)
      end
    end
  end
end
