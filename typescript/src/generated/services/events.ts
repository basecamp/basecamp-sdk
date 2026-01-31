/**
 * Events service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Event entity from the Basecamp API. */
export type Event = components["schemas"]["Event"];

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
   * @returns Array of Event
   */
  async list(projectId: number, recordingId: number): Promise<Event[]> {
    const response = await this.request(
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
    );
    return response ?? [];
  }
}