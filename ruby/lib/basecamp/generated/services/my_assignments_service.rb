# frozen_string_literal: true

module Basecamp
  module Services
    # Service for MyAssignments operations
    #
    # @generated from OpenAPI spec
    class MyAssignmentsService < BaseService

      # Get the current user's active assignments grouped into priorities and non_priorities.
      # @return [Hash] response data
      def get_my_assignments()
        with_operation(service: "myassignments", operation: "get_my_assignments", is_mutation: false) do
          http_get("/my/assignments.json").json
        end
      end

      # Get the current user's completed assignments.
      # @return [Hash] response data
      def get_my_completed_assignments()
        with_operation(service: "myassignments", operation: "get_my_completed_assignments", is_mutation: false) do
          http_get("/my/assignments/completed.json").json
        end
      end

      # Get the current user's assignments filtered by due date scope.
      # @param scope [String, nil] Filter by due date range: overdue, due_today, due_tomorrow,
      #   due_later_this_week, due_next_week, due_later
      # @return [Hash] response data
      def get_my_due_assignments(scope: nil)
        with_operation(service: "myassignments", operation: "get_my_due_assignments", is_mutation: false) do
          http_get("/my/assignments/due.json", params: compact_query_params(scope: scope)).json
        end
      end
    end
  end
end
