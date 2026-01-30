/**
 * Service for Events operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Events operations
 */
export class EventsService extends BaseService {

  /**
   * List all events for a recording
   */
  async list(projectId: number, recordingId: number): Promise<components["schemas"]["ListEventsResponseContent"]> {
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