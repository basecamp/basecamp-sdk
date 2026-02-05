/**
 * Lineup service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Request parameters for create.
 */
export interface CreateLineupRequest {
  /** Display name */
  name: string;
  /** Date */
  date: string;
}

/**
 * Request parameters for update.
 */
export interface UpdateLineupRequest {
  /** Display name */
  name?: string;
  /** Date */
  date?: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Lineup operations.
 */
export class LineupService extends BaseService {

  /**
   * Create a new lineup marker
   * @param req - Lineup_marker creation parameters
   * @returns void
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * await client.lineup.create({ name: "My example", date: "example" });
   * ```
   */
  async create(req: CreateLineupRequest): Promise<void> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    if (!req.date) {
      throw Errors.validation("Date is required");
    }
    await this.request(
      {
        service: "Lineup",
        operation: "CreateLineupMarker",
        resourceType: "lineup_marker",
        isMutation: true,
      },
      () =>
        this.client.POST("/lineup/markers.json", {
          body: {
            name: req.name,
            date: req.date,
          },
        })
    );
  }

  /**
   * Update an existing lineup marker
   * @param markerId - The marker ID
   * @param req - Lineup_marker update parameters
   * @returns void
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * await client.lineup.update(123, { });
   * ```
   */
  async update(markerId: number, req: UpdateLineupRequest): Promise<void> {
    await this.request(
      {
        service: "Lineup",
        operation: "UpdateLineupMarker",
        resourceType: "lineup_marker",
        isMutation: true,
        resourceId: markerId,
      },
      () =>
        this.client.PUT("/lineup/markers/{markerId}", {
          params: {
            path: { markerId },
          },
          body: {
            name: req.name,
            date: req.date,
          },
        })
    );
  }

  /**
   * Delete a lineup marker
   * @param markerId - The marker ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.lineup.delete(123);
   * ```
   */
  async delete(markerId: number): Promise<void> {
    await this.request(
      {
        service: "Lineup",
        operation: "DeleteLineupMarker",
        resourceType: "lineup_marker",
        isMutation: true,
        resourceId: markerId,
      },
      () =>
        this.client.DELETE("/lineup/markers/{markerId}", {
          params: {
            path: { markerId },
          },
        })
    );
  }
}