# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Webhooks operations
    #
    # @generated from OpenAPI spec
    class WebhooksService < BaseService

      # List all webhooks for a project
      # @param project_id [Integer] project id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:)
        wrap_paginated(service: "webhooks", operation: "list", is_mutation: false, project_id: project_id) do
          paginate(bucket_path(project_id, "/webhooks.json"))
        end
      end

      # Create a new webhook for a project
      # @param project_id [Integer] project id ID
      # @param payload_url [String] payload url
      # @param types [Array] types
      # @param active [Boolean, nil] active
      # @return [Hash] response data
      def create(project_id:, payload_url:, types:, active: nil)
        with_operation(service: "webhooks", operation: "create", is_mutation: true, project_id: project_id) do
          http_post(bucket_path(project_id, "/webhooks.json"), body: compact_params(payload_url: payload_url, types: types, active: active)).json
        end
      end

      # Get a single webhook by id
      # @param project_id [Integer] project id ID
      # @param webhook_id [Integer] webhook id ID
      # @return [Hash] response data
      def get(project_id:, webhook_id:)
        with_operation(service: "webhooks", operation: "get", is_mutation: false, project_id: project_id, resource_id: webhook_id) do
          http_get(bucket_path(project_id, "/webhooks/#{webhook_id}")).json
        end
      end

      # Update an existing webhook
      # @param project_id [Integer] project id ID
      # @param webhook_id [Integer] webhook id ID
      # @param payload_url [String, nil] payload url
      # @param types [Array, nil] types
      # @param active [Boolean, nil] active
      # @return [Hash] response data
      def update(project_id:, webhook_id:, payload_url: nil, types: nil, active: nil)
        with_operation(service: "webhooks", operation: "update", is_mutation: true, project_id: project_id, resource_id: webhook_id) do
          http_put(bucket_path(project_id, "/webhooks/#{webhook_id}"), body: compact_params(payload_url: payload_url, types: types, active: active)).json
        end
      end

      # Delete a webhook
      # @param project_id [Integer] project id ID
      # @param webhook_id [Integer] webhook id ID
      # @return [void]
      def delete(project_id:, webhook_id:)
        with_operation(service: "webhooks", operation: "delete", is_mutation: true, project_id: project_id, resource_id: webhook_id) do
          http_delete(bucket_path(project_id, "/webhooks/#{webhook_id}"))
          nil
        end
      end
    end
  end
end
