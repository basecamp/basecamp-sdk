# frozen_string_literal: true

module Basecamp
  module Services
    # Service for TodolistGroups operations
    #
    # @generated from OpenAPI spec
    class TodolistGroupsService < BaseService

      # Reposition a todolist group
      def reposition(project_id:, group_id:, **body)
        http_put(bucket_path(project_id, "/todolists/#{group_id}/position.json"), body: body)
        nil
      end

      # List groups in a todolist
      def list(project_id:, todolist_id:)
        paginate(bucket_path(project_id, "/todolists/#{todolist_id}/groups.json"))
      end

      # Create a new group in a todolist
      def create(project_id:, todolist_id:, **body)
        http_post(bucket_path(project_id, "/todolists/#{todolist_id}/groups.json"), body: body).json
      end
    end
  end
end
