/**
 * Service for CardColumns operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for CardColumns operations
 */
export class CardColumnsService extends BaseService {

  /**
   * Get a card column by ID
   */
  async get(projectId: number, columnId: number): Promise<components["schemas"]["GetCardColumnResponseContent"]> {
    const response = await this.request(
      {
        service: "CardColumns",
        operation: "GetCardColumn",
        resourceType: "card_column",
        isMutation: false,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/card_tables/columns/{columnId}", {
          params: {
            path: { projectId, columnId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing column
   */
  async update(projectId: number, columnId: number, req: components["schemas"]["UpdateCardColumnRequestContent"]): Promise<components["schemas"]["UpdateCardColumnResponseContent"]> {
    const response = await this.request(
      {
        service: "CardColumns",
        operation: "UpdateCardColumn",
        resourceType: "card_column",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/card_tables/columns/{columnId}", {
          params: {
            path: { projectId, columnId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Set the color of a column
   */
  async setColor(projectId: number, columnId: number, req: components["schemas"]["SetCardColumnColorRequestContent"]): Promise<components["schemas"]["SetCardColumnColorResponseContent"]> {
    const response = await this.request(
      {
        service: "CardColumns",
        operation: "SetCardColumnColor",
        resourceType: "card_column_color",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/card_tables/columns/{columnId}/color.json", {
          params: {
            path: { projectId, columnId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Enable on-hold section in a column
   */
  async enableOnHold(projectId: number, columnId: number): Promise<components["schemas"]["EnableCardColumnOnHoldResponseContent"]> {
    const response = await this.request(
      {
        service: "CardColumns",
        operation: "EnableCardColumnOnHold",
        resourceType: "card_column_on_hold",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json", {
          params: {
            path: { projectId, columnId },
          },
        })
    );
    return response;
  }

  /**
   * Disable on-hold section in a column
   */
  async disableOnHold(projectId: number, columnId: number): Promise<components["schemas"]["DisableCardColumnOnHoldResponseContent"]> {
    const response = await this.request(
      {
        service: "CardColumns",
        operation: "DisableCardColumnOnHold",
        resourceType: "card_column_on_hold",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json", {
          params: {
            path: { projectId, columnId },
          },
        })
    );
    return response;
  }

  /**
   * Subscribe to a card column (watch for changes)
   */
  async subscribeToColumn(projectId: number, columnId: number): Promise<void> {
    await this.request(
      {
        service: "CardColumns",
        operation: "SubscribeToCardColumn",
        resourceType: "to_card_column",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/lists/{columnId}/subscription.json", {
          params: {
            path: { projectId, columnId },
          },
        })
    );
  }

  /**
   * Unsubscribe from a card column (stop watching for changes)
   */
  async unsubscribeFromColumn(projectId: number, columnId: number): Promise<void> {
    await this.request(
      {
        service: "CardColumns",
        operation: "UnsubscribeFromCardColumn",
        resourceType: "from_card_column",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/card_tables/lists/{columnId}/subscription.json", {
          params: {
            path: { projectId, columnId },
          },
        })
    );
  }

  /**
   * Create a column in a card table
   */
  async create(projectId: number, cardTableId: number, req: components["schemas"]["CreateCardColumnRequestContent"]): Promise<components["schemas"]["CreateCardColumnResponseContent"]> {
    const response = await this.request(
      {
        service: "CardColumns",
        operation: "CreateCardColumn",
        resourceType: "card_column",
        isMutation: true,
        projectId,
        resourceId: cardTableId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/{cardTableId}/columns.json", {
          params: {
            path: { projectId, cardTableId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Move a column within a card table
   */
  async move(projectId: number, cardTableId: number, req: components["schemas"]["MoveCardColumnRequestContent"]): Promise<void> {
    await this.request(
      {
        service: "CardColumns",
        operation: "MoveCardColumn",
        resourceType: "card_column",
        isMutation: true,
        projectId,
        resourceId: cardTableId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/{cardTableId}/moves.json", {
          params: {
            path: { projectId, cardTableId },
          },
          body: req,
        })
    );
  }
}