# frozen_string_literal: true

module Basecamp
  module Services
    # Service for MyNotifications operations
    #
    # @generated from OpenAPI spec
    class MyNotificationsService < BaseService

      # Get the current user's notification inbox (the "Hey!" menu).
      # @param page [Integer, nil] Page number for paginating through read items. Defaults to 1.
      # @param limit_bubble_ups [Boolean, nil] Set to true to cap `bubble_ups` at 2 current bubble-ups and omit the
      #   `scheduled_bubble_ups` key entirely. Defaults to false. Use the dedicated
      #   bubble-ups endpoint (GetBubbleUps) to page through all current and
      #   scheduled bubble-ups.
      # @return [Hash] response data
      def get_my_notifications(page: nil, limit_bubble_ups: nil)
        with_operation(service: "mynotifications", operation: "get_my_notifications", is_mutation: false) do
          http_get("/my/readings.json", params: compact_query_params(page: page, limit_bubble_ups: limit_bubble_ups), operation: "GetMyNotifications").json
        end
      end

      # Get the current user's current and scheduled bubble-ups (paginated, 50 per page).
      # @param page [Integer, nil] Page number. Defaults to 1.
      # @return [Enumerator<Hash>] paginated results
      def get_bubble_ups(page: nil)
        wrap_paginated(service: "mynotifications", operation: "get_bubble_ups", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/my/readings/bubble_ups.json", params: params, operation: "GetBubbleUps")
        end
      end

      # Mark specified items as read
      # @param readables [Array] Array of readable_sgid values identifying the items to mark as read
      # @return [void]
      def mark_as_read(readables:)
        with_operation(service: "mynotifications", operation: "mark_as_read", is_mutation: true) do
          http_put("/my/unreads.json", body: compact_params(readables: readables))
          nil
        end
      end
    end
  end
end
