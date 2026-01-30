# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Projects operations
    #
    # @generated from OpenAPI spec
    class ProjectsService < BaseService

      # List projects (active by default; optionally archived/trashed)
      def list(status: nil)
        params = compact_params(status: status)
        paginate("/projects.json", params: params)
      end

      # Create a new project
      def create(**body)
        http_post("/projects.json", body: body).json
      end

      # Get a single project by id
      def get(project_id:)
        http_get("/projects/#{project_id}").json
      end

      # Update an existing project
      def update(project_id:, **body)
        http_put("/projects/#{project_id}", body: body).json
      end

      # Trash a project (returns 204 No Content)
      def trash(project_id:)
        http_delete("/projects/#{project_id}")
        nil
      end
    end
  end
end
