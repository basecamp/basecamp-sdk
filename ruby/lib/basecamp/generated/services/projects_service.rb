# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Projects operations
    #
    # @generated from OpenAPI spec
    class ProjectsService < BaseService

      # List projects (active by default; optionally archived/trashed)
      # @param status [String, nil] active|archived|trashed
      # @return [Enumerator<Hash>] paginated results
      def list(status: nil)
        params = compact_params(status: status)
        paginate("/projects.json", params: params)
      end

      # Create a new project
      # @param name [String] name
      # @param description [String, nil] description
      # @return [Hash] response data
      def create(name:, description: nil)
        http_post("/projects.json", body: compact_params(name: name, description: description)).json
      end

      # Get a single project by id
      # @param project_id [Integer] project id ID
      # @return [Hash] response data
      def get(project_id:)
        http_get("/projects/#{project_id}").json
      end

      # Update an existing project
      # @param project_id [Integer] project id ID
      # @param name [String] name
      # @param description [String, nil] description
      # @param admissions [String, nil] invite|employee|team
      # @param schedule_attributes [String, nil] schedule attributes
      # @return [Hash] response data
      def update(project_id:, name:, description: nil, admissions: nil, schedule_attributes: nil)
        http_put("/projects/#{project_id}", body: compact_params(name: name, description: description, admissions: admissions, schedule_attributes: schedule_attributes)).json
      end

      # Trash a project (returns 204 No Content)
      # @param project_id [Integer] project id ID
      # @return [void]
      def trash(project_id:)
        http_delete("/projects/#{project_id}")
        nil
      end
    end
  end
end
