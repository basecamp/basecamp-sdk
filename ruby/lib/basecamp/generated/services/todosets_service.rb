# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Todosets operations
    #
    # @generated from OpenAPI spec
    class TodosetsService < BaseService

      # Get a todoset (container for todolists in a project)
      # @param todoset_id [Integer] todoset id ID
      # @return [Hash] response data
      def get(todoset_id:)
        with_operation(service: "todosets", operation: "get", is_mutation: false, project_id: project_id, resource_id: todoset_id) do
          http_get("/todosets/#{todoset_id}").json
        end
      end
    end
  end
end
