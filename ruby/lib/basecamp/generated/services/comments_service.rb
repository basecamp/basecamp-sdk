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
        http_get("/comments/#{comment_id}").json
      end

      # Update an existing comment
      # @param comment_id [Integer] comment id ID
      # @param content [String] content
      # @return [Hash] response data
      def update(comment_id:, content:)
        http_put("/comments/#{comment_id}", body: compact_params(content: content)).json
      end

      # List comments on a recording
      # @param recording_id [Integer] recording id ID
      # @return [Enumerator<Hash>] paginated results
      def list(recording_id:)
        paginate("/recordings/#{recording_id}/comments.json")
      end

      # Create a new comment on a recording
      # @param recording_id [Integer] recording id ID
      # @param content [String] content
      # @return [Hash] response data
      def create(recording_id:, content:)
        http_post("/recordings/#{recording_id}/comments.json", body: compact_params(content: content)).json
      end
    end
  end
end
