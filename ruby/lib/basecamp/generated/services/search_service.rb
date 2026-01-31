# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Search operations
    #
    # @generated from OpenAPI spec
    class SearchService < BaseService

      # Search for content across the account
      # @param query [String] query
      # @param sort [String, nil] created_at|updated_at
      # @return [Hash] response data
      def search(query:, sort: nil)
        http_get("/search.json", params: compact_params(query: query, sort: sort)).json
      end

      # Get search metadata (available filter options)
      # @return [Hash] response data
      def metadata()
        http_get("/searches/metadata.json").json
      end
    end
  end
end
