# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Timeline operations
    #
    # @generated from OpenAPI spec
    class TimelineService < BaseService

      # Get project timeline
      # @param project_id [Integer] project id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def get_project_timeline(project_id:, page: nil, max_items: nil)
        wrap_paginated(service: "timeline", operation: "get_project_timeline", is_mutation: false, project_id: project_id) do
          params = compact_query_params(page: page)
          paginate("/projects/#{project_id}/timeline.json", params: params, operation: "GetProjectTimeline", max_items: max_items)
        end
      end
    end
  end
end
