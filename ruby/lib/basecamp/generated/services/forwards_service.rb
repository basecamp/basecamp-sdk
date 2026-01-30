# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Forwards operations
    #
    # @generated from OpenAPI spec
    class ForwardsService < BaseService

      # Get a forward by ID
      def get(project_id:, forward_id:)
        http_get(bucket_path(project_id, "/inbox_forwards/#{forward_id}")).json
      end

      # List all replies to a forward
      def list_replies(project_id:, forward_id:)
        paginate(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies.json"))
      end

      # Create a reply to a forward
      def create_reply(project_id:, forward_id:, **body)
        http_post(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies.json"), body: body).json
      end

      # Get a forward reply by ID
      def get_reply(project_id:, forward_id:, reply_id:)
        http_get(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies/#{reply_id}")).json
      end

      # Get an inbox by ID
      def get_inbox(project_id:, inbox_id:)
        http_get(bucket_path(project_id, "/inboxes/#{inbox_id}")).json
      end

      # List all forwards in an inbox
      def list(project_id:, inbox_id:)
        paginate(bucket_path(project_id, "/inboxes/#{inbox_id}/forwards.json"))
      end
    end
  end
end
