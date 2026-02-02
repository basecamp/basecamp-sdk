# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Recordings operations
    #
    # @generated from OpenAPI spec
    class RecordingsService < BaseService

      # List recordings of a given type across projects
      # @param type [String] Comment|Document|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule::Entry|Todo|Todolist|Upload|Vault
      # @param bucket [String, nil] bucket
      # @param status [String, nil] active|archived|trashed
      # @param sort [String, nil] created_at|updated_at
      # @param direction [String, nil] asc|desc
      # @return [Enumerator<Hash>] paginated results
      
      def list(type:, bucket: nil, status: nil, sort: nil, direction: nil)
        wrap_paginated(service: "recordings", operation: "list", is_mutation: false) do
          params = compact_params(type: type, bucket: bucket, status: status, sort: sort, direction: direction)
          paginate("/projects/recordings.json", params: params)
        end
      end

      # Get a single recording by id
      # @param recording_id [Integer] recording id ID
      # @return [Hash] response data
      def get(recording_id:)
        with_operation(service: "recordings", operation: "get", is_mutation: false, project_id: project_id, resource_id: recording_id) do
          http_get("/recordings/#{recording_id}").json
        end
      end

      # Unarchive a recording (restore to active status)
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def unarchive(recording_id:)
        with_operation(service: "recordings", operation: "unarchive", is_mutation: true, project_id: project_id, resource_id: recording_id) do
          http_put("/recordings/#{recording_id}/status/active.json")
          nil
        end
      end

      # Archive a recording
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def archive(recording_id:)
        with_operation(service: "recordings", operation: "archive", is_mutation: true, project_id: project_id, resource_id: recording_id) do
          http_put("/recordings/#{recording_id}/status/archived.json")
          nil
        end
      end

      # Trash a recording
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def trash(recording_id:)
        with_operation(service: "recordings", operation: "trash", is_mutation: true, project_id: project_id, resource_id: recording_id) do
          http_put("/recordings/#{recording_id}/status/trashed.json")
          nil
        end
      end
    end
  end
end
