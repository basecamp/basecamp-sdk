# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Search operations
    #
    # @generated from OpenAPI spec
    class SearchService < BaseService

      # Search for content across the account
      def search(query:, sort: nil)
        http_get("/search.json", params: compact_params(query: query, sort: sort)).json
      end

      # Get search metadata (available filter options)
      def metadata()
        http_get("/searches/metadata.json").json
      end
    end
  end
end
