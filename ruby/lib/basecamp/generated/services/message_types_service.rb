# frozen_string_literal: true

module Basecamp
  module Services
    # Service for MessageTypes operations
    #
    # @generated from OpenAPI spec
    class MessageTypesService < BaseService

      # List message types in a project
      # @param bucket_id [Integer] bucket id ID
      # @return [Enumerator<Hash>] paginated results
      def list(bucket_id:)
        wrap_paginated(service: "messagetypes", operation: "list", is_mutation: false, project_id: bucket_id) do
          paginate("/buckets/#{bucket_id}/categories.json", operation: "ListMessageTypes")
        end
      end

      # Create a new message type in a project
      # @param bucket_id [Integer] bucket id ID
      # @param name [String] name
      # @param icon [String] icon
      # @return [Hash] response data
      def create(bucket_id:, name:, icon:)
        with_operation(service: "messagetypes", operation: "create", is_mutation: true, project_id: bucket_id) do
          http_post("/buckets/#{bucket_id}/categories.json", body: compact_params(name: name, icon: icon)).json
        end
      end

      # Get a single message type by id
      # @param bucket_id [Integer] bucket id ID
      # @param type_id [Integer] type id ID
      # @return [Hash] response data
      def get(bucket_id:, type_id:)
        with_operation(service: "messagetypes", operation: "get", is_mutation: false, project_id: bucket_id, resource_id: type_id) do
          http_get("/buckets/#{bucket_id}/categories/#{type_id}", operation: "GetMessageType").json
        end
      end

      # Update an existing message type
      # @param bucket_id [Integer] bucket id ID
      # @param type_id [Integer] type id ID
      # @param name [String, nil] name
      # @param icon [String, nil] icon
      # @return [Hash] response data
      def update(bucket_id:, type_id:, name: nil, icon: nil)
        with_operation(service: "messagetypes", operation: "update", is_mutation: true, project_id: bucket_id, resource_id: type_id) do
          http_put("/buckets/#{bucket_id}/categories/#{type_id}", body: compact_params(name: name, icon: icon)).json
        end
      end

      # Delete a message type
      # @param bucket_id [Integer] bucket id ID
      # @param type_id [Integer] type id ID
      # @return [void]
      def delete(bucket_id:, type_id:)
        with_operation(service: "messagetypes", operation: "delete", is_mutation: true, project_id: bucket_id, resource_id: type_id) do
          http_delete("/buckets/#{bucket_id}/categories/#{type_id}")
          nil
        end
      end
    end
  end
end
