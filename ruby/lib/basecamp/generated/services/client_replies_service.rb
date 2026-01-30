# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientReplies operations
    #
    # @generated from OpenAPI spec
    class ClientRepliesService < BaseService

      # List all client replies for a recording (correspondence or approval)
      def list(project_id:, recording_id:)
        paginate(bucket_path(project_id, "/client/recordings/#{recording_id}/replies.json"))
      end

      # Get a single client reply by id
      def get(project_id:, recording_id:, reply_id:)
        http_get(bucket_path(project_id, "/client/recordings/#{recording_id}/replies/#{reply_id}")).json
      end
    end
  end
end
