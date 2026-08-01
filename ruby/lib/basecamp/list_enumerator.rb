# frozen_string_literal: true

module Basecamp
  # A lazy Enumerator over paginated items that carries pagination metadata.
  #
  # Behaves exactly like the plain Enumerator it replaces — each/next/peek/
  # take/first/lazy all work, and pages beyond the first are fetched only as
  # iteration demands them — while additionally exposing {#meta}.
  #
  # The metadata object is shared with the paginator: meta.total_count is
  # populated from the eagerly fetched first page, and meta.truncated is
  # finalized by consuming the enumeration (see {ListMeta}).
  class ListEnumerator < Enumerator
    # @return [ListMeta] pagination metadata
    attr_reader :meta

    # @param meta [ListMeta] metadata shared with the producing paginator
    def initialize(meta, &block)
      @meta = meta
      super(&block)
    end
  end
end
