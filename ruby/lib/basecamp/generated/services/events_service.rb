# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Events operations
    #
    # @generated from OpenAPI spec
    class EventsService < BaseService

      # List all events for a recording
      def list(project_id:, recording_id:)
        paginate(bucket_path(project_id, "/recordings/#{recording_id}/events.json"))
      end
    end
  end
end
