# frozen_string_literal: true

module Basecamp
  module Services
    # Service for message type (category) operations.
    #
    # Message types (also called categories) are used to categorize messages
    # on a message board. Each message type has a name and icon.
    #
    # @example List message types
    #   account.message_types.list(project_id: 123).each do |type|
    #     puts "#{type["icon"]} #{type["name"]}"
    #   end
    #
    # @example Create a message type
    #   type = account.message_types.create(
    #     project_id: 123,
    #     name: "Announcement",
    #     icon: "📢"
    #   )
    class MessageTypesService < BaseService
      # Lists all message types in a project.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @return [Enumerator<Hash>] message types
      def list(project_id:)
        paginate(bucket_path(project_id, "/categories.json"))
      end

      # Gets a message type by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param type_id [Integer, String] message type ID
      # @return [Hash] message type data
      def get(project_id:, type_id:)
        http_get(bucket_path(project_id, "/categories/#{type_id}")).json
      end

      # Creates a new message type in a project.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param name [String] message type name
      # @param icon [String] message type icon
      # @return [Hash] created message type
      def create(project_id:, name:, icon:)
        body = {
          name: name,
          icon: icon
        }
        http_post(bucket_path(project_id, "/categories.json"), body: body).json
      end

      # Updates an existing message type.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param type_id [Integer, String] message type ID
      # @param name [String, nil] new name
      # @param icon [String, nil] new icon
      # @return [Hash] updated message type
      def update(project_id:, type_id:, name: nil, icon: nil)
        body = compact_params(
          name: name,
          icon: icon
        )
        http_put(bucket_path(project_id, "/categories/#{type_id}"), body: body).json
      end

      # Deletes a message type from a project.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param type_id [Integer, String] message type ID
      # @return [void]
      def delete(project_id:, type_id:)
        http_delete(bucket_path(project_id, "/categories/#{type_id}"))
        nil
      end
    end
  end
end
