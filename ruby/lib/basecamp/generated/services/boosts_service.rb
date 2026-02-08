# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Boosts operations
    #
    # @generated from OpenAPI spec
    class BoostsService < BaseService

      # Get a single boost
      # @param project_id [Integer] project id ID
      # @param boost_id [Integer] boost id ID
      # @return [Hash] response data
      def get_boost(project_id:, boost_id:)
        http_get(bucket_path(project_id, "/boosts/#{boost_id}")).json
      end

      # Delete a boost
      # @param project_id [Integer] project id ID
      # @param boost_id [Integer] boost id ID
      # @return [void]
      def delete_boost(project_id:, boost_id:)
        http_delete(bucket_path(project_id, "/boosts/#{boost_id}"))
        nil
      end

      # List boosts on a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @return [Enumerator<Hash>] paginated results
      def list_recording_boosts(project_id:, recording_id:)
        paginate(bucket_path(project_id, "/recordings/#{recording_id}/boosts.json"))
      end

      # Create a boost on a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @param content [String] content
      # @return [Hash] response data
      def create_recording_boost(project_id:, recording_id:, content:)
        http_post(bucket_path(project_id, "/recordings/#{recording_id}/boosts.json"), body: compact_params(content: content)).json
      end

      # List boosts on a specific event within a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @param event_id [Integer] event id ID
      # @return [Enumerator<Hash>] paginated results
      def list_event_boosts(project_id:, recording_id:, event_id:)
        paginate(bucket_path(project_id, "/recordings/#{recording_id}/events/#{event_id}/boosts.json"))
      end

      # Create a boost on a specific event within a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @param event_id [Integer] event id ID
      # @param content [String] content
      # @return [Hash] response data
      def create_event_boost(project_id:, recording_id:, event_id:, content:)
        http_post(bucket_path(project_id, "/recordings/#{recording_id}/events/#{event_id}/boosts.json"), body: compact_params(content: content)).json
      end
    end
  end
end
