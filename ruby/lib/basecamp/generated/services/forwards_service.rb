# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Forwards operations
    #
    # @generated from OpenAPI spec
    class ForwardsService < BaseService

      # Get a forward by ID
      # @param project_id [Integer] project id ID
      # @param forward_id [Integer] forward id ID
      # @return [Hash] response data
      def get(project_id:, forward_id:)
        http_get(bucket_path(project_id, "/inbox_forwards/#{forward_id}")).json
      end

      # List all replies to a forward
      # @param project_id [Integer] project id ID
      # @param forward_id [Integer] forward id ID
      # @return [Enumerator<Hash>] paginated results
      def list_replies(project_id:, forward_id:)
        paginate(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies.json"))
      end

      # Create a reply to a forward
      # @param project_id [Integer] project id ID
      # @param forward_id [Integer] forward id ID
      # @param content [String] content
      # @return [Hash] response data
      def create_reply(project_id:, forward_id:, content:)
        http_post(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies.json"), body: compact_params(content: content)).json
      end

      # Get a forward reply by ID
      # @param project_id [Integer] project id ID
      # @param forward_id [Integer] forward id ID
      # @param reply_id [Integer] reply id ID
      # @return [Hash] response data
      def get_reply(project_id:, forward_id:, reply_id:)
        http_get(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies/#{reply_id}")).json
      end

      # Get an inbox by ID
      # @param project_id [Integer] project id ID
      # @param inbox_id [Integer] inbox id ID
      # @return [Hash] response data
      def get_inbox(project_id:, inbox_id:)
        http_get(bucket_path(project_id, "/inboxes/#{inbox_id}")).json
      end

      # List all forwards in an inbox
      # @param project_id [Integer] project id ID
      # @param inbox_id [Integer] inbox id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, inbox_id:)
        paginate(bucket_path(project_id, "/inboxes/#{inbox_id}/forwards.json"))
      end
    end
  end
end
