/**
 * Timeline service for the Basecamp API.
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


/**
 * Options for projectTimeline.
 */
export interface ProjectTimelineTimelineOptions extends PaginationOptions {
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Timeline operations.
 */
export class TimelineService extends BaseService {

  /**
   * Get project timeline
   * @param options - Optional query parameters
   * @returns All results across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.timeline.projectTimeline();
   * ```
   */
  async projectTimeline(options?: ProjectTimelineTimelineOptions): Promise<components["schemas"]["GetProjectTimelineResponseContent"]> {
    return this.requestPaginated(
      {
        service: "Timeline",
        operation: "GetProjectTimeline",
        resourceType: "project_timeline",
        isMutation: false,
      },
      () =>
        this.client.GET("/timeline.json", {
        })
      , options
    );
  }
}