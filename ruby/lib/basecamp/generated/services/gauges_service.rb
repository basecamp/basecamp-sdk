# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Gauges operations
    #
    # @generated from OpenAPI spec
    class GaugesService < BaseService

      # Get a gauge needle by ID
      # @param needle_id [Integer] needle id ID
      # @return [Hash] response data
      def get_gauge_needle(needle_id:)
        with_operation(service: "gauges", operation: "get_gauge_needle", is_mutation: false, resource_id: needle_id) do
          http_get("/gauge_needles/#{needle_id}").json
        end
      end

      # Update a gauge needle's description. Position and color are immutable.
      # @param needle_id [Integer] needle id ID
      # @param gauge_needle [Hash, nil] gauge needle
      # @return [Hash] response data
      def update_gauge_needle(needle_id:, gauge_needle: nil)
        with_operation(service: "gauges", operation: "update_gauge_needle", is_mutation: true, resource_id: needle_id) do
          http_put("/gauge_needles/#{needle_id}", body: compact_params(gauge_needle: gauge_needle)).json
        end
      end

      # Destroy a gauge needle
      # @param needle_id [Integer] needle id ID
      # @return [void]
      def destroy_gauge_needle(needle_id:)
        with_operation(service: "gauges", operation: "destroy_gauge_needle", is_mutation: true, resource_id: needle_id) do
          http_delete("/gauge_needles/#{needle_id}")
          nil
        end
      end

      # Enable or disable the gauge for a project. Only project admins can toggle gauges.
      # @param project_id [Integer] project id ID
      # @param gauge [Hash] gauge
      # @return [void]
      def toggle_gauge(project_id:, gauge:)
        with_operation(service: "gauges", operation: "toggle_gauge", is_mutation: true, project_id: project_id) do
          http_put("/projects/#{project_id}/gauge.json", body: compact_params(gauge: gauge))
          nil
        end
      end

      # List gauge needles for a project, ordered newest first.
      # @param project_id [Integer] project id ID
      # @return [Enumerator<Hash>] paginated results
      def list_gauge_needles(project_id:)
        wrap_paginated(service: "gauges", operation: "list_gauge_needles", is_mutation: false, project_id: project_id) do
          paginate("/projects/#{project_id}/gauge/needles.json")
        end
      end

      # Create a gauge needle (progress update) for a project
      # @param project_id [Integer] project id ID
      # @param gauge_needle [Hash] gauge needle
      # @param notify [String, nil] Who to notify: "everyone", "working_on", "custom", or omit for nobody
      # @param subscriptions [Array, nil] Array of people IDs to notify (only used when notify is "custom")
      # @return [Hash] response data
      def create_gauge_needle(project_id:, gauge_needle:, notify: nil, subscriptions: nil)
        with_operation(service: "gauges", operation: "create_gauge_needle", is_mutation: true, project_id: project_id) do
          http_post("/projects/#{project_id}/gauge/needles.json", body: compact_params(gauge_needle: gauge_needle, notify: notify, subscriptions: subscriptions)).json
        end
      end

      # List gauges across all projects the authenticated user has access to.
      # @param bucket_ids [String, nil] Comma-separated list of project IDs. When provided, results are returned
      #   in the order specified instead of by risk level.
      # @return [Enumerator<Hash>] paginated results
      def list_gauges(bucket_ids: nil)
        wrap_paginated(service: "gauges", operation: "list_gauges", is_mutation: false) do
          params = compact_query_params(bucket_ids: bucket_ids)
          paginate("/reports/gauges.json", params: params)
        end
      end
    end
  end
end
