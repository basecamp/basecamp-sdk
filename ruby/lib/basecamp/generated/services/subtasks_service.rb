# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Subtasks operations
    #
    # @generated from OpenAPI spec
    class SubtasksService < BaseService

      # List a recording's subtasks, in position order
      # @param recording_id [Integer] recording id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(recording_id:, page: nil, max_items: nil)
        wrap_paginated(service: "subtasks", operation: "list", is_mutation: false, resource_id: recording_id) do
          params = compact_query_params(page: page)
          paginate("/recordings/#{recording_id}/subtasks.json", params: params, operation: "ListSubtasks", max_items: max_items)
        end
      end

      # Create a subtask under a to-do or a card
      # @param recording_id [Integer] recording id ID
      # @param title [String] title
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param assignee_ids [Array, nil] assignee ids
      # @return [Hash] response data
      def create(recording_id:, title:, due_on: nil, assignee_ids: nil)
        with_operation(service: "subtasks", operation: "create", is_mutation: true, resource_id: recording_id) do
          http_post("/recordings/#{recording_id}/subtasks.json", body: compact_params(title: title, due_on: due_on, assignee_ids: assignee_ids)).json
        end
      end

      # Get a subtask by ID
      # @param subtask_id [Integer] subtask id ID
      # @return [Hash] response data
      def get(subtask_id:)
        with_operation(service: "subtasks", operation: "get", is_mutation: false, resource_id: subtask_id) do
          http_get("/subtasks/#{subtask_id}", operation: "GetSubtask").json
        end
      end

      # Update a subtask
      # @param subtask_id [Integer] subtask id ID
      # @param title [String, nil] title
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param assignee_ids [Array, nil] assignee ids
      # @return [Hash] response data
      def update(subtask_id:, title: nil, due_on: nil, assignee_ids: nil)
        with_operation(service: "subtasks", operation: "update", is_mutation: true, resource_id: subtask_id) do
          http_put("/subtasks/#{subtask_id}", body: compact_params(title: title, due_on: due_on, assignee_ids: assignee_ids)).json
        end
      end

      # Delete a subtask
      # @param subtask_id [Integer] subtask id ID
      # @return [void]
      def delete(subtask_id:)
        with_operation(service: "subtasks", operation: "delete", is_mutation: true, resource_id: subtask_id) do
          http_delete("/subtasks/#{subtask_id}")
          nil
        end
      end

      # Mark a subtask as completed
      # @param subtask_id [Integer] subtask id ID
      # @return [void]
      def complete(subtask_id:)
        with_operation(service: "subtasks", operation: "complete", is_mutation: true, resource_id: subtask_id) do
          http_post("/subtasks/#{subtask_id}/completion.json")
          nil
        end
      end

      # Mark a subtask as not completed
      # @param subtask_id [Integer] subtask id ID
      # @return [void]
      def uncomplete(subtask_id:)
        with_operation(service: "subtasks", operation: "uncomplete", is_mutation: true, resource_id: subtask_id) do
          http_delete("/subtasks/#{subtask_id}/completion.json")
          nil
        end
      end

      # Move a subtask to a new position among its siblings
      # @param subtask_id [Integer] subtask id ID
      # @param position [Integer] The 1-based position to move it to
      # @return [void]
      def reposition(subtask_id:, position:)
        with_operation(service: "subtasks", operation: "reposition", is_mutation: true, resource_id: subtask_id) do
          http_put("/subtasks/#{subtask_id}/position.json", body: compact_params(position: position))
          nil
        end
      end
    end
  end
end
