# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Todolists operations
    #
    # @generated from OpenAPI spec
    class TodolistsService < BaseService

      # Get a single todolist or todolist group by id
      # @param id [Integer] id ID
      # @return [Hash] response data
      def get(id:)
        http_get("/todolists/#{id}").json
      end

      # Update an existing todolist or todolist group
      # @param id [Integer] id ID
      # @param name [String, nil] Name (required for both Todolist and TodolistGroup)
      # @param description [String, nil] Description (Todolist only, ignored for groups)
      # @return [Hash] response data
      def update(id:, name: nil, description: nil)
        http_put("/todolists/#{id}", body: compact_params(name: name, description: description)).json
      end

      # List todolists in a todoset
      # @param todoset_id [Integer] todoset id ID
      # @param status [String, nil] active|archived|trashed
      # @return [Enumerator<Hash>] paginated results
      def list(todoset_id:, status: nil)
        params = compact_params(status: status)
        paginate("/todosets/#{todoset_id}/todolists.json", params: params)
      end

      # Create a new todolist in a todoset
      # @param todoset_id [Integer] todoset id ID
      # @param name [String] name
      # @param description [String, nil] description
      # @return [Hash] response data
      def create(todoset_id:, name:, description: nil)
        http_post("/todosets/#{todoset_id}/todolists.json", body: compact_params(name: name, description: description)).json
      end
    end
  end
end
