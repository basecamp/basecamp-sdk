/**
 * Service for Timeline operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Timeline operations
 */
export class TimelineService extends BaseService {

  /**
   * Get project timeline
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
    return response;
  }
}