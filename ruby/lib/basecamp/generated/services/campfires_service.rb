# frozen_string_literal: true

require "uri"

module Basecamp
  module Services
    # Service for Campfires operations
    #
    # @generated from OpenAPI spec
    class CampfiresService < BaseService

      # List all chatbots for a campfire
      # @param bucket_id [Integer] bucket id ID
      # @param campfire_id [Integer] campfire id ID
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_chatbots(bucket_id:, campfire_id:, max_items: nil)
        wrap_paginated(service: "campfires", operation: "list_chatbots", is_mutation: false, project_id: bucket_id, resource_id: campfire_id) do
          paginate("/buckets/#{bucket_id}/chats/#{campfire_id}/integrations.json", operation: "ListChatbots", max_items: max_items)
        end
      end

      # Create a new chatbot for a campfire
      # @param bucket_id [Integer] bucket id ID
      # @param campfire_id [Integer] campfire id ID
      # @param service_name [String] service name
      # @param command_url [String, nil] command url
      # @return [Hash] response data
      def create_chatbot(bucket_id:, campfire_id:, service_name:, command_url: nil)
        with_operation(service: "campfires", operation: "create_chatbot", is_mutation: true, project_id: bucket_id, resource_id: campfire_id) do
          http_post("/buckets/#{bucket_id}/chats/#{campfire_id}/integrations.json", body: compact_params(service_name: service_name, command_url: command_url)).json
        end
      end

      # Get a chatbot by ID
      # @param bucket_id [Integer] bucket id ID
      # @param campfire_id [Integer] campfire id ID
      # @param chatbot_id [Integer] chatbot id ID
      # @return [Hash] response data
      def get_chatbot(bucket_id:, campfire_id:, chatbot_id:)
        with_operation(service: "campfires", operation: "get_chatbot", is_mutation: false, project_id: bucket_id, resource_id: chatbot_id) do
          http_get("/buckets/#{bucket_id}/chats/#{campfire_id}/integrations/#{chatbot_id}", operation: "GetChatbot").json
        end
      end

      # Update an existing chatbot
      # @param bucket_id [Integer] bucket id ID
      # @param campfire_id [Integer] campfire id ID
      # @param chatbot_id [Integer] chatbot id ID
      # @param service_name [String] service name
      # @param command_url [String, nil] command url
      # @return [Hash] response data
      def update_chatbot(bucket_id:, campfire_id:, chatbot_id:, service_name:, command_url: nil)
        with_operation(service: "campfires", operation: "update_chatbot", is_mutation: true, project_id: bucket_id, resource_id: chatbot_id) do
          http_put("/buckets/#{bucket_id}/chats/#{campfire_id}/integrations/#{chatbot_id}", body: compact_params(service_name: service_name, command_url: command_url)).json
        end
      end

      # Delete a chatbot
      # @param bucket_id [Integer] bucket id ID
      # @param campfire_id [Integer] campfire id ID
      # @param chatbot_id [Integer] chatbot id ID
      # @return [void]
      def delete_chatbot(bucket_id:, campfire_id:, chatbot_id:)
        with_operation(service: "campfires", operation: "delete_chatbot", is_mutation: true, project_id: bucket_id, resource_id: chatbot_id) do
          http_delete("/buckets/#{bucket_id}/chats/#{campfire_id}/integrations/#{chatbot_id}")
          nil
        end
      end

      # List all campfires across the account
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(page: nil, max_items: nil)
        wrap_paginated(service: "campfires", operation: "list", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/chats.json", params: params, operation: "ListCampfires", max_items: max_items)
        end
      end

      # Get a campfire by ID
      # @param campfire_id [Integer] campfire id ID
      # @return [Hash] response data
      def get(campfire_id:)
        with_operation(service: "campfires", operation: "get", is_mutation: false, resource_id: campfire_id) do
          http_get("/chats/#{campfire_id}", operation: "GetCampfire").json
        end
      end

      # List all lines (messages) in a campfire
      # @param campfire_id [Integer] campfire id ID
      # @param sort [String, nil] created_at|updated_at
      # @param direction [String, nil] asc|desc
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_lines(campfire_id:, sort: nil, direction: nil, page: nil, max_items: nil)
        wrap_paginated(service: "campfires", operation: "list_lines", is_mutation: false, resource_id: campfire_id) do
          params = compact_query_params(sort: sort, direction: direction, page: page)
          paginate("/chats/#{campfire_id}/lines.json", params: params, operation: "ListCampfireLines", max_items: max_items)
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
          http_get("/chats/#{campfire_id}/lines/#{line_id}", operation: "GetCampfireLine").json
        end
      end

      # Update an existing campfire line; the content is always treated as rich text (HTML).
      # @param campfire_id [Integer] campfire id ID
      # @param line_id [Integer] line id ID
      # @param content [String] The new line content, interpreted as rich text (HTML)
      # @return [void]
      def update_line(campfire_id:, line_id:, content:)
        with_operation(service: "campfires", operation: "update_line", is_mutation: true, resource_id: line_id) do
          http_put("/chats/#{campfire_id}/lines/#{line_id}", body: compact_params(content: content))
          nil
        end
      end

      # Delete a campfire line; allowed for the line's creator or an admin.
      # @param campfire_id [Integer] campfire id ID
      # @param line_id [Integer] line id ID
      # @return [void]
      def delete_line(campfire_id:, line_id:)
        with_operation(service: "campfires", operation: "delete_line", is_mutation: true, resource_id: line_id) do
          http_delete("/chats/#{campfire_id}/lines/#{line_id}")
          nil
        end
      end

      # List uploaded files in a campfire
      # @param campfire_id [Integer] campfire id ID
      # @param sort [String, nil] created_at|updated_at
      # @param direction [String, nil] asc|desc
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_uploads(campfire_id:, sort: nil, direction: nil, page: nil, max_items: nil)
        wrap_paginated(service: "campfires", operation: "list_uploads", is_mutation: false, resource_id: campfire_id) do
          params = compact_query_params(sort: sort, direction: direction, page: page)
          paginate("/chats/#{campfire_id}/uploads.json", params: params, operation: "ListCampfireUploads", max_items: max_items)
        end
      end

      # Upload a file to a campfire
      # @param campfire_id [Integer] campfire id ID
      # @param data [String] Binary file data to upload
      # @param content_type [String] MIME type of the file (e.g., "application/pdf", "image/png")
      # @param name [String] Filename for the uploaded file (e.g. "report.pdf").
      # @return [Hash] response data
      def create_upload(campfire_id:, data:, content_type:, name:)
        with_operation(service: "campfires", operation: "create_upload", is_mutation: true, resource_id: campfire_id) do
          http_post_raw("/chats/#{campfire_id}/uploads.json?name=#{URI.encode_www_form_component(name.to_s)}", body: data, content_type: content_type).json
        end
      end
    end
  end
end
