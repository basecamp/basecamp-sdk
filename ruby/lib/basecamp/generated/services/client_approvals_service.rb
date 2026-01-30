# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientApprovals operations
    #
    # @generated from OpenAPI spec
    class ClientApprovalsService < BaseService

      # List all client approvals in a project
      def list(project_id:)
        paginate(bucket_path(project_id, "/client/approvals.json"))
      end

      # Get a single client approval by id
      def get(project_id:, approval_id:)
        http_get(bucket_path(project_id, "/client/approvals/#{approval_id}")).json
      end
    end
  end
end
