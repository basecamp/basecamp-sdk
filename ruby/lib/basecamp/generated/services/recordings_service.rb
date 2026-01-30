# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Recordings operations
    #
    # @generated from OpenAPI spec
    class RecordingsService < BaseService

      # Get a single recording by id
      def get(project_id:, recording_id:)
        http_get(bucket_path(project_id, "/recordings/#{recording_id}")).json
      end

      # Unarchive a recording (restore to active status)
      def unarchive(project_id:, recording_id:)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/status/active.json"))
        nil
      end

      # Archive a recording
      def archive(project_id:, recording_id:)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/status/archived.json"))
        nil
      end

      # Trash a recording
      def trash(project_id:, recording_id:)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/status/trashed.json"))
        nil
      end

      # List recordings of a given type across projects
      def list(type:, bucket: nil, status: nil, sort: nil, direction: nil)
        params = compact_params(type: type, bucket: bucket, status: status, sort: sort, direction: direction)
        paginate("/projects/recordings.json", params: params)
      end
    end
  end
end
