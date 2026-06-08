# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Tools operations
    #
    # @generated from OpenAPI spec
    class ToolsService < BaseService

      # Create a tool in a project dock
      # @param bucket_id [Integer] bucket id ID
      # @param tool_type [String] Tool type to add to the project dock. Values: Chat::Transcript|Inbox|Kanban::Board|Message::Board|Questionnaire|Schedule|Todoset|Vault.
      # @param title [String, nil] Title for the new tool. When omitted, Basecamp assigns the next available default title for the tool type.
      # @return [Hash] response data
      def create(bucket_id:, tool_type:, title: nil)
        with_operation(service: "tools", operation: "create", is_mutation: true, resource_id: bucket_id) do
          http_post("/buckets/#{bucket_id}/dock/tools.json", body: compact_params(tool_type: tool_type, title: title)).json
        end
      end

      # Get a dock tool by id
      # @param tool_id [Integer] tool id ID
      # @return [Hash] response data
      def get(tool_id:)
        with_operation(service: "tools", operation: "get", is_mutation: false, resource_id: tool_id) do
          http_get("/dock/tools/#{tool_id}").json
        end
      end

      # Update (rename) an existing tool
      # @param tool_id [Integer] tool id ID
      # @param title [String] title
      # @return [Hash] response data
      def update(tool_id:, title:)
        with_operation(service: "tools", operation: "update", is_mutation: true, resource_id: tool_id) do
          http_put("/dock/tools/#{tool_id}", body: compact_params(title: title)).json
        end
      end

      # Delete a tool (trash it)
      # @param tool_id [Integer] tool id ID
      # @return [void]
      def delete(tool_id:)
        with_operation(service: "tools", operation: "delete", is_mutation: true, resource_id: tool_id) do
          http_delete("/dock/tools/#{tool_id}")
          nil
        end
      end

      # Enable a tool (show it on the project dock)
      # @param tool_id [Integer] tool id ID
      # @return [void]
      def enable(tool_id:)
        with_operation(service: "tools", operation: "enable", is_mutation: true, resource_id: tool_id) do
          http_post("/recordings/#{tool_id}/position.json")
          nil
        end
      end

      # Reposition a tool on the project dock
      # @param tool_id [Integer] tool id ID
      # @param position [Integer] position
      # @return [void]
      def reposition(tool_id:, position:)
        with_operation(service: "tools", operation: "reposition", is_mutation: true, resource_id: tool_id) do
          http_put("/recordings/#{tool_id}/position.json", body: compact_params(position: position))
          nil
        end
      end

      # Disable a tool (hide it from the project dock)
      # @param tool_id [Integer] tool id ID
      # @return [void]
      def disable(tool_id:)
        with_operation(service: "tools", operation: "disable", is_mutation: true, resource_id: tool_id) do
          http_delete("/recordings/#{tool_id}/position.json")
          nil
        end
      end
    end
  end
end
