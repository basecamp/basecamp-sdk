# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Lineup operations
    #
    # @generated from OpenAPI spec
    class LineupService < BaseService

      # Create a new lineup marker
      # @param title [String] title
      # @param starts_on [String] starts on (YYYY-MM-DD)
      # @param ends_on [String] ends on (YYYY-MM-DD)
      # @param color [String, nil] color
      # @param description [String, nil] description
      # @return [void]
      def create(title:, starts_on:, ends_on:, color: nil, description: nil)
        http_post("/lineup/markers.json", body: compact_params(title: title, starts_on: starts_on, ends_on: ends_on, color: color, description: description))
        nil
      end

      # Update an existing lineup marker
      # @param marker_id [Integer] marker id ID
      # @param title [String, nil] title
      # @param starts_on [String, nil] starts on (YYYY-MM-DD)
      # @param ends_on [String, nil] ends on (YYYY-MM-DD)
      # @param color [String, nil] color
      # @param description [String, nil] description
      # @return [void]
      def update(marker_id:, title: nil, starts_on: nil, ends_on: nil, color: nil, description: nil)
        http_put("/lineup/markers/#{marker_id}", body: compact_params(title: title, starts_on: starts_on, ends_on: ends_on, color: color, description: description))
        nil
      end

      # Delete a lineup marker
      # @param marker_id [Integer] marker id ID
      # @return [void]
      def delete(marker_id:)
        http_delete("/lineup/markers/#{marker_id}")
        nil
      end
    end
  end
end
