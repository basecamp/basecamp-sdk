# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Events operations
    #
    # @generated from OpenAPI spec
    class EventsService < BaseService

      # List all events for a recording
      # @param recording_id [Integer] recording id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(recording_id:, page: nil, max_items: nil)
        wrap_paginated(service: "events", operation: "list", is_mutation: false, resource_id: recording_id) do
          params = compact_query_params(page: page)
          paginate("/recordings/#{recording_id}/events.json", params: params, operation: "ListEvents", max_items: max_items)
        end
      end
    end
  end
end
