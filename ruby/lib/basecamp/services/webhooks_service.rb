# frozen_string_literal: true

module Basecamp
  module Services
    # Service for webhook operations.
    #
    # Webhooks allow external services to receive notifications when events
    # occur in a Basecamp project.
    #
    # @example List webhooks
    #   account.webhooks.list(project_id: 123).each do |webhook|
    #     puts "#{webhook["payload_url"]} - #{webhook["active"]}"
    #   end
    #
    # @example Create a webhook
    #   webhook = account.webhooks.create(
    #     project_id: 123,
    #     payload_url: "https://example.com/webhooks/basecamp",
    #     types: ["Todo", "Message"]
    #   )
    class WebhooksService < BaseService
      # Lists all webhooks in a project.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @return [Enumerator<Hash>] webhooks
      def list(project_id:)
        paginate(bucket_path(project_id, "/webhooks.json"))
      end

      # Gets a specific webhook.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param webhook_id [Integer, String] webhook ID
      # @return [Hash] webhook data
      def get(project_id:, webhook_id:)
        http_get(bucket_path(project_id, "/webhooks/#{webhook_id}")).json
      end

      # Creates a new webhook.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param payload_url [String] URL to receive webhook payloads
      # @param types [Array<String>, nil] recording types to subscribe to
      # @return [Hash] created webhook
      def create(project_id:, payload_url:, types: nil)
        body = compact_params(
          payload_url: payload_url,
          types: types
        )
        http_post(bucket_path(project_id, "/webhooks.json"), body: body).json
      end

      # Updates an existing webhook.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param webhook_id [Integer, String] webhook ID
      # @param payload_url [String, nil] new URL
      # @param types [Array<String>, nil] new recording types
      # @param active [Boolean, nil] whether the webhook is active
      # @return [Hash] updated webhook
      def update(project_id:, webhook_id:, payload_url: nil, types: nil, active: nil)
        body = compact_params(
          payload_url: payload_url,
          types: types,
          active: active
        )
        http_put(bucket_path(project_id, "/webhooks/#{webhook_id}"), body: body).json
      end

      # Deletes a webhook.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param webhook_id [Integer, String] webhook ID
      # @return [void]
      def delete(project_id:, webhook_id:)
        http_delete(bucket_path(project_id, "/webhooks/#{webhook_id}"))
        nil
      end
    end
  end
end
