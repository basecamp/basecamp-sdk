# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Tools operations
    #
    # @generated from OpenAPI spec
    class ToolsService < BaseService

      # Clone an existing tool to create a new one
      # @param source_recording_id [Integer] source recording id
      # @return [Hash] response data
      def clone(source_recording_id:)
        http_post("/dock/tools.json", body: compact_params(source_recording_id: source_recording_id)).json
      end

      # Get a dock tool by id
      # @param tool_id [Integer] tool id ID
      # @return [Hash] response data
      def get(tool_id:)
        http_get("/dock/tools/#{tool_id}").json
      end

      # Update (rename) an existing tool
      # @param tool_id [Integer] tool id ID
      # @param title [String] title
      # @return [Hash] response data
      def update(tool_id:, title:)
        http_put("/dock/tools/#{tool_id}", body: compact_params(title: title)).json
      end

      # Delete a tool (trash it)
      # @param tool_id [Integer] tool id ID
      # @return [void]
      def delete(tool_id:)
        http_delete("/dock/tools/#{tool_id}")
        nil
      end

      # Enable a tool (show it on the project dock)
      # @param tool_id [Integer] tool id ID
      # @return [void]
      def enable(tool_id:)
        http_post("/recordings/#{tool_id}/position.json")
        nil
      end

      # Reposition a tool on the project dock
      # @param tool_id [Integer] tool id ID
      # @param position [Integer] position
      # @return [void]
      def reposition(tool_id:, position:)
        http_put("/recordings/#{tool_id}/position.json", body: compact_params(position: position))
        nil
      end

      # Disable a tool (hide it from the project dock)
      # @param tool_id [Integer] tool id ID
      # @return [void]
      def disable(tool_id:)
        http_delete("/recordings/#{tool_id}/position.json")
        nil
      end
    end
  end
end
