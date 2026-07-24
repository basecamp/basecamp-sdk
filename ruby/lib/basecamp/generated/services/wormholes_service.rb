# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Wormholes operations
    #
    # @generated from OpenAPI spec
    class WormholesService < BaseService

      # Update a wormhole's destination column
      # @param bucket_id [Integer] bucket id ID
      # @param wormhole_id [Integer] wormhole id ID
      # @param destination_recording_id [Integer] Id of the new destination column (on another accessible card table).
      # @return [Hash] response data
      def update(bucket_id:, wormhole_id:, destination_recording_id:)
        with_operation(service: "wormholes", operation: "update", is_mutation: true, resource_id: wormhole_id) do
          http_put("/buckets/#{bucket_id}/card_tables/wormholes/#{wormhole_id}", body: compact_params(destination_recording_id: destination_recording_id)).json
        end
      end

      # Delete a wormhole
      # @param bucket_id [Integer] bucket id ID
      # @param wormhole_id [Integer] wormhole id ID
      # @return [void]
      def delete(bucket_id:, wormhole_id:)
        with_operation(service: "wormholes", operation: "delete", is_mutation: true, resource_id: wormhole_id) do
          http_delete("/buckets/#{bucket_id}/card_tables/wormholes/#{wormhole_id}")
          nil
        end
      end

      # Create a wormhole linking this card table to a column on another card table.
      # @param bucket_id [Integer] bucket id ID
      # @param card_table_id [Integer] card table id ID
      # @param destination_recording_id [Integer] Id of the destination column (on another accessible card table) to link to.
      # @return [Hash] response data
      def create(bucket_id:, card_table_id:, destination_recording_id:)
        with_operation(service: "wormholes", operation: "create", is_mutation: true, resource_id: card_table_id) do
          http_post("/buckets/#{bucket_id}/card_tables/#{card_table_id}/wormholes.json", body: compact_params(destination_recording_id: destination_recording_id)).json
        end
      end
    end
  end
end
