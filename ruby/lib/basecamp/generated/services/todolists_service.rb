# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Todolists operations
    #
    # @generated from OpenAPI spec
    class TodolistsService < BaseService

      # Get a single todolist or todolist group by id
      # @param project_id [Integer] project id ID
      # @param id [Integer] id ID
      # @return [Hash] response data
      def get(project_id:, id:)
        http_get(bucket_path(project_id, "/todolists/#{id}")).json
      end

      # Update an existing todolist or todolist group
      # @param project_id [Integer] project id ID
      # @param id [Integer] id ID
      # @param name [String, nil] Name (required for both Todolist and TodolistGroup)
      # @param description [String, nil] Description (Todolist only, ignored for groups)
      # @return [Hash] response data
      def update(project_id:, id:, name: nil, description: nil)
        http_put(bucket_path(project_id, "/todolists/#{id}"), body: compact_params(name: name, description: description)).json
      end

      # List todolists in a todoset
      # @param project_id [Integer] project id ID
      # @param todoset_id [Integer] todoset id ID
      # @param status [String, nil] active|archived|trashed
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, todoset_id:, status: nil)
        params = compact_params(status: status)
        paginate(bucket_path(project_id, "/todosets/#{todoset_id}/todolists.json"), params: params)
      end

      # Create a new todolist in a todoset
      # @param project_id [Integer] project id ID
      # @param todoset_id [Integer] todoset id ID
      # @param name [String] name
      # @param description [String, nil] description
      # @return [Hash] response data
      def create(project_id:, todoset_id:, name:, description: nil)
        http_post(bucket_path(project_id, "/todosets/#{todoset_id}/todolists.json"), body: compact_params(name: name, description: description)).json
      end
    end
  end
end
