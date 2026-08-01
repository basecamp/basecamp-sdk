# frozen_string_literal: true

module Basecamp
  # Pagination metadata carried by {ListEnumerator}.
  #
  # total_count comes from the X-Total-Count header on the first page (0 when
  # absent) and is available as soon as the list call returns, because the
  # first page is fetched eagerly.
  #
  # truncated starts false and flips to true when enumeration discovers that
  # items beyond those yielded were available: items dropped by a max_items
  # cap, or a next Link left unfetched when the max_items or max_pages cap
  # stopped pagination. It is final only once enumeration completes; landing
  # exactly on the final item is not truncation.
  class ListMeta
    # @return [Integer] total item count from X-Total-Count (0 if absent)
    attr_reader :total_count

    # @return [Boolean] whether items beyond those yielded were available
    attr_reader :truncated

    alias truncated? truncated

    # @param total_count [Integer] value of the X-Total-Count header
    def initialize(total_count: 0)
      @total_count = total_count
      @truncated = false
    end

    # Records that truncation was discovered during enumeration.
    # @api private
    def mark_truncated!
      @truncated = true
    end

    # Re-initializes the metadata when a traversal restarts, so it describes
    # the restarted pass's own snapshot rather than a previous traversal's.
    # @api private
    def restart!(total_count:)
      @total_count = total_count
      @truncated = false
    end
  end
end
