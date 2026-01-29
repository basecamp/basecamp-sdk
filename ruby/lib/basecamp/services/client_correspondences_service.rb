# frozen_string_literal: true

module Basecamp
  module Services
    # Service for client correspondence operations.
    #
    # Client correspondences are messages sent to and from clients
    # within a project's client portal.
    #
    # @example List client correspondences
    #   account.client_correspondences.list(project_id: 123).each do |c|
    #     puts "#{c["subject"]} - #{c["replies_count"]} replies"
    #   end
    class ClientCorrespondencesService < BaseService
      # Lists all client correspondences in a project.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @return [Enumerator<Hash>] client correspondences
      def list(project_id:)
        paginate(bucket_path(project_id, "/client/correspondences.json"))
      end

      # Gets a client correspondence by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param correspondence_id [Integer, String] client correspondence ID
      # @return [Hash] client correspondence data
      def get(project_id:, correspondence_id:)
        http_get(bucket_path(project_id, "/client/correspondences/#{correspondence_id}.json")).json
      end
    end
  end
end
