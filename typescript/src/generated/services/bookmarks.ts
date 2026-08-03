/**
 * Bookmarks service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";

// =============================================================================
// Types
// =============================================================================

/** Bookmark entity from the Basecamp API. */
export type Bookmark = components["schemas"]["Bookmark"];
/** BookmarkStatus entity from the Basecamp API. */
export type BookmarkStatus = components["schemas"]["BookmarkStatus"];

/**
 * Options for listMyBookmarks.
 */
export interface ListMyBookmarksBookmarkOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Bookmarks operations.
 */
export class BookmarksService extends BaseService {

  /**
   * List the current user's bookmarks, most recently bookmarked first (paginated).
   * @param options - Optional query parameters
   * @returns All Bookmark across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.bookmarks.listMyBookmarks();
   *
   * // With options
   * const filtered = await client.bookmarks.listMyBookmarks({ page: 1 });
   * ```
   */
  async listMyBookmarks(options?: ListMyBookmarksBookmarkOptions): Promise<ListResult<Bookmark>> {
    return this.requestPaginated(
      {
        service: "Bookmarks",
        operation: "ListMyBookmarks",
        resourceType: "bookmark",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/bookmarks.json", {
          params: {
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Report whether the current user has bookmarked the recording.
   * @param recordingId - The recording ID
   * @returns The BookmarkStatus
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.bookmarks.getBookmark(123);
   * ```
   */
  async getBookmark(recordingId: number): Promise<BookmarkStatus> {
    const response = await this.request(
      {
        service: "Bookmarks",
        operation: "GetBookmark",
        resourceType: "bookmark",
        isMutation: false,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/recordings/{recordingId}/bookmark.json", {
          params: {
            path: { recordingId },
          },
        })
    );
    return response;
  }

  /**
   * Bookmark a recording for the current user.
   * @param recordingId - The recording ID
   * @returns The Bookmark
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.bookmarks.createBookmark(123);
   * ```
   */
  async createBookmark(recordingId: number): Promise<Bookmark> {
    const response = await this.request(
      {
        service: "Bookmarks",
        operation: "CreateBookmark",
        resourceType: "bookmark",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/recordings/{recordingId}/bookmark.json", {
          params: {
            path: { recordingId },
          },
        })
    );
    return response;
  }

  /**
   * Remove the current user's bookmark from a recording.
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.bookmarks.deleteBookmark(123);
   * ```
   */
  async deleteBookmark(recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Bookmarks",
        operation: "DeleteBookmark",
        resourceType: "bookmark",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.DELETE("/recordings/{recordingId}/bookmark.json", {
          params: {
            path: { recordingId },
          },
        })
    );
  }
}