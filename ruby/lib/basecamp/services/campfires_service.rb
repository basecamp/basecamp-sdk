# frozen_string_literal: true

module Basecamp
  module Services
    # Service for campfire (chat) operations.
    #
    # Campfires are real-time chat rooms within Basecamp projects.
    # They contain lines (messages) and can have chatbot integrations.
    #
    # @example List all campfires
    #   account.campfires.list.each do |campfire|
    #     puts campfire["title"]
    #   end
    #
    # @example Send a message
    #   line = account.campfires.create_line(
    #     project_id: 123,
    #     campfire_id: 456,
    #     content: "Hello team!"
    #   )
    class CampfiresService < BaseService
      # Lists all campfires across the account.
      #
      # @return [Enumerator<Hash>] campfires
      def list
        paginate("/chats.json")
      end

      # Gets a specific campfire.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param campfire_id [Integer, String] campfire ID
      # @return [Hash] campfire data
      def get(project_id:, campfire_id:)
        http_get(bucket_path(project_id, "/chats/#{campfire_id}.json")).json
      end

      # Lists all lines (messages) in a campfire.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param campfire_id [Integer, String] campfire ID
      # @return [Enumerator<Hash>] campfire lines
      def list_lines(project_id:, campfire_id:)
        paginate(bucket_path(project_id, "/chats/#{campfire_id}/lines.json"))
      end

      # Gets a specific line (message) from a campfire.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param campfire_id [Integer, String] campfire ID
      # @param line_id [Integer, String] line ID
      # @return [Hash] campfire line
      def get_line(project_id:, campfire_id:, line_id:)
        http_get(bucket_path(project_id, "/chats/#{campfire_id}/lines/#{line_id}.json")).json
      end

      # Creates a new line (message) in a campfire.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param campfire_id [Integer, String] campfire ID
      # @param content [String] plain text message content
      # @return [Hash] created line
      def create_line(project_id:, campfire_id:, content:)
        body = { content: content }
        http_post(bucket_path(project_id, "/chats/#{campfire_id}/lines.json"), body: body).json
      end

      # Deletes a line (message) from a campfire.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param campfire_id [Integer, String] campfire ID
      # @param line_id [Integer, String] line ID
      # @return [void]
      def delete_line(project_id:, campfire_id:, line_id:)
        http_delete(bucket_path(project_id, "/chats/#{campfire_id}/lines/#{line_id}.json"))
        nil
      end

      # Lists all chatbots for a campfire.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param campfire_id [Integer, String] campfire ID
      # @return [Enumerator<Hash>] chatbots
      def list_chatbots(project_id:, campfire_id:)
        paginate(bucket_path(project_id, "/chats/#{campfire_id}/integrations.json"))
      end

      # Gets a specific chatbot.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param campfire_id [Integer, String] campfire ID
      # @param chatbot_id [Integer, String] chatbot ID
      # @return [Hash] chatbot data
      def get_chatbot(project_id:, campfire_id:, chatbot_id:)
        http_get(bucket_path(project_id, "/chats/#{campfire_id}/integrations/#{chatbot_id}.json")).json
      end

      # Creates a new chatbot for a campfire.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param campfire_id [Integer, String] campfire ID
      # @param service_name [String] chatbot name (no spaces, emoji, or non-word characters)
      # @param command_url [String, nil] HTTPS URL for bot callbacks
      # @return [Hash] created chatbot with lines_url for posting
      def create_chatbot(project_id:, campfire_id:, service_name:, command_url: nil)
        body = compact_params(
          service_name: service_name,
          command_url: command_url
        )
        http_post(bucket_path(project_id, "/chats/#{campfire_id}/integrations.json"), body: body).json
      end

      # Updates an existing chatbot.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param campfire_id [Integer, String] campfire ID
      # @param chatbot_id [Integer, String] chatbot ID
      # @param service_name [String] new chatbot name
      # @param command_url [String, nil] new callback URL
      # @return [Hash] updated chatbot
      def update_chatbot(project_id:, campfire_id:, chatbot_id:, service_name:, command_url: nil)
        body = compact_params(
          service_name: service_name,
          command_url: command_url
        )
        http_put(bucket_path(project_id, "/chats/#{campfire_id}/integrations/#{chatbot_id}.json"), body: body).json
      end

      # Deletes a chatbot.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param campfire_id [Integer, String] campfire ID
      # @param chatbot_id [Integer, String] chatbot ID
      # @return [void]
      def delete_chatbot(project_id:, campfire_id:, chatbot_id:)
        http_delete(bucket_path(project_id, "/chats/#{campfire_id}/integrations/#{chatbot_id}.json"))
        nil
      end
    end
  end
end
