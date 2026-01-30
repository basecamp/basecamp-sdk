# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Subscriptions operations
    #
    # @generated from OpenAPI spec
    class SubscriptionsService < BaseService

      # Get subscription information for a recording
      def get(project_id:, recording_id:)
        http_get(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json")).json
      end

      # Subscribe the current user to a recording
      def subscribe(project_id:, recording_id:)
        http_post(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json")).json
      end

      # Update subscriptions by adding or removing specific users
      def update(project_id:, recording_id:, **body)
        http_put(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json"), body: body).json
      end

      # Unsubscribe the current user from a recording
      def unsubscribe(project_id:, recording_id:)
        http_delete(bucket_path(project_id, "/recordings/#{recording_id}/subscription.json"))
        nil
      end
    end
  end
end
