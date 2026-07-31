# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Drafts operations
    #
    # @generated from OpenAPI spec
    class DraftsService < BaseService

      # List the current user's drafts across their active projects, most recently
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1.
      # @return [Enumerator<Hash>] paginated results
      def list_my_drafts(page: nil)
        wrap_paginated(service: "drafts", operation: "list_my_drafts", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/my/drafts.json", params: params, operation: "ListMyDrafts")
        end
      end
    end
  end
end
