# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Messages operations
    #
    # @generated from OpenAPI spec
    class MessagesService < BaseService

      # List messages on a message board
      # @param board_id [Integer] board id ID
      # @return [Enumerator<Hash>] paginated results
      def list(board_id:)
        wrap_paginated(service: "messages", operation: "list", is_mutation: false, resource_id: board_id) do
          paginate("/message_boards/#{board_id}/messages.json")
        end
      end

      # Create a new message on a message board
      # @param board_id [Integer] board id ID
      # @param subject [String] subject
      # @param content [String, nil] content
      # @param status [String, nil] active|drafted
      # @param category_id [Integer, nil] category id
      # @return [Hash] response data
      def create(board_id:, subject:, content: nil, status: nil, category_id: nil)
        with_operation(service: "messages", operation: "create", is_mutation: true, resource_id: board_id) do
          http_post("/message_boards/#{board_id}/messages.json", body: compact_params(subject: subject, content: content, status: status, category_id: category_id)).json
        end
      end

      # Get a single message by id
      # @param message_id [Integer] message id ID
      # @return [Hash] response data
      def get(message_id:)
        with_operation(service: "messages", operation: "get", is_mutation: false, resource_id: message_id) do
          http_get("/messages/#{message_id}").json
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
