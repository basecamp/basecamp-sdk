# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientReplies operations
    #
    # @generated from OpenAPI spec
    class ClientRepliesService < BaseService

      # List all client replies for a recording (correspondence or approval)
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, recording_id:)
        wrap_paginated(service: "clientreplies", operation: "list", is_mutation: false, project_id: project_id, resource_id: recording_id) do
          paginate(bucket_path(project_id, "/client/recordings/#{recording_id}/replies.json"))
        end
      end

      # Get a single client reply by id
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @param reply_id [Integer] reply id ID
      # @return [Hash] response data
      def get(project_id:, recording_id:, reply_id:)
        with_operation(service: "clientreplies", operation: "get", is_mutation: false, project_id: project_id, resource_id: reply_id) do
          http_get(bucket_path(project_id, "/client/recordings/#{recording_id}/replies/#{reply_id}")).json
        end
      end
    end
  end
end
