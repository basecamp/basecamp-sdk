# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Events operations
    #
    # @generated from OpenAPI spec
    class EventsService < BaseService

      # List all events for a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, recording_id:)
        paginate(bucket_path(project_id, "/recordings/#{recording_id}/events.json"))
      end
    end
  end
end
