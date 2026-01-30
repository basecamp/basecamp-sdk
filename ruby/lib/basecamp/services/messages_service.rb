# frozen_string_literal: true

module Basecamp
  module Services
    # Service for message operations.
    #
    # Messages are posts on a project's message board. They have a subject,
    # content, and can be categorized with message types.
    #
    # @example List messages
    #   account.messages.list(project_id: 123, board_id: 456).each do |message|
    #     puts "#{message["subject"]} by #{message["creator"]["name"]}"
    #   end
    #
    # @example Create a message
    #   message = account.messages.create(
    #     project_id: 123,
    #     board_id: 456,
    #     subject: "Project Update",
    #     content: "<p>Here's what happened this week...</p>"
    #   )
    #
    # @example Pin a message
    #   account.messages.pin(project_id: 123, message_id: 789)
    class MessagesService < BaseService
      # Lists all messages on a message board.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param board_id [Integer, String] message board ID
      # @return [Enumerator<Hash>] messages
      def list(project_id:, board_id:)
        paginate(bucket_path(project_id, "/message_boards/#{board_id}/messages.json"))
      end

      # Gets a specific message.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param message_id [Integer, String] message ID
      # @return [Hash] message data
      def get(project_id:, message_id:)
        http_get(bucket_path(project_id, "/messages/#{message_id}")).json
      end

      # Creates a new message on a message board.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param board_id [Integer, String] message board ID
      # @param subject [String] message title
      # @param content [String, nil] message body in HTML
      # @param status [String, nil] "drafted" or "active" (defaults to active)
      # @param category_id [Integer, nil] message type ID
      # @return [Hash] created message
      def create(project_id:, board_id:, subject:, content: nil, status: nil, category_id: nil)
        body = compact_params(
          subject: subject,
          content: content,
          status: status,
          category_id: category_id
        )
        http_post(bucket_path(project_id, "/message_boards/#{board_id}/messages.json"), body: body).json
      end

      # Updates an existing message.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param message_id [Integer, String] message ID
      # @param subject [String, nil] new title
      # @param content [String, nil] new content in HTML
      # @param status [String, nil] "drafted" or "active"
      # @param category_id [Integer, nil] message type ID
      # @return [Hash] updated message
      def update(project_id:, message_id:, subject: nil, content: nil, status: nil, category_id: nil)
        body = compact_params(
          subject: subject,
          content: content,
          status: status,
          category_id: category_id
        )
        http_put(bucket_path(project_id, "/messages/#{message_id}"), body: body).json
      end

      # Pins a message to the top of the message board.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param message_id [Integer, String] message ID
      # @return [void]
      def pin(project_id:, message_id:)
        http_post(bucket_path(project_id, "/recordings/#{message_id}/pin.json"))
        nil
      end

      # Unpins a message from the top of the message board.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param message_id [Integer, String] message ID
      # @return [void]
      def unpin(project_id:, message_id:)
        http_delete(bucket_path(project_id, "/recordings/#{message_id}/pin.json"))
        nil
      end

      # Archives a message.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param message_id [Integer, String] message ID
      # @return [void]
      def archive(project_id:, message_id:)
        http_put(bucket_path(project_id, "/recordings/#{message_id}/status/archived.json"))
        nil
      end

      # Restores an archived message to active status.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param message_id [Integer, String] message ID
      # @return [void]
      def unarchive(project_id:, message_id:)
        http_put(bucket_path(project_id, "/recordings/#{message_id}/status/active.json"))
        nil
      end

      # Moves a message to the trash.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param message_id [Integer, String] message ID
      # @return [void]
      def trash(project_id:, message_id:)
        http_put(bucket_path(project_id, "/recordings/#{message_id}/status/trashed.json"))
        nil
      end
    end
  end
end
