# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Documents operations
    #
    # @generated from OpenAPI spec
    class DocumentsService < BaseService

      # Get a single document by id
      # @param document_id [Integer] document id ID
      # @return [Hash] response data
      def get(document_id:)
        with_operation(service: "documents", operation: "get", is_mutation: false, project_id: project_id, resource_id: document_id) do
          http_get("/documents/#{document_id}").json
        end
      end

      # Update an existing document
      # @param document_id [Integer] document id ID
      # @param title [String, nil] title
      # @param content [String, nil] content
      # @return [Hash] response data
      def update(document_id:, title: nil, content: nil)
        with_operation(service: "documents", operation: "update", is_mutation: true, project_id: project_id, resource_id: document_id) do
          http_put("/documents/#{document_id}", body: compact_params(title: title, content: content)).json
        end
      end

      # List documents in a vault
      # @param vault_id [Integer] vault id ID
      # @return [Enumerator<Hash>] paginated results
      def list(vault_id:)
        wrap_paginated(service: "documents", operation: "list", is_mutation: false, project_id: project_id, resource_id: vault_id) do
          paginate("/vaults/#{vault_id}/documents.json")
        end
      end

      # Create a new document in a vault
      # @param vault_id [Integer] vault id ID
      # @param title [String] title
      # @param content [String, nil] content
      # @param status [String, nil] active|drafted
      # @return [Hash] response data
      def create(vault_id:, title:, content: nil, status: nil)
        with_operation(service: "documents", operation: "create", is_mutation: true, project_id: project_id, resource_id: vault_id) do
          http_post("/vaults/#{vault_id}/documents.json", body: compact_params(title: title, content: content, status: status)).json
        end
      end
    end
  end
end
