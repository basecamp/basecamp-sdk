# frozen_string_literal: true

module Basecamp
  module Services
    # Service for MessageBoards operations
    #
    # @generated from OpenAPI spec
    class MessageBoardsService < BaseService

      # Get a message board
      def get(project_id:, board_id:)
        http_get(bucket_path(project_id, "/message_boards/#{board_id}")).json
      end
    end
  end
end
