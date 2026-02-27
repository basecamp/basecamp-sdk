# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientCorrespondences operations
    #
    # @generated from OpenAPI spec
    class ClientCorrespondencesService < BaseService

      # List all client correspondences in a project
      # @return [Enumerator<Hash>] paginated results
      def list()
        wrap_paginated(service: "clientcorrespondences", operation: "list", is_mutation: false) do
          paginate("/client/correspondences.json")
        end
      end

      # Get a single client correspondence by id
      # @param correspondence_id [Integer] correspondence id ID
      # @return [Hash] response data
      def get(correspondence_id:)
        with_operation(service: "clientcorrespondences", operation: "get", is_mutation: false, resource_id: correspondence_id) do
          http_get("/client/correspondences/#{correspondence_id}").json
        end
      end
    end
  end
end
