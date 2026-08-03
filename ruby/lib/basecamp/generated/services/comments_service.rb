# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Comments operations
    #
    # @generated from OpenAPI spec
    class CommentsService < BaseService

      # Get a single comment by id
      # @param comment_id [Integer] comment id ID
      # @return [Hash] response data
      def get(comment_id:)
        with_operation(service: "comments", operation: "get", is_mutation: false, resource_id: comment_id) do
          http_get("/comments/#{comment_id}", operation: "GetComment").json
        end
      end

      # Update an existing comment
      # @param comment_id [Integer] comment id ID
      # @param content [String] content
      # @return [Hash] response data
      def update(comment_id:, content:)
        with_operation(service: "comments", operation: "update", is_mutation: true, resource_id: comment_id) do
          http_put("/comments/#{comment_id}", body: compact_params(content: content)).json
        end
      end

      # List comments on a recording
      # @param recording_id [Integer] recording id ID
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(recording_id:, page: nil, max_items: nil)
        wrap_paginated(service: "comments", operation: "list", is_mutation: false, resource_id: recording_id) do
          params = compact_query_params(page: page)
          paginate("/recordings/#{recording_id}/comments.json", params: params, operation: "ListComments", max_items: max_items)
        end
      end

      # Create a new comment on a recording
      # @param recording_id [Integer] recording id ID
      # @param content [String] content
      # @return [Hash] response data
      def create(recording_id:, content:)
        with_operation(service: "comments", operation: "create", is_mutation: true, resource_id: recording_id) do
          http_post("/recordings/#{recording_id}/comments.json", body: compact_params(content: content)).json
        end
      end
    end
  end
end
