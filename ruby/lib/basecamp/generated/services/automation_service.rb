# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Automation operations
    #
    # @generated from OpenAPI spec
    class AutomationService < BaseService

      # List all lineup markers for the account
      # @return [Array<Hash>] response data
      def list_lineup_markers()
        with_operation(service: "automation", operation: "list_lineup_markers", is_mutation: false) do
          http_get("/lineup/markers.json", operation: "ListLineupMarkers").json
        end
      end
    end
  end
end
