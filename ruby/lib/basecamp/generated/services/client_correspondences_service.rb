# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientCorrespondences operations
    #
    # @generated from OpenAPI spec
    class ClientCorrespondencesService < BaseService

      # List all client correspondences in a project
      # @param sort [String, nil] created_at|updated_at
      # @param direction [String, nil] asc|desc
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(sort: nil, direction: nil, page: nil, max_items: nil)
        wrap_paginated(service: "clientcorrespondences", operation: "list", is_mutation: false) do
          params = compact_query_params(sort: sort, direction: direction, page: page)
          paginate("/client/correspondences.json", params: params, operation: "ListClientCorrespondences", max_items: max_items)
        end
      end

      # Get a single client correspondence by id
      # @param correspondence_id [Integer] correspondence id ID
      # @return [Hash] response data
      def get(correspondence_id:)
        with_operation(service: "clientcorrespondences", operation: "get", is_mutation: false, resource_id: correspondence_id) do
          http_get("/client/correspondences/#{correspondence_id}", operation: "GetClientCorrespondence").json
        end
      end
    end
  end
end
