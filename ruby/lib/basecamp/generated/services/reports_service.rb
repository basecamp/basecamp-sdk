# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Reports operations
    #
    # @generated from OpenAPI spec
    class ReportsService < BaseService

      # Get account-wide activity feed (progress report)
      def progress()
        paginate("/reports/progress.json")
      end

      # Get upcoming schedule entries within a date window
      def upcoming(window_starts_on: nil, window_ends_on: nil)
        http_get("/reports/schedules/upcoming.json", params: compact_params(window_starts_on: window_starts_on, window_ends_on: window_ends_on)).json
      end

      # Get todos assigned to a specific person
      def assigned(person_id:, group_by: nil)
        http_get("/reports/todos/assigned/#{person_id}", params: compact_params(group_by: group_by)).json
      end

      # Get overdue todos grouped by lateness
      def overdue()
        http_get("/reports/todos/overdue.json").json
      end

      # Get a person's activity timeline
      def person_progress(person_id:)
        http_get("/reports/users/progress/#{person_id}").json
      end
    end
  end
end
