# frozen_string_literal: true

module Basecamp
  module Services
    # Service for comment operations.
    #
    # Comments can be added to most recordings (todos, messages, etc.)
    # in Basecamp. They support HTML content.
    #
    # @example List comments on a todo
    #   account.comments.list(project_id: 123, recording_id: 456).each do |comment|
    #     puts "#{comment["creator"]["name"]}: #{comment["content"]}"
    #   end
    #
    # @example Create a comment
    #   comment = account.comments.create(
    #     project_id: 123,
    #     recording_id: 456,
    #     content: "<p>Great work on this!</p>"
    #   )
    class CommentsService < BaseService
      # Lists all comments on a recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID (todo, message, etc.)
      # @return [Enumerator<Hash>] comments
      def list(project_id:, recording_id:)
        paginate(bucket_path(project_id, "/recordings/#{recording_id}/comments.json"))
      end

      # Gets a specific comment.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param comment_id [Integer, String] comment ID
      # @return [Hash] comment data
      def get(project_id:, comment_id:)
        http_get(bucket_path(project_id, "/comments/#{comment_id}.json")).json
      end

      # Creates a new comment on a recording.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] recording ID to comment on
      # @param content [String] comment content in HTML
      # @return [Hash] created comment
      def create(project_id:, recording_id:, content:)
        body = { content: content }
        http_post(bucket_path(project_id, "/recordings/#{recording_id}/comments.json"), body: body).json
      end

      # Updates an existing comment.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param comment_id [Integer, String] comment ID
      # @param content [String] new comment content in HTML
      # @return [Hash] updated comment
      def update(project_id:, comment_id:, content:)
        body = { content: content }
        http_put(bucket_path(project_id, "/comments/#{comment_id}.json"), body: body).json
      end
    end
  end
end
