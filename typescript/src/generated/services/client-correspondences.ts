/**
 * Service for ClientCorrespondences operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for ClientCorrespondences operations
 */
export class ClientCorrespondencesService extends BaseService {

  /**
   * List all client correspondences in a project
   */
  async list(projectId: number): Promise<components["schemas"]["ListClientCorrespondencesResponseContent"]> {
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
   */
  async get(projectId: number, correspondenceId: number): Promise<components["schemas"]["GetClientCorrespondenceResponseContent"]> {
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