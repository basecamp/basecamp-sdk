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
        params = compact_params(status: status, completed: completed)
        paginate(bucket_path(project_id, "/todolists/#{todolist_id}/todos.json"), params: params)
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
        http_post(bucket_path(project_id, "/todolists/#{todolist_id}/todos.json"), body: compact_params(content: content, description: description, assignee_ids: assignee_ids, completion_subscriber_ids: completion_subscriber_ids, notify: notify, due_on: due_on, starts_on: starts_on)).json
      end

      # Get a single todo by id
      # @param project_id [Integer] project id ID
      # @param todo_id [Integer] todo id ID
      # @return [Hash] response data
      def get(project_id:, todo_id:)
        http_get(bucket_path(project_id, "/todos/#{todo_id}")).json
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
        http_put(bucket_path(project_id, "/todos/#{todo_id}"), body: compact_params(content: content, description: description, assignee_ids: assignee_ids, completion_subscriber_ids: completion_subscriber_ids, notify: notify, due_on: due_on, starts_on: starts_on)).json
      end

      # Trash a todo (returns 204 No Content)
      # @param project_id [Integer] project id ID
      # @param todo_id [Integer] todo id ID
      # @return [void]
      def trash(project_id:, todo_id:)
        http_delete(bucket_path(project_id, "/todos/#{todo_id}"))
        nil
      end

      # Mark a todo as complete
      # @param project_id [Integer] project id ID
      # @param todo_id [Integer] todo id ID
      # @return [void]
      def complete(project_id:, todo_id:)
        http_post(bucket_path(project_id, "/todos/#{todo_id}/completion.json"))
        nil
      end

      # Mark a todo as incomplete
      # @param project_id [Integer] project id ID
      # @param todo_id [Integer] todo id ID
      # @return [void]
      def uncomplete(project_id:, todo_id:)
        http_delete(bucket_path(project_id, "/todos/#{todo_id}/completion.json"))
        nil
      end

      # Reposition a todo within its todolist
      # @param project_id [Integer] project id ID
      # @param todo_id [Integer] todo id ID
      # @param position [Integer] position
      # @return [void]
      def reposition(project_id:, todo_id:, position:)
        http_put(bucket_path(project_id, "/todos/#{todo_id}/position.json"), body: compact_params(position: position))
        nil
      end
    end
  end
end
