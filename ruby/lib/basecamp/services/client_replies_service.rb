# frozen_string_literal: true

module Basecamp
  module Services
    # Service for client reply operations.
    #
    # Client replies are responses to client correspondences or approvals
    # within a project's client portal.
    #
    # @example List client replies
    #   account.client_replies.list(project_id: 123, recording_id: 456).each do |r|
    #     puts "#{r["creator"]["name"]}: #{r["content"]}"
    #   end
    class ClientRepliesService < BaseService
      # Lists all replies for a client recording (correspondence or approval).
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] parent correspondence/approval ID
      # @return [Enumerator<Hash>] client replies
      def list(project_id:, recording_id:)
        paginate(bucket_path(project_id, "/client/recordings/#{recording_id}/replies.json"))
      end

      # Gets a specific client reply.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param recording_id [Integer, String] parent correspondence/approval ID
      # @param reply_id [Integer, String] client reply ID
      # @return [Hash] client reply data
      def get(project_id:, recording_id:, reply_id:)
        http_get(bucket_path(project_id, "/client/recordings/#{recording_id}/replies/#{reply_id}.json")).json
      end
    end
  end
end
