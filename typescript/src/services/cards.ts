/**
 * Cards services for the Basecamp API.
 *
 * Card Tables are kanban-style boards with columns containing cards.
 * Cards can have steps (checklist items), assignees, and due dates.
 *
 * This module exports:
 * - CardTablesService: Get card tables (kanban boards)
 * - CardsService: CRUD operations on cards
 * - CardColumnsService: Column management
 * - CardStepsService: Step (checklist) management
 *
 * @example
 * ```ts
 * const table = await client.cardTables.get(projectId, tableId);
 * const cards = await client.cards.list(projectId, columnId);
 * await client.cards.move(projectId, cardId, newColumnId);
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp card table (kanban board).
 */
export type CardTable = components["schemas"]["CardTable"];

/**
 * A column in a card table.
 */
export type CardColumn = components["schemas"]["CardColumn"];

/**
 * A card in a card table column.
 */
export type Card = components["schemas"]["Card"];

/**
 * A step (checklist item) on a card.
 */
export type CardStep = components["schemas"]["CardStep"];

/**
 * Request to create a new card.
 */
export interface CreateCardRequest {
  /** Card title (required) */
  title: string;
  /** Card body in HTML (optional) */
  content?: string;
  /** Due date in ISO 8601 format (YYYY-MM-DD) (optional) */
  dueOn?: string;
  /** Notify assignees when true (optional) */
  notify?: boolean;
}

/**
 * Request to update an existing card.
 */
export interface UpdateCardRequest {
  /** Card title (optional) */
  title?: string;
  /** Card body in HTML (optional) */
  content?: string;
  /** Due date in ISO 8601 format (YYYY-MM-DD) (optional) */
  dueOn?: string;
  /** Person IDs to assign this card to (optional) */
  assigneeIds?: number[];
}

/**
 * Request to create a new column.
 */
export interface CreateColumnRequest {
  /** Column title (required) */
  title: string;
  /** Column description (optional) */
  description?: string;
}

/**
 * Request to update an existing column.
 */
export interface UpdateColumnRequest {
  /** Column title (optional) */
  title?: string;
  /** Column description (optional) */
  description?: string;
}

/**
 * Request to move a column within a card table.
 */
export interface MoveColumnRequest {
  /** Column ID to move (required) */
  sourceId: number;
  /** Column ID to move relative to (required) */
  targetId: number;
  /** Position relative to target (optional) */
  position?: number;
}

/**
 * Valid column colors.
 */
export type ColumnColor =
  | "white"
  | "red"
  | "orange"
  | "yellow"
  | "green"
  | "blue"
  | "aqua"
  | "purple"
  | "gray"
  | "pink"
  | "brown";

/**
 * Request to create a new step.
 */
export interface CreateStepRequest {
  /** Step title (required) */
  title: string;
  /** Due date in ISO 8601 format (YYYY-MM-DD) (optional) */
  dueOn?: string;
  /** Person IDs to assign this step to (optional) */
  assignees?: number[];
}

/**
 * Request to update an existing step.
 */
export interface UpdateStepRequest {
  /** Step title (optional) */
  title?: string;
  /** Due date in ISO 8601 format (YYYY-MM-DD) (optional) */
  dueOn?: string;
  /** Person IDs to assign this step to (optional) */
  assignees?: number[];
}

// =============================================================================
// Card Tables Service
// =============================================================================

/**
 * Service for managing Basecamp card tables (kanban boards).
 */
export class CardTablesService extends BaseService {
  /**
   * Gets a card table by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param cardTableId - The card table ID
   * @returns The card table with its columns
   * @throws BasecampError with code "not_found" if card table doesn't exist
   *
   * @example
   * ```ts
   * const table = await client.cardTables.get(projectId, tableId);
   * console.log(table.title, table.lists.length);
   * ```
   */
  async get(projectId: number, cardTableId: number): Promise<CardTable> {
    const response = await this.request(
      {
        service: "CardTables",
        operation: "Get",
        resourceType: "card_table",
        isMutation: false,
        projectId,
        resourceId: cardTableId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/card_tables/{cardTableId}", {
          params: { path: { projectId, cardTableId } },
        })
    );

