# frozen_string_literal: true

module Basecamp
  module Services
    # Service for GoogleDocuments operations
    #
    # @generated from OpenAPI spec
    class GoogleDocumentsService < BaseService

      # Create a new Google document in a vault.
      # @param bucket_id [Integer] bucket id ID
      # @param vault_id [Integer] vault id ID
      # @param url [String] url
      # @param document_type [String] One of "doc", "sheet", "slide", "other". Backed by a Rails enum, so an
      #   unrecognized value is rejected up front with a field-keyed 422
      #   ({"errors": {"document_type": ["is not a valid document type"]}}) rather
      #   than reaching validation.
      # @param title [String, nil] title
      # @param description [String, nil] description
      # @param status [String, nil] active|drafted — defaults to drafted
      # @param subscriptions [Array, nil] subscriptions
      # @param visible_to_clients [Boolean, nil] Whether the document is visible to the project's clients. Applies only
      #   when creating directly in the tool's vault — an item created inside a
      #   folder inherits the folder's visibility and ignores this. A client caller
      #   always creates client-visible records regardless of what is sent.
      # @return [Hash] response data
      def create_google_document(bucket_id:, vault_id:, url:, document_type:, title: nil, description: nil, status: nil, subscriptions: nil, visible_to_clients: nil)
        with_operation(service: "googledocuments", operation: "create_google_document", is_mutation: true, project_id: bucket_id, resource_id: vault_id) do
          http_post("/buckets/#{bucket_id}/vaults/#{vault_id}/google_documents.json", body: compact_params(url: url, document_type: document_type, title: title, description: description, status: status, subscriptions: subscriptions, visible_to_clients: visible_to_clients)).json
        end
      end

      # Get a single Google document by id
      # @param google_document_id [Integer] google document id ID
      # @return [Hash] response data
      def get_google_document(google_document_id:)
        with_operation(service: "googledocuments", operation: "get_google_document", is_mutation: false, resource_id: google_document_id) do
          http_get("/google_documents/#{google_document_id}", operation: "GetGoogleDocument").json
        end
      end

      # Replace a Google document with a new complete representation.
      # @param google_document_id [Integer] google document id ID
      # @param url [String] url
      # @param document_type [String] One of "doc", "sheet", "slide", "other". Backed by a Rails enum, so an
      #   unrecognized value is rejected up front with a field-keyed 422
      #   ({"errors": {"document_type": ["is not a valid document type"]}}) rather
      #   than reaching validation.
      # @param title [String, nil] title
      # @param description [String, nil] description
      # @param status [String, nil] active|drafted
      # @param subscriptions [Array, nil] subscriptions
      # @return [Hash] response data
      def update_google_document(google_document_id:, url:, document_type:, title: nil, description: nil, status: nil, subscriptions: nil)
        with_operation(service: "googledocuments", operation: "update_google_document", is_mutation: true, resource_id: google_document_id) do
          http_put("/google_documents/#{google_document_id}", body: compact_params(url: url, document_type: document_type, title: title, description: description, status: status, subscriptions: subscriptions)).json
        end
      end
    end
  end
end
