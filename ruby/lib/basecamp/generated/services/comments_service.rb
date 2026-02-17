# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Comments operations
    #
    # @generated from OpenAPI spec
    class CommentsService < BaseService

      # Get a single comment by id
      # @param project_id [Integer] project id ID
      # @param comment_id [Integer] comment id ID
      # @return [Hash] response data
      def get(project_id:, comment_id:)
        with_operation(service: "comments", operation: "get", is_mutation: false, project_id: project_id, resource_id: comment_id) do
          http_get(bucket_path(project_id, "/comments/#{comment_id}")).json
        end
      end

      # Update an existing comment
      # @param project_id [Integer] project id ID
      # @param comment_id [Integer] comment id ID
      # @param content [String] content
      # @return [Hash] response data
      def update(project_id:, comment_id:, content:)
        with_operation(service: "comments", operation: "update", is_mutation: true, project_id: project_id, resource_id: comment_id) do
          http_put(bucket_path(project_id, "/comments/#{comment_id}"), body: compact_params(content: content)).json
        end
      end

      # List comments on a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:, recording_id:)
        wrap_paginated(service: "comments", operation: "list", is_mutation: false, project_id: project_id, resource_id: recording_id) do
          paginate(bucket_path(project_id, "/recordings/#{recording_id}/comments.json"))
        end
      end

      # Create a new comment on a recording
      # @param project_id [Integer] project id ID
      # @param recording_id [Integer] recording id ID
      # @param content [String] content
      # @return [Hash] response data
      def create(project_id:, recording_id:, content:)
        with_operation(service: "comments", operation: "create", is_mutation: true, project_id: project_id, resource_id: recording_id) do
          http_post(bucket_path(project_id, "/recordings/#{recording_id}/comments.json"), body: compact_params(content: content)).json
        end
      end
    end
  end
end
