# frozen_string_literal: true

module Basecamp
  module Services
    # Service for dock tool operations.
    #
    # Tools are dock items in a Basecamp project (e.g., Message Board,
    # Todos, Schedule, etc.). This service allows you to manage these tools.
    #
    # @example Get a tool
    #   tool = account.tools.get(project_id: 123, tool_id: 456)
    #   puts "#{tool["name"]} - #{tool["enabled"] ? "enabled" : "disabled"}"
    #
    # @example Enable and reposition a tool
    #   account.tools.enable(project_id: 123, tool_id: 456)
    #   account.tools.reposition(project_id: 123, tool_id: 456, position: 1)
    class ToolsService < BaseService
      # Gets a tool by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param tool_id [Integer, String] tool ID
      # @return [Hash] tool data
      def get(project_id:, tool_id:)
        http_get(bucket_path(project_id, "/dock/tools/#{tool_id}")).json
      end

      # Clones an existing tool to create a new one.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param source_tool_id [Integer, String] ID of the tool to clone
      # @return [Hash] newly created tool
      def clone(project_id:, source_tool_id:)
        http_post(bucket_path(project_id, "/dock/tools/#{source_tool_id}/clone.json")).json
      end

      # Updates (renames) an existing tool.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param tool_id [Integer, String] tool ID
      # @param title [String] new title for the tool
      # @return [Hash] updated tool
      def update(project_id:, tool_id:, title:)
        http_put(bucket_path(project_id, "/dock/tools/#{tool_id}"), body: { title: title }).json
      end

      # Deletes a tool (moves it to trash).
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param tool_id [Integer, String] tool ID
      # @return [void]
      def delete(project_id:, tool_id:)
        http_delete(bucket_path(project_id, "/dock/tools/#{tool_id}"))
        nil
      end

      # Enables a tool (shows it on the project dock).
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param tool_id [Integer, String] tool ID
      # @return [void]
      def enable(project_id:, tool_id:)
        http_post(bucket_path(project_id, "/dock/tools/#{tool_id}/position.json"))
        nil
      end

      # Disables a tool (hides it from the project dock).
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param tool_id [Integer, String] tool ID
      # @return [void]
      def disable(project_id:, tool_id:)
        http_delete(bucket_path(project_id, "/dock/tools/#{tool_id}/position.json"))
        nil
      end

      # Changes the position of a tool on the project dock.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param tool_id [Integer, String] tool ID
      # @param position [Integer] new position (1-based, 1 = first on dock)
      # @return [void]
      def reposition(project_id:, tool_id:, position:)
        http_put(bucket_path(project_id, "/dock/tools/#{tool_id}/position.json"),
                 body: { position: position })
        nil
      end
    end
  end
end
