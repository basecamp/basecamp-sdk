# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Everything operations
    #
    # @generated from OpenAPI spec
    class EverythingService < BaseService

      # Get every boost across all accessible projects, newest-first (paginated).
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1.
      # @return [Enumerator<Hash>] paginated results
      def get_everything_boosts(page: nil)
        wrap_paginated(service: "everything", operation: "get_everything_boosts", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/boosts.json", params: params)
        end
      end

      # Get every overdue card across all accessible projects, oldest-due-date-first.
      # @return [Hash] response data
      def get_everything_overdue_cards()
        with_operation(service: "everything", operation: "get_everything_overdue_cards", is_mutation: false) do
          http_get("/cards/overdue.json").json
        end
      end

      # Get every automatic check-in answer across all accessible projects, newest-first.
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1.
      # @return [Enumerator<Hash>] paginated results
      def get_everything_checkins(page: nil)
        wrap_paginated(service: "everything", operation: "get_everything_checkins", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/checkins.json", params: params)
        end
      end

      # Get every comment across all accessible projects, newest-first (paginated).
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1.
      # @return [Enumerator<Hash>] paginated results
      def get_everything_comments(page: nil)
        wrap_paginated(service: "everything", operation: "get_everything_comments", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/comments.json", params: params)
        end
      end

      # Get every file recording across all accessible projects, newest-first
      # @param kind [String, nil] Filter by file kind: all (default), images, pdfs, documents, or videos.
      # @param people_ids [Array, nil] Restrict to files created by the given people (repeatable).
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1.
      # @return [Enumerator<Hash>] paginated results
      def get_everything_files(kind: nil, people_ids: nil, page: nil)
        wrap_paginated(service: "everything", operation: "get_everything_files", is_mutation: false) do
          params = compact_query_params(kind: kind, people_ids: people_ids, page: page)
          paginate("/files.json", params: params)
        end
      end

      # Get every inbox forward across all accessible projects, newest-first
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1.
      # @return [Enumerator<Hash>] paginated results
      def get_everything_forwards(page: nil)
        wrap_paginated(service: "everything", operation: "get_everything_forwards", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/forwards.json", params: params)
        end
      end

      # Get every message across all accessible projects, newest-first (paginated).
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1.
      # @return [Enumerator<Hash>] paginated results
      def get_everything_messages(page: nil)
        wrap_paginated(service: "everything", operation: "get_everything_messages", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/messages.json", params: params)
        end
      end

      # Get every overdue to-do across all accessible projects, oldest-due-date-first.
      # @return [Hash] response data
      def get_everything_overdue_todos()
        with_operation(service: "everything", operation: "get_everything_overdue_todos", is_mutation: false) do
          http_get("/todos/overdue.json").json
        end
      end
    end
  end
end
