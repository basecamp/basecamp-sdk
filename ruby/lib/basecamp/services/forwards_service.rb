# frozen_string_literal: true

module Basecamp
  module Services
    # Service for email forward operations.
    #
    # Forwards are emails that have been forwarded to a project's inbox.
    # Team members can reply to forwarded emails from within Basecamp.
    #
    # @example List forwards in an inbox
    #   account.forwards.list(project_id: 123, inbox_id: 456).each do |f|
    #     puts "#{f["subject"]} - from #{f["from"]}"
    #   end
    #
    # @example Create a reply
    #   reply = account.forwards.create_reply(
    #     project_id: 123,
    #     forward_id: 456,
    #     content: "<p>Thanks for reaching out!</p>"
    #   )
    class ForwardsService < BaseService
      # Gets an inbox by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param inbox_id [Integer, String] inbox ID
      # @return [Hash] inbox data
      def get_inbox(project_id:, inbox_id:)
        http_get(bucket_path(project_id, "/inboxes/#{inbox_id}.json")).json
      end

      # Lists all forwards in an inbox.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param inbox_id [Integer, String] inbox ID
      # @return [Enumerator<Hash>] forwards
      def list(project_id:, inbox_id:)
        paginate(bucket_path(project_id, "/inboxes/#{inbox_id}/forwards.json"))
      end

      # Gets a forward by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param forward_id [Integer, String] forward ID
      # @return [Hash] forward data
      def get(project_id:, forward_id:)
        http_get(bucket_path(project_id, "/inbox_forwards/#{forward_id}.json")).json
      end

      # Lists all replies to a forward.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param forward_id [Integer, String] forward ID
      # @return [Enumerator<Hash>] replies
      def list_replies(project_id:, forward_id:)
        paginate(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies.json"))
      end

      # Gets a specific reply.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param forward_id [Integer, String] forward ID
      # @param reply_id [Integer, String] reply ID
      # @return [Hash] reply data
      def get_reply(project_id:, forward_id:, reply_id:)
        http_get(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies/#{reply_id}.json")).json
      end

      # Creates a reply to a forwarded email.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param forward_id [Integer, String] forward ID
      # @param content [String] reply body in HTML
      # @return [Hash] created reply
      def create_reply(project_id:, forward_id:, content:)
        body = { content: content }
        http_post(bucket_path(project_id, "/inbox_forwards/#{forward_id}/replies.json"), body: body).json
      end
    end
  end
end
