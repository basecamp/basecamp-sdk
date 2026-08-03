# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Recordings operations
    #
    # @generated from OpenAPI spec
    class RecordingsService < BaseService

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

      # Unarchive a recording (restore to active status)
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def unarchive(recording_id:)
        with_operation(service: "recordings", operation: "unarchive", is_mutation: true, resource_id: recording_id) do
          http_put("/recordings/#{recording_id}/status/active.json")
          nil
        end
      end

      # Archive a recording
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def archive(recording_id:)
        with_operation(service: "recordings", operation: "archive", is_mutation: true, resource_id: recording_id) do
          http_put("/recordings/#{recording_id}/status/archived.json")
          nil
        end
      end

      # Trash a recording
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
