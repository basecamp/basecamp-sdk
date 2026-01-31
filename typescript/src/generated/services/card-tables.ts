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
   * @param projectId - The project ID
   * @param cardTableId - The card table ID
   * @returns The CardTable
   */
  async get(projectId: number, cardTableId: number): Promise<CardTable> {
    const response = await this.request(
      {
        service: "CardTables",
        operation: "GetCardTable",
        resourceType: "card_table",
        isMutation: false,
        projectId,
        resourceId: cardTableId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/card_tables/{cardTableId}", {
          params: {
            path: { projectId, cardTableId },
          },
        })
    );
    return response;
  }
}