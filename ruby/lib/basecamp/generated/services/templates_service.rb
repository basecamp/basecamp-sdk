# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Templates operations
    #
    # @generated from OpenAPI spec
    class TemplatesService < BaseService

      # Templatify a to-do list or card table
      # @param bucket_id [Integer] bucket id ID
      # @param recording_id [Integer] The to-do list or card table to templatify. Anything else is a 403.
      # @param template_name [String, nil] What to call the template. Defaults to the source recording's own title.
      # @param copy_comments [Boolean, nil] Carry the comments across.
      # @param copy_assignments [Boolean, nil] Carry assignees and the people involved across, adding them to the library
      #   if they are not already there.
      # @param move_cards_to_triage [Boolean, nil] Gather the cards into Triage. Card tables only, and ignored otherwise.
      # @return [Hash] response data
      def create_templatification(bucket_id:, recording_id:, template_name: nil, copy_comments: nil, copy_assignments: nil, move_cards_to_triage: nil)
        with_operation(service: "templates", operation: "create_templatification", is_mutation: true, project_id: bucket_id, resource_id: recording_id) do
          http_post("/buckets/#{bucket_id}/recordings/#{recording_id}/templatifications.json", body: compact_params(template_name: template_name, copy_comments: copy_comments, copy_assignments: copy_assignments, move_cards_to_triage: move_cards_to_triage)).json
        end
      end

      # Get a templatification
      # @param bucket_id [Integer] bucket id ID
      # @param recording_id [Integer] recording id ID
      # @param templatification_id [Integer] templatification id ID
      # @return [Hash] response data
      def get_templatification(bucket_id:, recording_id:, templatification_id:)
        with_operation(service: "templates", operation: "get_templatification", is_mutation: false, project_id: bucket_id, resource_id: templatification_id) do
          http_get("/buckets/#{bucket_id}/recordings/#{recording_id}/templatifications/#{templatification_id}", operation: "GetTemplatification").json
        end
      end

      # Get the account's card table templates
      # @return [Hash] response data
      def get_library_card_tables()
        with_operation(service: "templates", operation: "get_library_card_tables", is_mutation: false) do
          http_get("/template_library/card_tables.json", operation: "GetTemplateLibraryCardTables").json
        end
      end

      # Create a card table template with the default columns
      # @param name [String] The template's name. Write-only: the response carries it as `title`.
      # @return [Hash] response data
      def create_library_card_table(name:)
        with_operation(service: "templates", operation: "create_library_card_table", is_mutation: true) do
          http_post("/template_library/card_tables.json", body: compact_params(name: name)).json
        end
      end

      # Start copying a to-do list or card table template into a project
      # @param template_recording_id [Integer] The to-do list or card table in the library to copy.
      # @param destination_project_id [Integer, nil] The destination project. Basecamp resolves the container from the
      #   template's kind, so a caller naming a project needs to know nothing about
      #   docks or to-do sets. Supply this or destination_parent_id; if both are
      #   sent, destination_parent_id wins.
      # @param destination_parent_id [Integer, nil] The container to copy into, for a caller that already holds one: the
      #   project's to-do set for a to-do list template, or its dock for a card
      #   table. A caller who may not edit it gets 404, not 403.
      # @param adding_people_confirmed [Boolean, nil] Confirm granting destination-project access to people referenced by the template.
      # @return [Hash] response data
      def create_library_copy(template_recording_id:, destination_project_id: nil, destination_parent_id: nil, adding_people_confirmed: nil)
        with_operation(service: "templates", operation: "create_library_copy", is_mutation: true) do
          http_post("/template_library/copies.json", body: compact_params(template_recording_id: template_recording_id, destination_project_id: destination_project_id, destination_parent_id: destination_parent_id, adding_people_confirmed: adding_people_confirmed)).json
        end
      end

      # Get the current status of a to-do list template copy
      # @param copy_id [Integer] copy id ID
      # @return [Hash] response data
      def get_library_copy(copy_id:)
        with_operation(service: "templates", operation: "get_library_copy", is_mutation: false, resource_id: copy_id) do
          http_get("/template_library/copies/#{copy_id}", operation: "GetTemplateLibraryCopy").json
        end
      end

      # Get the account's to-do list templates
      # @return [Hash] response data
      def get_library_todolists()
        with_operation(service: "templates", operation: "get_library_todolists", is_mutation: false) do
          http_get("/template_library/todolists.json", operation: "GetTemplateLibraryTodolists").json
        end
      end

      # Create an empty to-do list template
      # @param name [String] What to call the template.
      # @param description [String, nil] Rich text describing the template.
      # @return [Hash] response data
      def create_library_todolist(name:, description: nil)
        with_operation(service: "templates", operation: "create_library_todolist", is_mutation: true) do
          http_post("/template_library/todolists.json", body: compact_params(name: name, description: description)).json
        end
      end

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
