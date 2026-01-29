# frozen_string_literal: true

module Basecamp
  module Services
    # Service for search operations.
    #
    # Provides full-text search across all content in your Basecamp account.
    #
    # @example Search for content
    #   results = account.search.search(query: "quarterly report")
    #   results.each { |r| puts r["title"] }
    #
    # @example Get search metadata
    #   metadata = account.search.metadata
    #   puts "Available projects: #{metadata["projects"].length}"
    class SearchService < BaseService
      # Searches for content across the account.
      #
      # @param query [String] the search query string
      # @param sort [String, nil] sort order ("created_at" or "updated_at")
      # @return [Enumerator<Hash>] search results
      def search(query:, sort: nil)
        params = compact_params(query: query, sort: sort)
        paginate("/search.json", params: params)
      end

      # Returns metadata about available search scopes.
      # This includes the list of projects available for filtering.
      #
      # @return [Hash] search metadata with available filter options
      def metadata
        http_get("/searches/metadata.json").json
      end
    end
  end
end
