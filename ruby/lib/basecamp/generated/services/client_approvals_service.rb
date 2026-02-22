# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientApprovals operations
    #
    # @generated from OpenAPI spec
    class ClientApprovalsService < BaseService

      # List all client approvals in a project
      # @return [Enumerator<Hash>] paginated results
      def list()
        wrap_paginated(service: "clientapprovals", operation: "list", is_mutation: false) do
          paginate("/client/approvals.json")
        end
      end

      # Get a single client approval by id
      # @param approval_id [Integer] approval id ID
      # @return [Hash] response data
      def get(approval_id:)
        with_operation(service: "clientapprovals", operation: "get", is_mutation: false, resource_id: approval_id) do
          http_get("/client/approvals/#{approval_id}").json
        end
      end
    end
  end
end
