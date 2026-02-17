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
        with_operation(service: "forwards", operation: "get", is_mutation: false, project_id: project_id, resource_id: forward_id) do
          http_get(bucket_path(project_id, "/inbox_forwards/#{forward_id}")).json
        end
      end

      # List all replies to a forward
      # @param project_id [Integer] project id ID
      # @param forward_id [Integer] forward id ID
      # @return [Enumerator<Hash>] paginated results
      def list_replies(project_id:, forward_id:)
        wrap_paginated(service: "forwards", operation: "list_replies", is_mutation: false, project_id: project_id, resource_id: forward_id) do
          paginate(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies.json"))
        end
      end

      # Create a reply to a forward
      # @param project_id [Integer] project id ID
      # @param forward_id [Integer] forward id ID
      # @param content [String] content
      # @return [Hash] response data
      def create_reply(project_id:, forward_id:, content:)
        with_operation(service: "forwards", operation: "create_reply", is_mutation: true, project_id: project_id, resource_id: forward_id) do
          http_post(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies.json"), body: compact_params(content: content)).json
        end
      end

      # Get a forward reply by ID
      # @param project_id [Integer] project id ID
      # @param forward_id [Integer] forward id ID
      # @param reply_id [Integer] reply id ID
      # @return [Hash] response data
      def get_reply(project_id:, forward_id:, reply_id:)
        with_operation(service: "forwards", operation: "get_reply", is_mutation: false, project_id: project_id, resource_id: reply_id) do
          http_get(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies/#{reply_id}")).json
        end
      end

      # Get an inbox by ID
      # @param project_id [Integer] project id ID
      # @param inbox_id [Integer] inbox id ID
      # @return [Hash] response data
      def get_inbox(project_id:, inbox_id:)
        with_operation(service: "forwards", operation: "get_inbox", is_mutation: false, project_id: project_id, resource_id: inbox_id) do
          http_get(bucket_path(project_id, "/inboxes/#{inbox_id}")).json
        end
      end

      # List all forwards in an inbox
      # @param project_id [Integer] project id ID
      # @param inbox_id [Integer] inbox id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, inbox_id:)
        wrap_paginated(service: "forwards", operation: "list", is_mutation: false, project_id: project_id, resource_id: inbox_id) do
          paginate(bucket_path(project_id, "/inboxes/#{inbox_id}/forwards.json"))
        end
      end
    end
  end
end
