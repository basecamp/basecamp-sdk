# frozen_string_literal: true

module Basecamp
  module Services
    # Service for recording operations.
    #
    # Recordings are the base type for most content in Basecamp, including
    # todos, messages, comments, documents, uploads, etc. This service
    # provides common operations that work across all recording types.
    #
    # @example List all recordings in a project
    #   account.recordings.list(type: "Todo", bucket: 123).each do |recording|
    #     puts "#{recording["title"]} - #{recording["status"]}"
    #   end
    #
    # @example Archive a recording
    #   account.recordings.archive(project_id: 123, recording_id: 456)
    class RecordingsService < BaseService
      # Lists recordings across projects.
      #
      # @param type [String] recording type (e.g., "Todo", "Message", "Comment")
      # @param bucket [Integer, nil] filter by project ID
      # @param status [String, nil] filter by status ("active", "archived", "trashed")
      # @param sort [String, nil] sort field ("created_at", "updated_at")
      # @param direction [String, nil] sort direction ("asc", "desc")
      # @return [Enumerator<Hash>] recordings
      def list(type:, bucket: nil, status: nil, sort: nil, direction: nil)
        params = compact_params(
          type: type,
          bucket: bucket,
          status: status,
          sort: sort,
          direction: direction
        )
        paginate("/projects/recordings.json", params: params)
      end

      # Gets a specific recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [Hash] recording data
      def get(project_id:, recording_id:)
        http_get(bucket_path(project_id, "/recordings/#{recording_id}")).json
      end

      # Archives a recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [void]
      def archive(project_id:, recording_id:)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/status/archived.json"))
        nil
      end

      # Unarchives a recording (restores to active).
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [void]
      def unarchive(project_id:, recording_id:)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/status/active.json"))
        nil
      end

      # Moves a recording to the trash.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [void]
      def trash(project_id:, recording_id:)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/status/trashed.json"))
        nil
      end

      # Lists events (change history) for a recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [Enumerator<Hash>] events
      def list_events(project_id:, recording_id:)
        paginate(bucket_path(project_id, "/recordings/#{recording_id}/events.json"))
      end

      # Gets the subscription status for a recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [Hash] subscription data
      def get_subscription(project_id:, recording_id:)
        http_get(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json")).json
      end

      # Subscribes to a recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [Hash] subscription data
      def subscribe(project_id:, recording_id:)
        http_post(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json")).json
      end

      # Unsubscribes from a recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [void]
      def unsubscribe(project_id:, recording_id:)
        http_delete(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json"))
        nil
      end

      # Sets client visibility for a recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @param visible [Boolean] whether clients can see this recording
      # @return [void]
      def set_client_visibility(project_id:, recording_id:, visible:)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/client_visibility.json"),
                 body: { visible: visible })
        nil
      end
    end
  end
end
