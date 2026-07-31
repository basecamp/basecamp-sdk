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
          http_get("/my/assignments.json", operation: "GetMyAssignments").json
        end
      end

      # Get the current user's completed assignments.
      # @return [Array<Hash>] response data
      def get_my_completed_assignments()
        with_operation(service: "myassignments", operation: "get_my_completed_assignments", is_mutation: false) do
          http_get("/my/assignments/completed.json", operation: "GetMyCompletedAssignments").json
        end
      end

      # Get the current user's assignments filtered by due date scope.
      # @param scope [String, nil] Filter by due date range: overdue, due_today, due_tomorrow,
      #   due_later_this_week, due_next_week, due_later
      # @return [Array<Hash>] response data
      def get_my_due_assignments(scope: nil)
        with_operation(service: "myassignments", operation: "get_my_due_assignments", is_mutation: false) do
          http_get("/my/assignments/due.json", params: compact_query_params(scope: scope), operation: "GetMyDueAssignments").json
        end
      end

      # Add a recording to Up Next — the current user's ordered list of prioritized
      # @param id [Integer] The recording id to prioritize.
      # @return [void]
      def prioritize_assignment(id:)
        with_operation(service: "myassignments", operation: "prioritize_assignment", is_mutation: true) do
          http_post("/my/priorities.json", body: compact_params(id: id))
          nil
        end
      end

      # Remove a recording from Up Next (returns 204 No Content). Exact-target:
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def deprioritize_assignment(recording_id:)
        with_operation(service: "myassignments", operation: "deprioritize_assignment", is_mutation: true, resource_id: recording_id) do
          http_delete("/my/priorities/#{recording_id}")
          nil
        end
      end

      # Move an already-prioritized recording to a new 1-based position in Up Next
      # @param source_id [Integer] The recording id to move, chosen the same way as when prioritizing.
      # @param position [Integer] The 1-based position to move it to.
      # @return [void]
      def reorder_up_next(source_id:, position:)
        with_operation(service: "myassignments", operation: "reorder_up_next", is_mutation: true) do
          http_post("/my/priority_moves.json", body: compact_params(source_id: source_id, position: position))
          nil
        end
      end
    end
  end
end
