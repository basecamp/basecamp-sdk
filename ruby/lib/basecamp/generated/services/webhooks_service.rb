# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Webhooks operations
    #
    # @generated from OpenAPI spec
    class WebhooksService < BaseService

      # List all webhooks for a project
      # @return [Enumerator<Hash>] paginated results
      def list()
        paginate("/webhooks.json")
      end

      # Create a new webhook for a project
      # @param payload_url [String] payload url
      # @param types [Array] types
      # @param active [Boolean, nil] active
      # @return [Hash] response data
      def create(payload_url:, types:, active: nil)
        http_post("/webhooks.json", body: compact_params(payload_url: payload_url, types: types, active: active)).json
      end

      # Get a single webhook by id
      # @param webhook_id [Integer] webhook id ID
      # @return [Hash] response data
      def get(webhook_id:)
        http_get("/webhooks/#{webhook_id}").json
      end

      # Update an existing webhook
      # @param webhook_id [Integer] webhook id ID
      # @param payload_url [String, nil] payload url
      # @param types [Array, nil] types
      # @param active [Boolean, nil] active
      # @return [Hash] response data
      def update(webhook_id:, payload_url: nil, types: nil, active: nil)
        http_put("/webhooks/#{webhook_id}", body: compact_params(payload_url: payload_url, types: types, active: active)).json
      end

      # Delete a webhook
      # @param webhook_id [Integer] webhook id ID
      # @return [void]
      def delete(webhook_id:)
        http_delete("/webhooks/#{webhook_id}")
        nil
      end
    end
  end
end
