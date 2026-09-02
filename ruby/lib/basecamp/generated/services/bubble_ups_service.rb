# frozen_string_literal: true

module Basecamp
  module Services
    # Service for BubbleUps operations
    #
    # @generated from OpenAPI spec
    class BubbleUpsService < BaseService

      # Bubble up a recording for the current user, resurfacing it in the current
      # @param recording_id [Integer] recording id ID
      # @param at [String, nil] Timing for the bubble-up. `"now"` bubbles up immediately; a scheduling
      #   keyword (`"today"`, `"tomorrow"`, `"weekend"`, `"next_week"`) or an ISO8601
      #   date (e.g. `"2026-09-10"`) schedules it to resurface later. bc3 requires a
      #   value — omitting `at` errors server-side (`Date.iso8601(nil)`) — so send
      #   `"now"` for the immediate case.
      # @return [void]
      def create_bubble_up(recording_id:, at: nil)
        with_operation(service: "bubbleups", operation: "create_bubble_up", is_mutation: true, resource_id: recording_id) do
          http_post("/recordings/#{recording_id}/bubble_up.json", body: compact_params(at: at))
          nil
        end
      end

      # Remove the current user's bubble-up from a recording (returns 204 No Content).
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def delete_bubble_up(recording_id:)
        with_operation(service: "bubbleups", operation: "delete_bubble_up", is_mutation: true, resource_id: recording_id) do
          http_delete("/recordings/#{recording_id}/bubble_up.json")
          nil
        end
      end
    end
  end
end
