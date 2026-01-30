/**
 * Search service for the Basecamp API.
 *
 * Provides full-text search across all content in your Basecamp account.
 *
 * @example
 * ```ts
 * const results = await client.search.search("quarterly report");
 * const metadata = await client.search.metadata();
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A search result from Basecamp.
 */
export type SearchResult = components["schemas"]["SearchResult"];

/**
 * Search metadata including available filter options.
 */
export type SearchMetadata = components["schemas"]["SearchMetadata"];

/**
 * A project available for search scope filtering.
 */
export type SearchProject = components["schemas"]["SearchProject"];

/**
 * Options for search.
 */
export interface SearchOptions {
  /**
   * Sort order for results.
   * "created_at" or "updated_at" (default: relevance)
   */
  sort?: "created_at" | "updated_at";
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for searching Basecamp content.
 */
export class SearchService extends BaseService {
  /**
   * Searches for content across the account.
   *
   * @param query - The search query string
   * @param options - Optional search parameters
   * @returns Array of search results
   * @throws BasecampError with code "validation" if query is empty
   *
   * @example
   * ```ts
   * // Basic search
   * const results = await client.search.search("project plan");
   *
   * // Search with sorting
   * const results = await client.search.search("quarterly report", {
   *   sort: "updated_at",
   * });
   * ```
   */
  async search(query: string, options?: SearchOptions): Promise<SearchResult[]> {
    if (!query || query.trim() === "") {
      throw Errors.validation("Search query is required");
    }

    const response = await this.request(
      {
        service: "Search",
        operation: "Search",
        resourceType: "search",
        isMutation: false,
      },
      () =>
        this.client.GET("/search.json", {
          params: {
            query: {
              query,
              sort: options?.sort,
            },
          },
        })
    );

    return response ?? [];
  }

  /**
   * Returns metadata about available search scopes.
   * This includes the list of projects available for filtering.
   *
   * @returns Search metadata with available filter options
   *
   * @example
   * ```ts
   * const metadata = await client.search.metadata();
   * console.log("Available projects:", metadata.projects);
   * ```
   */
  async metadata(): Promise<SearchMetadata> {
    const response = await this.request(
      {
        service: "Search",
        operation: "Metadata",
        resourceType: "search",
        isMutation: false,
      },
      () => this.client.GET("/searches/metadata.json")
    );

    return response ?? { projects: [] };
  }
}
