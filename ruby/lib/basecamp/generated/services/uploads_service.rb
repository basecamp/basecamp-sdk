# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Uploads operations
    #
    # @generated from OpenAPI spec
    class UploadsService < BaseService

      # Get a single upload by id
      # @param upload_id [Integer] upload id ID
      # @return [Hash] response data
      def get(upload_id:)
        with_operation(service: "uploads", operation: "get", is_mutation: false, project_id: project_id, resource_id: upload_id) do
          http_get("/uploads/#{upload_id}").json
        end
      end

      # Update an existing upload
      # @param upload_id [Integer] upload id ID
      # @param description [String, nil] description
      # @param base_name [String, nil] base name
      # @return [Hash] response data
      def update(upload_id:, description: nil, base_name: nil)
        with_operation(service: "uploads", operation: "update", is_mutation: true, project_id: project_id, resource_id: upload_id) do
          http_put("/uploads/#{upload_id}", body: compact_params(description: description, base_name: base_name)).json
        end
      end

      # List versions of an upload
      # @param upload_id [Integer] upload id ID
      # @return [Enumerator<Hash>] paginated results
      def list_versions(upload_id:)
        wrap_paginated(service: "uploads", operation: "list_versions", is_mutation: false, project_id: project_id, resource_id: upload_id) do
          paginate("/uploads/#{upload_id}/versions.json")
        end
      end

      # List uploads in a vault
      # @param vault_id [Integer] vault id ID
      # @return [Enumerator<Hash>] paginated results
      def list(vault_id:)
        wrap_paginated(service: "uploads", operation: "list", is_mutation: false, project_id: project_id, resource_id: vault_id) do
          paginate("/vaults/#{vault_id}/uploads.json")
        end
      end

      # Create a new upload in a vault
      # @param vault_id [Integer] vault id ID
      # @param attachable_sgid [String] attachable sgid
      # @param description [String, nil] description
      # @param base_name [String, nil] base name
      # @return [Hash] response data
      def create(vault_id:, attachable_sgid:, description: nil, base_name: nil)
        with_operation(service: "uploads", operation: "create", is_mutation: true, project_id: project_id, resource_id: vault_id) do
          http_post("/vaults/#{vault_id}/uploads.json", body: compact_params(attachable_sgid: attachable_sgid, description: description, base_name: base_name)).json
        end
      end
    end
  end
end
