# frozen_string_literal: true

module Basecamp
  module Services
    # Service for MessageBoards operations
    #
    # @generated from OpenAPI spec
    class MessageBoardsService < BaseService

      # Get a message board
      # @param project_id [Integer] project id ID
      # @param board_id [Integer] board id ID
      # @return [Hash] response data
      def get(project_id:, board_id:)
        http_get(bucket_path(project_id, "/message_boards/#{board_id}")).json
      end
    end
  end
end
