/**
 * Events service for the Basecamp API.
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

/** Event entity from the Basecamp API. */
export type Event = components["schemas"]["Event"];

/**
 * Options for list.
 */
export interface ListEventOptions extends PaginationOptions {
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Events operations.
 */
export class EventsService extends BaseService {

  /**
   * List all events for a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param options - Optional query parameters
   * @returns All Event across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.events.list(123, 123);
   * ```
   */
  async list(projectId: number, recordingId: number, options?: ListEventOptions): Promise<ListResult<Event>> {
    return this.requestPaginated(
      {
        service: "Events",
        operation: "ListEvents",
        resourceType: "event",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}/events.json", {
          params: {
            path: { projectId, recordingId },
          },
        })
      , options
    );
  }
}