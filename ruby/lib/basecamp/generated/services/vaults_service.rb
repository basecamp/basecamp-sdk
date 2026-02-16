# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Vaults operations
    #
    # @generated from OpenAPI spec
    class VaultsService < BaseService

      # Get a single vault by id
      # @param project_id [Integer] project id ID
      # @param vault_id [Integer] vault id ID
      # @return [Hash] response data
      def get(project_id:, vault_id:)
        with_operation(service: "vaults", operation: "get", is_mutation: false, project_id: project_id, resource_id: vault_id) do
          http_get(bucket_path(project_id, "/vaults/#{vault_id}")).json
        end
      end

      # Update an existing vault
      # @param project_id [Integer] project id ID
      # @param vault_id [Integer] vault id ID
      # @param title [String, nil] title
      # @return [Hash] response data
      def update(project_id:, vault_id:, title: nil)
        with_operation(service: "vaults", operation: "update", is_mutation: true, project_id: project_id, resource_id: vault_id) do
          http_put(bucket_path(project_id, "/vaults/#{vault_id}"), body: compact_params(title: title)).json
        end
      end

      # List vaults (subfolders) in a vault
      # @param project_id [Integer] project id ID
      # @param vault_id [Integer] vault id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, vault_id:)
        wrap_paginated(service: "vaults", operation: "list", is_mutation: false, project_id: project_id, resource_id: vault_id) do
          paginate(bucket_path(project_id, "/vaults/#{vault_id}/vaults.json"))
        end
      end

      # Create a new vault (subfolder) in a vault
      # @param project_id [Integer] project id ID
      # @param vault_id [Integer] vault id ID
      # @param title [String] title
      # @return [Hash] response data
      def create(project_id:, vault_id:, title:)
        with_operation(service: "vaults", operation: "create", is_mutation: true, project_id: project_id, resource_id: vault_id) do
          http_post(bucket_path(project_id, "/vaults/#{vault_id}/vaults.json"), body: compact_params(title: title)).json
        end
      end
    end
  end
end
