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
   * @param projectId - The project ID
   * @returns Array of ClientCorrespondence
   */
  async list(projectId: number): Promise<ClientCorrespondence[]> {
    const response = await this.request(
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
    );
    return response ?? [];
  }

  /**
   * Get a single client correspondence by id
   * @param projectId - The project ID
   * @param correspondenceId - The correspondence ID
   * @returns The ClientCorrespondence
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