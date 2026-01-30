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
        http_get(bucket_path(project_id, "/vaults/#{vault_id}")).json
      end

      # Update an existing vault
      # @param project_id [Integer] project id ID
      # @param vault_id [Integer] vault id ID
      # @param title [String, nil] title
      # @return [Hash] response data
      def update(project_id:, vault_id:, title: nil)
        http_put(bucket_path(project_id, "/vaults/#{vault_id}"), body: compact_params(title: title)).json
      end

      # List vaults (subfolders) in a vault
      # @param project_id [Integer] project id ID
      # @param vault_id [Integer] vault id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, vault_id:)
        paginate(bucket_path(project_id, "/vaults/#{vault_id}/vaults.json"))
      end

      # Create a new vault (subfolder) in a vault
      # @param project_id [Integer] project id ID
      # @param vault_id [Integer] vault id ID
      # @param title [String] title
      # @return [Hash] response data
      def create(project_id:, vault_id:, title:)
        http_post(bucket_path(project_id, "/vaults/#{vault_id}/vaults.json"), body: compact_params(title: title)).json
      end
    end
  end
end
