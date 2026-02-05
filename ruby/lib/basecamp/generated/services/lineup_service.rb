# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Lineup operations
    #
    # @generated from OpenAPI spec
    class LineupService < BaseService

      # Create a new lineup marker
      # @param name [String] name
      # @param date [String] date
      # @return [void]
      def create(name:, date:)
        http_post("/lineup/markers.json", body: compact_params(name: name, date: date))
        nil
      end

      # Update an existing lineup marker
      # @param marker_id [Integer] marker id ID
      # @param name [String, nil] name
      # @param date [String, nil] date
      # @return [void]
      def update(marker_id:, name: nil, date: nil)
        http_put("/lineup/markers/#{marker_id}", body: compact_params(name: name, date: date))
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
