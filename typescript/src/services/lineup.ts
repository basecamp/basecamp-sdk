/**
 * Lineup service for the Basecamp API.
 *
 * The Lineup is Basecamp's visual timeline tool for tracking
 * project schedules and milestones.
 *
 * @example
 * ```ts
 * const marker = await client.lineup.createMarker({
 *   title: "Launch Day",
 *   startsOn: "2024-03-01",
 *   endsOn: "2024-03-01",
 *   color: "green",
 * });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A marker on the Basecamp Lineup.
 */
export type LineupMarker = components["schemas"]["LineupMarker"];

/**
 * Valid colors for lineup markers.
 */
export type MarkerColor =
  | "white"
  | "red"
  | "orange"
  | "yellow"
  | "green"
  | "blue"
  | "aqua"
  | "purple"
  | "gray"
  | "pink"
  | "brown";

/**
 * Request to create a new lineup marker.
 */
export interface CreateMarkerRequest {
  /** Marker title (required) */
  title: string;
  /** Start date in YYYY-MM-DD format (required) */
  startsOn: string;
  /** End date in YYYY-MM-DD format (required) */
  endsOn: string;
  /** Marker color (optional) */
  color?: MarkerColor;
  /** Description in HTML (optional) */
  description?: string;
}

/**
 * Request to update an existing lineup marker.
 */
export interface UpdateMarkerRequest {
  /** Marker title (optional) */
  title?: string;
  /** Start date in YYYY-MM-DD format (optional) */
  startsOn?: string;
  /** End date in YYYY-MM-DD format (optional) */
  endsOn?: string;
  /** Marker color (optional) */
  color?: MarkerColor;
  /** Description in HTML (optional) */
  description?: string;
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
   * @returns The created marker
   * @throws BasecampError with code "validation" if required fields are missing
   *
   * @example
   * ```ts
   * const marker = await client.lineup.createMarker({
   *   title: "Product Launch",
   *   startsOn: "2024-03-01",
   *   endsOn: "2024-03-15",
   *   color: "green",
   *   description: "<p>Major product release</p>",
   * });
   * ```
   */
  async createMarker(req: CreateMarkerRequest): Promise<LineupMarker> {
    if (!req.title) {
      throw Errors.validation("Marker title is required");
    }
    if (!req.startsOn) {
      throw Errors.validation("Marker starts_on date is required");
    }
    if (!req.endsOn) {
      throw Errors.validation("Marker ends_on date is required");
    }
    if (!isValidDateFormat(req.startsOn)) {
      throw Errors.validation("Marker starts_on must be in YYYY-MM-DD format");
    }
    if (!isValidDateFormat(req.endsOn)) {
      throw Errors.validation("Marker ends_on must be in YYYY-MM-DD format");
    }

    const response = await this.request(
      {
        service: "Lineup",
        operation: "CreateMarker",
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
   * Updates an existing marker.
   *
   * @param markerId - The marker ID
   * @param req - Marker update parameters
   * @returns The updated marker
   *
   * @example
   * ```ts
   * const marker = await client.lineup.updateMarker(markerId, {
   *   title: "Updated Launch Date",
   *   endsOn: "2024-03-20",
   *   color: "blue",
   * });
   * ```
   */
  async updateMarker(markerId: number, req: UpdateMarkerRequest): Promise<LineupMarker> {
    if (req.startsOn && !isValidDateFormat(req.startsOn)) {
      throw Errors.validation("Marker starts_on must be in YYYY-MM-DD format");
    }
    if (req.endsOn && !isValidDateFormat(req.endsOn)) {
      throw Errors.validation("Marker ends_on must be in YYYY-MM-DD format");
    }

    const response = await this.request(
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
