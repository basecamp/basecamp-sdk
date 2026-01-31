/**
 * Timeline service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Timeline operations.
 */
export class TimelineService extends BaseService {

  /**
   * Get project timeline
   * @param projectId - The project ID
   * @returns Array of results
   */
  async projectTimeline(projectId: number): Promise<components["schemas"]["GetProjectTimelineResponseContent"]> {
    const response = await this.request(
      {
        service: "Timeline",
        operation: "GetProjectTimeline",
        resourceType: "project_timeline",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/timeline.json", {
          params: {
            path: { projectId },
          },
        })
    );
    return response ?? [];
  }
}