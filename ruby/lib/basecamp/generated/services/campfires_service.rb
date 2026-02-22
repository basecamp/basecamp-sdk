# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Campfires operations
    #
    # @generated from OpenAPI spec
    class CampfiresService < BaseService

      # List all campfires across the account
      # @return [Enumerator<Hash>] paginated results
      def list()
        wrap_paginated(service: "campfires", operation: "list", is_mutation: false) do
          paginate("/chats.json")
        end
      end

      # Get a campfire by ID
      # @param campfire_id [Integer] campfire id ID
      # @return [Hash] response data
      def get(campfire_id:)
        with_operation(service: "campfires", operation: "get", is_mutation: false, resource_id: campfire_id) do
          http_get("/chats/#{campfire_id}").json
        end
      end

      # List all chatbots for a campfire
      # @param campfire_id [Integer] campfire id ID
      # @return [Enumerator<Hash>] paginated results
      def list_chatbots(campfire_id:)
        wrap_paginated(service: "campfires", operation: "list_chatbots", is_mutation: false, resource_id: campfire_id) do
          paginate("/chats/#{campfire_id}/integrations.json")
        end
      end

      # Create a new chatbot for a campfire
      # @param campfire_id [Integer] campfire id ID
      # @param service_name [String] service name
      # @param command_url [String, nil] command url
      # @return [Hash] response data
      def create_chatbot(campfire_id:, service_name:, command_url: nil)
        with_operation(service: "campfires", operation: "create_chatbot", is_mutation: true, resource_id: campfire_id) do
          http_post("/chats/#{campfire_id}/integrations.json", body: compact_params(service_name: service_name, command_url: command_url)).json
        end
      end

      # Get a chatbot by ID
      # @param campfire_id [Integer] campfire id ID
      # @param chatbot_id [Integer] chatbot id ID
      # @return [Hash] response data
      def get_chatbot(campfire_id:, chatbot_id:)
        with_operation(service: "campfires", operation: "get_chatbot", is_mutation: false, resource_id: chatbot_id) do
          http_get("/chats/#{campfire_id}/integrations/#{chatbot_id}").json
        end
      end

      # Update an existing chatbot
      # @param campfire_id [Integer] campfire id ID
      # @param chatbot_id [Integer] chatbot id ID
      # @param service_name [String] service name
      # @param command_url [String, nil] command url
      # @return [Hash] response data
      def update_chatbot(campfire_id:, chatbot_id:, service_name:, command_url: nil)
        with_operation(service: "campfires", operation: "update_chatbot", is_mutation: true, resource_id: chatbot_id) do
          http_put("/chats/#{campfire_id}/integrations/#{chatbot_id}", body: compact_params(service_name: service_name, command_url: command_url)).json
        end
      end

      # Delete a chatbot
      # @param campfire_id [Integer] campfire id ID
      # @param chatbot_id [Integer] chatbot id ID
      # @return [void]
      def delete_chatbot(campfire_id:, chatbot_id:)
        with_operation(service: "campfires", operation: "delete_chatbot", is_mutation: true, resource_id: chatbot_id) do
          http_delete("/chats/#{campfire_id}/integrations/#{chatbot_id}")
          nil
        end
      end

      # List all lines (messages) in a campfire
      # @param campfire_id [Integer] campfire id ID
      # @return [Enumerator<Hash>] paginated results
      def list_lines(campfire_id:)
        wrap_paginated(service: "campfires", operation: "list_lines", is_mutation: false, resource_id: campfire_id) do
          paginate("/chats/#{campfire_id}/lines.json")
        end
      end

      # Create a new line (message) in a campfire
      # @param campfire_id [Integer] campfire id ID
      # @param content [String] content
      # @param content_type [String, nil] content type
      # @return [Hash] response data
      def create_line(campfire_id:, content:, content_type: nil)
        with_operation(service: "campfires", operation: "create_line", is_mutation: true, resource_id: campfire_id) do
          http_post("/chats/#{campfire_id}/lines.json", body: compact_params(content: content, content_type: content_type)).json
        end
      end

      # Get a campfire line by ID
      # @param campfire_id [Integer] campfire id ID
      # @param line_id [Integer] line id ID
      # @return [Hash] response data
      def get_line(campfire_id:, line_id:)
        with_operation(service: "campfires", operation: "get_line", is_mutation: false, resource_id: line_id) do
          http_get("/chats/#{campfire_id}/lines/#{line_id}").json
        end
      end

      # Delete a campfire line
      # @param campfire_id [Integer] campfire id ID
      # @param line_id [Integer] line id ID
      # @return [void]
      def delete_line(campfire_id:, line_id:)
        with_operation(service: "campfires", operation: "delete_line", is_mutation: true, resource_id: line_id) do
          http_delete("/chats/#{campfire_id}/lines/#{line_id}")
          nil
        end
      end
    end
  end
end
