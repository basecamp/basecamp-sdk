# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Todolists operations
    #
    # @generated from OpenAPI spec
    class TodolistsService < BaseService

      # Get a single todolist or todolist group by id
      # @param id [Integer] id ID
      # @return [Hash] response data
      def get(id:)
        with_operation(service: "todolists", operation: "get", is_mutation: false, resource_id: id) do
          http_get("/todolists/#{id}", operation: "GetTodolistOrGroup").json
        end
      end

      # Replace a todolist (or todolist group) with a new complete representation.
      # @param id [Integer] id ID
      # @param name [String] Name (required for a to-do list and for a group alike) - presence-validated server-side, so omitting it is a 422, not a preserve
      # @param description [String, nil] Description (rich text HTML) - writable for a todolist group as well as a todolist, and omitting it clears it either way
      # @return [Hash] response data
      def replace(id:, name:, description: nil)
        with_operation(service: "todolists", operation: "replace", is_mutation: true, resource_id: id) do
          http_put("/todolists/#{id}", body: compact_params(name: name, description: description)).json
        end
      end

      # Reposition a to-do list within its to-do set.
      # @param todolist_id [Integer] todolist id ID
      # @param position [Integer] position
      # @return [void]
      def reposition(todolist_id:, position:)
        with_operation(service: "todolists", operation: "reposition", is_mutation: true, resource_id: todolist_id) do
          http_put("/todosets/todolists/#{todolist_id}/position.json", body: compact_params(position: position))
          nil
        end
      end

      # List todolists in a todoset
      # @param todoset_id [Integer] todoset id ID
      # @param status [String, nil] active|archived|trashed
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(todoset_id:, status: nil, page: nil, max_items: nil)
        wrap_paginated(service: "todolists", operation: "list", is_mutation: false, resource_id: todoset_id) do
          params = compact_query_params(status: status, page: page)
          paginate("/todosets/#{todoset_id}/todolists.json", params: params, operation: "ListTodolists", max_items: max_items)
        end
      end

      # Create a new todolist in a todoset
      # @param todoset_id [Integer] todoset id ID
      # @param name [String] name
      # @param description [String, nil] description
      # @param visible_to_clients [Boolean, nil] visible to clients
      # @return [Hash] response data
      def create(todoset_id:, name:, description: nil, visible_to_clients: nil)
        with_operation(service: "todolists", operation: "create", is_mutation: true, resource_id: todoset_id) do
          http_post("/todosets/#{todoset_id}/todolists.json", body: compact_params(name: name, description: description, visible_to_clients: visible_to_clients)).json
        end
      end
    end
  end
end
