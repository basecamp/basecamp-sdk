# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Uploads operations
    #
    # @generated from OpenAPI spec
    class UploadsService < BaseService

      # Get a single upload by id
      # @param upload_id [Integer] upload id ID
      # @return [Hash] response data
      def get(upload_id:)
        with_operation(service: "uploads", operation: "get", is_mutation: false, resource_id: upload_id) do
          http_get("/uploads/#{upload_id}", operation: "GetUpload").json
        end
      end

      # Update an existing upload
      # @param upload_id [Integer] upload id ID
      # @param description [String, nil] description
      # @param base_name [String, nil] base name
      # @return [Hash] response data
      def update(upload_id:, description: nil, base_name: nil)
        with_operation(service: "uploads", operation: "update", is_mutation: true, resource_id: upload_id) do
          http_put("/uploads/#{upload_id}", body: compact_params(description: description, base_name: base_name)).json
        end
      end

      # List versions of an upload
      # @param upload_id [Integer] upload id ID
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_versions(upload_id:, max_items: nil)
        wrap_paginated(service: "uploads", operation: "list_versions", is_mutation: false, resource_id: upload_id) do
          paginate("/uploads/#{upload_id}/versions.json", operation: "ListUploadVersions", max_items: max_items)
        end
      end

      # List uploads in a vault
      # @param vault_id [Integer] vault id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(vault_id:, page: nil, max_items: nil)
        wrap_paginated(service: "uploads", operation: "list", is_mutation: false, resource_id: vault_id) do
          params = compact_query_params(page: page)
          paginate("/vaults/#{vault_id}/uploads.json", params: params, operation: "ListUploads", max_items: max_items)
        end
      end

      # Create a new upload in a vault
      # @param vault_id [Integer] vault id ID
      # @param attachable_sgid [String] attachable sgid
      # @param description [String, nil] description
      # @param base_name [String, nil] base name
      # @param subscriptions [Array, nil] subscriptions
      # @param visible_to_clients [Boolean, nil] visible to clients
      # @return [Hash] response data
      def create(vault_id:, attachable_sgid:, description: nil, base_name: nil, subscriptions: nil, visible_to_clients: nil)
        with_operation(service: "uploads", operation: "create", is_mutation: true, resource_id: vault_id) do
          http_post("/vaults/#{vault_id}/uploads.json", body: compact_params(attachable_sgid: attachable_sgid, description: description, base_name: base_name, subscriptions: subscriptions, visible_to_clients: visible_to_clients)).json
        end
      end

      # Download an upload's file content in one call.
      # Fetches upload metadata, then delegates to the AccountClient download
      # primitive so the auth'd-hop + 302-follow flow lives in one place.
      # @param upload_id [Integer] upload id ID
      # @return [Basecamp::DownloadResult]
      def download(upload_id:)
        with_operation(service: "uploads", operation: "download", is_mutation: false, resource_id: upload_id) do
          upload = get(upload_id: upload_id)
          url = upload["download_url"]
          raise UsageError.new("upload #{upload_id} has no download_url") if url.nil? || url.empty?

          result = @client.download_url(url)
          filename = upload["filename"]
          filename.to_s.empty? ? result : result.with(filename: filename)
        end
      end
    end
  end
end
