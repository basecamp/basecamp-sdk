# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Documents operations
    #
    # @generated from OpenAPI spec
    class DocumentsService < BaseService

      # Get a single document by id
      def get(project_id:, document_id:)
        http_get(bucket_path(project_id, "/documents/#{document_id}")).json
      end

      # Update an existing document
      def update(project_id:, document_id:, **body)
        http_put(bucket_path(project_id, "/documents/#{document_id}"), body: body).json
      end

      # List documents in a vault
      def list(project_id:, vault_id:)
        paginate(bucket_path(project_id, "/vaults/#{vault_id}/documents.json"))
      end

      # Create a new document in a vault
      def create(project_id:, vault_id:, **body)
        http_post(bucket_path(project_id, "/vaults/#{vault_id}/documents.json"), body: body).json
      end
    end
  end
end
