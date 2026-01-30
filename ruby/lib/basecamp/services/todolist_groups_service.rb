# frozen_string_literal: true

module Basecamp
  module Services
    # Service for todolist group operations.
    #
    # Todolist groups are organizational folders within a todolist that help
    # organize related todos together.
    #
    # @example List groups in a todolist
    #   groups = account.todolist_groups.list(project_id: 123, todolist_id: 456)
    #
    # @example Create a new group
    #   group = account.todolist_groups.create(
    #     project_id: 123,
    #     todolist_id: 456,
    #     name: "Phase 1"
    #   )
    #
    # @example Update a group name
    #   group = account.todolist_groups.update(
    #     project_id: 123,
    #     group_id: 789,
    #     name: "Phase 1 - Complete"
    #   )
    class TodolistGroupsService < BaseService
      # Lists all groups in a todolist.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todolist_id [Integer, String] todolist ID
      # @return [Enumerator<Hash>] todolist groups
      def list(project_id:, todolist_id:)
        paginate(bucket_path(project_id, "/todolists/#{todolist_id}/groups.json"))
      end

      # Gets a specific todolist group.
      #
      # Note: Groups are fetched via the todolists endpoint (polymorphic).
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param group_id [Integer, String] group ID
      # @return [Hash] group data
      def get(project_id:, group_id:)
        http_get(bucket_path(project_id, "/todolists/#{group_id}.json")).json
      end

      # Creates a new group in a todolist.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todolist_id [Integer, String] todolist ID
      # @param name [String] group name (required)
      # @return [Hash] created group
      # @raise [ArgumentError] if name is empty
      def create(project_id:, todolist_id:, name:)
        raise ArgumentError, "group name is required" if name.nil? || name.empty?

        body = { name: name }
        http_post(bucket_path(project_id, "/todolists/#{todolist_id}/groups.json"), body: body).json
      end

      # Updates an existing todolist group.
      #
      # Note: Groups are updated via the todolists endpoint (polymorphic).
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param group_id [Integer, String] group ID
      # @param name [String, nil] new group name
      # @return [Hash] updated group
      def update(project_id:, group_id:, name: nil)
        body = compact_params(name: name)
        http_put(bucket_path(project_id, "/todolists/#{group_id}.json"), body: body).json
      end

      # Repositions a group within its todolist.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param group_id [Integer, String] group ID
      # @param position [Integer] new position (1-based, 1 = first position)
      # @return [void]
      # @raise [ArgumentError] if position is less than 1
      def reposition(project_id:, group_id:, position:)
        raise ArgumentError, "position must be at least 1" if position < 1

        http_put(bucket_path(project_id, "/todolists/#{group_id}/position.json"), body: { position: position })
        nil
      end

      # Moves a group to the trash.
      # Trashed groups can be recovered from the trash.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param group_id [Integer, String] group ID
      # @return [void]
      def trash(project_id:, group_id:)
        http_delete(bucket_path(project_id, "/recordings/#{group_id}/status/trashed.json"))
        nil
      end
    end
  end
end
