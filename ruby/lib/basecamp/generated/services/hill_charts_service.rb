# frozen_string_literal: true

module Basecamp
  module Services
    # Service for HillCharts operations
    #
    # @generated from OpenAPI spec
    class HillChartsService < BaseService

      # Get the hill chart for a todoset
      # @param todoset_id [Integer] todoset id ID
      # @return [Hash] response data
      def get(todoset_id:)
        with_operation(service: "hillcharts", operation: "get", is_mutation: false, resource_id: todoset_id) do
          http_get("/todosets/#{todoset_id}/hill.json").json
        end
      end

      # Track or untrack todolists on a hill chart
      # @param todoset_id [Integer] todoset id ID
      # @param tracked [Array, nil] tracked
      # @param untracked [Array, nil] untracked
      # @return [Hash] response data
      def update_settings(todoset_id:, tracked: nil, untracked: nil)
        with_operation(service: "hillcharts", operation: "update_settings", is_mutation: true, resource_id: todoset_id) do
          http_put("/todosets/#{todoset_id}/hills/settings.json", body: compact_params(tracked: tracked, untracked: untracked)).json
        end
      end
    end
  end
end
