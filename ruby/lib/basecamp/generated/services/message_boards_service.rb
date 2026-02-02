# frozen_string_literal: true

module Basecamp
  module Services
    # Service for MessageBoards operations
    #
    # @generated from OpenAPI spec
    class MessageBoardsService < BaseService

      # Get a message board
      # @param board_id [Integer] board id ID
      # @return [Hash] response data
      def get(board_id:)
        with_operation(service: "messageboards", operation: "get", is_mutation: false, project_id: project_id, resource_id: board_id) do
          http_get("/message_boards/#{board_id}").json
        end
      end
    end
  end
end
