# frozen_string_literal: true

module Basecamp
  module Services
    # Service for message board operations.
    #
    # Each project has a message board where team members can post messages
    # (announcements, updates, etc.).
    #
    # @example Get a message board
    #   board = account.message_boards.get(project_id: 123, board_id: 456)
    #   puts "#{board["title"]} - #{board["messages_count"]} messages"
    class MessageBoardsService < BaseService
      # Gets a specific message board.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param board_id [Integer, String] message board ID
      # @return [Hash] message board data
      def get(project_id:, board_id:)
        http_get(bucket_path(project_id, "/message_boards/#{board_id}")).json
      end
    end
  end
end
