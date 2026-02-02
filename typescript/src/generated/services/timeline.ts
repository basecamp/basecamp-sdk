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
   * @returns Array of results
   *
   * @example
   * ```ts
   * const result = await client.timeline.projectTimeline();
   * ```
   */
  async projectTimeline(): Promise<components["schemas"]["GetProjectTimelineResponseContent"]> {
    const response = await this.request(
      {
        service: "Timeline",
        operation: "GetProjectTimeline",
        resourceType: "project_timeline",
        isMutation: false,
      },
      () =>
        this.client.GET("/timeline.json", {
        })
    );
    return response ?? [];
  }
}