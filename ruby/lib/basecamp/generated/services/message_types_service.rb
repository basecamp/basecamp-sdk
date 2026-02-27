# frozen_string_literal: true

module Basecamp
  module Services
    # Service for MessageTypes operations
    #
    # @generated from OpenAPI spec
    class MessageTypesService < BaseService

      # List message types in a project
      # @return [Enumerator<Hash>] paginated results
      def list()
        wrap_paginated(service: "messagetypes", operation: "list", is_mutation: false) do
          paginate("/categories.json")
        end
      end

      # Create a new message type in a project
      # @param name [String] name
      # @param icon [String] icon
      # @return [Hash] response data
      def create(name:, icon:)
        with_operation(service: "messagetypes", operation: "create", is_mutation: true) do
          http_post("/categories.json", body: compact_params(name: name, icon: icon)).json
        end
      end

      # Get a single message type by id
      # @param type_id [Integer] type id ID
      # @return [Hash] response data
      def get(type_id:)
        with_operation(service: "messagetypes", operation: "get", is_mutation: false, resource_id: type_id) do
          http_get("/categories/#{type_id}").json
        end
      end

      # Update an existing message type
      # @param type_id [Integer] type id ID
      # @param name [String, nil] name
      # @param icon [String, nil] icon
      # @return [Hash] response data
      def update(type_id:, name: nil, icon: nil)
        with_operation(service: "messagetypes", operation: "update", is_mutation: true, resource_id: type_id) do
          http_put("/categories/#{type_id}", body: compact_params(name: name, icon: icon)).json
        end
      end

      # Delete a message type
      # @param type_id [Integer] type id ID
      # @return [void]
      def delete(type_id:)
        with_operation(service: "messagetypes", operation: "delete", is_mutation: true, resource_id: type_id) do
          http_delete("/categories/#{type_id}")
          nil
        end
      end
    end
  end
end
