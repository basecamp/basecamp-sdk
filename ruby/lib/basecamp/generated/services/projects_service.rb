# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Projects operations
    #
    # @generated from OpenAPI spec
    class ProjectsService < BaseService

      # List the projects the current user has most recently visited, most recent visit first.
      # @return [Array<Hash>] response data
      def list_recent_projects()
        with_operation(service: "projects", operation: "list_recent_projects", is_mutation: false) do
          http_get("/my/recent_projects.json", operation: "ListRecentProjects").json(operation: "ListRecentProjects")
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
      # @param color [String, nil] The project's color, used for it on calendars: white, red, orange,
      #   yellow, green, blue, aqua, purple, gray, pink or brown. Omitted, Basecamp
      #   derives one from the project's name. Basecamp answers a value outside the enum
      #   with a server error, not a validation error.
      # @param purpose [String, nil] What kind of project it is: `topic` (a regular project, the default),
      #   `team`, or `company_hq` (the account's Company HQ, at most one per
      #   account; a second answers 422). BC3's enum also has `guide`, which marks
      #   a project as a sample nobody can edit afterwards, so it isn't offered.
      # @param admissions [String, nil] invite|employee|team
      # @param timesheet_enabled [Boolean, nil] Turn on the project's timesheet. Basecamp ignores this when the caller
      #   can't use timesheets (the account lacks them, or the caller is a client).
      # @param wallpaper_mode [String, nil] How a wallpaper uploaded later is laid out. UpdateProjectWallpaper
      #   uploads one.
      # @return [Hash] response data
      def create(name:, description: nil, color: nil, purpose: nil, admissions: nil, timesheet_enabled: nil, wallpaper_mode: nil)
        with_operation(service: "projects", operation: "create", is_mutation: true) do
          http_post("/projects.json", body: compact_params(name: name, description: description, color: color, purpose: purpose, admissions: admissions, timesheet_enabled: timesheet_enabled, wallpaper_mode: wallpaper_mode)).json(operation: "CreateProject")
        end
      end

      # Get a single project by id
      # @param project_id [Integer] project id ID
      # @return [Hash] response data
      def get(project_id:)
        with_operation(service: "projects", operation: "get", is_mutation: false, project_id: project_id) do
          http_get("/projects/#{project_id}", operation: "GetProject").json(operation: "GetProject")
        end
      end

      # Update an existing project
      # @param project_id [Integer] project id ID
      # @param name [String] name
      # @param description [String, nil] description
      # @param admissions [String, nil] invite|employee|team
      # @param schedule_attributes [Hash, nil] schedule attributes
      # @param wallpaper_mode [String, nil] How the project's wallpaper is laid out. Changes only the layout of a
      #   wallpaper already attached; UpdateProjectWallpaper uploads one.
      # @param color [String, nil] The project's color, used for it on calendars: white, red, orange,
      #   yellow, green, blue, aqua, purple, gray, pink or brown. An empty string clears
      #   it, and Basecamp derives one from the project's name again. A value
      #   outside the enum answers a server error, not a validation error. The
      #   `color` a project read returns is something else: the caller's own color
      #   for the project on their home screen, null unless they set one.
      # @param purpose [String, nil] What kind of project it is: `topic` (a regular project), `team`, or
      #   `company_hq` (the account's Company HQ). Changing a `topic` to anything
      #   else deletes the project's start and end dates. An account has at most
      #   one `company_hq`; asking for a second answers 422. BC3's enum also has
      #   `guide`, which marks a project as a sample nobody can edit afterwards,
      #   so it isn't offered.
      # @param timesheet_enabled [Boolean, nil] Turn the project's timesheet on or off. Basecamp ignores this when the
      #   caller can't use timesheets (the account lacks them, or the caller is a
      #   client).
      # @return [Hash] response data
      def update(project_id:, name:, description: nil, admissions: nil, schedule_attributes: nil, wallpaper_mode: nil, color: nil, purpose: nil, timesheet_enabled: nil)
        with_operation(service: "projects", operation: "update", is_mutation: true, project_id: project_id) do
          http_put("/projects/#{project_id}", body: compact_params(name: name, description: description, admissions: admissions, schedule_attributes: schedule_attributes, wallpaper_mode: wallpaper_mode, color: color, purpose: purpose, timesheet_enabled: timesheet_enabled)).json(operation: "UpdateProject")
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

      # Upload or replace the project's logo (returns 204 No Content).
      # @param project_id [Integer] project id ID
      # @param io [IO, String] File data to upload (IO object or string)
      # @param filename [String] Display name for the uploaded file
      # @param content_type [String] MIME type of the file (e.g., "image/png")
      # @return [void]
      def update_project_logo(project_id:, io:, filename:, content_type:)
        with_operation(service: "projects", operation: "update_project_logo", is_mutation: true, project_id: project_id) do
          http_put_multipart("/projects/#{project_id}/logo.json", io: io, filename: filename, content_type: content_type, field: "logo")
          nil
        end
      end

      # Remove the project's logo (returns 204 even when absent). Wallpaper is unchanged.
      # @param project_id [Integer] project id ID
      # @return [void]
      def remove_project_logo(project_id:)
        with_operation(service: "projects", operation: "remove_project_logo", is_mutation: true, project_id: project_id) do
          http_delete("/projects/#{project_id}/logo.json")
          nil
        end
      end

      # Record that the current user visited a project, moving it to the front of ListRecentProjects (returns 204 No Content).
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

      # Upload or replace the project's wallpaper (returns 204 No Content).
      # @param project_id [Integer] project id ID
      # @param io [IO, String] File data to upload (IO object or string)
      # @param filename [String] Display name for the uploaded file
      # @param content_type [String] MIME type of the file (e.g., "image/png")
      # @param mode [String, nil] How the wallpaper is laid out. Omitted, the project's current mode is kept.
      # @return [void]
      def update_project_wallpaper(project_id:, io:, filename:, content_type:, mode: nil)
        with_operation(service: "projects", operation: "update_project_wallpaper", is_mutation: true, project_id: project_id) do
          http_put_multipart("/projects/#{project_id}/wallpaper.json", io: io, filename: filename, content_type: content_type, field: "wallpaper", params: compact_query_params(mode: mode))
          nil
        end
      end

      # Remove the project's wallpaper (returns 204 No Content).
      # @param project_id [Integer] project id ID
      # @return [void]
      def remove_project_wallpaper(project_id:)
        with_operation(service: "projects", operation: "remove_project_wallpaper", is_mutation: true, project_id: project_id) do
          http_delete("/projects/#{project_id}/wallpaper.json")
          nil
        end
      end
    end
  end
end
