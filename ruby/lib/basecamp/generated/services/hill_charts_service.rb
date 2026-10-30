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
          http_get("/todosets/#{todoset_id}/hill.json", operation: "GetHillChart").json(operation: "GetHillChart")
        end
      end

      # Track or untrack todolists on a hill chart
      # @param todoset_id [Integer] todoset id ID
      # @param tracked [Array, nil] tracked
      # @param untracked [Array, nil] untracked
      # @return [Hash] response data
      def update_settings(todoset_id:, tracked: nil, untracked: nil)
        with_operation(service: "hillcharts", operation: "update_settings", is_mutation: true, resource_id: todoset_id) do
          http_put("/todosets/#{todoset_id}/hills/settings.json", body: compact_params(tracked: tracked, untracked: untracked)).json(operation: "UpdateHillChartSettings")
        end
      end

      # List the progress updates on a todoset's hill chart, newest first.
      # @param todoset_id [Integer] todoset id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_versions(todoset_id:, page: nil, max_items: nil)
        wrap_paginated(service: "hillcharts", operation: "list_versions", is_mutation: false, resource_id: todoset_id) do
          params = compact_query_params(page: page)
          paginate("/todosets/#{todoset_id}/hills/versions.json", params: params, operation: "ListHillChartVersions", max_items: max_items)
        end
      end

      # Post a progress update on a todoset's hill chart.
      # @param todoset_id [Integer] todoset id ID
      # @param description [String, nil] A note about the update, as rich text (HTML).
      # @param dots [Array, nil] Todolists to move, each with its ID and new position. Todolists left
      #   out keep their current position.
      # @param subscriptions [Array, nil] People to notify of the update and subscribe to it. If absent, nobody
      #   else is notified.
      # @return [Hash] response data
      def create_version(todoset_id:, description: nil, dots: nil, subscriptions: nil)
        with_operation(service: "hillcharts", operation: "create_version", is_mutation: true, resource_id: todoset_id) do
          http_post("/todosets/#{todoset_id}/hills/versions.json", body: compact_params(description: description, dots: dots, subscriptions: subscriptions)).json(operation: "CreateHillChartVersion")
        end
      end

      # Get one progress update from a todoset's hill chart
      # @param todoset_id [Integer] todoset id ID
      # @param version_id [Integer] version id ID
      # @return [Hash] response data
      def get_version(todoset_id:, version_id:)
        with_operation(service: "hillcharts", operation: "get_version", is_mutation: false, resource_id: version_id) do
          http_get("/todosets/#{todoset_id}/hills/versions/#{version_id}", operation: "GetHillChartVersion").json(operation: "GetHillChartVersion")
        end
      end
    end
  end
end
