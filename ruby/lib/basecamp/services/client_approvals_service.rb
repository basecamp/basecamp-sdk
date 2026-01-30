# frozen_string_literal: true

module Basecamp
  module Services
    # Service for client approval operations.
    #
    # Client approvals allow you to request approval from clients
    # on specific deliverables or decisions within a project.
    #
    # @example List client approvals
    #   account.client_approvals.list(project_id: 123).each do |approval|
    #     puts "#{approval["subject"]} - #{approval["approval_status"]}"
    #   end
    class ClientApprovalsService < BaseService
      # Lists all client approvals in a project.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @return [Enumerator<Hash>] client approvals
      def list(project_id:)
        paginate(bucket_path(project_id, "/client/approvals.json"))
      end

      # Gets a client approval by ID.
      #
      # @param project_id [Integer, String] project (bucket) ID
      # @param approval_id [Integer, String] client approval ID
      # @return [Hash] client approval data
      def get(project_id:, approval_id:)
        http_get(bucket_path(project_id, "/client/approvals/#{approval_id}.json")).json
      end
    end
  end
end
