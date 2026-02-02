/**
 * ClientCorrespondences service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** ClientCorrespondence entity from the Basecamp API. */
export type ClientCorrespondence = components["schemas"]["ClientCorrespondence"];

// =============================================================================
// Service
// =============================================================================

/**
 * Service for ClientCorrespondences operations.
 */
export class ClientCorrespondencesService extends BaseService {

  /**
   * List all client correspondences in a project
   * @returns Array of ClientCorrespondence
   *
   * @example
   * ```ts
   * const result = await client.clientCorrespondences.list();
   * ```
   */
  async list(): Promise<ClientCorrespondence[]> {
    const response = await this.request(
      {
        service: "ClientCorrespondences",
        operation: "ListClientCorrespondences",
        resourceType: "client_correspondence",
        isMutation: false,
      },
      () =>
        this.client.GET("/client/correspondences.json", {
        })
    );
    return response ?? [];
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