# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Tools operations
    #
    # @generated from OpenAPI spec
    class ToolsService < BaseService

      # Clone an existing tool to create a new one
      def clone(project_id:, source_tool_id:)
        http_post(bucket_path(project_id, "/dock/tools/#{source_tool_id}/clone.json")).json
      end

      # Get a dock tool by id
      def get(project_id:, tool_id:)
        http_get(bucket_path(project_id, "/dock/tools/#{tool_id}")).json
      end

      # Update (rename) an existing tool
      def update(project_id:, tool_id:, **body)
        http_put(bucket_path(project_id, "/dock/tools/#{tool_id}"), body: body).json
      end

      # Delete a tool (trash it)
      def delete(project_id:, tool_id:)
        http_delete(bucket_path(project_id, "/dock/tools/#{tool_id}"))
        nil
      end

      # Enable a tool (show it on the project dock)
      def enable(project_id:, tool_id:)
        http_post(bucket_path(project_id, "/dock/tools/#{tool_id}/position.json"))
        nil
      end

      # Reposition a tool on the project dock
      def reposition(project_id:, tool_id:, **body)
        http_put(bucket_path(project_id, "/dock/tools/#{tool_id}/position.json"), body: body)
        nil
      end

      # Disable a tool (hide it from the project dock)
      def disable(project_id:, tool_id:)
        http_delete(bucket_path(project_id, "/dock/tools/#{tool_id}/position.json"))
        nil
      end
    end
  end
end
