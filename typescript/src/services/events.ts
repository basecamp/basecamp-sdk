/**
 * Events service for the Basecamp API.
 *
 * Events are activity records that track changes to recordings.
 * An event is created any time a recording is modified (created,
 * updated, completed, etc.).
 *
 * @example
 * ```ts
 * // List events for a recording
 * const events = await client.events.list(projectId, recordingId);
 * for (const event of events) {
 *   console.log(event.action, event.created_at, event.creator?.name);
 * }
 * ```
 */

import { BaseService } from "./base.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp event (activity record).
 */
export type Event = components["schemas"]["Event"];

/**
 * Event details with action-specific information.
 */
export type EventDetails = components["schemas"]["EventDetails"];

/**
 * A person associated with the event (creator).
 */
export type Person = components["schemas"]["Person"];

// =============================================================================
// Service
// =============================================================================

/**
 * Service for viewing Basecamp events.
 */
export class EventsService extends BaseService {
  /**
   * Lists all events for a recording.
   * Events track all changes made to a recording over time.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The recording ID
   * @returns Array of events
   *
   * @example
   * ```ts
   * const events = await client.events.list(projectId, todoId);
   * for (const event of events) {
   *   console.log(`${event.action} by ${event.creator?.name} at ${event.created_at}`);
   *
   *   // Check for assignment changes
   *   if (event.details?.added_person_ids?.length) {
   *     console.log("Assigned to:", event.details.added_person_ids);
   *   }
   * }
   * ```
   */
  async list(projectId: number, recordingId: number): Promise<Event[]> {
    const response = await this.request(
      {
        service: "Events",
        operation: "List",
        resourceType: "event",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}/events.json", {
          params: { path: { projectId, recordingId } },
        })
    );

    return response?.events ?? [];
  }
}
