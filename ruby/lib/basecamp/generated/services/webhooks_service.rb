# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Webhooks operations
    #
    # @generated from OpenAPI spec
    class WebhooksService < BaseService

      # List all webhooks for a project
      # @param bucket_id [Integer] bucket id ID
      # @return [Enumerator<Hash>] paginated results
      def list(bucket_id:)
        wrap_paginated(service: "webhooks", operation: "list", is_mutation: false, resource_id: bucket_id) do
          paginate("/buckets/#{bucket_id}/webhooks.json")
        end
      end

      # Create a new webhook for a project
      # @param bucket_id [Integer] bucket id ID
      # @param payload_url [String] payload url
      # @param types [Array] types
      # @param active [Boolean, nil] active
      # @return [Hash] response data
      def create(bucket_id:, payload_url:, types:, active: nil)
        with_operation(service: "webhooks", operation: "create", is_mutation: true, resource_id: bucket_id) do
          http_post("/buckets/#{bucket_id}/webhooks.json", body: compact_params(payload_url: payload_url, types: types, active: active)).json
        end
      end

      # Get a single webhook by id
      # @param webhook_id [Integer] webhook id ID
      # @return [Hash] response data
      def get(webhook_id:)
        with_operation(service: "webhooks", operation: "get", is_mutation: false, resource_id: webhook_id) do
          http_get("/webhooks/#{webhook_id}").json
        end
      end

      # Update an existing webhook
      # @param webhook_id [Integer] webhook id ID
      # @param payload_url [String, nil] payload url
      # @param types [Array, nil] types
      # @param active [Boolean, nil] active
      # @return [Hash] response data
      def update(webhook_id:, payload_url: nil, types: nil, active: nil)
        with_operation(service: "webhooks", operation: "update", is_mutation: true, resource_id: webhook_id) do
          http_put("/webhooks/#{webhook_id}", body: compact_params(payload_url: payload_url, types: types, active: active)).json
        end
      end

      # Delete a webhook
      # @param webhook_id [Integer] webhook id ID
      # @return [void]
      def delete(webhook_id:)
        with_operation(service: "webhooks", operation: "delete", is_mutation: true, resource_id: webhook_id) do
          http_delete("/webhooks/#{webhook_id}")
          nil
        end
      end
    end
  end
end
