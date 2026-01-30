# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Comments operations
    #
    # @generated from OpenAPI spec
    class CommentsService < BaseService

      # Get a single comment by id
      def get(project_id:, comment_id:)
        http_get(bucket_path(project_id, "/comments/#{comment_id}")).json
      end

      # Update an existing comment
      def update(project_id:, comment_id:, **body)
        http_put(bucket_path(project_id, "/comments/#{comment_id}"), body: body).json
      end

      # List comments on a recording
      def list(project_id:, recording_id:)
        paginate(bucket_path(project_id, "/recordings/#{recording_id}/comments.json"))
      end

      # Create a new comment on a recording
      def create(project_id:, recording_id:, **body)
        http_post(bucket_path(project_id, "/recordings/#{recording_id}/comments.json"), body: body).json
      end
    end
  end
end
