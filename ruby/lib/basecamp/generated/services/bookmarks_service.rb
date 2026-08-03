# frozen_string_literal: true

module Basecamp
  module Services
    # Service for Bookmarks operations
    #
    # @generated from OpenAPI spec
    class BookmarksService < BaseService

      # List the current user's bookmarks, most recently bookmarked first (paginated).
      # @param page [Integer, nil] Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
      # @param max_items [Integer, nil] cap on items yielded across pages; nil or non-positive means no cap
      # @return [ListEnumerator<Hash>] lazily paginated results (#meta carries pagination metadata)
      def list_my_bookmarks(page: nil, max_items: nil)
        wrap_paginated(service: "bookmarks", operation: "list_my_bookmarks", is_mutation: false) do
          params = compact_query_params(page: page)
          paginate("/my/bookmarks.json", params: params, operation: "ListMyBookmarks", max_items: max_items)
        end
      end

      # Report whether the current user has bookmarked the recording.
      # @param recording_id [Integer] recording id ID
      # @return [Hash] response data
      def get_bookmark(recording_id:)
        with_operation(service: "bookmarks", operation: "get_bookmark", is_mutation: false, resource_id: recording_id) do
          http_get("/recordings/#{recording_id}/bookmark.json", operation: "GetBookmark").json
        end
      end

      # Bookmark a recording for the current user.
      # @param recording_id [Integer] recording id ID
      # @return [Hash] response data
      def create_bookmark(recording_id:)
        with_operation(service: "bookmarks", operation: "create_bookmark", is_mutation: true, resource_id: recording_id) do
          http_post("/recordings/#{recording_id}/bookmark.json").json
        end
      end

      # Remove the current user's bookmark from a recording (returns 204 No Content).
      # @param recording_id [Integer] recording id ID
      # @return [void]
      def delete_bookmark(recording_id:)
        with_operation(service: "bookmarks", operation: "delete_bookmark", is_mutation: true, resource_id: recording_id) do
          http_delete("/recordings/#{recording_id}/bookmark.json")
          nil
        end
      end
    end
  end
end
