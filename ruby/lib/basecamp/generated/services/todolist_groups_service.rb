# frozen_string_literal: true

module Basecamp
  module Services
    # Service for TodolistGroups operations
    #
    # @generated from OpenAPI spec
    class TodolistGroupsService < BaseService

      # Reposition a todolist group
      # @param project_id [Integer] project id ID
      # @param group_id [Integer] group id ID
      # @param position [Integer] position
      # @return [void]
      def reposition(project_id:, group_id:, position:)
        http_put(bucket_path(project_id, "/todolists/#{group_id}/position.json"), body: compact_params(position: position))
        nil
      end

      # List groups in a todolist
      # @param project_id [Integer] project id ID
      # @param todolist_id [Integer] todolist id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, todolist_id:)
        paginate(bucket_path(project_id, "/todolists/#{todolist_id}/groups.json"))
      end

      # Create a new group in a todolist
      # @param project_id [Integer] project id ID
      # @param todolist_id [Integer] todolist id ID
      # @param name [String] name
      # @return [Hash] response data
      def create(project_id:, todolist_id:, name:)
        http_post(bucket_path(project_id, "/todolists/#{todolist_id}/groups.json"), body: compact_params(name: name)).json
      end
    end
  end
end
