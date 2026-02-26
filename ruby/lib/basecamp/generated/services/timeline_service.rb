# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Timeline operations
    #
    # @generated from OpenAPI spec
    class TimelineService < BaseService

      # Get project timeline
      # @param project_id [Integer] project id ID
      # @return [Enumerator<Hash>] paginated results
      def get_project_timeline(project_id:)
        wrap_paginated(service: "timeline", operation: "get_project_timeline", is_mutation: false, project_id: project_id) do
          paginate("/buckets/#{project_id}/timeline.json")
        end
      end
    end
  end
end
