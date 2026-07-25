/**
 * Wormholes service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Wormhole entity from the Basecamp API. */
export type Wormhole = components["schemas"]["Wormhole"];

/**
 * Request parameters for update.
 */
export interface UpdateWormholeRequest {
  /** Id of the new destination column (on another accessible card table). */
  destinationRecordingId: number;
}

/**
 * Request parameters for create.
 */
export interface CreateWormholeRequest {
  /** Id of the destination column (on another accessible card table) to link to. */
  destinationRecordingId: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Wormholes operations.
 */
export class WormholesService extends BaseService {

  /**
   * Update a wormhole's destination column
   * @param bucketId - The bucket ID
   * @param wormholeId - The wormhole ID
   * @param req - Wormhole update parameters
   * @returns The Wormhole
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.wormholes.update(123, 123, { destinationRecordingId: 1 });
   * ```
   */
  async update(bucketId: number, wormholeId: number, req: UpdateWormholeRequest): Promise<Wormhole> {
    const response = await this.request(
      {
        service: "Wormholes",
        operation: "UpdateWormhole",
        resourceType: "wormhole",
        isMutation: true,
        resourceId: wormholeId,
      },
      () =>
        this.client.PUT("/buckets/{bucketId}/card_tables/wormholes/{wormholeId}", {
          params: {
            path: { bucketId, wormholeId },
          },
          body: {
            destination_recording_id: req.destinationRecordingId,
          },
        })
    );
    return response;
  }

  /**
   * Delete a wormhole
   * @param bucketId - The bucket ID
   * @param wormholeId - The wormhole ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.wormholes.delete(123, 123);
   * ```
   */
  async delete(bucketId: number, wormholeId: number): Promise<void> {
    await this.request(
      {
        service: "Wormholes",
        operation: "DeleteWormhole",
        resourceType: "wormhole",
        isMutation: true,
        resourceId: wormholeId,
      },
      () =>
        this.client.DELETE("/buckets/{bucketId}/card_tables/wormholes/{wormholeId}", {
          params: {
            path: { bucketId, wormholeId },
          },
        })
    );
  }

  /**
   * Create a wormhole linking this card table to a column on another card table.
   * @param bucketId - The bucket ID
   * @param cardTableId - The card table ID
   * @param req - Wormhole creation parameters
   * @returns The Wormhole
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.wormholes.create(123, 123, { destinationRecordingId: 1 });
   * ```
   */
  async create(bucketId: number, cardTableId: number, req: CreateWormholeRequest): Promise<Wormhole> {
    const response = await this.request(
      {
        service: "Wormholes",
        operation: "CreateWormhole",
        resourceType: "wormhole",
        isMutation: true,
        resourceId: cardTableId,
      },
      () =>
        this.client.POST("/buckets/{bucketId}/card_tables/{cardTableId}/wormholes.json", {
          params: {
            path: { bucketId, cardTableId },
          },
          body: {
            destination_recording_id: req.destinationRecordingId,
          },
        })
    );
    return response;
  }
}