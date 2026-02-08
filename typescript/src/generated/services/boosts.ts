/**
 * Boosts service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Options for listRecordingBoosts.
 */
export interface ListRecordingBoostsBoostOptions extends PaginationOptions {
}

/**
 * Request parameters for createRecordingBoost.
 */
export interface CreateRecordingBoostBoostRequest {
  /** Text content */
  content: string;
}

/**
 * Options for listEventBoosts.
 */
export interface ListEventBoostsBoostOptions extends PaginationOptions {
}

/**
 * Request parameters for createEventBoost.
 */
export interface CreateEventBoostBoostRequest {
  /** Text content */
  content: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Boosts operations.
 */
export class BoostsService extends BaseService {

  /**
   * Get a single boost
   * @param projectId - The project ID
   * @param boostId - The boost ID
   * @returns The boost
   *
   * @example
   * ```ts
   * const result = await client.boosts.boost(123, 123);
   * ```
   */
  async boost(projectId: number, boostId: number): Promise<components["schemas"]["GetBoostResponseContent"]> {
    const response = await this.request(
      {
        service: "Boosts",
        operation: "GetBoost",
        resourceType: "boost",
        isMutation: false,
        projectId,
        resourceId: boostId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/boosts/{boostId}", {
          params: {
            path: { projectId, boostId },
          },
        })
    );
    return response;
  }

  /**
   * Delete a boost
   * @param projectId - The project ID
   * @param boostId - The boost ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.boosts.deleteBoost(123, 123);
   * ```
   */
  async deleteBoost(projectId: number, boostId: number): Promise<void> {
    await this.request(
      {
        service: "Boosts",
        operation: "DeleteBoost",
        resourceType: "boost",
        isMutation: true,
        projectId,
        resourceId: boostId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/boosts/{boostId}", {
          params: {
            path: { projectId, boostId },
          },
        })
    );
  }

  /**
   * List boosts on a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param options - Optional query parameters
   * @returns All results across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.boosts.listRecordingBoosts(123, 123);
   * ```
   */
  async listRecordingBoosts(projectId: number, recordingId: number, options?: ListRecordingBoostsBoostOptions): Promise<components["schemas"]["ListRecordingBoostsResponseContent"]> {
    return this.requestPaginated(
      {
        service: "Boosts",
        operation: "ListRecordingBoosts",
        resourceType: "recording_boost",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}/boosts.json", {
          params: {
            path: { projectId, recordingId },
          },
        })
      , options
    );
  }

  /**
   * Create a boost on a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param req - Recording_boost creation parameters
   * @returns The recording_boost
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.boosts.createRecordingBoost(123, 123, { content: "Hello world" });
   * ```
   */
  async createRecordingBoost(projectId: number, recordingId: number, req: CreateRecordingBoostBoostRequest): Promise<components["schemas"]["CreateRecordingBoostResponseContent"]> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    const response = await this.request(
      {
        service: "Boosts",
        operation: "CreateRecordingBoost",
        resourceType: "recording_boost",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/recordings/{recordingId}/boosts.json", {
          params: {
            path: { projectId, recordingId },
          },
          body: {
            content: req.content,
          },
        })
    );
    return response;
  }

  /**
   * List boosts on a specific event within a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param eventId - The event ID
   * @param options - Optional query parameters
   * @returns All results across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.boosts.listEventBoosts(123, 123, 123);
   * ```
   */
  async listEventBoosts(projectId: number, recordingId: number, eventId: number, options?: ListEventBoostsBoostOptions): Promise<components["schemas"]["ListEventBoostsResponseContent"]> {
    return this.requestPaginated(
      {
        service: "Boosts",
        operation: "ListEventBoosts",
        resourceType: "event_boost",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}/events/{eventId}/boosts.json", {
          params: {
            path: { projectId, recordingId, eventId },
          },
        })
      , options
    );
  }

  /**
   * Create a boost on a specific event within a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param eventId - The event ID
   * @param req - Event_boost creation parameters
   * @returns The event_boost
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.boosts.createEventBoost(123, 123, 123, { content: "Hello world" });
   * ```
   */
  async createEventBoost(projectId: number, recordingId: number, eventId: number, req: CreateEventBoostBoostRequest): Promise<components["schemas"]["CreateEventBoostResponseContent"]> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    const response = await this.request(
      {
        service: "Boosts",
        operation: "CreateEventBoost",
        resourceType: "event_boost",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/recordings/{recordingId}/events/{eventId}/boosts.json", {
          params: {
            path: { projectId, recordingId, eventId },
          },
          body: {
            content: req.content,
          },
        })
    );
    return response;
  }
}