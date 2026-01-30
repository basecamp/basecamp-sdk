# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Uploads operations
    #
    # @generated from OpenAPI spec
    class UploadsService < BaseService

      # Get a single upload by id
      def get(project_id:, upload_id:)
        http_get(bucket_path(project_id, "/uploads/#{upload_id}")).json
      end

      # Update an existing upload
      def update(project_id:, upload_id:, **body)
        http_put(bucket_path(project_id, "/uploads/#{upload_id}"), body: body).json
      end

      # List versions of an upload
      def list_versions(project_id:, upload_id:)
        paginate(bucket_path(project_id, "/uploads/#{upload_id}/versions.json"))
      end

      # List uploads in a vault
      def list(project_id:, vault_id:)
        paginate(bucket_path(project_id, "/vaults/#{vault_id}/uploads.json"))
      end

      # Create a new upload in a vault
      def create(project_id:, vault_id:, **body)
        http_post(bucket_path(project_id, "/vaults/#{vault_id}/uploads.json"), body: body).json
      end
    end
  end
end
