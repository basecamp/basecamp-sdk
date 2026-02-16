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
        with_operation(service: "lineup", operation: "create", is_mutation: true) do
          http_post("/lineup/markers.json", body: compact_params(name: name, date: date))
          nil
        end
      end

      # Update an existing lineup marker
      # @param marker_id [Integer] marker id ID
      # @param name [String, nil] name
      # @param date [String, nil] date
      # @return [void]
      def update(marker_id:, name: nil, date: nil)
        with_operation(service: "lineup", operation: "update", is_mutation: true, resource_id: marker_id) do
          http_put("/lineup/markers/#{marker_id}", body: compact_params(name: name, date: date))
          nil
        end
      end

      # Delete a lineup marker
      # @param marker_id [Integer] marker id ID
      # @return [void]
      def delete(marker_id:)
        with_operation(service: "lineup", operation: "delete", is_mutation: true, resource_id: marker_id) do
          http_delete("/lineup/markers/#{marker_id}")
          nil
        end
      end
    end
  end
end
