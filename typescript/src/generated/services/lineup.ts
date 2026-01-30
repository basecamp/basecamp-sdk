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

/** LineupMarker entity from the Basecamp API. */
export type LineupMarker = components["schemas"]["LineupMarker"];

/**
 * Request parameters for create.
 */
export interface CreateLineupRequest {
  /** title */
  title: string;
  /** starts on (YYYY-MM-DD) */
  startsOn: string;
  /** ends on (YYYY-MM-DD) */
  endsOn: string;
  /** color */
  color?: string;
  /** description */
  description?: string;
}

/**
 * Request parameters for update.
 */
export interface UpdateLineupRequest {
  /** title */
  title?: string;
  /** starts on (YYYY-MM-DD) */
  startsOn?: string;
  /** ends on (YYYY-MM-DD) */
  endsOn?: string;
  /** color */
  color?: string;
  /** description */
  description?: string;
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
   * @returns The LineupMarker
   *
   * @example
   * ```ts
   * const result = await client.lineup.create({ ... });
   * ```
   */
  async create(req: CreateLineupRequest): Promise<LineupMarker> {
    const response = await this.request(
      {
        service: "Lineup",
        operation: "CreateLineupMarker",
        resourceType: "lineup_marker",
        isMutation: true,
      },
      () =>
        this.client.POST("/lineup/markers.json", {
          body: {
            title: req.title,
            starts_on: req.startsOn,
            ends_on: req.endsOn,
            color: req.color,
            description: req.description,
          },
        })
    );
    return response;
  }

  /**
   * Update an existing lineup marker
   * @param markerId - The marker ID
   * @param req - Request parameters
   * @returns The LineupMarker
   */
  async update(markerId: number, req: UpdateLineupRequest): Promise<LineupMarker> {
    const response = await this.request(
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
            title: req.title,
            starts_on: req.startsOn,
            ends_on: req.endsOn,
            color: req.color,
            description: req.description,
          },
        })
    );
    return response;
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