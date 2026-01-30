# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Timeline operations
    #
    # @generated from OpenAPI spec
    class TimelineService < BaseService

      # Get project timeline
      def get_project_timeline(project_id:)
        paginate(bucket_path(project_id, "/timeline.json"))
      end
    end
  end
end
