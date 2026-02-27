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
          http_get("/comments/#{comment_id}").json
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
      # @return [Enumerator<Hash>] paginated results
      def list(recording_id:)
        wrap_paginated(service: "comments", operation: "list", is_mutation: false, resource_id: recording_id) do
          paginate("/recordings/#{recording_id}/comments.json")
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
