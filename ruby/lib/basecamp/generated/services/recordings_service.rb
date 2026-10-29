# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Recordings operations
    #
    # @generated from OpenAPI spec
    class RecordingsService < BaseService

      # Get the state of a move or copy started by MoveRecording or CopyRecording.
      # @param filing_id [Integer] filing id ID
      # @return [Hash] response data
      def get_filing(filing_id:)
        with_operation(service: "recordings", operation: "get_filing", is_mutation: false, resource_id: filing_id) do
          http_get("/filings/#{filing_id}", operation: "GetFiling").json(operation: "GetFiling")
        end
      end

      # List recordings of a given type across projects
      # @param type [String] Comment|Document|Door|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule::Entry|Todo|Todolist|Upload|Vault
      # @param bucket [String, nil] bucket
      # @param status [String, nil] active|archived|trashed
      # @param sort [String, nil] created_at|updated_at
      # @param direction [String, nil] asc|desc
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(type:, bucket: nil, status: nil, sort: nil, direction: nil, page: nil, max_items: nil)
        wrap_paginated(service: "recordings", operation: "list", is_mutation: false) do
          params = compact_query_params(type: type, bucket: bucket, status: status, sort: sort, direction: direction, page: page)
          paginate("/projects/recordings.json", params: params, operation: "ListRecordings", max_items: max_items)
        end
      end

      # Get any recording by its id, without knowing its type.
      # @param recording_id [Integer] recording id ID
      # @return [Hash] response data
      def get(recording_id:)
        with_operation(service: "recordings", operation: "get", is_mutation: false, resource_id: recording_id) do
          http_get("/recordings/#{recording_id}", operation: "GetRecording").json(operation: "GetRecording")
        end
      end

      # Start copying a recording into another project; poll GetFiling for the result.
      # @param recording_id [Integer] recording id ID
      # @param filing_destination_id [Integer, nil] The container to copy into, chosen as for MoveRecording's
      #   filing_destination_id: a container of the kind the recording sits in now,
      #   such as a to-do list, in this project or another, for a to-do. For a dock
      #   tool, the destination project's dock. Give this or destination_project_id.
      # @param destination_project_id [Integer, nil] The project to copy a dock tool to. Applies to tools only, and wins over
      #   filing_destination_id when a request for a tool gives both.
      # @param parent_id [Integer, nil] When copying into a vault, a folder inside it to put the copy in.
      # @param include_comments [Boolean, nil] Copy the recording's comments too. Defaults to false.
      # @return [Hash] response data
      def copy(recording_id:, filing_destination_id: nil, destination_project_id: nil, parent_id: nil, include_comments: nil)
        with_operation(service: "recordings", operation: "copy", is_mutation: true, resource_id: recording_id) do
          http_post("/recordings/#{recording_id}/copies.json", body: compact_params(filing_destination_id: filing_destination_id, destination_project_id: destination_project_id, parent_id: parent_id, include_comments: include_comments)).json(operation: "CopyRecording")
        end
      end

      # Move a document, upload or vault into another vault in its project, or reorder it in its own vault.
      # @param recording_id [Integer] recording id ID
      # @param parent_id [Integer] The destination vault. The recording's current vault keeps it where it is
      #   and changes only its position.
      # @param position [Integer, nil] 1-indexed position within the destination vault. Defaults to 1 (first); a
      #   position past the end places it last.
      # @return [void]
      def move_to_vault(recording_id:, parent_id:, position: nil)
        with_operation(service: "recordings", operation: "move_to_vault", is_mutation: true, resource_id: recording_id) do
          http_post("/recordings/#{recording_id}/filing.json", body: compact_params(parent_id: parent_id, position: position))
          nil
        end
      end

      # Start moving a recording into another project; poll GetFiling for the result.
      # @param recording_id [Integer] recording id ID
      # @param filing_destination_id [Integer, nil] The container to move into, of the kind the recording sits in now: a card
      #   table for a card, a message board for a message, a vault for a document,
      #   upload, cloud file, Google document or folder, a schedule for a schedule
      #   entry, a questionnaire for a check-in question, an inbox for an email
      #   forward, a to-do set for a to-do list, and a to-do list, in this project
      #   or another, for a to-do or to-do list group. For a dock tool, the
      #   destination project's dock. Give this or destination_project_id.
      # @param destination_project_id [Integer, nil] The project to move a dock tool to. Applies to tools only, and wins over
      #   filing_destination_id when a request for a tool gives both.
      # @param parent_id [Integer, nil] When moving into a vault, a folder inside it to put the recording in.
      # @return [Hash] response data
      def move(recording_id:, filing_destination_id: nil, destination_project_id: nil, parent_id: nil)
        with_operation(service: "recordings", operation: "move", is_mutation: true, resource_id: recording_id) do
          http_post("/recordings/#{recording_id}/moves.json", body: compact_params(filing_destination_id: filing_destination_id, destination_project_id: destination_project_id, parent_id: parent_id)).json(operation: "MoveRecording")
        end
      end

      # Put a recording's card in the spotlight area on its project or template home page.
      # @param recording_id [Integer] recording id ID
      # @return [Hash] response data
      def spotlight(recording_id:)
        with_operation(service: "recordings", operation: "spotlight", is_mutation: true, resource_id: recording_id) do
          http_post("/recordings/#{recording_id}/spotlight.json").json(operation: "SpotlightRecording")
        end
      end

      # Remove a recording from the spotlight area.
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def unspotlight(recording_id:)
        with_operation(service: "recordings", operation: "unspotlight", is_mutation: true, resource_id: recording_id) do
          http_delete("/recordings/#{recording_id}/spotlight.json")
          nil
        end
      end

      # Unarchive a recording (restore to active status)
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def unarchive(recording_id:)
        with_operation(service: "recordings", operation: "unarchive", is_mutation: true, resource_id: recording_id) do
          http_put("/recordings/#{recording_id}/status/active.json")
          nil
        end
      end

      # Archive a recording; bc3 answers 403 for types it never lets be archived, timesheet entries among them
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def archive(recording_id:)
        with_operation(service: "recordings", operation: "archive", is_mutation: true, resource_id: recording_id) do
          http_put("/recordings/#{recording_id}/status/archived.json")
          nil
        end
      end

      # Trash a recording; bc3 answers 403 for types it never lets be trashed, timesheet entries among them (DestroyTimesheetEntry removes those, permanently)
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def trash(recording_id:)
        with_operation(service: "recordings", operation: "trash", is_mutation: true, resource_id: recording_id) do
          http_put("/recordings/#{recording_id}/status/trashed.json")
          nil
        end
      end
    end
  end
end
