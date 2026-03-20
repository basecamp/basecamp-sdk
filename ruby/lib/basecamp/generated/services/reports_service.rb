# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Reports operations
    #
    # @generated from OpenAPI spec
    class ReportsService < BaseService

      # Get the current user's assignments grouped into priorities and non-priorities
      # @return [Hash] response data
      def my_assignments()
        with_operation(service: "reports", operation: "my_assignments", is_mutation: false) do
          http_get("/my/assignments.json").json
        end
      end

      # Get the current user's completed assignments
      # @return [Hash] response data
      def my_assignments_completed()
        with_operation(service: "reports", operation: "my_assignments_completed", is_mutation: false) do
          http_get("/my/assignments/completed.json").json
        end
      end

      # Get the current user's due assignments filtered by scope
      # @param scope [String, nil] Filter by due scope: overdue, due_today, due_tomorrow, due_later_this_week, due_next_week, due_later
      # @return [Hash] response data
      def my_assignments_due(scope: nil)
        with_operation(service: "reports", operation: "my_assignments_due", is_mutation: false) do
          http_get("/my/assignments/due.json", params: compact_params(scope: scope)).json
        end
      end

      # Get account-wide activity feed (progress report)
      # @return [Enumerator<Hash>] paginated results
      def progress()
        wrap_paginated(service: "reports", operation: "progress", is_mutation: false) do
          paginate("/reports/progress.json")
        end
      end

      # Get upcoming schedule entries within a date window
      # @param window_starts_on [String, nil] window starts on
      # @param window_ends_on [String, nil] window ends on
      # @return [Hash] response data
      def upcoming(window_starts_on: nil, window_ends_on: nil)
        with_operation(service: "reports", operation: "upcoming", is_mutation: false) do
          http_get("/reports/schedules/upcoming.json", params: compact_params(window_starts_on: window_starts_on, window_ends_on: window_ends_on)).json
        end
      end

      # Get todos assigned to a specific person
      # @param person_id [Integer] person id ID
      # @param group_by [String, nil] Group by "bucket" or "date"
      # @return [Hash] response data
      def assigned(person_id:, group_by: nil)
        with_operation(service: "reports", operation: "assigned", is_mutation: false, resource_id: person_id) do
          http_get("/reports/todos/assigned/#{person_id}", params: compact_params(group_by: group_by)).json
        end
      end

      # Get overdue todos grouped by lateness
      # @return [Hash] response data
      def overdue()
        with_operation(service: "reports", operation: "overdue", is_mutation: false) do
          http_get("/reports/todos/overdue.json").json
        end
      end

      # Get a person's activity timeline
      # @param person_id [Integer] person id ID
      # @return [Hash] response data
      def person_progress(person_id:)
        wrap_paginated_wrapped(key: "events", service: "reports", operation: "person_progress", is_mutation: false, resource_id: person_id) do
          paginate_wrapped("/reports/users/progress/#{person_id}.json", key: "events")
        end
      end
    end
  end
end
