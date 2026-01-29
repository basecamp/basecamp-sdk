# frozen_string_literal: true

module Basecamp
  module Services
    # Service for document operations.
    #
    # Documents are rich text files stored within vaults. They support
    # HTML content and can be in draft or active status.
    #
    # @example List documents in a vault
    #   account.documents.list(project_id: 123, vault_id: 456).each do |doc|
    #     puts "#{doc["title"]} - #{doc["comments_count"]} comments"
    #   end
    #
    # @example Create a document
    #   doc = account.documents.create(
    #     project_id: 123,
    #     vault_id: 456,
    #     title: "Meeting Notes",
    #     content: "<p>Notes from today's meeting...</p>"
    #   )
    class DocumentsService < BaseService
      # Lists all documents in a vault.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param vault_id [Integer, String] vault ID
      # @return [Enumerator<Hash>] documents
      def list(project_id:, vault_id:)
        paginate(bucket_path(project_id, "/vaults/#{vault_id}/documents.json"))
      end

      # Gets a specific document.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param document_id [Integer, String] document ID
      # @return [Hash] document data
      def get(project_id:, document_id:)
        http_get(bucket_path(project_id, "/documents/#{document_id}.json")).json
      end

      # Creates a new document in a vault.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param vault_id [Integer, String] vault ID
      # @param title [String] document title
      # @param content [String, nil] document body in HTML
      # @param status [String, nil] status ("drafted" or "active")
      # @return [Hash] created document
      def create(project_id:, vault_id:, title:, content: nil, status: nil)
        body = compact_params(
          title: title,
          content: content,
          status: status
        )
        http_post(bucket_path(project_id, "/vaults/#{vault_id}/documents.json"), body: body).json
      end

      # Updates an existing document.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param document_id [Integer, String] document ID
      # @param title [String, nil] new title
      # @param content [String, nil] new content
      # @return [Hash] updated document
      def update(project_id:, document_id:, title: nil, content: nil)
        body = compact_params(
          title: title,
          content: content
        )
        http_put(bucket_path(project_id, "/documents/#{document_id}.json"), body: body).json
      end
    end
  end
end
