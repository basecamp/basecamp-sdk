# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Messages operations
    #
    # @generated from OpenAPI spec
    class MessagesService < BaseService

      # List messages on a message board
      # @param board_id [Integer] board id ID
      # @param sort [String, nil] created_at|updated_at
      # @param direction [String, nil] asc|desc
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(board_id:, sort: nil, direction: nil, page: nil, max_items: nil)
        wrap_paginated(service: "messages", operation: "list", is_mutation: false, resource_id: board_id) do
          params = compact_query_params(sort: sort, direction: direction, page: page)
          paginate("/message_boards/#{board_id}/messages.json", params: params, operation: "ListMessages", max_items: max_items)
        end
      end

      # Create a new message on a message board
      # @param board_id [Integer] board id ID
      # @param subject [String] subject
      # @param content [String, nil] content
      # @param status [String, nil] active|drafted
      # @param category_id [Integer, nil] category id
      # @param subscriptions [Array, nil] subscriptions
      # @param visible_to_clients [Boolean, nil] visible to clients
      # @return [Hash] response data
      def create(board_id:, subject:, content: nil, status: nil, category_id: nil, subscriptions: nil, visible_to_clients: nil)
        with_operation(service: "messages", operation: "create", is_mutation: true, resource_id: board_id) do
          http_post("/message_boards/#{board_id}/messages.json", body: compact_params(subject: subject, content: content, status: status, category_id: category_id, subscriptions: subscriptions, visible_to_clients: visible_to_clients)).json
        end
      end

      # Get a single message by id
      # @param message_id [Integer] message id ID
      # @return [Hash] response data
      def get(message_id:)
        with_operation(service: "messages", operation: "get", is_mutation: false, resource_id: message_id) do
          http_get("/messages/#{message_id}", operation: "GetMessage").json
        end
      end

      # Update an existing message
      # @param message_id [Integer] message id ID
      # @param subject [String, nil] subject
      # @param content [String, nil] content
      # @param status [String, nil] active|drafted
      # @param category_id [Integer, nil] category id
      # @return [Hash] response data
      def update(message_id:, subject: nil, content: nil, status: nil, category_id: nil)
        with_operation(service: "messages", operation: "update", is_mutation: true, resource_id: message_id) do
          http_put("/messages/#{message_id}", body: compact_params(subject: subject, content: content, status: status, category_id: category_id)).json
        end
      end

      # Pin a message to the top of the message board
      # @param message_id [Integer] message id ID
      # @return [void]
      def pin(message_id:)
        with_operation(service: "messages", operation: "pin", is_mutation: true, resource_id: message_id) do
          http_post("/recordings/#{message_id}/pin.json")
          nil
        end
      end

      # Unpin a message from the message board
      # @param message_id [Integer] message id ID
      # @return [void]
      def unpin(message_id:)
        with_operation(service: "messages", operation: "unpin", is_mutation: true, resource_id: message_id) do
          http_delete("/recordings/#{message_id}/pin.json")
          nil
        end
      end
    end
  end
end
