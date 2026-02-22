# frozen_string_literal: true

module Basecamp
  module Services
    # Service for TodolistGroups operations
    #
    # @generated from OpenAPI spec
    class TodolistGroupsService < BaseService

      # Reposition a todolist group
      # @param group_id [Integer] group id ID
      # @param position [Integer] position
      # @return [void]
      def reposition(group_id:, position:)
        with_operation(service: "todolistgroups", operation: "reposition", is_mutation: true, resource_id: group_id) do
          http_put("/todolists/#{group_id}/position.json", body: compact_params(position: position))
          nil
        end
      end

      # List groups in a todolist
      # @param todolist_id [Integer] todolist id ID
      # @return [Enumerator<Hash>] paginated results
      def list(todolist_id:)
        wrap_paginated(service: "todolistgroups", operation: "list", is_mutation: false, resource_id: todolist_id) do
          paginate("/todolists/#{todolist_id}/groups.json")
        end
      end

      # Create a new group in a todolist
      # @param todolist_id [Integer] todolist id ID
      # @param name [String] name
      # @return [Hash] response data
      def create(todolist_id:, name:)
        with_operation(service: "todolistgroups", operation: "create", is_mutation: true, resource_id: todolist_id) do
          http_post("/todolists/#{todolist_id}/groups.json", body: compact_params(name: name)).json
        end
      end
    end
  end
end
