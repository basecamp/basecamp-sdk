# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Search operations
    #
    # @generated from OpenAPI spec
    class SearchService < BaseService

      # Search for content across the account
      # @param q [String] q
      # @param sort [String, nil] created_at|updated_at
      # @return [Enumerator<Hash>] paginated results
      def search(q:, sort: nil)
        wrap_paginated(service: "search", operation: "search", is_mutation: false) do
          params = compact_params(q: q, sort: sort)
          paginate("/search.json", params: params)
        end
      end

      # Get search metadata (available filter options)
      # @return [Hash] response data
      def metadata()
        with_operation(service: "search", operation: "metadata", is_mutation: false) do
          http_get("/searches/metadata.json").json
        end
      end
    end
  end
end