    return response;
  }
}

// =============================================================================
// Cards Service
// =============================================================================

/**
 * Service for managing Basecamp cards.
 */
export class CardsService extends BaseService {
  /**
   * Lists all cards in a column.
   *
   * @param projectId - The project (bucket) ID
   * @param columnId - The column ID
   * @returns Array of cards
   *
   * @example
   * ```ts
   * const cards = await client.cards.list(projectId, columnId);
   * ```
   */
  async list(projectId: number, columnId: number): Promise<Card[]> {
    const response = await this.request(
      {
        service: "Cards",
        operation: "List",
        resourceType: "card",
        isMutation: false,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/card_tables/lists/{columnId}/cards.json", {
          params: { path: { projectId, columnId } },
        })
    );

    return response ?? [];
  }

  /**
   * Gets a card by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param cardId - The card ID
   * @returns The card
   * @throws BasecampError with code "not_found" if card doesn't exist
   *
   * @example
   * ```ts
   * const card = await client.cards.get(projectId, cardId);
   * console.log(card.title, card.completed);
   * ```
   */
  async get(projectId: number, cardId: number): Promise<Card> {
    const response = await this.request(
      {
        service: "Cards",
        operation: "Get",
        resourceType: "card",
        isMutation: false,
        projectId,
        resourceId: cardId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/card_tables/cards/{cardId}", {
          params: { path: { projectId, cardId } },
        })
    );

    return response;
  }

  /**
   * Creates a new card in a column.
   *
   * @param projectId - The project (bucket) ID
   * @param columnId - The column ID
   * @param req - Card creation parameters
   * @returns The created card
   * @throws BasecampError with code "validation" if title is missing
   *
   * @example
   * ```ts
   * const card = await client.cards.create(projectId, columnId, {
   *   title: "New Feature",
   *   content: "<p>Feature description</p>",
   *   dueOn: "2024-12-31",
   * });
   * ```
   */
  async create(projectId: number, columnId: number, req: CreateCardRequest): Promise<Card> {
    if (!req.title) {
      throw Errors.validation("Card title is required");
    }

    if (req.dueOn && !isValidDateFormat(req.dueOn)) {
      throw Errors.validation("Card due_on must be in YYYY-MM-DD format");
    }

    const response = await this.request(
      {
        service: "Cards",
        operation: "Create",
        resourceType: "card",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/lists/{columnId}/cards.json", {
          params: { path: { projectId, columnId } },
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

  /**
   * Updates an existing card.
   *
   * @param projectId - The project (bucket) ID
   * @param cardId - The card ID
   * @param req - Card update parameters
   * @returns The updated card
   *
   * @example
   * ```ts
   * const card = await client.cards.update(projectId, cardId, {
   *   title: "Updated Title",
   *   assigneeIds: [1234, 5678],
   * });
   * ```
   */
  async update(projectId: number, cardId: number, req: UpdateCardRequest): Promise<Card> {
    if (req.dueOn && !isValidDateFormat(req.dueOn)) {
      throw Errors.validation("Card due_on must be in YYYY-MM-DD format");
    }

    const response = await this.request(
      {
        service: "Cards",
        operation: "Update",
        resourceType: "card",
        isMutation: true,
        projectId,
        resourceId: cardId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/card_tables/cards/{cardId}", {
          params: { path: { projectId, cardId } },
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
   * Moves a card to a different column.
   *
   * @param projectId - The project (bucket) ID
   * @param cardId - The card ID
   * @param columnId - The destination column ID
   *
   * @example
   * ```ts
   * // Move card to "In Progress" column
   * await client.cards.move(projectId, cardId, inProgressColumnId);
   * ```
   */
  async move(projectId: number, cardId: number, columnId: number): Promise<void> {
    await this.request(
      {
        service: "Cards",
        operation: "Move",
        resourceType: "card",
        isMutation: true,
        projectId,
        resourceId: cardId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/cards/{cardId}/moves.json", {
          params: { path: { projectId, cardId } },
          body: { column_id: columnId },
        })
    );
  }

  /**
   * Moves a card to the trash.
   * Trashed cards can be recovered from the trash.
   *
   * @param projectId - The project (bucket) ID
   * @param cardId - The card ID
   *
   * @example
   * ```ts
   * await client.cards.trash(projectId, cardId);
   * ```
   */
  async trash(projectId: number, cardId: number): Promise<void> {
    await this.request(
      {
        service: "Cards",
        operation: "Trash",
        resourceType: "card",
        isMutation: true,
        projectId,
        resourceId: cardId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: { path: { projectId, recordingId: cardId } },
        })
    );
  }
}

// =============================================================================
// Card Columns Service
// =============================================================================

/**
 * Service for managing Basecamp card columns.
 */
export class CardColumnsService extends BaseService {
  /**
   * Gets a column by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param columnId - The column ID
   * @returns The column
   * @throws BasecampError with code "not_found" if column doesn't exist
   *
   * @example
   * ```ts
   * const column = await client.cardColumns.get(projectId, columnId);
   * console.log(column.title, column.cards_count);
   * ```
   */
  async get(projectId: number, columnId: number): Promise<CardColumn> {
    const response = await this.request(
      {
        service: "CardColumns",
        operation: "Get",
        resourceType: "card_column",
        isMutation: false,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/card_tables/columns/{columnId}", {
          params: { path: { projectId, columnId } },
        })
    );

    return response;
  }

  /**
   * Creates a new column in a card table.
   *
   * @param projectId - The project (bucket) ID
   * @param cardTableId - The card table ID
   * @param req - Column creation parameters
   * @returns The created column
   * @throws BasecampError with code "validation" if title is missing
   *
   * @example
   * ```ts
   * const column = await client.cardColumns.create(projectId, tableId, {
   *   title: "In Review",
   *   description: "Cards awaiting review",
   * });
   * ```
   */
  async create(
    projectId: number,
    cardTableId: number,
    req: CreateColumnRequest
  ): Promise<CardColumn> {
    if (!req.title) {
      throw Errors.validation("Column title is required");
    }

    const response = await this.request(
      {
        service: "CardColumns",
        operation: "Create",
        resourceType: "card_column",
        isMutation: true,
        projectId,
        resourceId: cardTableId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/{cardTableId}/columns.json", {
          params: { path: { projectId, cardTableId } },
          body: {
            title: req.title,
            description: req.description,
          },
        })
    );

    return response;
  }

  /**
   * Updates an existing column.
   *
   * @param projectId - The project (bucket) ID
   * @param columnId - The column ID
   * @param req - Column update parameters
   * @returns The updated column
   *
   * @example
   * ```ts
   * const column = await client.cardColumns.update(projectId, columnId, {
   *   title: "Updated Title",
   * });
   * ```
   */
  async update(
    projectId: number,
    columnId: number,
    req: UpdateColumnRequest
  ): Promise<CardColumn> {
    const response = await this.request(
      {
        service: "CardColumns",
        operation: "Update",
        resourceType: "card_column",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/card_tables/columns/{columnId}", {
          params: { path: { projectId, columnId } },
          body: {
            title: req.title,
            description: req.description,
          },
        })
    );

    return response;
  }

  /**
   * Moves a column within a card table.
   *
   * @param projectId - The project (bucket) ID
   * @param cardTableId - The card table ID
   * @param req - Move parameters
   *
   * @example
   * ```ts
   * await client.cardColumns.move(projectId, tableId, {
   *   sourceId: columnToMove,
   *   targetId: targetColumn,
   *   position: 1,
   * });
   * ```
   */
  async move(projectId: number, cardTableId: number, req: MoveColumnRequest): Promise<void> {
    await this.request(
      {
        service: "CardColumns",
        operation: "Move",
        resourceType: "card_column",
        isMutation: true,
        projectId,
        resourceId: cardTableId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/{cardTableId}/moves.json", {
          params: { path: { projectId, cardTableId } },
          body: {
            source_id: req.sourceId,
            target_id: req.targetId,
            position: req.position,
          },
        })
    );
  }

  /**
   * Sets the color of a column.
   *
   * @param projectId - The project (bucket) ID
   * @param columnId - The column ID
   * @param color - The column color
   * @returns The updated column
   * @throws BasecampError with code "validation" if color is missing
   *
   * @example
   * ```ts
   * const column = await client.cardColumns.setColor(projectId, columnId, "blue");
   * ```
   */
  async setColor(projectId: number, columnId: number, color: ColumnColor): Promise<CardColumn> {
    if (!color) {
      throw Errors.validation("Color is required");
    }

    const response = await this.request(
      {
        service: "CardColumns",
        operation: "SetColor",
        resourceType: "card_column",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/card_tables/columns/{columnId}/color.json", {
          params: { path: { projectId, columnId } },
          body: { color },
        })
    );

    return response;
  }

  /**
   * Adds an on-hold section to a column.
   *
   * @param projectId - The project (bucket) ID
   * @param columnId - The column ID
   * @returns The updated column
   *
   * @example
   * ```ts
   * const column = await client.cardColumns.enableOnHold(projectId, columnId);
   * ```
   */
  async enableOnHold(projectId: number, columnId: number): Promise<CardColumn> {
    const response = await this.request(
      {
        service: "CardColumns",
        operation: "EnableOnHold",
        resourceType: "card_column",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json", {
          params: { path: { projectId, columnId } },
        })
    );

    return response;
  }

  /**
   * Removes the on-hold section from a column.
   *
   * @param projectId - The project (bucket) ID
   * @param columnId - The column ID
   * @returns The updated column
   *
   * @example
   * ```ts
   * const column = await client.cardColumns.disableOnHold(projectId, columnId);
   * ```
   */
  async disableOnHold(projectId: number, columnId: number): Promise<CardColumn> {
    const response = await this.request(
      {
        service: "CardColumns",
        operation: "DisableOnHold",
        resourceType: "card_column",
        isMutation: true,
        projectId,
        resourceId: columnId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json", {
          params: { path: { projectId, columnId } },
        })
    );

    return response;
  }
}

