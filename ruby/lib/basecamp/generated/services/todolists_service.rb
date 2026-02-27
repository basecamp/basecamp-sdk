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
        with_operation(service: "todolists", operation: "get", is_mutation: false, resource_id: id) do
          http_get("/todolists/#{id}").json
        end
      end

      # Update an existing todolist or todolist group
      # @param id [Integer] id ID
      # @param name [String, nil] Name (required for both Todolist and TodolistGroup)
      # @param description [String, nil] Description (Todolist only, ignored for groups)
      # @return [Hash] response data
      def update(id:, name: nil, description: nil)
        with_operation(service: "todolists", operation: "update", is_mutation: true, resource_id: id) do
          http_put("/todolists/#{id}", body: compact_params(name: name, description: description)).json
        end
      end

      # List todolists in a todoset
      # @param todoset_id [Integer] todoset id ID
      # @param status [String, nil] active|archived|trashed
      # @return [Enumerator<Hash>] paginated results
      def list(todoset_id:, status: nil)
        wrap_paginated(service: "todolists", operation: "list", is_mutation: false, resource_id: todoset_id) do
          params = compact_params(status: status)
          paginate("/todosets/#{todoset_id}/todolists.json", params: params)
        end
      end

      # Create a new todolist in a todoset
      # @param todoset_id [Integer] todoset id ID
      # @param name [String] name
      # @param description [String, nil] description
      # @return [Hash] response data
      def create(todoset_id:, name:, description: nil)
        with_operation(service: "todolists", operation: "create", is_mutation: true, resource_id: todoset_id) do
          http_post("/todosets/#{todoset_id}/todolists.json", body: compact_params(name: name, description: description)).json
        end
      end
    end
  end
end
