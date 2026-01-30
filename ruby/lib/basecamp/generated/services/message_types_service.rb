# frozen_string_literal: true

module Basecamp
  module Services
    # Service for MessageTypes operations
    #
    # @generated from OpenAPI spec
    class MessageTypesService < BaseService

      # List message types in a project
      def list(project_id:)
        paginate(bucket_path(project_id, "/categories.json"))
      end

      # Create a new message type in a project
      def create(project_id:, **body)
        http_post(bucket_path(project_id, "/categories.json"), body: body).json
      end

      # Get a single message type by id
      def get(project_id:, type_id:)
        http_get(bucket_path(project_id, "/categories/#{type_id}")).json
      end

      # Update an existing message type
      def update(project_id:, type_id:, **body)
        http_put(bucket_path(project_id, "/categories/#{type_id}"), body: body).json
      end

      # Delete a message type
      def delete(project_id:, type_id:)
        http_delete(bucket_path(project_id, "/categories/#{type_id}"))
        nil
      end
    end
  end
end
