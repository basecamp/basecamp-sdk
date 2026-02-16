# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientCorrespondences operations
    #
    # @generated from OpenAPI spec
    class ClientCorrespondencesService < BaseService

      # List all client correspondences in a project
      # @param project_id [Integer] project id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:)
        wrap_paginated(service: "clientcorrespondences", operation: "list", is_mutation: false, project_id: project_id) do
          paginate(bucket_path(project_id, "/client/correspondences.json"))
        end
      end

      # Get a single client correspondence by id
      # @param project_id [Integer] project id ID
      # @param correspondence_id [Integer] correspondence id ID
      # @return [Hash] response data
      def get(project_id:, correspondence_id:)
        with_operation(service: "clientcorrespondences", operation: "get", is_mutation: false, project_id: project_id, resource_id: correspondence_id) do
          http_get(bucket_path(project_id, "/client/correspondences/#{correspondence_id}")).json
        end
      end
    end
  end
end
