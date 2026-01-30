/**
 * Service for ClientVisibility operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for ClientVisibility operations
 */
export class ClientVisibilityService extends BaseService {

  /**
   * Set client visibility for a recording
   */
  async setVisibility(projectId: number, recordingId: number, req: components["schemas"]["SetClientVisibilityRequestContent"]): Promise<components["schemas"]["SetClientVisibilityResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }
}