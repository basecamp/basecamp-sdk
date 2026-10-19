# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Pings operations
    #
    # @generated from OpenAPI spec
    class PingsService < BaseService

      # Get Pings: the caller's active Pings that have a line in them, most
      # @return [Array<Hash>] response data
      def list()
        with_operation(service: "pings", operation: "list", is_mutation: false) do
          http_get("/circles.json", operation: "ListPings").json(operation: "ListPings")
        end
      end

      # Create a Ping with people, or return the one that already holds exactly
      # @param user_ids [Array] Person ids of the people to ping. People the caller cannot reach are
      #   dropped.
      # @param new_group [Boolean, nil] Create a new group even when a Ping with exactly these people exists.
      # @return [Hash] response data
      def create(user_ids:, new_group: nil)
        with_operation(service: "pings", operation: "create", is_mutation: true) do
          http_post("/circles.json", body: compact_params(user_ids: user_ids, new_group: new_group)).json(operation: "CreatePing")
        end
      end

      # Get a Ping by its id, which is its bucket's id.
      # @param ping_id [Integer] A Ping's id is the id of the bucket that holds it (the bucket type is
      #   "Circle", which is also the wire path), so the `bucket.id` a Ping's
      #   recordings carry is the id GetPing takes.
      # @return [Hash] response data
      def get(ping_id:)
        with_operation(service: "pings", operation: "get", is_mutation: false, resource_id: ping_id) do
          http_get("/circles/#{ping_id}", operation: "GetPing").json(operation: "GetPing")
        end
      end
    end
  end
end
