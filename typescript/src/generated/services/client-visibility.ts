/**
 * ClientVisibility service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Recording entity from the Basecamp API. */
export type Recording = components["schemas"]["Recording"];

/**
 * Request parameters for setVisibility.
 */
export interface SetVisibilityClientVisibilityRequest {
  /** Visible to clients */
  visibleToClients: boolean;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for ClientVisibility operations.
 */
export class ClientVisibilityService extends BaseService {

  /**
   * Set client visibility for a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param req - Client_visibility request parameters
   * @returns The Recording
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.clientVisibility.setVisibility(123, 123, { visibleToClients: true });
   * ```
   */
  async setVisibility(projectId: number, recordingId: number, req: SetVisibilityClientVisibilityRequest): Promise<Recording> {
    const response = await this.request(
      {
        service: "ClientVisibility",
        operation: "SetClientVisibility",
        resourceType: "client_visibility",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/client_visibility.json", {
          params: {
            path: { projectId, recordingId },
          },
          body: {
            visible_to_clients: req.visibleToClients,
          },
        })
    );
    return response;
  }
}