# frozen_string_literal: true

module Basecamp
  module Services
    # Service for template operations.
    #
    # Templates allow you to create reusable project structures.
    #
    # @example List templates
    #   account.templates.list.each do |template|
    #     puts template["name"]
    #   end
    #
    # @example Create a project from template
    #   construction = account.templates.create_project(
    #     template_id: 123,
    #     name: "Q1 Planning"
    #   )
    class TemplatesService < BaseService
      # Lists all templates visible to the current user.
      #
      # @return [Enumerator<Hash>] templates
      def list
        paginate("/templates.json")
      end

      # Gets a template by ID.
      #
      # @param template_id [Integer, String] template ID
      # @return [Hash] template data
      def get(template_id:)
        http_get("/templates/#{template_id}").json
      end

      # Creates a new template.
      #
      # @param name [String] template name
      # @param description [String, nil] template description
      # @return [Hash] created template
      def create(name:, description: nil)
        body = compact_params(
          name: name,
          description: description
        )
        http_post("/templates.json", body: body).json
      end

      # Updates an existing template.
      #
      # @param template_id [Integer, String] template ID
      # @param name [String] new name
      # @param description [String, nil] new description
      # @return [Hash] updated template
      def update(template_id:, name:, description: nil)
        body = compact_params(
          name: name,
          description: description
        )
        http_put("/templates/#{template_id}", body: body).json
      end

      # Deletes a template.
      #
      # @param template_id [Integer, String] template ID
      # @return [void]
      def delete(template_id:)
        http_delete("/templates/#{template_id}")
        nil
      end

      # Creates a new project from a template.
      # This operation is asynchronous. Use get_construction to check status.
      #
      # @param template_id [Integer, String] template ID
      # @param name [String] project name
      # @param description [String, nil] project description
      # @return [Hash] project construction status
      def create_project(template_id:, name:, description: nil)
        body = compact_params(
          name: name,
          description: description
        )
        http_post("/templates/#{template_id}/project_constructions.json", body: body).json
      end

      # Gets the status of a project construction.
      #
      # @param template_id [Integer, String] template ID
      # @param construction_id [Integer, String] construction ID
      # @return [Hash] project construction status
      def get_construction(template_id:, construction_id:)
        http_get("/templates/#{template_id}/project_constructions/#{construction_id}").json
      end
    end
  end
end
