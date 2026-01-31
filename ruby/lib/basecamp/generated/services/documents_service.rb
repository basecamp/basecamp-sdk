# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Documents operations
    #
    # @generated from OpenAPI spec
    class DocumentsService < BaseService

      # Get a single document by id
      # @param project_id [Integer] project id ID
      # @param document_id [Integer] document id ID
      # @return [Hash] response data
      def get(project_id:, document_id:)
        http_get(bucket_path(project_id, "/documents/#{document_id}")).json
      end

      # Update an existing document
      # @param project_id [Integer] project id ID
      # @param document_id [Integer] document id ID
      # @param title [String, nil] title
      # @param content [String, nil] content
      # @return [Hash] response data
      def update(project_id:, document_id:, title: nil, content: nil)
        http_put(bucket_path(project_id, "/documents/#{document_id}"), body: compact_params(title: title, content: content)).json
      end

      # List documents in a vault
      # @param project_id [Integer] project id ID
      # @param vault_id [Integer] vault id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, vault_id:)
        paginate(bucket_path(project_id, "/vaults/#{vault_id}/documents.json"))
      end

      # Create a new document in a vault
      # @param project_id [Integer] project id ID
      # @param vault_id [Integer] vault id ID
      # @param title [String] title
      # @param content [String, nil] content
      # @param status [String, nil] active|drafted
      # @return [Hash] response data
      def create(project_id:, vault_id:, title:, content: nil, status: nil)
        http_post(bucket_path(project_id, "/vaults/#{vault_id}/documents.json"), body: compact_params(title: title, content: content, status: status)).json
      end
    end
  end
end
