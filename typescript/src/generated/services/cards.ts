/**
 * Service for Cards operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Cards operations
 */
export class CardsService extends BaseService {

  /**
   * Get a card by ID
   */
  async get(projectId: number, cardId: number): Promise<components["schemas"]["GetCardResponseContent"]> {
    const response = await this.request(
      {
        service: "Cards",
        operation: "GetCard",
        resourceType: "card",
        isMutation: false,
        projectId,
        resourceId: cardId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/card_tables/cards/{cardId}", {
          params: {
            path: { projectId, cardId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing card
   */
  async update(projectId: number, cardId: number, req: components["schemas"]["UpdateCardRequestContent"]): Promise<components["schemas"]["UpdateCardResponseContent"]> {
    const response = await this.request(
      {
        service: "Cards",
        operation: "UpdateCard",
        resourceType: "card",
        isMutation: true,
        projectId,
        resourceId: cardId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/card_tables/cards/{cardId}", {
          params: {
            path: { projectId, cardId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Move a card to a different column
   */
  async move(projectId: number, cardId: number, req: components["schemas"]["MoveCardRequestContent"]): Promise<void> {
    await this.request(
      {
        service: "Cards",
        operation: "MoveCard",
        resourceType: "card",
        isMutation: true,
        projectId,
        resourceId: cardId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/cards/{cardId}/moves.json", {
          params: {
            path: { projectId, cardId },
          },
          body: req,
        })
    );
  }

  /**
   * List cards in a column
   */
  async list(projectId: number, columnId: number): Promise<components["schemas"]["ListCardsResponseContent"]> {
    const response = await this.request(
      {
        service: "Cards",
        operation: "ListCards",
        resourceType: "card",
        isMutation: false,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/card_tables/lists/{columnId}/cards.json", {
          params: {
            path: { projectId, columnId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a card in a column
   */
  async create(projectId: number, columnId: number, req: components["schemas"]["CreateCardRequestContent"]): Promise<components["schemas"]["CreateCardResponseContent"]> {
    const response = await this.request(
      {
        service: "Cards",
        operation: "CreateCard",
        resourceType: "card",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/lists/{columnId}/cards.json", {
          params: {
            path: { projectId, columnId },
          },
          body: req,
        })
    );
    return response;
  }
}