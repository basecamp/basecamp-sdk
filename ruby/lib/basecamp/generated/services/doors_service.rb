# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Doors operations
    #
    # @generated from OpenAPI spec
    class DoorsService < BaseService

      # List the active external links in a project
      # @param bucket_id [Integer] bucket id ID
      # @return [Array<Hash>] response data
      def list(bucket_id:)
        with_operation(service: "doors", operation: "list", is_mutation: false, project_id: bucket_id) do
          http_get("/buckets/#{bucket_id}/dock/doors.json", operation: "ListDoors").json(operation: "ListDoors")
        end
      end

      # Create an external link in a project
      # @param bucket_id [Integer] bucket id ID
      # @param door [Hash] door
      # @param visible_to_clients [Boolean, nil] Whether the project's clients can see the external link. Omitted means
      #   false: the link is private to the team. A client caller always creates a
      #   client-visible link regardless of what is sent.
      # @return [Hash] response data
      def create(bucket_id:, door:, visible_to_clients: nil)
        with_operation(service: "doors", operation: "create", is_mutation: true, project_id: bucket_id) do
          http_post("/buckets/#{bucket_id}/dock/doors.json", body: compact_params(door: door, visible_to_clients: visible_to_clients)).json(operation: "CreateDoor")
        end
      end

      # Get an external link by id
      # @param door_id [Integer] door id ID
      # @return [Hash] response data
      def get(door_id:)
        with_operation(service: "doors", operation: "get", is_mutation: false, resource_id: door_id) do
          http_get("/dock/doors/#{door_id}", operation: "GetDoor").json(operation: "GetDoor")
        end
      end

      # Update an external link
      # @param door_id [Integer] door id ID
      # @param door [Hash, nil] door
      # @param visible_to_clients [Boolean, nil] Whether the project's clients can see the external link. Omitted keeps
      #   the current visibility.
      # @return [Hash] response data
      def update(door_id:, door: nil, visible_to_clients: nil)
        with_operation(service: "doors", operation: "update", is_mutation: true, resource_id: door_id) do
          http_put("/dock/doors/#{door_id}", body: compact_params(door: door, visible_to_clients: visible_to_clients)).json(operation: "UpdateDoor")
        end
      end

      # Delete an external link (returns 204 No Content)
      # @param door_id [Integer] door id ID
      # @return [void]
      def delete(door_id:)
        with_operation(service: "doors", operation: "delete", is_mutation: true, resource_id: door_id) do
          http_delete("/dock/doors/#{door_id}")
          nil
        end
      end
    end
  end
end
