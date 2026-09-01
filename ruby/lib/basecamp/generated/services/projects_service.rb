# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Projects operations
    #
    # @generated from OpenAPI spec
    class ProjectsService < BaseService

      # List the projects the current user has most recently visited, most recent
      # @return [Array<Hash>] response data
      def list_recent_projects()
        with_operation(service: "projects", operation: "list_recent_projects", is_mutation: false) do
          http_get("/my/recent_projects.json", operation: "ListRecentProjects").json
        end
      end

      # List projects (active by default; optionally archived/trashed)
      # @param status [String, nil] active|archived|trashed
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(status: nil, page: nil, max_items: nil)
        wrap_paginated(service: "projects", operation: "list", is_mutation: false) do
          params = compact_query_params(status: status, page: page)
          paginate("/projects.json", params: params, operation: "ListProjects", max_items: max_items)
        end
      end

      # Create a new project
      # @param name [String] name
      # @param description [String, nil] description
      # @return [Hash] response data
      def create(name:, description: nil)
        with_operation(service: "projects", operation: "create", is_mutation: true) do
          http_post("/projects.json", body: compact_params(name: name, description: description)).json
        end
      end

      # Get a single project by id
      # @param project_id [Integer] project id ID
      # @return [Hash] response data
      def get(project_id:)
        with_operation(service: "projects", operation: "get", is_mutation: false, project_id: project_id) do
          http_get("/projects/#{project_id}", operation: "GetProject").json
        end
      end

      # Update an existing project
      # @param project_id [Integer] project id ID
      # @param name [String] name
      # @param description [String, nil] description
      # @param admissions [String, nil] invite|employee|team
      # @param schedule_attributes [Hash, nil] schedule attributes
      # @return [Hash] response data
      def update(project_id:, name:, description: nil, admissions: nil, schedule_attributes: nil)
        with_operation(service: "projects", operation: "update", is_mutation: true, project_id: project_id) do
          http_put("/projects/#{project_id}", body: compact_params(name: name, description: description, admissions: admissions, schedule_attributes: schedule_attributes)).json
        end
      end

      # Trash a project (returns 204 No Content)
      # @param project_id [Integer] project id ID
      # @return [void]
      def trash(project_id:)
        with_operation(service: "projects", operation: "trash", is_mutation: true, project_id: project_id) do
          http_delete("/projects/#{project_id}")
          nil
        end
      end

      # Record that the current user visited a project, moving it to the front of
      # @param project_id [Integer] project id ID
      # @return [void]
      def record_project_visit(project_id:)
        with_operation(service: "projects", operation: "record_project_visit", is_mutation: true, project_id: project_id) do
          http_post("/projects/#{project_id}/recent_visit.json")
          nil
        end
      end

      # Restore a project to active status from trash as well as from the archive (returns 204 No Content).
      # @param project_id [Integer] project id ID
      # @return [void]
      def unarchive(project_id:)
        with_operation(service: "projects", operation: "unarchive", is_mutation: true, project_id: project_id) do
          http_put("/projects/#{project_id}/status/active.json")
          nil
        end
      end

      # Archive a project, removing it from the active project list (returns 204 No Content).
      # @param project_id [Integer] project id ID
      # @return [void]
      def archive(project_id:)
        with_operation(service: "projects", operation: "archive", is_mutation: true, project_id: project_id) do
          http_put("/projects/#{project_id}/status/archived.json")
          nil
        end
      end
    end
  end
end
