# frozen_string_literal: true

module Basecamp
  module Services
    # Service for subscription operations.
    #
    # Subscriptions control who receives notifications for a specific recording
    # (like a todo, message, or comment). Users can subscribe or unsubscribe
    # themselves, and you can batch update subscriptions for multiple users.
    #
    # @example Get subscription info
    #   subscription = account.subscriptions.get(project_id: 123, recording_id: 456)
    #   puts "Subscribed: #{subscription["subscribed"]}, Count: #{subscription["count"]}"
    #
    # @example Batch update subscriptions
    #   account.subscriptions.update(
    #     project_id: 123,
    #     recording_id: 456,
    #     subscriptions: [user_id_1, user_id_2],
    #     unsubscriptions: [user_id_3]
    #   )
    class SubscriptionsService < BaseService
      # Gets the subscription information for a recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [Hash] subscription info with subscribed, count, subscribers
      def get(project_id:, recording_id:)
        http_get(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json")).json
      end

      # Subscribes the current user to the recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [Hash] updated subscription information
      def subscribe(project_id:, recording_id:)
        http_post(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json")).json
      end

      # Unsubscribes the current user from the recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @return [void]
      def unsubscribe(project_id:, recording_id:)
        http_delete(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json"))
        nil
      end

      # Batch modifies subscriptions by adding or removing specific users.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID
      # @param subscriptions [Array<Integer>, nil] person IDs to subscribe
      # @param unsubscriptions [Array<Integer>, nil] person IDs to unsubscribe
      # @return [Hash] updated subscription information
      def update(project_id:, recording_id:, subscriptions: nil, unsubscriptions: nil)
        body = compact_params(
          subscriptions: subscriptions,
          unsubscriptions: unsubscriptions
        )
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json"), body: body).json
      end
    end
  end
end
