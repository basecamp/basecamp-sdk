# frozen_string_literal: true

module Basecamp
  module Services
    # Service for MessageTypes operations
    #
    # @generated from OpenAPI spec
    class MessageTypesService < BaseService

      # List message types in a project
      # @param project_id [Integer] project id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:)
        wrap_paginated(service: "messagetypes", operation: "list", is_mutation: false, project_id: project_id) do
          paginate(bucket_path(project_id, "/categories.json"))
        end
      end

      # Create a new message type in a project
      # @param project_id [Integer] project id ID
      # @param name [String] name
      # @param icon [String] icon
      # @return [Hash] response data
      def create(project_id:, name:, icon:)
        with_operation(service: "messagetypes", operation: "create", is_mutation: true, project_id: project_id) do
          http_post(bucket_path(project_id, "/categories.json"), body: compact_params(name: name, icon: icon)).json
        end
      end

      # Get a single message type by id
      # @param project_id [Integer] project id ID
      # @param type_id [Integer] type id ID
      # @return [Hash] response data
      def get(project_id:, type_id:)
        with_operation(service: "messagetypes", operation: "get", is_mutation: false, project_id: project_id, resource_id: type_id) do
          http_get(bucket_path(project_id, "/categories/#{type_id}")).json
        end
      end

      # Update an existing message type
      # @param project_id [Integer] project id ID
      # @param type_id [Integer] type id ID
      # @param name [String, nil] name
      # @param icon [String, nil] icon
      # @return [Hash] response data
      def update(project_id:, type_id:, name: nil, icon: nil)
        with_operation(service: "messagetypes", operation: "update", is_mutation: true, project_id: project_id, resource_id: type_id) do
          http_put(bucket_path(project_id, "/categories/#{type_id}"), body: compact_params(name: name, icon: icon)).json
        end
      end

      # Delete a message type
      # @param project_id [Integer] project id ID
      # @param type_id [Integer] type id ID
      # @return [void]
      def delete(project_id:, type_id:)
        with_operation(service: "messagetypes", operation: "delete", is_mutation: true, project_id: project_id, resource_id: type_id) do
          http_delete(bucket_path(project_id, "/categories/#{type_id}"))
          nil
        end
      end
    end
  end
end
