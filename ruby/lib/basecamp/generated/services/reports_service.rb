# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Reports operations
    #
    # @generated from OpenAPI spec
    class ReportsService < BaseService

      # Get account-wide activity feed (progress report)
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def progress(page: nil, max_items: nil)
        wrap_paginated(service: "reports", operation: "progress", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/reports/progress.json", params: params, operation: "GetProgressReport", max_items: max_items)
        end
      end

      # Get upcoming schedule entries and assignable items within a date window.
      # @param window_starts_on [String, nil] window starts on
      # @param window_ends_on [String, nil] window ends on
      # @return [Hash] response data
      def upcoming(window_starts_on: nil, window_ends_on: nil)
        with_operation(service: "reports", operation: "upcoming", is_mutation: false) do
          http_get("/reports/schedules/upcoming.json", params: compact_query_params(window_starts_on: window_starts_on, window_ends_on: window_ends_on), operation: "GetUpcomingSchedule").json
        end
      end

      # Get todos assigned to a specific person
      # @param person_id [Integer] person id ID
      # @param group_by [String, nil] Group by "bucket" or "date"
      # @return [Hash] response data
      def assigned(person_id:, group_by: nil)
        with_operation(service: "reports", operation: "assigned", is_mutation: false, resource_id: person_id) do
          http_get("/reports/todos/assigned/#{person_id}", params: compact_query_params(group_by: group_by), operation: "GetAssignedTodos").json
        end
      end

      # Get overdue todos grouped by lateness
      # @return [Hash] response data
      def overdue()
        with_operation(service: "reports", operation: "overdue", is_mutation: false) do
          http_get("/reports/todos/overdue.json", operation: "GetOverdueTodos").json
        end
      end

      # Get a person's activity timeline
      # @param person_id [Integer] person id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [Hash] wrapper fields merged with a ListEnumerator of the paginated items
      def person_progress(person_id:, page: nil, max_items: nil)
        wrap_paginated_wrapped(key: "events", service: "reports", operation: "person_progress", is_mutation: false, resource_id: person_id) do
          params = compact_query_params(page: page)
          paginate_wrapped("/reports/users/progress/#{person_id}.json", key: "events", params: params, operation: "GetPersonProgress", max_items: max_items)
        end
      end
    end
  end
end
