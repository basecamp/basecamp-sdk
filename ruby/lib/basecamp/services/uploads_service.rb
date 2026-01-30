# frozen_string_literal: true

module Basecamp
  module Services
    # Service for upload operations.
    #
    # Uploads are files stored within vaults. They are created from
    # attachments (via attachable_sgid) and can have descriptions
    # and version history.
    #
    # @example List uploads in a vault
    #   account.uploads.list(project_id: 123, vault_id: 456).each do |upload|
    #     puts "#{upload["filename"]} - #{upload["byte_size"]} bytes"
    #   end
    #
    # @example Create an upload
    #   upload = account.uploads.create(
    #     project_id: 123,
    #     vault_id: 456,
    #     attachable_sgid: attachment_sgid,
    #     description: "Q4 financial report"
    #   )
    class UploadsService < BaseService
      # Lists all uploads in a vault.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param vault_id [Integer, String] vault ID
      # @return [Enumerator<Hash>] uploads
      def list(project_id:, vault_id:)
        paginate(bucket_path(project_id, "/vaults/#{vault_id}/uploads.json"))
      end

      # Gets a specific upload.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param upload_id [Integer, String] upload ID
      # @return [Hash] upload data
      def get(project_id:, upload_id:)
        http_get(bucket_path(project_id, "/uploads/#{upload_id}")).json
      end

      # Creates a new upload in a vault.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param vault_id [Integer, String] vault ID
      # @param attachable_sgid [String] signed global ID from attachment upload
      # @param description [String, nil] upload description in HTML
      # @param base_name [String, nil] filename without extension
      # @return [Hash] created upload
      def create(project_id:, vault_id:, attachable_sgid:, description: nil, base_name: nil)
        body = compact_params(
          attachable_sgid: attachable_sgid,
          description: description,
          base_name: base_name
        )
        http_post(bucket_path(project_id, "/vaults/#{vault_id}/uploads.json"), body: body).json
      end

      # Updates an existing upload.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param upload_id [Integer, String] upload ID
      # @param description [String, nil] new description
      # @param base_name [String, nil] new filename without extension
      # @return [Hash] updated upload
      def update(project_id:, upload_id:, description: nil, base_name: nil)
        body = compact_params(
          description: description,
          base_name: base_name
        )
        http_put(bucket_path(project_id, "/uploads/#{upload_id}"), body: body).json
      end

      # Lists all versions of an upload.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param upload_id [Integer, String] upload ID
      # @return [Enumerator<Hash>] upload versions
      def list_versions(project_id:, upload_id:)
        paginate(bucket_path(project_id, "/uploads/#{upload_id}/versions.json"))
      end
    end
  end
end
