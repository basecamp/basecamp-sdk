/**
 * Lineup service for the Basecamp API.
 *
 * The Lineup is Basecamp's visual timeline tool for tracking
 * project schedules and milestones.
 *
 * Note: This hand-written service is NOT loaded at runtime.
 * Generated services in src/generated/services/ are the runtime default.
 *
 * @example
 * ```ts
 * await client.lineup.create({
 *   name: "Launch Day",
 *   date: "2024-03-01",
 * });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";


// =============================================================================
// Types
// =============================================================================

/**
 * Request to create a new lineup marker.
 */
export interface CreateMarkerRequest {
  /** Marker name (required) */
  name: string;
  /** Date in YYYY-MM-DD format (required) */
  date: string;
}

/**
 * Request to update an existing lineup marker.
 */
export interface UpdateMarkerRequest {
  /** Marker name (optional) */
  name?: string;
  /** Date in YYYY-MM-DD format (optional) */
  date?: string;
}

// =============================================================================
// Helpers
// =============================================================================

/**
 * Validates that a string is in YYYY-MM-DD format.
 */
function isValidDateFormat(date: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(date);
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp Lineup markers.
 */
export class LineupService extends BaseService {
  /**
   * Creates a new marker on the lineup.
   *
   * @param req - Marker creation parameters
   * @throws BasecampError with code "validation" if required fields are missing
   *
   * @example
   * ```ts
   * await client.lineup.createMarker({
   *   name: "Product Launch",
   *   date: "2024-03-01",
   * });
   * ```
   */
  async createMarker(req: CreateMarkerRequest): Promise<void> {
    if (!req.name) {
      throw Errors.validation("Marker name is required");
    }
    if (!req.date) {
      throw Errors.validation("Marker date is required");
    }
    if (!isValidDateFormat(req.date)) {
      throw Errors.validation("Marker date must be in YYYY-MM-DD format");
    }

    await this.request(
      {
        service: "Lineup",
        operation: "CreateMarker",
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
   * Updates an existing marker.
   *
   * @param markerId - The marker ID
   * @param req - Marker update parameters
   *
   * @example
   * ```ts
   * await client.lineup.updateMarker(markerId, {
   *   name: "Updated Launch Date",
   *   date: "2024-03-20",
   * });
   * ```
   */
  async updateMarker(markerId: number, req: UpdateMarkerRequest): Promise<void> {
    if (req.date && !isValidDateFormat(req.date)) {
      throw Errors.validation("Marker date must be in YYYY-MM-DD format");
    }

    await this.request(
      {
        service: "Lineup",
        operation: "UpdateMarker",
        resourceType: "lineup_marker",
        isMutation: true,
        resourceId: markerId,
      },
      () =>
        this.client.PUT("/lineup/markers/{markerId}", {
          params: { path: { markerId } },
          body: {
            name: req.name,
            date: req.date,
          },
        })
    );
  }

  /**
   * Deletes a marker.
   *
   * @param markerId - The marker ID
   *
   * @example
   * ```ts
   * await client.lineup.deleteMarker(markerId);
   * ```
   */
  async deleteMarker(markerId: number): Promise<void> {
    await this.request(
      {
        service: "Lineup",
        operation: "DeleteMarker",
        resourceType: "lineup_marker",
        isMutation: true,
        resourceId: markerId,
      },
      () =>
        this.client.DELETE("/lineup/markers/{markerId}", {
          params: { path: { markerId } },
        })
    );
  }
}
