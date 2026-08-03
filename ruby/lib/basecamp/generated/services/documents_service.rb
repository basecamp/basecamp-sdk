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
        with_operation(service: "documents", operation: "get", is_mutation: false, resource_id: document_id) do
          http_get("/documents/#{document_id}", operation: "GetDocument").json
        end
      end

      # Replace a document with a new complete representation.
      # @param document_id [Integer] document id ID
      # @param title [String, nil] title
      # @param content [String, nil] content
      # @return [Hash] response data
      def replace(document_id:, title: nil, content: nil)
        with_operation(service: "documents", operation: "replace", is_mutation: true, resource_id: document_id) do
          http_put("/documents/#{document_id}", body: compact_params(title: title, content: content)).json
        end
      end

      # List documents in a vault
      # @param vault_id [Integer] vault id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(vault_id:, page: nil, max_items: nil)
        wrap_paginated(service: "documents", operation: "list", is_mutation: false, resource_id: vault_id) do
          params = compact_query_params(page: page)
          paginate("/vaults/#{vault_id}/documents.json", params: params, operation: "ListDocuments", max_items: max_items)
        end
      end

      # Create a new document in a vault
      # @param vault_id [Integer] vault id ID
      # @param title [String] title
      # @param content [String, nil] content
      # @param status [String, nil] active|drafted
      # @param subscriptions [Array, nil] subscriptions
      # @param visible_to_clients [Boolean, nil] visible to clients
      # @return [Hash] response data
      def create(vault_id:, title:, content: nil, status: nil, subscriptions: nil, visible_to_clients: nil)
        with_operation(service: "documents", operation: "create", is_mutation: true, resource_id: vault_id) do
          http_post("/vaults/#{vault_id}/documents.json", body: compact_params(title: title, content: content, status: status, subscriptions: subscriptions, visible_to_clients: visible_to_clients)).json
        end
      end
    end
  end
end
