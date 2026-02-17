# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Todos operations
    #
    # @generated from OpenAPI spec
    class TodosService < BaseService

      # List todos in a todolist
      # @param project_id [Integer] project id ID
      # @param todolist_id [Integer] todolist id ID
      # @param status [String, nil] active|archived|trashed
      # @param completed [Boolean, nil] completed
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, todolist_id:, status: nil, completed: nil)
        wrap_paginated(service: "todos", operation: "list", is_mutation: false, project_id: project_id, resource_id: todolist_id) do
          params = compact_params(status: status, completed: completed)
          paginate(bucket_path(project_id, "/todolists/#{todolist_id}/todos.json"), params: params)
        end
      end

      # Create a new todo in a todolist
      # @param project_id [Integer] project id ID
      # @param todolist_id [Integer] todolist id ID
      # @param content [String] content
      # @param description [String, nil] description
      # @param assignee_ids [Array, nil] assignee ids
      # @param completion_subscriber_ids [Array, nil] completion subscriber ids
      # @param notify [Boolean, nil] notify
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param starts_on [String, nil] starts on (YYYY-MM-DD)
      # @return [Hash] response data
      def create(project_id:, todolist_id:, content:, description: nil, assignee_ids: nil, completion_subscriber_ids: nil, notify: nil, due_on: nil, starts_on: nil)
        with_operation(service: "todos", operation: "create", is_mutation: true, project_id: project_id, resource_id: todolist_id) do
          http_post(bucket_path(project_id, "/todolists/#{todolist_id}/todos.json"), body: compact_params(content: content, description: description, assignee_ids: assignee_ids, completion_subscriber_ids: completion_subscriber_ids, notify: notify, due_on: due_on, starts_on: starts_on)).json
        end
      end

      # Get a single todo by id
      # @param project_id [Integer] project id ID
      # @param todo_id [Integer] todo id ID
      # @return [Hash] response data
      def get(project_id:, todo_id:)
        with_operation(service: "todos", operation: "get", is_mutation: false, project_id: project_id, resource_id: todo_id) do
          http_get(bucket_path(project_id, "/todos/#{todo_id}")).json
        end
      end

      # Update an existing todo
      # @param project_id [Integer] project id ID
      # @param todo_id [Integer] todo id ID
      # @param content [String, nil] content
      # @param description [String, nil] description
      # @param assignee_ids [Array, nil] assignee ids
      # @param completion_subscriber_ids [Array, nil] completion subscriber ids
      # @param notify [Boolean, nil] notify
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param starts_on [String, nil] starts on (YYYY-MM-DD)
      # @return [Hash] response data
      def update(project_id:, todo_id:, content: nil, description: nil, assignee_ids: nil, completion_subscriber_ids: nil, notify: nil, due_on: nil, starts_on: nil)
        with_operation(service: "todos", operation: "update", is_mutation: true, project_id: project_id, resource_id: todo_id) do
          http_put(bucket_path(project_id, "/todos/#{todo_id}"), body: compact_params(content: content, description: description, assignee_ids: assignee_ids, completion_subscriber_ids: completion_subscriber_ids, notify: notify, due_on: due_on, starts_on: starts_on)).json
        end
      end

      # Trash a todo (returns 204 No Content)
      # @param project_id [Integer] project id ID
      # @param todo_id [Integer] todo id ID
      # @return [void]
      def trash(project_id:, todo_id:)
        with_operation(service: "todos", operation: "trash", is_mutation: true, project_id: project_id, resource_id: todo_id) do
          http_delete(bucket_path(project_id, "/todos/#{todo_id}"))
          nil
        end
      end

      # Mark a todo as complete
      # @param project_id [Integer] project id ID
      # @param todo_id [Integer] todo id ID
      # @return [void]
      def complete(project_id:, todo_id:)
        with_operation(service: "todos", operation: "complete", is_mutation: true, project_id: project_id, resource_id: todo_id) do
          http_post(bucket_path(project_id, "/todos/#{todo_id}/completion.json"))
          nil
        end
      end

      # Mark a todo as incomplete
      # @param project_id [Integer] project id ID
      # @param todo_id [Integer] todo id ID
      # @return [void]
      def uncomplete(project_id:, todo_id:)
        with_operation(service: "todos", operation: "uncomplete", is_mutation: true, project_id: project_id, resource_id: todo_id) do
          http_delete(bucket_path(project_id, "/todos/#{todo_id}/completion.json"))
          nil
        end
      end

      # Reposition a todo within its todolist
      # @param project_id [Integer] project id ID
      # @param todo_id [Integer] todo id ID
      # @param position [Integer] position
      # @param parent_id [Integer, nil] Optional todolist ID to move the todo to a different parent
      # @return [void]
      def reposition(project_id:, todo_id:, position:, parent_id: nil)
        with_operation(service: "todos", operation: "reposition", is_mutation: true, project_id: project_id, resource_id: todo_id) do
          http_put(bucket_path(project_id, "/todos/#{todo_id}/position.json"), body: compact_params(position: position, parent_id: parent_id))
          nil
        end
      end
    end
  end
end
