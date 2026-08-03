# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Templates operations
    #
    # @generated from OpenAPI spec
    class TemplatesService < BaseService

      # List all templates visible to the current user
      # @param status [String, nil] active|archived|trashed
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(status: nil, page: nil, max_items: nil)
        wrap_paginated(service: "templates", operation: "list", is_mutation: false) do
          params = compact_query_params(status: status, page: page)
          paginate("/templates.json", params: params, operation: "ListTemplates", max_items: max_items)
        end
      end

      # Create a new template
      # @param name [String] name
      # @param description [String, nil] description
      # @return [Hash] response data
      def create(name:, description: nil)
        with_operation(service: "templates", operation: "create", is_mutation: true) do
          http_post("/templates.json", body: compact_params(name: name, description: description)).json
        end
      end

      # Get a single template by id
      # @param template_id [Integer] template id ID
      # @return [Hash] response data
      def get(template_id:)
        with_operation(service: "templates", operation: "get", is_mutation: false, resource_id: template_id) do
          http_get("/templates/#{template_id}", operation: "GetTemplate").json
        end
      end

      # Update an existing template
      # @param template_id [Integer] template id ID
      # @param name [String, nil] name
      # @param description [String, nil] description
      # @return [Hash] response data
      def update(template_id:, name: nil, description: nil)
        with_operation(service: "templates", operation: "update", is_mutation: true, resource_id: template_id) do
          http_put("/templates/#{template_id}", body: compact_params(name: name, description: description)).json
        end
      end

      # Delete a template (trash it)
      # @param template_id [Integer] template id ID
      # @return [void]
      def delete(template_id:)
        with_operation(service: "templates", operation: "delete", is_mutation: true, resource_id: template_id) do
          http_delete("/templates/#{template_id}")
          nil
        end
      end

      # Create a project from a template (asynchronous)
      # @param template_id [Integer] template id ID
      # @param project [Hash] project
      # @return [Hash] response data
      def create_project(template_id:, project:)
        with_operation(service: "templates", operation: "create_project", is_mutation: true, resource_id: template_id) do
          http_post("/templates/#{template_id}/project_constructions.json", body: compact_params(project: project)).json
        end
      end

      # Get the status of a project construction
      # @param template_id [Integer] template id ID
      # @param construction_id [Integer] construction id ID
      # @return [Hash] response data
      def get_construction(template_id:, construction_id:)
        with_operation(service: "templates", operation: "get_construction", is_mutation: false, resource_id: construction_id) do
          http_get("/templates/#{template_id}/project_constructions/#{construction_id}", operation: "GetProjectConstruction").json
        end
      end
    end
  end
end
