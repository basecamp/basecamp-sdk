# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Events operations
    #
    # @generated from OpenAPI spec
    class EventsService < BaseService

      # List all events for a recording
      # @param recording_id [Integer] recording id ID
      # @return [Enumerator<Hash>] paginated results
      def list(recording_id:)
        wrap_paginated(service: "events", operation: "list", is_mutation: false, resource_id: recording_id) do
          paginate("/recordings/#{recording_id}/events.json")
        end
      end
    end
  end
end
