# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Templates operations
    #
    # @generated from OpenAPI spec
    class TemplatesService < BaseService

      # List all templates visible to the current user
      def list(status: nil)
        params = compact_params(status: status)
        paginate("/templates.json", params: params)
      end

      # Create a new template
      def create(**body)
        http_post("/templates.json", body: body).json
      end

      # Get a single template by id
      def get(template_id:)
        http_get("/templates/#{template_id}").json
      end

      # Update an existing template
      def update(template_id:, **body)
        http_put("/templates/#{template_id}", body: body).json
      end

      # Delete a template (trash it)
      def delete(template_id:)
        http_delete("/templates/#{template_id}")
        nil
      end

      # Create a project from a template (asynchronous)
      def create_project(template_id:, **body)
        http_post("/templates/#{template_id}/project_constructions.json", body: body).json
      end

      # Get the status of a project construction
      def get_construction(template_id:, construction_id:)
        http_get("/templates/#{template_id}/project_constructions/#{construction_id}").json
      end
    end
  end
end