// =============================================================================
// Card Steps Service
// =============================================================================

/**
 * Service for managing Basecamp card steps (checklist items).
 */
export class CardStepsService extends BaseService {
  /**
   * Creates a new step on a card.
   *
   * @param projectId - The project (bucket) ID
   * @param cardId - The card ID
   * @param req - Step creation parameters
   * @returns The created step
   * @throws BasecampError with code "validation" if title is missing
   *
   * @example
   * ```ts
   * const step = await client.cardSteps.create(projectId, cardId, {
   *   title: "Review code",
   *   dueOn: "2024-12-15",
   *   assignees: [1234],
   * });
   * ```
   */
  async create(projectId: number, cardId: number, req: CreateStepRequest): Promise<CardStep> {
    if (!req.title) {
      throw Errors.validation("Step title is required");
    }

    if (req.dueOn && !isValidDateFormat(req.dueOn)) {
      throw Errors.validation("Step due_on must be in YYYY-MM-DD format");
    }

    const response = await this.request(
      {
        service: "CardSteps",
        operation: "Create",
        resourceType: "card_step",
        isMutation: true,
        projectId,
        resourceId: cardId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/cards/{cardId}/steps.json", {
          params: { path: { projectId, cardId } },
          body: {
            title: req.title,
            due_on: req.dueOn,
            assignees: req.assignees,
          },
        })
    );

