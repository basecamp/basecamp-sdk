# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Lineup operations
    #
    # @generated from OpenAPI spec
    class LineupService < BaseService

      # Create a new lineup marker
      def create(**body)
        http_post("/lineup/markers.json", body: body).json
      end

      # Update an existing lineup marker
      def update(marker_id:, **body)
        http_put("/lineup/markers/#{marker_id}", body: body).json
      end

      # Delete a lineup marker
      def delete(marker_id:)
        http_delete("/lineup/markers/#{marker_id}")
        nil
      end
    end
  end
end
