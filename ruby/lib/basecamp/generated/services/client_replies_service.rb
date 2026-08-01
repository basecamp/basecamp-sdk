# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientReplies operations
    #
    # @generated from OpenAPI spec
    class ClientRepliesService < BaseService

      # List all client replies for a recording (correspondence or approval)
      # @param recording_id [Integer] recording id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(recording_id:, page: nil, max_items: nil)
        wrap_paginated(service: "clientreplies", operation: "list", is_mutation: false, resource_id: recording_id) do
          params = compact_query_params(page: page)
          paginate("/client/recordings/#{recording_id}/replies.json", params: params, operation: "ListClientReplies", max_items: max_items)
        end
      end

      # Get a single client reply by id
      # @param recording_id [Integer] recording id ID
      # @param reply_id [Integer] reply id ID
      # @return [Hash] response data
      def get(recording_id:, reply_id:)
        with_operation(service: "clientreplies", operation: "get", is_mutation: false, resource_id: reply_id) do
          http_get("/client/recordings/#{recording_id}/replies/#{reply_id}", operation: "GetClientReply").json
        end
      end
    end
  end
end
