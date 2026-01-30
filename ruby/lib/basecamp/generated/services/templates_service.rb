# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Templates operations
    #
    # @generated from OpenAPI spec
    class TemplatesService < BaseService

      # List all templates visible to the current user
      # @param status [String, nil] active|archived|trashed
      # @return [Enumerator<Hash>] paginated results
      def list(status: nil)
        params = compact_params(status: status)
        paginate("/templates.json", params: params)
      end

      # Create a new template
      # @param name [String] name
      # @param description [String, nil] description
      # @return [Hash] response data
      def create(name:, description: nil)
        http_post("/templates.json", body: compact_params(name: name, description: description)).json
      end

      # Get a single template by id
      # @param template_id [Integer] template id ID
      # @return [Hash] response data
      def get(template_id:)
        http_get("/templates/#{template_id}").json
      end

      # Update an existing template
      # @param template_id [Integer] template id ID
      # @param name [String, nil] name
      # @param description [String, nil] description
      # @return [Hash] response data
      def update(template_id:, name: nil, description: nil)
        http_put("/templates/#{template_id}", body: compact_params(name: name, description: description)).json
      end

      # Delete a template (trash it)
      # @param template_id [Integer] template id ID
      # @return [void]
      def delete(template_id:)
        http_delete("/templates/#{template_id}")
        nil
      end

      # Create a project from a template (asynchronous)
      # @param template_id [Integer] template id ID
      # @param name [String] name
      # @param description [String, nil] description
      # @return [Hash] response data
      def create_project(template_id:, name:, description: nil)
        http_post("/templates/#{template_id}/project_constructions.json", body: compact_params(name: name, description: description)).json
      end

      # Get the status of a project construction
      # @param template_id [Integer] template id ID
      # @param construction_id [Integer] construction id ID
      # @return [Hash] response data
      def get_construction(template_id:, construction_id:)
        http_get("/templates/#{template_id}/project_constructions/#{construction_id}").json
      end
    end
  end
end
