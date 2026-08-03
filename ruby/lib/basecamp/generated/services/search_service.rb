# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Search operations
    #
    # @generated from OpenAPI spec
    class SearchService < BaseService

      # Search for content across the account
      # @param q [String] q
      # @param type_names [Array, nil] Recording types to include. Use `key` values from the metadata
      #   endpoint's `recording_search_types`. Available since Basecamp 5.
      # @param bucket_ids [Array, nil] Project IDs to filter by. Available since Basecamp 5.
      # @param creator_ids [Array, nil] Creator person IDs to filter by. Available since Basecamp 5.
      # @param file_type [String, nil] Filter attachments by type. Use `key` values from the metadata
      #   endpoint's `file_search_types`.
      # @param exclude_chat [Boolean, nil] Set to true to exclude chat results.
      # @param since [String, nil] last_7_days|last_30_days|last_90_days|last_12_months|forever
      # @param sort [String, nil] best_match|recency
      # @param type [String, nil] Deprecated: prefer type_names[].
      # @param bucket_id [Integer, nil] Deprecated: prefer bucket_ids[].
      # @param creator_id [Integer, nil] Deprecated: prefer creator_ids[].
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def search(q:, type_names: nil, bucket_ids: nil, creator_ids: nil, file_type: nil, exclude_chat: nil, since: nil, sort: nil, type: nil, bucket_id: nil, creator_id: nil, page: nil, max_items: nil)
        wrap_paginated(service: "search", operation: "search", is_mutation: false) do
          params = compact_query_params(q: q, type_names: type_names, bucket_ids: bucket_ids, creator_ids: creator_ids, file_type: file_type, exclude_chat: exclude_chat, since: since, sort: sort, type: type, bucket_id: bucket_id, creator_id: creator_id, page: page)
          paginate("/search.json", params: params, operation: "Search", max_items: max_items)
        end
      end

      # Get search metadata (available filter options)
      # @return [Hash] response data
      def metadata()
        with_operation(service: "search", operation: "metadata", is_mutation: false) do
          http_get("/searches/metadata.json", operation: "GetSearchMetadata").json
        end
      end
    end
  end
end
