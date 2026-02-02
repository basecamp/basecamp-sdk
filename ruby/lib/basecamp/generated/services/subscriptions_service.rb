# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Subscriptions operations
    #
    # @generated from OpenAPI spec
    class SubscriptionsService < BaseService

      # Get subscription information for a recording
      # @param recording_id [Integer] recording id ID
      # @return [Hash] response data
      def get(recording_id:)
        http_get("/recordings/#{recording_id}/subscription.json").json
      end

      # Subscribe the current user to a recording
      # @param recording_id [Integer] recording id ID
      # @return [Hash] response data
      def subscribe(recording_id:)
        http_post("/recordings/#{recording_id}/subscription.json").json
      end

      # Update subscriptions by adding or removing specific users
      # @param recording_id [Integer] recording id ID
      # @param subscriptions [Array, nil] subscriptions
      # @param unsubscriptions [Array, nil] unsubscriptions
      # @return [Hash] response data
      def update(recording_id:, subscriptions: nil, unsubscriptions: nil)
        http_put("/recordings/#{recording_id}/subscription.json", body: compact_params(subscriptions: subscriptions, unsubscriptions: unsubscriptions)).json
      end

      # Unsubscribe the current user from a recording
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def unsubscribe(recording_id:)
        http_delete("/recordings/#{recording_id}/subscription.json")
        nil
      end
    end
  end
end
