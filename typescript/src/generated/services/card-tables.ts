/**
 * CardTables service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** CardTable entity from the Basecamp API. */
export type CardTable = components["schemas"]["CardTable"];

// =============================================================================
// Service
// =============================================================================

/**
 * Service for CardTables operations.
 */
export class CardTablesService extends BaseService {

  /**
   * Get a card table by ID
   * @param cardTableId - The card table ID
   * @returns The CardTable
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.cardTables.get(123);
   * ```
   */
  async get(cardTableId: number): Promise<CardTable> {
    const response = await this.request(
      {
        service: "CardTables",
        operation: "GetCardTable",
        resourceType: "card_table",
        isMutation: false,
        resourceId: cardTableId,
      },
      () =>
        this.client.GET("/card_tables/{cardTableId}", {
          params: {
            path: { cardTableId },
          },
        })
    );
    return response;
  }
}