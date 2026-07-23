/**
 * Search service for the Basecamp API.
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


/**
 * Options for search.
 */
export interface SearchSearchOptions extends PaginationOptions {
  /** Recording types to include. Use `key` values from the metadata
endpoint's `recording_search_types`. Available since Basecamp 5. */
  typeNames?: string[];
  /** Project IDs to filter by. Available since Basecamp 5. */
  bucketIds?: number[];
  /** Creator person IDs to filter by. Available since Basecamp 5. */
  creatorIds?: number[];
  /** Filter attachments by type. Use `key` values from the metadata
endpoint's `file_search_types`. */
  fileType?: string;
  /** Set to true to exclude chat results. */
  excludeChat?: boolean;
  /** Filter by since */
  since?: "last_7_days" | "last_30_days" | "last_90_days" | "last_12_months" | "forever";
  /** Filter by sort */
  sort?: "best_match" | "recency";
  /** Deprecated: prefer type_names[]. */
  type?: string;
  /** Deprecated: prefer bucket_ids[]. */
  bucketId?: number;
  /** Deprecated: prefer creator_ids[]. */
  creatorId?: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Search operations.
 */
export class SearchService extends BaseService {

  /**
   * Search for content across the account
   * @param q - q
   * @param options - Optional query parameters
   * @returns All results across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.search.search("q");
   * ```
   */
  async search(q: string, options?: SearchSearchOptions): Promise<components["schemas"]["SearchResponseContent"]> {
    return this.requestPaginated(
      {
        service: "Search",
        operation: "Search",
        resourceType: "resource",
        isMutation: false,
      },
      () =>
        this.client.GET("/search.json", {
          params: {
            query: { q: q, "type_names[]": options?.typeNames, "bucket_ids[]": options?.bucketIds, "creator_ids[]": options?.creatorIds, "file_type": options?.fileType, "exclude_chat": options?.excludeChat, since: options?.since, sort: options?.sort, type: options?.type, "bucket_id": options?.bucketId, "creator_id": options?.creatorId },
          },
        })
      , options
    );
  }

  /**
   * Get search metadata (available filter options)
   * @returns The search_metadata
   *
   * @example
   * ```ts
   * const result = await client.search.metadata();
   * ```
   */
  async metadata(): Promise<components["schemas"]["GetSearchMetadataResponseContent"]> {
    const response = await this.request(
      {
        service: "Search",
        operation: "GetSearchMetadata",
        resourceType: "search_metadata",
        isMutation: false,
      },
      () =>
        this.client.GET("/searches/metadata.json", {
        })
    );
    return response;
  }
}