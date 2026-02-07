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
   * @param projectId - The project ID
   * @param options - Optional query parameters
   * @returns All ClientCorrespondence across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.clientCorrespondences.list(123);
   * ```
   */
  async list(projectId: number, options?: ListClientCorrespondenceOptions): Promise<ListResult<ClientCorrespondence>> {
    return this.requestPaginated(
      {
        service: "ClientCorrespondences",
        operation: "ListClientCorrespondences",
        resourceType: "client_correspondence",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/correspondences.json", {
          params: {
            path: { projectId },
          },
        })
      , options
    );
  }

  /**
   * Get a single client correspondence by id
   * @param projectId - The project ID
   * @param correspondenceId - The correspondence ID
   * @returns The ClientCorrespondence
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.clientCorrespondences.get(123, 123);
   * ```
   */
  async get(projectId: number, correspondenceId: number): Promise<ClientCorrespondence> {
    const response = await this.request(
      {
        service: "ClientCorrespondences",
        operation: "GetClientCorrespondence",
        resourceType: "client_correspondence",
        isMutation: false,
        projectId,
        resourceId: correspondenceId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/correspondences/{correspondenceId}", {
          params: {
            path: { projectId, correspondenceId },
          },
        })
    );
    return response;
  }
}