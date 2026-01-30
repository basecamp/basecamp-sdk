/**
 * Service for Lineup operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Lineup operations
 */
export class LineupService extends BaseService {

  /**
   * Create a new lineup marker
   */
  async create(req: components["schemas"]["CreateLineupMarkerRequestContent"]): Promise<components["schemas"]["CreateLineupMarkerResponseContent"]> {
    const response = await this.request(
      {
        service: "Lineup",
        operation: "CreateLineupMarker",
        resourceType: "lineup_marker",
        isMutation: true,
      },
      () =>
        this.client.POST("/lineup/markers.json", {
          body: req,
        })
    );
    return response;
  }

  /**
   * Update an existing lineup marker
   */
  async update(markerId: number, req: components["schemas"]["UpdateLineupMarkerRequestContent"]): Promise<components["schemas"]["UpdateLineupMarkerResponseContent"]> {
    const response = await this.request(
      {
        service: "Lineup",
        operation: "UpdateLineupMarker",
        resourceType: "lineup_marker",
        isMutation: true,
        resourceId: markerId,
      },
      () =>
        this.client.PUT("/lineup/markers/{markerId}", {
          params: {
            path: { markerId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Delete a lineup marker
   */
  async delete(markerId: number): Promise<void> {
    await this.request(
      {
        service: "Lineup",
        operation: "DeleteLineupMarker",
        resourceType: "lineup_marker",
        isMutation: true,
        resourceId: markerId,
      },
      () =>
        this.client.DELETE("/lineup/markers/{markerId}", {
          params: {
            path: { markerId },
          },
        })
    );
  }
}