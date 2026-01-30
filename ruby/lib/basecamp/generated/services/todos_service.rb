# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Todos operations
    #
    # @generated from OpenAPI spec
    class TodosService < BaseService

      # List todos in a todolist
      def list(project_id:, todolist_id:, status: nil, completed: nil)
        params = compact_params(status: status, completed: completed)
        paginate(bucket_path(project_id, "/todolists/#{todolist_id}/todos.json"), params: params)
      end

      # Create a new todo in a todolist
      def create(project_id:, todolist_id:, **body)
        http_post(bucket_path(project_id, "/todolists/#{todolist_id}/todos.json"), body: body).json
      end

      # Get a single todo by id
      def get(project_id:, todo_id:)
        http_get(bucket_path(project_id, "/todos/#{todo_id}")).json
      end

      # Update an existing todo
      def update(project_id:, todo_id:, **body)
        http_put(bucket_path(project_id, "/todos/#{todo_id}"), body: body).json
      end

      # Trash a todo (returns 204 No Content)
      def trash(project_id:, todo_id:)
        http_delete(bucket_path(project_id, "/todos/#{todo_id}"))
        nil
      end

      # Mark a todo as complete
      def complete(project_id:, todo_id:)
        http_post(bucket_path(project_id, "/todos/#{todo_id}/completion.json"))
        nil
      end

      # Mark a todo as incomplete
      def uncomplete(project_id:, todo_id:)
        http_delete(bucket_path(project_id, "/todos/#{todo_id}/completion.json"))
        nil
      end

      # Reposition a todo within its todolist
      def reposition(project_id:, todo_id:, **body)
        http_put(bucket_path(project_id, "/todos/#{todo_id}/position.json"), body: body)
        nil
      end
    end
  end
end
