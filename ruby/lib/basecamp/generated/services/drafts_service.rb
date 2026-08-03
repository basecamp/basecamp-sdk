# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Drafts operations
    #
    # @generated from OpenAPI spec
    class DraftsService < BaseService

      # List the current user's drafts across their active projects, most recently
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_my_drafts(page: nil, max_items: nil)
        wrap_paginated(service: "drafts", operation: "list_my_drafts", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/my/drafts.json", params: params, operation: "ListMyDrafts", max_items: max_items)
        end
      end
    end
  end
end
