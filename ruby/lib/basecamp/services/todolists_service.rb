# frozen_string_literal: true

module Basecamp
  module Services
    # Service for todolist operations.
    #
    # Todolists are collections of todos within a todoset. A project has one
    # todoset which contains multiple todolists.
    #
    # @example List todolists
    #   account.todolists.list(project_id: 123, todoset_id: 456).each do |list|
    #     puts "#{list["name"]} - #{list["completed_ratio"]}"
    #   end
    #
    # @example Create a todolist
    #   todolist = account.todolists.create(
    #     project_id: 123,
    #     todoset_id: 456,
    #     name: "Launch Tasks"
    #   )
    class TodolistsService < BaseService
      # Lists all todolists in a todoset.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todoset_id [Integer, String] todoset ID
      # @param status [String, nil] filter by status ("archived", "trashed")
      # @return [Enumerator<Hash>] todolists
      def list(project_id:, todoset_id:, status: nil)
        params = compact_params(status: status)
        paginate(bucket_path(project_id, "/todosets/#{todoset_id}/todolists.json"), params: params)
      end

      # Gets a specific todolist.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todolist_id [Integer, String] todolist ID
      # @return [Hash] todolist data
      def get(project_id:, todolist_id:)
        http_get(bucket_path(project_id, "/todolists/#{todolist_id}")).json
      end

      # Creates a new todolist.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todoset_id [Integer, String] todoset ID
      # @param name [String] todolist name
      # @param description [String, nil] todolist description in HTML
      # @return [Hash] created todolist
      def create(project_id:, todoset_id:, name:, description: nil)
        body = compact_params(
          name: name,
          description: description
        )
        http_post(bucket_path(project_id, "/todosets/#{todoset_id}/todolists.json"), body: body).json
      end

      # Updates a todolist.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todolist_id [Integer, String] todolist ID
      # @param name [String, nil] new name
      # @param description [String, nil] new description
      # @return [Hash] updated todolist
      def update(project_id:, todolist_id:, name: nil, description: nil)
        body = compact_params(
          name: name,
          description: description
        )
        http_put(bucket_path(project_id, "/todolists/#{todolist_id}"), body: body).json
      end

      # Lists all todolist groups within a todolist.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todolist_id [Integer, String] todolist ID
      # @return [Enumerator<Hash>] todolist groups
      def list_groups(project_id:, todolist_id:)
        paginate(bucket_path(project_id, "/todolists/#{todolist_id}/groups.json"))
      end

      # Creates a new todolist group.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todolist_id [Integer, String] todolist ID
      # @param name [String] group name
      # @return [Hash] created group
      def create_group(project_id:, todolist_id:, name:)
        body = { name: name }
        http_post(bucket_path(project_id, "/todolists/#{todolist_id}/groups.json"), body: body).json
      end

      # Repositions a todolist group.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param todolist_id [Integer, String] todolist ID (the group's ID)
      # @param position [Integer] new position (1-based)
      # @return [void]
      def reposition_group(project_id:, todolist_id:, position:)
        http_put(bucket_path(project_id, "/todolists/#{todolist_id}/position.json"), body: { position: position })
        nil
      end
    end
  end
end
