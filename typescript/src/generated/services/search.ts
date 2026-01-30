/**
 * Service for Search operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Search operations
 */
export class SearchService extends BaseService {

  /**
   * Search for content across the account
   */
  async search(query: string, options?: { sort?: string }): Promise<components["schemas"]["SearchResponseContent"]> {
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
            query: { query: query, sort: options?.sort },
          },
        })
    );
    return response;
  }

  /**
   * Get search metadata (available filter options)
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