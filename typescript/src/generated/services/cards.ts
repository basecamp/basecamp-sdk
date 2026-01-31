/**
 * Cards service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Card entity from the Basecamp API. */
export type Card = components["schemas"]["Card"];

/**
 * Request parameters for update.
 */
export interface UpdateCardRequest {
  /** title */
  title?: string;
  /** content */
  content?: string;
  /** due on (YYYY-MM-DD) */
  dueOn?: string;
  /** assignee ids */
  assigneeIds?: number[];
}

/**
 * Request parameters for move.
 */
export interface MoveCardRequest {
  /** column id */
  columnId: number;
}

/**
 * Request parameters for create.
 */
export interface CreateCardRequest {
  /** title */
  title: string;
  /** content */
  content?: string;
  /** due on (YYYY-MM-DD) */
  dueOn?: string;
  /** notify */
  notify?: boolean;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Cards operations.
 */
export class CardsService extends BaseService {

  /**
   * Get a card by ID
   * @param projectId - The project ID
   * @param cardId - The card ID
   * @returns The Card
   */
  async get(projectId: number, cardId: number): Promise<Card> {
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
   * @param projectId - The project ID
   * @param cardId - The card ID
   * @param req - Request parameters
   * @returns The Card
   */
  async update(projectId: number, cardId: number, req: UpdateCardRequest): Promise<Card> {
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
          body: {
            title: req.title,
            content: req.content,
            due_on: req.dueOn,
            assignee_ids: req.assigneeIds,
          },
        })
    );
    return response;
  }

  /**
   * Move a card to a different column
   * @param projectId - The project ID
   * @param cardId - The card ID
   * @param req - Request parameters
   * @returns void
   */
  async move(projectId: number, cardId: number, req: MoveCardRequest): Promise<void> {
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
          body: {
            column_id: req.columnId,
          },
        })
    );
  }

  /**
   * List cards in a column
   * @param projectId - The project ID
   * @param columnId - The column ID
   * @returns Array of Card
   */
  async list(projectId: number, columnId: number): Promise<Card[]> {
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
   * @param projectId - The project ID
   * @param columnId - The column ID
   * @param req - Request parameters
   * @returns The Card
   *
   * @example
   * ```ts
   * const result = await client.cards.create(123, 123, { ... });
   * ```
   */
  async create(projectId: number, columnId: number, req: CreateCardRequest): Promise<Card> {
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
          body: {
            title: req.title,
            content: req.content,
            due_on: req.dueOn,
            notify: req.notify,
          },
        })
    );
    return response;
  }
}