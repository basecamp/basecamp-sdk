# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Reports operations
    #
    # @generated from OpenAPI spec
    class ReportsService < BaseService

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
        with_operation(service: "reports", operation: "person_progress", is_mutation: false, resource_id: person_id) do
          http_get("/reports/users/progress/#{person_id}").json
        end
      end
    end
  end
end
