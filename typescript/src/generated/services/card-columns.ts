/**
 * CardColumns service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** CardColumn entity from the Basecamp API. */
export type CardColumn = components["schemas"]["CardColumn"];

/**
 * Request parameters for update.
 */
export interface UpdateCardColumnRequest {
  /** Title */
  title?: string;
  /** Rich text description (HTML) */
  description?: string;
}

/**
 * Request parameters for setColor.
 */
export interface SetColorCardColumnRequest {
  /** Valid colors: white, red, orange, yellow, green, blue, aqua, purple, gray, pink, brown */
  color: string;
}

/**
 * Request parameters for create.
 */
export interface CreateCardColumnRequest {
  /** Title */
  title: string;
  /** Rich text description (HTML) */
  description?: string;
}

/**
 * Request parameters for move.
 */
export interface MoveCardColumnRequest {
  /** Source id */
  sourceId: number;
  /** Target id */
  targetId: number;
  /** Position for ordering (1-based) */
  position?: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for CardColumns operations.
 */
export class CardColumnsService extends BaseService {

  /**
   * Get a card column by ID
   * @param projectId - The project ID
   * @param columnId - The column ID
   * @returns The CardColumn
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.cardColumns.get(123, 123);
   * ```
   */
  async get(projectId: number, columnId: number): Promise<CardColumn> {
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
   * @param projectId - The project ID
   * @param columnId - The column ID
   * @param req - Card_column update parameters
   * @returns The CardColumn
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.cardColumns.update(123, 123, { });
   * ```
   */
  async update(projectId: number, columnId: number, req: UpdateCardColumnRequest): Promise<CardColumn> {
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
          body: {
            title: req.title,
            description: req.description,
          },
        })
    );
    return response;
  }

  /**
   * Set the color of a column
   * @param projectId - The project ID
   * @param columnId - The column ID
   * @param req - Card_column_color request parameters
   * @returns The CardColumn
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.cardColumns.setColor(123, 123, { color: "example" });
   * ```
   */
  async setColor(projectId: number, columnId: number, req: SetColorCardColumnRequest): Promise<CardColumn> {
    if (!req.color) {
      throw Errors.validation("Color is required");
    }
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
          body: {
            color: req.color,
          },
        })
    );
    return response;
  }

  /**
   * Enable on-hold section in a column
   * @param projectId - The project ID
   * @param columnId - The column ID
   * @returns The CardColumn
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.cardColumns.enableOnHold(123, 123);
   * ```
   */
  async enableOnHold(projectId: number, columnId: number): Promise<CardColumn> {
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
   * @param projectId - The project ID
   * @param columnId - The column ID
   * @returns The CardColumn
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.cardColumns.disableOnHold(123, 123);
   * ```
   */
  async disableOnHold(projectId: number, columnId: number): Promise<CardColumn> {
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
   * @param projectId - The project ID
   * @param columnId - The column ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.cardColumns.subscribeToColumn(123, 123);
   * ```
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
   * @param projectId - The project ID
   * @param columnId - The column ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.cardColumns.unsubscribeFromColumn(123, 123);
   * ```
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
   * @param projectId - The project ID
   * @param cardTableId - The card table ID
   * @param req - Card_column creation parameters
   * @returns The CardColumn
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.cardColumns.create(123, 123, { title: "example" });
   * ```
   */
  async create(projectId: number, cardTableId: number, req: CreateCardColumnRequest): Promise<CardColumn> {
    if (!req.title) {
      throw Errors.validation("Title is required");
    }
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
          body: {
            title: req.title,
            description: req.description,
          },
        })
    );
    return response;
  }

  /**
   * Move a column within a card table
   * @param projectId - The project ID
   * @param cardTableId - The card table ID
   * @param req - Card_column request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.cardColumns.move(123, 123, { sourceId: 1, targetId: 1 });
   * ```
   */
  async move(projectId: number, cardTableId: number, req: MoveCardColumnRequest): Promise<void> {
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
          body: {
            source_id: req.sourceId,
            target_id: req.targetId,
            position: req.position,
          },
        })
    );
  }
}