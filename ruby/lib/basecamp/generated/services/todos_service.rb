# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Todos operations
    #
    # @generated from OpenAPI spec
    class TodosService < BaseService

      # List todos in a todolist
      # @param todolist_id [Integer] todolist id ID
      # @param status [String, nil] active|archived|trashed
      # @param completed [Boolean, nil] completed
      # @return [Enumerator<Hash>] paginated results
      def list(todolist_id:, status: nil, completed: nil)
        params = compact_params(status: status, completed: completed)
        paginate("/todolists/#{todolist_id}/todos.json", params: params)
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
        http_post("/todolists/#{todolist_id}/todos.json", body: compact_params(content: content, description: description, assignee_ids: assignee_ids, completion_subscriber_ids: completion_subscriber_ids, notify: notify, due_on: due_on, starts_on: starts_on)).json
      end

      # Get a single todo by id
      # @param todo_id [Integer] todo id ID
      # @return [Hash] response data
      def get(todo_id:)
        http_get("/todos/#{todo_id}").json
      end

      # Update an existing todo
      # @param todo_id [Integer] todo id ID
      # @param content [String, nil] content
      # @param description [String, nil] description
      # @param assignee_ids [Array, nil] assignee ids
      # @param completion_subscriber_ids [Array, nil] completion subscriber ids
      # @param notify [Boolean, nil] notify
      # @param due_on [String, nil] due on (YYYY-MM-DD)
      # @param starts_on [String, nil] starts on (YYYY-MM-DD)
      # @return [Hash] response data
      def update(todo_id:, content: nil, description: nil, assignee_ids: nil, completion_subscriber_ids: nil, notify: nil, due_on: nil, starts_on: nil)
        http_put("/todos/#{todo_id}", body: compact_params(content: content, description: description, assignee_ids: assignee_ids, completion_subscriber_ids: completion_subscriber_ids, notify: notify, due_on: due_on, starts_on: starts_on)).json
      end

      # Trash a todo (returns 204 No Content)
      # @param todo_id [Integer] todo id ID
      # @return [void]
      def trash(todo_id:)
        http_delete("/todos/#{todo_id}")
        nil
      end

      # Mark a todo as complete
      # @param todo_id [Integer] todo id ID
      # @return [void]
      def complete(todo_id:)
        http_post("/todos/#{todo_id}/completion.json")
        nil
      end

      # Mark a todo as incomplete
      # @param todo_id [Integer] todo id ID
      # @return [void]
      def uncomplete(todo_id:)
        http_delete("/todos/#{todo_id}/completion.json")
        nil
      end

      # Reposition a todo within its todolist
      # @param todo_id [Integer] todo id ID
      # @param position [Integer] position
      # @param parent_id [Integer, nil] Optional todolist ID to move the todo to a different parent
      # @return [void]
      def reposition(todo_id:, position:, parent_id: nil)
        http_put("/todos/#{todo_id}/position.json", body: compact_params(position: position, parent_id: parent_id))
        nil
      end
    end
  end
end
