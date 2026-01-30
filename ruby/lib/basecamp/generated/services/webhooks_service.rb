# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Webhooks operations
    #
    # @generated from OpenAPI spec
    class WebhooksService < BaseService

      # List all webhooks for a project
      def list(project_id:)
        paginate(bucket_path(project_id, "/webhooks.json"))
      end

      # Create a new webhook for a project
      def create(project_id:, **body)
        http_post(bucket_path(project_id, "/webhooks.json"), body: body).json
      end

      # Get a single webhook by id
      def get(project_id:, webhook_id:)
        http_get(bucket_path(project_id, "/webhooks/#{webhook_id}")).json
      end

      # Update an existing webhook
      def update(project_id:, webhook_id:, **body)
        http_put(bucket_path(project_id, "/webhooks/#{webhook_id}"), body: body).json
      end

      # Delete a webhook
      def delete(project_id:, webhook_id:)
        http_delete(bucket_path(project_id, "/webhooks/#{webhook_id}"))
        nil
      end
    end
  end
end
