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
          http_put("/todolists/groups/#{group_id}/position.json", body: compact_params(position: position))
          nil
        end
      end

      # List groups in a todolist
      # @param todolist_id [Integer] todolist id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(todolist_id:, page: nil, max_items: nil)
        wrap_paginated(service: "todolistgroups", operation: "list", is_mutation: false, resource_id: todolist_id) do
          params = compact_query_params(page: page)
          paginate("/todolists/#{todolist_id}/groups.json", params: params, operation: "ListTodolistGroups", max_items: max_items)
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
