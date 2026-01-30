# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Messages operations
    #
    # @generated from OpenAPI spec
    class MessagesService < BaseService

      # List messages on a message board
      def list(project_id:, board_id:)
        paginate(bucket_path(project_id, "/message_boards/#{board_id}/messages.json"))
      end

      # Create a new message on a message board
      def create(project_id:, board_id:, **body)
        http_post(bucket_path(project_id, "/message_boards/#{board_id}/messages.json"), body: body).json
      end

      # Get a single message by id
      def get(project_id:, message_id:)
        http_get(bucket_path(project_id, "/messages/#{message_id}")).json
      end

      # Update an existing message
      def update(project_id:, message_id:, **body)
        http_put(bucket_path(project_id, "/messages/#{message_id}"), body: body).json
      end

      # Pin a message to the top of the message board
      def pin(project_id:, message_id:)
        http_post(bucket_path(project_id, "/recordings/#{message_id}/pin.json"))
        nil
      end

      # Unpin a message from the message board
      def unpin(project_id:, message_id:)
        http_delete(bucket_path(project_id, "/recordings/#{message_id}/pin.json"))
        nil
      end
    end
  end
end
