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
 * Options for listForRecording.
 */
export interface ListForRecordingBoostOptions extends PaginationOptions {
}

/**
 * Request parameters for createForRecording.
 */
export interface CreateForRecordingBoostRequest {
  /** Text content */
  content: string;
}

/**
 * Options for listForEvent.
 */
export interface ListForEventBoostOptions extends PaginationOptions {
}

/**
 * Request parameters for createForEvent.
 */
export interface CreateForEventBoostRequest {
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
   * @param boostId - The boost ID
   * @returns The boost
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.boosts.get(123);
   * ```
   */
  async get(boostId: number): Promise<components["schemas"]["GetBoostResponseContent"]> {
    const response = await this.request(
      {
        service: "Boosts",
        operation: "GetBoost",
        resourceType: "boost",
        isMutation: false,
        resourceId: boostId,
      },
      () =>
        this.client.GET("/boosts/{boostId}", {
          params: {
            path: { boostId },
          },
        })
    );
    return response;
  }

  /**
   * Delete a boost
   * @param boostId - The boost ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.boosts.delete(123);
   * ```
   */
  async delete(boostId: number): Promise<void> {
    await this.request(
      {
        service: "Boosts",
        operation: "DeleteBoost",
        resourceType: "boost",
        isMutation: true,
        resourceId: boostId,
      },
      () =>
        this.client.DELETE("/boosts/{boostId}", {
          params: {
            path: { boostId },
          },
        })
    );
  }

  /**
   * List boosts on a recording
   * @param recordingId - The recording ID
   * @param options - Optional query parameters
   * @returns All results across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.boosts.listForRecording(123);
   * ```
   */
  async listForRecording(recordingId: number, options?: ListForRecordingBoostOptions): Promise<components["schemas"]["ListRecordingBoostsResponseContent"]> {
    return this.requestPaginated(
      {
        service: "Boosts",
        operation: "ListRecordingBoosts",
        resourceType: "recording_boost",
        isMutation: false,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/recordings/{recordingId}/boosts.json", {
          params: {
            path: { recordingId },
          },
        })
      , options
    );
  }

  /**
   * Create a boost on a recording
   * @param recordingId - The recording ID
   * @param req - Recording_boost creation parameters
   * @returns The recording_boost
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.boosts.createForRecording(123, { content: "Hello world" });
   * ```
   */
  async createForRecording(recordingId: number, req: CreateForRecordingBoostRequest): Promise<components["schemas"]["CreateRecordingBoostResponseContent"]> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    const response = await this.request(
      {
        service: "Boosts",
        operation: "CreateRecordingBoost",
        resourceType: "recording_boost",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/recordings/{recordingId}/boosts.json", {
          params: {
            path: { recordingId },
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
   * @param recordingId - The recording ID
   * @param eventId - The event ID
   * @param options - Optional query parameters
   * @returns All results across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.boosts.listForEvent(123, 123);
   * ```
   */
  async listForEvent(recordingId: number, eventId: number, options?: ListForEventBoostOptions): Promise<components["schemas"]["ListEventBoostsResponseContent"]> {
    return this.requestPaginated(
      {
        service: "Boosts",
        operation: "ListEventBoosts",
        resourceType: "event_boost",
        isMutation: false,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/recordings/{recordingId}/events/{eventId}/boosts.json", {
          params: {
            path: { recordingId, eventId },
          },
        })
      , options
    );
  }

  /**
   * Create a boost on a specific event within a recording
   * @param recordingId - The recording ID
   * @param eventId - The event ID
   * @param req - Event_boost creation parameters
   * @returns The event_boost
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.boosts.createForEvent(123, 123, { content: "Hello world" });
   * ```
   */
  async createForEvent(recordingId: number, eventId: number, req: CreateForEventBoostRequest): Promise<components["schemas"]["CreateEventBoostResponseContent"]> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    const response = await this.request(
      {
        service: "Boosts",
        operation: "CreateEventBoost",
        resourceType: "event_boost",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/recordings/{recordingId}/events/{eventId}/boosts.json", {
          params: {
            path: { recordingId, eventId },
          },
          body: {
            content: req.content,
          },
        })
    );
    return response;
  }
}