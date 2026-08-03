# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Forwards operations
    #
    # @generated from OpenAPI spec
    class ForwardsService < BaseService

      # Get a forward by ID
      # @param forward_id [Integer] forward id ID
      # @return [Hash] response data
      def get(forward_id:)
        with_operation(service: "forwards", operation: "get", is_mutation: false, resource_id: forward_id) do
          http_get("/inbox_forwards/#{forward_id}", operation: "GetForward").json
        end
      end

      # List all replies to a forward
      # @param forward_id [Integer] forward id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_replies(forward_id:, page: nil, max_items: nil)
        wrap_paginated(service: "forwards", operation: "list_replies", is_mutation: false, resource_id: forward_id) do
          params = compact_query_params(page: page)
          paginate("/inbox_forwards/#{forward_id}/replies.json", params: params, operation: "ListForwardReplies", max_items: max_items)
        end
      end

      # Get a forward reply by ID
      # @param forward_id [Integer] forward id ID
      # @param reply_id [Integer] reply id ID
      # @return [Hash] response data
      def get_reply(forward_id:, reply_id:)
        with_operation(service: "forwards", operation: "get_reply", is_mutation: false, resource_id: reply_id) do
          http_get("/inbox_forwards/#{forward_id}/replies/#{reply_id}", operation: "GetForwardReply").json
        end
      end

      # Get an inbox by ID
      # @param inbox_id [Integer] inbox id ID
      # @return [Hash] response data
      def get_inbox(inbox_id:)
        with_operation(service: "forwards", operation: "get_inbox", is_mutation: false, resource_id: inbox_id) do
          http_get("/inboxes/#{inbox_id}", operation: "GetInbox").json
        end
      end

      # List all forwards in an inbox
      # @param inbox_id [Integer] inbox id ID
      # @param sort [String, nil] created_at|updated_at
      # @param direction [String, nil] asc|desc
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(inbox_id:, sort: nil, direction: nil, page: nil, max_items: nil)
        wrap_paginated(service: "forwards", operation: "list", is_mutation: false, resource_id: inbox_id) do
          params = compact_query_params(sort: sort, direction: direction, page: page)
          paginate("/inboxes/#{inbox_id}/inbox_forwards.json", params: params, operation: "ListForwards", max_items: max_items)
        end
      end
    end
  end
end
