/**
 * Lineup service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Request parameters for create.
 */
export interface CreateLineupRequest {
  /** name */
  name: string;
  /** date */
  date: string;
}

/**
 * Request parameters for update.
 */
export interface UpdateLineupRequest {
  /** name */
  name?: string;
  /** date */
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
   * @param req - Request parameters
   * @returns void
   *
   * @example
   * ```ts
   * const result = await client.lineup.create({ ... });
   * ```
   */
  async create(req: CreateLineupRequest): Promise<void> {
    await this.request(
      {
        service: "Lineup",
        operation: "CreateLineupMarker",
        resourceType: "lineup_marker",
        isMutation: true,
      },
      () =>
        this.client.POST("/lineup/markers.json", {
          body: req as any,
        })
    );
  }

  /**
   * Update an existing lineup marker
   * @param markerId - The marker ID
   * @param req - Request parameters
   * @returns void
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
          body: req as any,
        })
    );
  }

  /**
   * Delete a lineup marker
   * @param markerId - The marker ID
   * @returns void
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