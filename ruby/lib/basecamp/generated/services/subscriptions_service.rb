# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Subscriptions operations
    #
    # @generated from OpenAPI spec
    class SubscriptionsService < BaseService

      # Get subscription information for a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @return [Hash] response data
      def get(project_id:, recording_id:)
        http_get(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json")).json
      end

      # Subscribe the current user to a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @return [Hash] response data
      def subscribe(project_id:, recording_id:)
        http_post(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json")).json
      end

      # Update subscriptions by adding or removing specific users
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @param subscriptions [Array, nil] subscriptions
      # @param unsubscriptions [Array, nil] unsubscriptions
      # @return [Hash] response data
      def update(project_id:, recording_id:, subscriptions: nil, unsubscriptions: nil)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json"), body: compact_params(subscriptions: subscriptions, unsubscriptions: unsubscriptions)).json
      end

      # Unsubscribe the current user from a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def unsubscribe(project_id:, recording_id:)
        http_delete(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json"))
        nil
      end
    end
  end
end