    return response;
  }

  /**
   * Updates an existing step.
   *
   * @param projectId - The project (bucket) ID
   * @param stepId - The step ID
   * @param req - Step update parameters
   * @returns The updated step
   *
   * @example
   * ```ts
   * const step = await client.cardSteps.update(projectId, stepId, {
   *   title: "Updated step",
   * });
   * ```
   */
  async update(projectId: number, stepId: number, req: UpdateStepRequest): Promise<CardStep> {
    if (req.dueOn && !isValidDateFormat(req.dueOn)) {
      throw Errors.validation("Step due_on must be in YYYY-MM-DD format");
    }

    const response = await this.request(
      {
        service: "CardSteps",
        operation: "Update",
        resourceType: "card_step",
        isMutation: true,
        projectId,
        resourceId: stepId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/card_tables/steps/{stepId}", {
          params: { path: { projectId, stepId } },
          body: {
            title: req.title,
            due_on: req.dueOn,
            assignees: req.assignees,
          },
        })
    );

    return response;
  }

  /**
   * Marks a step as completed.
   *
   * @param projectId - The project (bucket) ID
   * @param stepId - The step ID
   * @returns The updated step
   *
   * @example
   * ```ts
   * const step = await client.cardSteps.complete(projectId, stepId);
   * ```
   */
  async complete(projectId: number, stepId: number): Promise<CardStep> {
    const response = await this.request(
      {
        service: "CardSteps",
        operation: "Complete",
        resourceType: "card_step",
        isMutation: true,
        projectId,
        resourceId: stepId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/card_tables/steps/{stepId}/completions.json", {
          params: { path: { projectId, stepId } },
        })
    );

    return response;
  }

  /**
   * Marks a step as incomplete.
   *
   * @param projectId - The project (bucket) ID
   * @param stepId - The step ID
   * @returns The updated step
   *
   * @example
   * ```ts
   * const step = await client.cardSteps.uncomplete(projectId, stepId);
   * ```
   */
  async uncomplete(projectId: number, stepId: number): Promise<CardStep> {
    const response = await this.request(
      {
        service: "CardSteps",
        operation: "Uncomplete",
        resourceType: "card_step",
        isMutation: true,
        projectId,
        resourceId: stepId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/card_tables/steps/{stepId}/completions.json", {
          params: { path: { projectId, stepId } },
        })
    );

    return response;
  }

  /**
   * Changes the position of a step within a card.
   *
   * @param projectId - The project (bucket) ID
   * @param cardId - The card ID
   * @param stepId - The step ID
   * @param position - New position (0-indexed)
   * @throws BasecampError with code "validation" if position < 0
   *
   * @example
   * ```ts
   * // Move step to first position
   * await client.cardSteps.reposition(projectId, cardId, stepId, 0);
   * ```
   */
  async reposition(
    projectId: number,
    cardId: number,
    stepId: number,
    position: number
  ): Promise<void> {
    if (position < 0) {
      throw Errors.validation("Position must be at least 0");
    }

    await this.request(
      {
        service: "CardSteps",
        operation: "Reposition",
        resourceType: "card_step",
        isMutation: true,
        projectId,
        resourceId: stepId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/cards/{cardId}/positions.json", {
          params: { path: { projectId, cardId } },
          body: {
            source_id: stepId,
            position,
          },
        })
    );
  }

  /**
   * Deletes a step (moves it to trash).
   *
   * @param projectId - The project (bucket) ID
   * @param stepId - The step ID
   *
   * @example
   * ```ts
   * await client.cardSteps.delete(projectId, stepId);
   * ```
   */
  async delete(projectId: number, stepId: number): Promise<void> {
    await this.request(
      {
        service: "CardSteps",
        operation: "Delete",
        resourceType: "card_step",
        isMutation: true,
        projectId,
        resourceId: stepId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: { path: { projectId, recordingId: stepId } },
        })
    );
  }
}

// =============================================================================
// Helpers
// =============================================================================

/**
 * Validates that a string is in YYYY-MM-DD format.
 */
function isValidDateFormat(date: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(date);
}
