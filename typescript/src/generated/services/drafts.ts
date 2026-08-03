/**
 * Drafts service for the Basecamp API.
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

/** Draft entity from the Basecamp API. */
export type Draft = components["schemas"]["Draft"];

/**
 * Options for listMyDrafts.
 */
export interface ListMyDraftsDraftOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Drafts operations.
 */
export class DraftsService extends BaseService {

  /**
   * List the current user's drafts across their active projects, most recently
   * @param options - Optional query parameters
   * @returns All Draft across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.drafts.listMyDrafts();
   *
   * // With options
   * const filtered = await client.drafts.listMyDrafts({ page: 1 });
   * ```
   */
  async listMyDrafts(options?: ListMyDraftsDraftOptions): Promise<ListResult<Draft>> {
    return this.requestPaginated(
      {
        service: "Drafts",
        operation: "ListMyDrafts",
        resourceType: "my_draft",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/drafts.json", {
          params: {
            query: { page: options?.page },
          },
        })
      , options
    );
  }
}