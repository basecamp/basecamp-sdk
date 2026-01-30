# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Recordings operations
    #
    # @generated from OpenAPI spec
    class RecordingsService < BaseService

      # Get a single recording by id
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @return [Hash] response data
      def get(project_id:, recording_id:)
        http_get(bucket_path(project_id, "/recordings/#{recording_id}")).json
      end

      # Unarchive a recording (restore to active status)
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def unarchive(project_id:, recording_id:)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/status/active.json"))
        nil
      end

      # Archive a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def archive(project_id:, recording_id:)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/status/archived.json"))
        nil
      end

      # Trash a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def trash(project_id:, recording_id:)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/status/trashed.json"))
        nil
      end

      # List recordings of a given type across projects
      # @param type [String] Comment|Document|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule::Entry|Todo|Todolist|Upload|Vault
      # @param bucket [String, nil] bucket
      # @param status [String, nil] active|archived|trashed
      # @param sort [String, nil] created_at|updated_at
      # @param direction [String, nil] asc|desc
      # @return [Enumerator<Hash>] paginated results
      def list(type:, bucket: nil, status: nil, sort: nil, direction: nil)
        params = compact_params(type: type, bucket: bucket, status: status, sort: sort, direction: direction)
        paginate("/projects/recordings.json", params: params)
      end
    end
  end
end
