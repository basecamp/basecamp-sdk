# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Calendars operations
    #
    # @generated from OpenAPI spec
    class CalendarsService < BaseService

      # Get a calendar by its bucket id. A Calendar is a top-level BC5 bucketable
      # @param calendar_id [Integer] calendar id ID
      # @return [Hash] response data
      def get_calendar(calendar_id:)
        with_operation(service: "calendars", operation: "get_calendar", is_mutation: false, resource_id: calendar_id) do
          http_get("/calendars/#{calendar_id}", operation: "GetCalendar").json
        end
      end

      # Update a calendar's display color. An unknown color returns 422 with a JSON
      # @param calendar_id [Integer] calendar id ID
      # @param calendar [Hash] calendar
      # @return [Hash] response data
      def update_calendar(calendar_id:, calendar:)
        with_operation(service: "calendars", operation: "update_calendar", is_mutation: true, resource_id: calendar_id) do
          http_put("/calendars/#{calendar_id}", body: compact_params(calendar: calendar)).json
        end
      end
    end
  end
end
