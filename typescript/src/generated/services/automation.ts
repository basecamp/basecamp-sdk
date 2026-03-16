/**
 * Automation service for the Basecamp API.
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

// =============================================================================
// Service
// =============================================================================

/**
 * Service for Automation operations.
 */
export class AutomationService extends BaseService {

  /**
   * List all lineup markers for the account
   * @returns Array of LineupMarker
   *
   * @example
   * ```ts
   * const result = await client.automation.listLineupMarkers();
   * ```
   */
  async listLineupMarkers(): Promise<LineupMarker[]> {
    const response = await this.request(
      {
        service: "Automation",
        operation: "ListLineupMarkers",
        resourceType: "lineup_marker",
        isMutation: false,
      },
      () =>
        this.client.GET("/lineup/markers.json", {
        })
    );
    return response ?? [];
  }
}