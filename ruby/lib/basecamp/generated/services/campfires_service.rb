# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Campfires operations
    #
    # @generated from OpenAPI spec
    class CampfiresService < BaseService

      # Get a campfire by ID
      # @param project_id [Integer] project id ID
      # @param campfire_id [Integer] campfire id ID
      # @return [Hash] response data
      def get(project_id:, campfire_id:)
        http_get(bucket_path(project_id, "/chats/#{campfire_id}")).json
      end

      # List all chatbots for a campfire
      # @param project_id [Integer] project id ID
      # @param campfire_id [Integer] campfire id ID
      # @return [Enumerator<Hash>] paginated results
      def list_chatbots(project_id:, campfire_id:)
        paginate(bucket_path(project_id, "/chats/#{campfire_id}/integrations.json"))
      end

      # Create a new chatbot for a campfire
      # @param project_id [Integer] project id ID
      # @param campfire_id [Integer] campfire id ID
      # @param service_name [String] service name
      # @param command_url [String, nil] command url
      # @return [Hash] response data
      def create_chatbot(project_id:, campfire_id:, service_name:, command_url: nil)
        http_post(bucket_path(project_id, "/chats/#{campfire_id}/integrations.json"), body: compact_params(service_name: service_name, command_url: command_url)).json
      end

      # Get a chatbot by ID
      # @param project_id [Integer] project id ID
      # @param campfire_id [Integer] campfire id ID
      # @param chatbot_id [Integer] chatbot id ID
      # @return [Hash] response data
      def get_chatbot(project_id:, campfire_id:, chatbot_id:)
        http_get(bucket_path(project_id, "/chats/#{campfire_id}/integrations/#{chatbot_id}")).json
      end

      # Update an existing chatbot
      # @param project_id [Integer] project id ID
      # @param campfire_id [Integer] campfire id ID
      # @param chatbot_id [Integer] chatbot id ID
      # @param service_name [String] service name
      # @param command_url [String, nil] command url
      # @return [Hash] response data
      def update_chatbot(project_id:, campfire_id:, chatbot_id:, service_name:, command_url: nil)
        http_put(bucket_path(project_id, "/chats/#{campfire_id}/integrations/#{chatbot_id}"), body: compact_params(service_name: service_name, command_url: command_url)).json
      end

      # Delete a chatbot
      # @param project_id [Integer] project id ID
      # @param campfire_id [Integer] campfire id ID
      # @param chatbot_id [Integer] chatbot id ID
      # @return [void]
      def delete_chatbot(project_id:, campfire_id:, chatbot_id:)
        http_delete(bucket_path(project_id, "/chats/#{campfire_id}/integrations/#{chatbot_id}"))
        nil
      end

      # List all lines (messages) in a campfire
      # @param project_id [Integer] project id ID
      # @param campfire_id [Integer] campfire id ID
      # @return [Enumerator<Hash>] paginated results
      def list_lines(project_id:, campfire_id:)
        paginate(bucket_path(project_id, "/chats/#{campfire_id}/lines.json"))
      end

      # Create a new line (message) in a campfire
      # @param project_id [Integer] project id ID
      # @param campfire_id [Integer] campfire id ID
      # @param content [String] content
      # @return [Hash] response data
      def create_line(project_id:, campfire_id:, content:)
        http_post(bucket_path(project_id, "/chats/#{campfire_id}/lines.json"), body: compact_params(content: content)).json
      end

      # Get a campfire line by ID
      # @param project_id [Integer] project id ID
      # @param campfire_id [Integer] campfire id ID
      # @param line_id [Integer] line id ID
      # @return [Hash] response data
      def get_line(project_id:, campfire_id:, line_id:)
        http_get(bucket_path(project_id, "/chats/#{campfire_id}/lines/#{line_id}")).json
      end

      # Delete a campfire line
      # @param project_id [Integer] project id ID
      # @param campfire_id [Integer] campfire id ID
      # @param line_id [Integer] line id ID
      # @return [void]
      def delete_line(project_id:, campfire_id:, line_id:)
        http_delete(bucket_path(project_id, "/chats/#{campfire_id}/lines/#{line_id}"))
        nil
      end

      # List all campfires across the account
      # @return [Enumerator<Hash>] paginated results
      def list()
        paginate("/chats.json")
      end
    end
  end
end
