# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientApprovals operations
    #
    # @generated from OpenAPI spec
    class ClientApprovalsService < BaseService

      # List all client approvals in a project
      # @param sort [String, nil] created_at|updated_at
      # @param direction [String, nil] asc|desc
      # @return [Enumerator<Hash>] paginated results
      def list(sort: nil, direction: nil)
        wrap_paginated(service: "clientapprovals", operation: "list", is_mutation: false) do
          params = compact_query_params(sort: sort, direction: direction)
          paginate("/client/approvals.json", params: params)
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
