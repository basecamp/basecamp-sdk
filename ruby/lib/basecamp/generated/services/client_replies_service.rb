# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientReplies operations
    #
    # @generated from OpenAPI spec
    class ClientRepliesService < BaseService

      # List all client replies for a recording (correspondence or approval)
      # @param recording_id [Integer] recording id ID
      # @return [Enumerator<Hash>] paginated results
      def list(recording_id:)
        wrap_paginated(service: "clientreplies", operation: "list", is_mutation: false, project_id: project_id, resource_id: recording_id) do
          paginate("/client/recordings/#{recording_id}/replies.json")
        end
      end

      # Get a single client reply by id
      # @param recording_id [Integer] recording id ID
      # @param reply_id [Integer] reply id ID
      # @return [Hash] response data
      def get(recording_id:, reply_id:)
        with_operation(service: "clientreplies", operation: "get", is_mutation: false, project_id: project_id, resource_id: reply_id) do
          http_get("/client/recordings/#{recording_id}/replies/#{reply_id}").json
        end
      end
    end
  end
end
