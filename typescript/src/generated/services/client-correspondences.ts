/**
 * ClientCorrespondences service for the Basecamp API.
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

/** ClientCorrespondence entity from the Basecamp API. */
export type ClientCorrespondence = components["schemas"]["ClientCorrespondence"];

/**
 * Options for list.
 */
export interface ListClientCorrespondenceOptions extends PaginationOptions {
  /** Filter by sort */
  sort?: "created_at" | "updated_at";
  /** Filter by direction */
  direction?: "asc" | "desc";
  /** Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8. */
  page?: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for ClientCorrespondences operations.
 */
export class ClientCorrespondencesService extends BaseService {

  /**
   * List all client correspondences in a project
   * @param bucketId - The bucket ID
   * @param options - Optional query parameters
   * @returns All ClientCorrespondence across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.clientCorrespondences.list(123);
   *
   * // With options
   * const filtered = await client.clientCorrespondences.list(123, { sort: "created_at" });
   * ```
   */
  async list(bucketId: number, options?: ListClientCorrespondenceOptions): Promise<ListResult<ClientCorrespondence>> {
    return this.requestPaginated(
      {
        service: "ClientCorrespondences",
        operation: "ListClientCorrespondences",
        resourceType: "client_correspondence",
        isMutation: false,
        projectId: bucketId,
      },
      () =>
        this.client.GET("/buckets/{bucketId}/client/correspondences.json", {
          params: {
            path: { bucketId },
            query: { sort: options?.sort, direction: options?.direction, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get a single client correspondence by id
   * @param correspondenceId - The correspondence ID
   * @returns The ClientCorrespondence
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.clientCorrespondences.get(123);
   * ```
   */
  async get(correspondenceId: number): Promise<ClientCorrespondence> {
    const response = await this.request(
      {
        service: "ClientCorrespondences",
        operation: "GetClientCorrespondence",
        resourceType: "client_correspondence",
        isMutation: false,
        resourceId: correspondenceId,
      },
      () =>
        this.client.GET("/client/correspondences/{correspondenceId}", {
          params: {
            path: { correspondenceId },
          },
        })
    );
    return response;
  }
}