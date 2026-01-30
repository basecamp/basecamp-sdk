# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientCorrespondences operations
    #
    # @generated from OpenAPI spec
    class ClientCorrespondencesService < BaseService

      # List all client correspondences in a project
      def list(project_id:)
        paginate(bucket_path(project_id, "/client/correspondences.json"))
      end

      # Get a single client correspondence by id
      def get(project_id:, correspondence_id:)
        http_get(bucket_path(project_id, "/client/correspondences/#{correspondence_id}")).json
      end
    end
  end
end
