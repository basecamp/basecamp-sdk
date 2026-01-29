# frozen_string_literal: true

module Basecamp
  module Services
    # Service for todo operations.
    #
    # @example List todos in a todolist
    #   account.todos.list(project_id: 123, todolist_id: 456).each do |todo|
    #     puts "#{todo["content"]} (#{todo["completed"] ? "done" : "pending"})"
    #   end
    #
    # @example Create a todo
    #   todo = account.todos.create(
    #     project_id: 123,
    #     todolist_id: 456,
    #     content: "Write documentation",
    #     assignee_ids: [789]
    #   )
    #
    # @example Complete a todo
    #   account.todos.complete(project_id: 123, todo_id: 456)
    class TodosService < BaseService
      # Lists todos in a todolist.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todolist_id [Integer, String] todolist ID
      # @param status [String, nil] filter by status ("archived", "trashed")
      # @param completed [Boolean, nil] filter by completion status
      # @return [Enumerator<Hash>] todos
      def list(project_id:, todolist_id:, status: nil, completed: nil)
        params = compact_params(status: status, completed: completed)
        paginate(bucket_path(project_id, "/todolists/#{todolist_id}/todos.json"), params: params)
      end

      # Gets a specific todo.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todo_id [Integer, String] todo ID
      # @return [Hash] todo data
      def get(project_id:, todo_id:)
        http_get(bucket_path(project_id, "/todos/#{todo_id}.json")).json
      end

      # Creates a new todo.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todolist_id [Integer, String] todolist ID
      # @param content [String] todo content (can include HTML)
      # @param description [String, nil] extended description (HTML)
      # @param assignee_ids [Array<Integer>, nil] user IDs to assign
      # @param completion_subscriber_ids [Array<Integer>, nil] user IDs to notify on completion
      # @param notify [Boolean] whether to notify assignees
      # @param due_on [String, nil] due date (YYYY-MM-DD)
      # @param starts_on [String, nil] start date (YYYY-MM-DD)
      # @return [Hash] created todo
      def create(
        project_id:,
        todolist_id:,
        content:,
        description: nil,
        assignee_ids: nil,
        completion_subscriber_ids: nil,
        notify: true,
        due_on: nil,
        starts_on: nil
      )
        body = compact_params(
          content: content,
          description: description,
          assignee_ids: assignee_ids,
          completion_subscriber_ids: completion_subscriber_ids,
          notify: notify,
          due_on: due_on,
          starts_on: starts_on
        )
        http_post(bucket_path(project_id, "/todolists/#{todolist_id}/todos.json"), body: body).json
      end

      # Updates a todo.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todo_id [Integer, String] todo ID
      # @param content [String, nil] new content
      # @param description [String, nil] new description
      # @param assignee_ids [Array<Integer>, nil] new assignee IDs
      # @param completion_subscriber_ids [Array<Integer>, nil] new completion subscriber IDs
      # @param notify [Boolean, nil] whether to notify
      # @param due_on [String, nil] new due date
      # @param starts_on [String, nil] new start date
      # @return [Hash] updated todo
      def update(
        project_id:,
        todo_id:,
        content: nil,
        description: nil,
        assignee_ids: nil,
        completion_subscriber_ids: nil,
        notify: nil,
        due_on: nil,
        starts_on: nil
      )
        body = compact_params(
          content: content,
          description: description,
          assignee_ids: assignee_ids,
          completion_subscriber_ids: completion_subscriber_ids,
          notify: notify,
          due_on: due_on,
          starts_on: starts_on
        )
        http_put(bucket_path(project_id, "/todos/#{todo_id}.json"), body: body).json
      end

      # Completes a todo.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todo_id [Integer, String] todo ID
      # @return [void]
      def complete(project_id:, todo_id:)
        http_post(bucket_path(project_id, "/todos/#{todo_id}/completion.json"))
        nil
      end

      # Uncompletes a todo.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todo_id [Integer, String] todo ID
      # @return [void]
      def uncomplete(project_id:, todo_id:)
        http_delete(bucket_path(project_id, "/todos/#{todo_id}/completion.json"))
        nil
      end

      # Repositions a todo within its todolist.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todo_id [Integer, String] todo ID
      # @param position [Integer] new position (1-based)
      # @return [void]
      def reposition(project_id:, todo_id:, position:)
        http_put(bucket_path(project_id, "/todos/#{todo_id}/position.json"), body: { position: position })
        nil
      end

      # Trashes a todo.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todo_id [Integer, String] todo ID
      # @return [void]
      def trash(project_id:, todo_id:)
        http_delete(bucket_path(project_id, "/todos/#{todo_id}.json"))
        nil
      end
    end
  end
end
