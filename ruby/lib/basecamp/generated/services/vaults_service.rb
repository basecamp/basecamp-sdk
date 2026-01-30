# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Vaults operations
    #
    # @generated from OpenAPI spec
    class VaultsService < BaseService

      # Get a single vault by id
      def get(project_id:, vault_id:)
        http_get(bucket_path(project_id, "/vaults/#{vault_id}")).json
      end

      # Update an existing vault
      def update(project_id:, vault_id:, **body)
        http_put(bucket_path(project_id, "/vaults/#{vault_id}"), body: body).json
      end

      # List vaults (subfolders) in a vault
      def list(project_id:, vault_id:)
        paginate(bucket_path(project_id, "/vaults/#{vault_id}/vaults.json"))
      end

      # Create a new vault (subfolder) in a vault
      def create(project_id:, vault_id:, **body)
        http_post(bucket_path(project_id, "/vaults/#{vault_id}/vaults.json"), body: body).json
      end
    end
  end
end
