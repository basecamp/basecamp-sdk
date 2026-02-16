# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientApprovals operations
    #
    # @generated from OpenAPI spec
    class ClientApprovalsService < BaseService

      # List all client approvals in a project
      # @param project_id [Integer] project id ID
      # @return [Enumerator<Hash>] paginated results
      def list(project_id:)
        wrap_paginated(service: "clientapprovals", operation: "list", is_mutation: false, project_id: project_id) do
          paginate(bucket_path(project_id, "/client/approvals.json"))
        end
      end

      # Get a single client approval by id
      # @param project_id [Integer] project id ID
      # @param approval_id [Integer] approval id ID
      # @return [Hash] response data
      def get(project_id:, approval_id:)
        with_operation(service: "clientapprovals", operation: "get", is_mutation: false, project_id: project_id, resource_id: approval_id) do
          http_get(bucket_path(project_id, "/client/approvals/#{approval_id}")).json
        end
      end
    end
  end
end
