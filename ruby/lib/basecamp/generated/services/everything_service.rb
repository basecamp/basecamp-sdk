# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Everything operations
    #
    # @generated from OpenAPI spec
    class EverythingService < BaseService

      # Completed cards across all accessible projects, grouped by project (paginated).
      # @param assignee_ids [Array, nil] Restrict to tasks assigned to at least one of the given people (repeatable).
      #   Assignees on nested steps are not considered.
      # @param due [String, nil] Filter by due date: with, without, or overdue. Unrecognized values are ignored.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_completed_cards(assignee_ids: nil, due: nil, page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_completed_cards", is_mutation: false) do
          params = compact_query_params(assignee_ids: assignee_ids, due: due, page: page)
          paginate("/cards/completed.json", params: params, operation: "GetEverythingCompletedCards", max_items: max_items)
        end
      end

      # Open cards with no due date across all accessible projects, grouped by project (paginated).
      # @param assignee_ids [Array, nil] Restrict to tasks assigned to at least one of the given people (repeatable).
      #   Assignees on nested steps are not considered.
      # @param due [String, nil] Filter by due date: with, without, or overdue. Unrecognized values are ignored.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_no_due_date_cards(assignee_ids: nil, due: nil, page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_no_due_date_cards", is_mutation: false) do
          params = compact_query_params(assignee_ids: assignee_ids, due: due, page: page)
          paginate("/cards/no_due_date.json", params: params, operation: "GetEverythingNoDueDateCards", max_items: max_items)
        end
      end

      # Cards parked in a project's "Not now" column across all accessible projects, grouped by project (paginated).
      # @param assignee_ids [Array, nil] Restrict to tasks assigned to at least one of the given people (repeatable).
      #   Assignees on nested steps are not considered.
      # @param due [String, nil] Filter by due date: with, without, or overdue. Unrecognized values are ignored.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_not_now_cards(assignee_ids: nil, due: nil, page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_not_now_cards", is_mutation: false) do
          params = compact_query_params(assignee_ids: assignee_ids, due: due, page: page)
          paginate("/cards/not_now.json", params: params, operation: "GetEverythingNotNowCards", max_items: max_items)
        end
      end

      # Incomplete cards in active columns across all accessible projects, grouped by project (paginated).
      # @param assignee_ids [Array, nil] Restrict to tasks assigned to at least one of the given people (repeatable).
      #   Assignees on nested steps are not considered.
      # @param due [String, nil] Filter by due date: with, without, or overdue. Unrecognized values are ignored.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_open_cards(assignee_ids: nil, due: nil, page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_open_cards", is_mutation: false) do
          params = compact_query_params(assignee_ids: assignee_ids, due: due, page: page)
          paginate("/cards/open.json", params: params, operation: "GetEverythingOpenCards", max_items: max_items)
        end
      end

      # Get every overdue card across all accessible projects, oldest-due-date-first.
      # @param assignee_ids [Array, nil] Restrict to tasks assigned to at least one of the given people (repeatable).
      #   Assignees on nested steps are not considered.
      # @param due [String, nil] Filter by due date: with, without, or overdue. Unrecognized values are ignored.
      # @return [Array<Hash>] response data
      def get_everything_overdue_cards(assignee_ids: nil, due: nil)
        with_operation(service: "everything", operation: "get_everything_overdue_cards", is_mutation: false) do
          http_get("/cards/overdue.json", params: compact_query_params(assignee_ids: assignee_ids, due: due), operation: "GetEverythingOverdueCards").json
        end
      end

      # Open, unassigned cards across all accessible projects, grouped by project (paginated).
      # @param assignee_ids [Array, nil] Restrict to tasks assigned to at least one of the given people (repeatable).
      #   Assignees on nested steps are not considered.
      # @param due [String, nil] Filter by due date: with, without, or overdue. Unrecognized values are ignored.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_unassigned_cards(assignee_ids: nil, due: nil, page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_unassigned_cards", is_mutation: false) do
          params = compact_query_params(assignee_ids: assignee_ids, due: due, page: page)
          paginate("/cards/unassigned.json", params: params, operation: "GetEverythingUnassignedCards", max_items: max_items)
        end
      end

      # Get every automatic check-in answer across all accessible projects, newest-first.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_checkins(page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_checkins", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/checkins.json", params: params, operation: "GetEverythingCheckins", max_items: max_items)
        end
      end

      # Get every comment across all accessible projects, newest-first (paginated).
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_comments(page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_comments", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/comments.json", params: params, operation: "GetEverythingComments", max_items: max_items)
        end
      end

      # Get every file recording across all accessible projects, newest-first (paginated).
      # @param kind [String, nil] Filter by file kind: all (default), images, pdfs, documents, or videos.
      # @param people_ids [Array, nil] Restrict to files created by the given people (repeatable).
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_files(kind: nil, people_ids: nil, page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_files", is_mutation: false) do
          params = compact_query_params(kind: kind, people_ids: people_ids, page: page)
          paginate("/files.json", params: params, operation: "GetEverythingFiles", max_items: max_items)
        end
      end

      # Get every inbox forward across all accessible projects, newest-first (paginated).
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_forwards(page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_forwards", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/forwards.json", params: params, operation: "GetEverythingForwards", max_items: max_items)
        end
      end

      # Get every message across all accessible projects, newest-first (paginated).
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_messages(page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_messages", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/messages.json", params: params, operation: "GetEverythingMessages", max_items: max_items)
        end
      end

      # Completed to-dos across all accessible projects, grouped by project (paginated).
      # @param assignee_ids [Array, nil] Restrict to tasks assigned to at least one of the given people (repeatable).
      #   Assignees on nested steps are not considered.
      # @param due [String, nil] Filter by due date: with, without, or overdue. Unrecognized values are ignored.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_completed_todos(assignee_ids: nil, due: nil, page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_completed_todos", is_mutation: false) do
          params = compact_query_params(assignee_ids: assignee_ids, due: due, page: page)
          paginate("/todos/completed.json", params: params, operation: "GetEverythingCompletedTodos", max_items: max_items)
        end
      end

      # Open to-dos with no due date across all accessible projects, grouped by project (paginated).
      # @param assignee_ids [Array, nil] Restrict to tasks assigned to at least one of the given people (repeatable).
      #   Assignees on nested steps are not considered.
      # @param due [String, nil] Filter by due date: with, without, or overdue. Unrecognized values are ignored.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_no_due_date_todos(assignee_ids: nil, due: nil, page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_no_due_date_todos", is_mutation: false) do
          params = compact_query_params(assignee_ids: assignee_ids, due: due, page: page)
          paginate("/todos/no_due_date.json", params: params, operation: "GetEverythingNoDueDateTodos", max_items: max_items)
        end
      end

      # Active, incomplete to-dos across all accessible projects, grouped by project (paginated).
      # @param assignee_ids [Array, nil] Restrict to tasks assigned to at least one of the given people (repeatable).
      #   Assignees on nested steps are not considered.
      # @param due [String, nil] Filter by due date: with, without, or overdue. Unrecognized values are ignored.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_open_todos(assignee_ids: nil, due: nil, page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_open_todos", is_mutation: false) do
          params = compact_query_params(assignee_ids: assignee_ids, due: due, page: page)
          paginate("/todos/open.json", params: params, operation: "GetEverythingOpenTodos", max_items: max_items)
        end
      end

      # Get every overdue to-do across all accessible projects, oldest-due-date-first.
      # @param assignee_ids [Array, nil] Restrict to tasks assigned to at least one of the given people (repeatable).
      #   Assignees on nested steps are not considered.
      # @param due [String, nil] Filter by due date: with, without, or overdue. Unrecognized values are ignored.
      # @return [Array<Hash>] response data
      def get_everything_overdue_todos(assignee_ids: nil, due: nil)
        with_operation(service: "everything", operation: "get_everything_overdue_todos", is_mutation: false) do
          http_get("/todos/overdue.json", params: compact_query_params(assignee_ids: assignee_ids, due: due), operation: "GetEverythingOverdueTodos").json
        end
      end

      # Open, unassigned to-dos across all accessible projects, grouped by project (paginated).
      # @param assignee_ids [Array, nil] Restrict to tasks assigned to at least one of the given people (repeatable).
      #   Assignees on nested steps are not considered.
      # @param due [String, nil] Filter by due date: with, without, or overdue. Unrecognized values are ignored.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_everything_unassigned_todos(assignee_ids: nil, due: nil, page: nil, max_items: nil)
        wrap_paginated(service: "everything", operation: "get_everything_unassigned_todos", is_mutation: false) do
          params = compact_query_params(assignee_ids: assignee_ids, due: due, page: page)
          paginate("/todos/unassigned.json", params: params, operation: "GetEverythingUnassignedTodos", max_items: max_items)
        end
      end
    end
  end
end
