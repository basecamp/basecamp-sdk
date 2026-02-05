/**
 * Search service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Options for search.
 */
export interface SearchSearchOptions {
  /** Filter by sort */
  sort?: "created_at" | "updated_at";
  /** Page */
  page?: number;
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
   * @param query - query
   * @param options - Optional query parameters
   * @returns Array of results
   *
   * @example
   * ```ts
   * const result = await client.search.search("query");
   * ```
   */
  async search(query: string, options?: SearchSearchOptions): Promise<components["schemas"]["SearchResponseContent"]> {
    const response = await this.request(
      {
        service: "Search",
        operation: "Search",
        resourceType: "resource",
        isMutation: false,
      },
      () =>
        this.client.GET("/search.json", {
          params: {
            query: { query: query, sort: options?.sort, page: options?.page },
          },
        })
    );
    return response ?? [];
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