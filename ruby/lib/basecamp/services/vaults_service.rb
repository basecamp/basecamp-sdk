# frozen_string_literal: true

module Basecamp
  module Services
    # Service for vault (folder) operations.
    #
    # Vaults are folders in the Files & Documents tool. They can contain
    # documents, uploads (files), and nested vaults (subfolders).
    #
    # @example Get a vault
    #   vault = account.vaults.get(project_id: 123, vault_id: 456)
    #
    # @example List subfolders
    #   account.vaults.list(project_id: 123, vault_id: 456).each do |folder|
    #     puts folder["title"]
    #   end
    #
    # @example Create a subfolder
    #   vault = account.vaults.create(
    #     project_id: 123,
    #     vault_id: 456,
    #     title: "2024 Reports"
    #   )
    class VaultsService < BaseService
      # Gets a specific vault.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param vault_id [Integer, String] vault ID
      # @return [Hash] vault data
      def get(project_id:, vault_id:)
        http_get(bucket_path(project_id, "/vaults/#{vault_id}")).json
      end

      # Lists all child vaults (subfolders) in a vault.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param vault_id [Integer, String] parent vault ID
      # @return [Enumerator<Hash>] child vaults
      def list(project_id:, vault_id:)
        paginate(bucket_path(project_id, "/vaults/#{vault_id}/vaults.json"))
      end

      # Creates a new child vault (subfolder).
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param vault_id [Integer, String] parent vault ID
      # @param title [String] vault name
      # @return [Hash] created vault
      def create(project_id:, vault_id:, title:)
        body = { title: title }
        http_post(bucket_path(project_id, "/vaults/#{vault_id}/vaults.json"), body: body).json
      end

      # Updates an existing vault.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param vault_id [Integer, String] vault ID
      # @param title [String, nil] new title
      # @return [Hash] updated vault
      def update(project_id:, vault_id:, title: nil)
        body = compact_params(title: title)
        http_put(bucket_path(project_id, "/vaults/#{vault_id}"), body: body).json
      end

      # Lists all documents in a vault.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param vault_id [Integer, String] vault ID
      # @return [Enumerator<Hash>] documents
      def list_documents(project_id:, vault_id:)
        paginate(bucket_path(project_id, "/vaults/#{vault_id}/documents.json"))
      end

      # Lists all uploads (files) in a vault.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param vault_id [Integer, String] vault ID
      # @return [Enumerator<Hash>] uploads
      def list_uploads(project_id:, vault_id:)
        paginate(bucket_path(project_id, "/vaults/#{vault_id}/uploads.json"))
      end
    end
  end
end
