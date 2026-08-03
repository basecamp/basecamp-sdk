# frozen_string_literal: true

module Basecamp
  module Services
    # Service for ClientApprovals operations
    #
    # @generated from OpenAPI spec
    class ClientApprovalsService < BaseService

      # List all client approvals in a project
      # @param bucket_id [Integer] bucket id ID
      # @param sort [String, nil] created_at|updated_at
      # @param direction [String, nil] asc|desc
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list(bucket_id:, sort: nil, direction: nil, page: nil, max_items: nil)
        wrap_paginated(service: "clientapprovals", operation: "list", is_mutation: false, project_id: bucket_id) do
          params = compact_query_params(sort: sort, direction: direction, page: page)
          paginate("/buckets/#{bucket_id}/client/approvals.json", params: params, operation: "ListClientApprovals", max_items: max_items)
        end
      end

      # Get a single client approval by id
      # @param approval_id [Integer] approval id ID
      # @return [Hash] response data
      def get(approval_id:)
        with_operation(service: "clientapprovals", operation: "get", is_mutation: false, resource_id: approval_id) do
          http_get("/client/approvals/#{approval_id}", operation: "GetClientApproval").json
        end
      end
    end
  end
end
