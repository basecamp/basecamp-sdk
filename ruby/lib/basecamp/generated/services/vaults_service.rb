# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Vaults operations
    #
    # @generated from OpenAPI spec
    class VaultsService < BaseService

      # Get a single vault by id
      # @param vault_id [Integer] vault id ID
      # @return [Hash] response data
      def get(vault_id:)
        with_operation(service: "vaults", operation: "get", is_mutation: false, resource_id: vault_id) do
          http_get("/vaults/#{vault_id}", operation: "GetVault").json
        end
      end

      # Update an existing vault
      # @param vault_id [Integer] vault id ID
      # @param title [String, nil] title
      # @return [Hash] response data
      def update(vault_id:, title: nil)
        with_operation(service: "vaults", operation: "update", is_mutation: true, resource_id: vault_id) do
          http_put("/vaults/#{vault_id}", body: compact_params(title: title)).json
        end
      end

      # List vaults (subfolders) in a vault
      # @param vault_id [Integer] vault id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(vault_id:, page: nil, max_items: nil)
        wrap_paginated(service: "vaults", operation: "list", is_mutation: false, resource_id: vault_id) do
          params = compact_query_params(page: page)
          paginate("/vaults/#{vault_id}/vaults.json", params: params, operation: "ListVaults", max_items: max_items)
        end
      end

      # Create a new vault (subfolder) in a vault
      # @param vault_id [Integer] vault id ID
      # @param title [String] title
      # @return [Hash] response data
      def create(vault_id:, title:)
        with_operation(service: "vaults", operation: "create", is_mutation: true, resource_id: vault_id) do
          http_post("/vaults/#{vault_id}/vaults.json", body: compact_params(title: title)).json
        end
      end
    end
  end
end
