# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Reports operations
    #
    # @generated from OpenAPI spec
    class ReportsService < BaseService

      # Get account-wide activity feed (progress report)
      # @param date [String, nil] The day to read (YYYY-MM-DD). The feed serves one day per request and defaults to today, so step this back a day at a time to read history; `page` pages within the day. The server reads an unparseable date as today.
      # @param project_ids [Array, nil] Only events in these projects. When omitted, the server applies the project filter the caller last saved in the web's activity view, not "all projects". If none of the IDs is a project the caller can access, the response is empty and the server clears that saved filter, so send only IDs the API just returned.
      # @param people_ids [Array, nil] Only events created by these people. When omitted, the server applies the people filter the caller last saved in the web's activity view, not "everyone". If none of the IDs is a person the caller can see, the response is empty and the server clears that saved filter, so send only IDs the API just returned.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def progress(date: nil, project_ids: nil, people_ids: nil, page: nil, max_items: nil)
        wrap_paginated(service: "reports", operation: "progress", is_mutation: false) do
          params = compact_query_params(date: date, project_ids: project_ids, people_ids: people_ids, page: page)
          paginate("/reports/progress.json", params: params, operation: "GetProgressReport", max_items: max_items)
        end
      end

      # Get upcoming schedule entries and assignable items within a date window.
      # @param window_starts_on [String] Inclusive first day of the window, `YYYY-MM-DD`. Required — BC3 answers 400 without it.
      # @param window_ends_on [String] Inclusive last day of the window, `YYYY-MM-DD`. Required — BC3 answers 400 without it.
      # @return [Hash] response data
      def upcoming(window_starts_on:, window_ends_on:)
        with_operation(service: "reports", operation: "upcoming", is_mutation: false) do
          http_get("/reports/schedules/upcoming.json", params: compact_query_params(window_starts_on: window_starts_on, window_ends_on: window_ends_on), operation: "GetUpcomingSchedule").json(operation: "GetUpcomingSchedule")
        end
      end

      # Get todos assigned to a specific person
      # @param person_id [Integer] person id ID
      # @param group_by [String, nil] Group by "bucket" or "date"
      # @return [Hash] response data
      def assigned(person_id:, group_by: nil)
        with_operation(service: "reports", operation: "assigned", is_mutation: false, resource_id: person_id) do
          http_get("/reports/todos/assigned/#{person_id}", params: compact_query_params(group_by: group_by), operation: "GetAssignedTodos").json(operation: "GetAssignedTodos")
        end
      end

      # Get overdue todos grouped by lateness
      # @return [Hash] response data
      def overdue()
        with_operation(service: "reports", operation: "overdue", is_mutation: false) do
          http_get("/reports/todos/overdue.json", operation: "GetOverdueTodos").json(operation: "GetOverdueTodos")
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
