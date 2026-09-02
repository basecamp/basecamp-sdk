/**
 * BubbleUps service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Request parameters for createBubbleUp.
 */
export interface CreateBubbleUpBubbleUpRequest {
  /** Timing for the bubble-up. `"now"` bubbles up immediately; a scheduling
keyword (`"today"`, `"tomorrow"`, `"weekend"`, `"next_week"`) or an ISO8601
date (e.g. `"2026-09-10"`) schedules it to resurface later. bc3 requires a
value — omitting `at` errors server-side (`Date.iso8601(nil)`) — so send
`"now"` for the immediate case. */
  at?: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for BubbleUps operations.
 */
export class BubbleUpsService extends BaseService {

  /**
   * Bubble up a recording for the current user, resurfacing it in the current
   * @param recordingId - The recording ID
   * @param req - Bubble_up creation parameters
   * @returns void
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * await client.bubbleUps.createBubbleUp(123, { });
   * ```
   */
  async createBubbleUp(recordingId: number, req: CreateBubbleUpBubbleUpRequest): Promise<void> {
    await this.request(
      {
        service: "BubbleUps",
        operation: "CreateBubbleUp",
        resourceType: "bubble_up",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/recordings/{recordingId}/bubble_up.json", {
          params: {
            path: { recordingId },
          },
          body: {
            at: req.at,
          },
        })
    );
  }

  /**
   * Remove the current user's bubble-up from a recording.
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.bubbleUps.deleteBubbleUp(123);
   * ```
   */
  async deleteBubbleUp(recordingId: number): Promise<void> {
    await this.request(
      {
        service: "BubbleUps",
        operation: "DeleteBubbleUp",
        resourceType: "bubble_up",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.DELETE("/recordings/{recordingId}/bubble_up.json", {
          params: {
            path: { recordingId },
          },
        })
    );
  }
}