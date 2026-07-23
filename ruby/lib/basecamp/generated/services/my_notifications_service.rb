# frozen_string_literal: true

module Basecamp
  module Services
    # Service for MyNotifications operations
    #
    # @generated from OpenAPI spec
    class MyNotificationsService < BaseService

      # Get the current user's notification inbox (the "Hey!" menu).
      # @param page [Integer, nil] Page number for paginating through read items. Defaults to 1.
      # @return [Hash] response data
      def get_my_notifications(page: nil)
        with_operation(service: "mynotifications", operation: "get_my_notifications", is_mutation: false) do
          http_get("/my/readings.json", params: compact_query_params(page: page)).json
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
