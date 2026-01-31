# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Todosets operations
    #
    # @generated from OpenAPI spec
    class TodosetsService < BaseService

      # Get a todoset (container for todolists in a project)
      # @param project_id [Integer] project id ID
      # @param todoset_id [Integer] todoset id ID
      # @return [Hash] response data
      def get(project_id:, todoset_id:)
        http_get(bucket_path(project_id, "/todosets/#{todoset_id}")).json
      end
    end
  end
end
