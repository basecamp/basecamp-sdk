/**
 * Cards service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Card entity from the Basecamp API. */
export type Card = components["schemas"]["Card"];

/**
 * Request parameters for update.
 */
export interface UpdateCardRequest {
  /** Title */
  title?: string;
  /** Text content */
  content?: string;
  /** Due date (YYYY-MM-DD) */
  dueOn?: string;
  /** Person IDs to assign to */
  assigneeIds?: number[];
}

/**
 * Request parameters for move.
 */
export interface MoveCardRequest {
  /** Column id */
  columnId: number;
}

/**
 * Options for list.
 */
export interface ListCardOptions extends PaginationOptions {
}

/**
 * Request parameters for create.
 */
export interface CreateCardRequest {
  /** Title */
  title: string;
  /** Text content */
  content?: string;
  /** Due date (YYYY-MM-DD) */
  dueOn?: string;
  /** Whether to send notifications to relevant people */
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
   * @param cardId - The card ID
   * @returns The Card
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.cards.get(123);
   * ```
   */
  async get(cardId: number): Promise<Card> {
    const response = await this.request(
      {
        service: "Cards",
        operation: "GetCard",
        resourceType: "card",
        isMutation: false,
        resourceId: cardId,
      },
      () =>
        this.client.GET("/card_tables/cards/{cardId}", {
          params: {
            path: { cardId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing card
   * @param cardId - The card ID
   * @param req - Card update parameters
   * @returns The Card
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.cards.update(123, { });
   * ```
   */
  async update(cardId: number, req: UpdateCardRequest): Promise<Card> {
    if (req.dueOn && !/^\d{4}-\d{2}-\d{2}$/.test(req.dueOn)) {
      throw Errors.validation("Due on must be in YYYY-MM-DD format");
    }
    const response = await this.request(
      {
        service: "Cards",
        operation: "UpdateCard",
        resourceType: "card",
        isMutation: true,
        resourceId: cardId,
      },
      () =>
        this.client.PUT("/card_tables/cards/{cardId}", {
          params: {
            path: { cardId },
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
   * @param cardId - The card ID
   * @param req - Card request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.cards.move(123, { columnId: 1 });
   * ```
   */
  async move(cardId: number, req: MoveCardRequest): Promise<void> {
    await this.request(
      {
        service: "Cards",
        operation: "MoveCard",
        resourceType: "card",
        isMutation: true,
        resourceId: cardId,
      },
      () =>
        this.client.POST("/card_tables/cards/{cardId}/moves.json", {
          params: {
            path: { cardId },
          },
          body: {
            column_id: req.columnId,
          },
        })
    );
  }

  /**
   * List cards in a column
   * @param columnId - The column ID
   * @param options - Optional query parameters
   * @returns All Card across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.cards.list(123);
   * ```
   */
  async list(columnId: number, options?: ListCardOptions): Promise<ListResult<Card>> {
    return this.requestPaginated(
      {
        service: "Cards",
        operation: "ListCards",
        resourceType: "card",
        isMutation: false,
        resourceId: columnId,
      },
      () =>
        this.client.GET("/card_tables/lists/{columnId}/cards.json", {
          params: {
            path: { columnId },
          },
        })
      , options
    );
  }

  /**
   * Create a card in a column
   * @param columnId - The column ID
   * @param req - Card creation parameters
   * @returns The Card
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.cards.create(123, { title: "example" });
   * ```
   */
  async create(columnId: number, req: CreateCardRequest): Promise<Card> {
    if (!req.title) {
      throw Errors.validation("Title is required");
    }
    if (req.dueOn && !/^\d{4}-\d{2}-\d{2}$/.test(req.dueOn)) {
      throw Errors.validation("Due on must be in YYYY-MM-DD format");
    }
    const response = await this.request(
      {
        service: "Cards",
        operation: "CreateCard",
        resourceType: "card",
        isMutation: true,
        resourceId: columnId,
      },
      () =>
        this.client.POST("/card_tables/lists/{columnId}/cards.json", {
          params: {
            path: { columnId },
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