# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Campfires operations
    #
    # @generated from OpenAPI spec
    class CampfiresService < BaseService

      # Get a campfire by ID
      def get(project_id:, campfire_id:)
        http_get(bucket_path(project_id, "/chats/#{campfire_id}")).json
      end

      # List all chatbots for a campfire
      def list_chatbots(project_id:, campfire_id:)
        paginate(bucket_path(project_id, "/chats/#{campfire_id}/integrations.json"))
      end

      # Create a new chatbot for a campfire
      def create_chatbot(project_id:, campfire_id:, **body)
        http_post(bucket_path(project_id, "/chats/#{campfire_id}/integrations.json"), body: body).json
      end

      # Get a chatbot by ID
      def get_chatbot(project_id:, campfire_id:, chatbot_id:)
        http_get(bucket_path(project_id, "/chats/#{campfire_id}/integrations/#{chatbot_id}")).json
      end

      # Update an existing chatbot
      def update_chatbot(project_id:, campfire_id:, chatbot_id:, **body)
        http_put(bucket_path(project_id, "/chats/#{campfire_id}/integrations/#{chatbot_id}"), body: body).json
      end

      # Delete a chatbot
      def delete_chatbot(project_id:, campfire_id:, chatbot_id:)
        http_delete(bucket_path(project_id, "/chats/#{campfire_id}/integrations/#{chatbot_id}"))
        nil
      end

      # List all lines (messages) in a campfire
      def list_lines(project_id:, campfire_id:)
        paginate(bucket_path(project_id, "/chats/#{campfire_id}/lines.json"))
      end

      # Create a new line (message) in a campfire
      def create_line(project_id:, campfire_id:, **body)
        http_post(bucket_path(project_id, "/chats/#{campfire_id}/lines.json"), body: body).json
      end

      # Get a campfire line by ID
      def get_line(project_id:, campfire_id:, line_id:)
        http_get(bucket_path(project_id, "/chats/#{campfire_id}/lines/#{line_id}")).json
      end

      # Delete a campfire line
      def delete_line(project_id:, campfire_id:, line_id:)
        http_delete(bucket_path(project_id, "/chats/#{campfire_id}/lines/#{line_id}"))
        nil
      end

      # List all campfires across the account
      def list()
        paginate("/chats.json")
      end
    end
  end
end
