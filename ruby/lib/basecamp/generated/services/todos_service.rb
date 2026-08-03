# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Todos operations
    #
    # @generated from OpenAPI spec
    class TodosService < BaseService

      # Create a to-do directly under a project's to-do set, outside any to-do list.
      # @param bucket_id [Integer] bucket id ID
      # @param todoset_id [Integer] todoset id ID
      # @param content [String] content
      # @param description [String, nil] description
      # @param assignee_ids [Array, nil] assignee ids
      # @param completion_subscriber_ids [Array, nil] completion subscriber ids
      # @param notify [Boolean, nil] notify
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param starts_on [String, nil] starts on (YYYY-MM-DD)
      # @return [Hash] response data
      def create_todoset_todo(bucket_id:, todoset_id:, content:, description: nil, assignee_ids: nil, completion_subscriber_ids: nil, notify: nil, due_on: nil, starts_on: nil)
        with_operation(service: "todos", operation: "create_todoset_todo", is_mutation: true, project_id: bucket_id, resource_id: todoset_id) do
          http_post("/buckets/#{bucket_id}/todosets/#{todoset_id}/todos.json", body: compact_params(content: content, description: description, assignee_ids: assignee_ids, completion_subscriber_ids: completion_subscriber_ids, notify: notify, due_on: due_on, starts_on: starts_on)).json
        end
      end

      # List todos in a todolist
      # @param todolist_id [Integer] todolist id ID
      # @param status [String, nil] active|archived|trashed
      # @param completed [Boolean, nil] completed
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(todolist_id:, status: nil, completed: nil, page: nil, max_items: nil)
        wrap_paginated(service: "todos", operation: "list", is_mutation: false, resource_id: todolist_id) do
          params = compact_query_params(status: status, completed: completed, page: page)
          paginate("/todolists/#{todolist_id}/todos.json", params: params, operation: "ListTodos", max_items: max_items)
        end
      end

      # Create a new todo in a todolist
      # @param todolist_id [Integer] todolist id ID
      # @param content [String] content
      # @param description [String, nil] description
      # @param assignee_ids [Array, nil] assignee ids
      # @param completion_subscriber_ids [Array, nil] completion subscriber ids
      # @param notify [Boolean, nil] notify
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param starts_on [String, nil] starts on (YYYY-MM-DD)
      # @return [Hash] response data
      def create(todolist_id:, content:, description: nil, assignee_ids: nil, completion_subscriber_ids: nil, notify: nil, due_on: nil, starts_on: nil)
        with_operation(service: "todos", operation: "create", is_mutation: true, resource_id: todolist_id) do
          http_post("/todolists/#{todolist_id}/todos.json", body: compact_params(content: content, description: description, assignee_ids: assignee_ids, completion_subscriber_ids: completion_subscriber_ids, notify: notify, due_on: due_on, starts_on: starts_on)).json
        end
      end

      # Get a single todo by id
      # @param todo_id [Integer] todo id ID
      # @return [Hash] response data
      def get(todo_id:)
        with_operation(service: "todos", operation: "get", is_mutation: false, resource_id: todo_id) do
          http_get("/todos/#{todo_id}", operation: "GetTodo").json
        end
      end

      # Replace a todo with a new complete representation.
      # @param todo_id [Integer] todo id ID
      # @param content [String] content
      # @param description [String, nil] description
      # @param assignee_ids [Array, nil] assignee ids
      # @param completion_subscriber_ids [Array, nil] completion subscriber ids
      # @param notify [Boolean, nil] notify
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param starts_on [String, nil] starts on (YYYY-MM-DD)
      # @return [Hash] response data
      def replace(todo_id:, content:, description: nil, assignee_ids: nil, completion_subscriber_ids: nil, notify: nil, due_on: nil, starts_on: nil)
        with_operation(service: "todos", operation: "replace", is_mutation: true, resource_id: todo_id) do
          http_put("/todos/#{todo_id}", body: compact_params(content: content, description: description, assignee_ids: assignee_ids, completion_subscriber_ids: completion_subscriber_ids, notify: notify, due_on: due_on, starts_on: starts_on)).json
        end
      end

      # Mark a todo as complete
      # @param todo_id [Integer] todo id ID
      # @return [void]
      def complete(todo_id:)
        with_operation(service: "todos", operation: "complete", is_mutation: true, resource_id: todo_id) do
          http_post("/todos/#{todo_id}/completion.json")
          nil
        end
      end

      # Mark a todo as incomplete
      # @param todo_id [Integer] todo id ID
      # @return [void]
      def uncomplete(todo_id:)
        with_operation(service: "todos", operation: "uncomplete", is_mutation: true, resource_id: todo_id) do
          http_delete("/todos/#{todo_id}/completion.json")
          nil
        end
      end

      # Reposition a todo within its todolist
      # @param todo_id [Integer] todo id ID
      # @param position [Integer] position
      # @param parent_id [Integer, nil] Optional todolist ID to move the todo to a different parent
      # @return [void]
      def reposition(todo_id:, position:, parent_id: nil)
        with_operation(service: "todos", operation: "reposition", is_mutation: true, resource_id: todo_id) do
          http_put("/todos/#{todo_id}/position.json", body: compact_params(position: position, parent_id: parent_id))
          nil
        end
      end
    end
  end
end
