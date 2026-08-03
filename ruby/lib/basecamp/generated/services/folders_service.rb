# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Folders operations
    #
    # @generated from OpenAPI spec
    class FoldersService < BaseService

      # List the authenticated user's folders in home-screen order.
      # @return [Array<Hash>] response data
      def list_folders()
        with_operation(service: "folders", operation: "list_folders", is_mutation: false) do
          http_get("/stacks.json", operation: "ListFolders").json
        end
      end

      # Create a folder for the authenticated user and file the given projects into it.
      # @param name [String, nil] The folder's name. Defaults to `New folder` when blank, null, or omitted.
      # @param project_ids [Array, nil] IDs of the projects to file into the folder — the same ids the folder
      #   reports back as `bucket_ids` and expands as `projects`. This does not
      #   round-trip under its own name. Omit it, or send null or an empty array,
      #   for an empty folder.
      # @return [Hash] response data
      def create_folder(name: nil, project_ids: nil)
        with_operation(service: "folders", operation: "create_folder", is_mutation: true) do
          http_post("/stacks.json", body: compact_params(name: name, project_ids: project_ids)).json
        end
      end

      # Get one folder, with the projects grouped inside it expanded under `projects`.
      # @param folder_id [Integer] folder id ID
      # @return [Hash] response data
      def get_folder(folder_id:)
        with_operation(service: "folders", operation: "get_folder", is_mutation: false, resource_id: folder_id) do
          http_get("/stacks/#{folder_id}", operation: "GetFolder").json
        end
      end

      # Rename a folder.
      # @param folder_id [Integer] folder id ID
      # @param name [String] The folder's new name. Blank is rejected with 422 — unlike create, update
      #   does not fall back to a default name.
      # @return [Hash] response data
      def update_folder(folder_id:, name:)
        with_operation(service: "folders", operation: "update_folder", is_mutation: true, resource_id: folder_id) do
          http_put("/stacks/#{folder_id}", body: compact_params(name: name)).json
        end
      end

      # Delete a folder and unpin its projects from the home screen (returns 204 No Content).
      # @param folder_id [Integer] folder id ID
      # @return [void]
      def delete_folder(folder_id:)
        with_operation(service: "folders", operation: "delete_folder", is_mutation: true, resource_id: folder_id) do
          http_delete("/stacks/#{folder_id}")
          nil
        end
      end
    end
  end
end
