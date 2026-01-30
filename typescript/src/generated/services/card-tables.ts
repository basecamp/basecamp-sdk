/**
 * Service for CardTables operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for CardTables operations
 */
export class CardTablesService extends BaseService {

  /**
   * Get a card table by ID
   */
  async get(projectId: number, cardTableId: number): Promise<components["schemas"]["GetCardTableResponseContent"]> {
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