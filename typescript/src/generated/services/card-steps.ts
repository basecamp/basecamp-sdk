/**
 * CardSteps service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** CardStep entity from the Basecamp API. */
export type CardStep = components["schemas"]["CardStep"];

/**
 * Request parameters for reposition.
 */
export interface RepositionCardStepRequest {
  /** Source id */
  sourceId: number;
  /** 0-indexed position */
  position: number;
}

/**
 * Request parameters for create.
 */
export interface CreateCardStepRequest {
  /** Title */
  title: string;
  /** Due date (YYYY-MM-DD) */
  dueOn?: string;
  /** Assignees */
  assignees?: number[];
}

/**
 * Request parameters for update.
 */
export interface UpdateCardStepRequest {
  /** Title */
  title?: string;
  /** Due date (YYYY-MM-DD) */
  dueOn?: string;
  /** Assignees */
  assignees?: number[];
}

/**
 * Request parameters for setCompletion.
 */
export interface SetCompletionCardStepRequest {
  /** Set to "on" to complete the step, "" (empty) to uncomplete */
  completion: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for CardSteps operations.
 */
export class CardStepsService extends BaseService {

  /**
   * Reposition a step within a card
   * @param projectId - The project ID
   * @param cardId - The card ID
   * @param req - Card_step request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.cardSteps.reposition(123, 123, { sourceId: 1, position: 1 });
   * ```
   */
  async reposition(projectId: number, cardId: number, req: RepositionCardStepRequest): Promise<void> {
    await this.request(
      {
        service: "CardSteps",
        operation: "RepositionCardStep",
        resourceType: "card_step",
        isMutation: true,
        projectId,
        resourceId: cardId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/cards/{cardId}/positions.json", {
          params: {
            path: { projectId, cardId },
          },
          body: {
            source_id: req.sourceId,
            position: req.position,
          },
        })
    );
  }

  /**
   * Create a step on a card
   * @param projectId - The project ID
   * @param cardId - The card ID
   * @param req - Card_step creation parameters
   * @returns The CardStep
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.cardSteps.create(123, 123, { title: "example" });
   * ```
   */
  async create(projectId: number, cardId: number, req: CreateCardStepRequest): Promise<CardStep> {
    if (!req.title) {
      throw Errors.validation("Title is required");
    }
    if (req.dueOn && !/^\d{4}-\d{2}-\d{2}$/.test(req.dueOn)) {
      throw Errors.validation("Due on must be in YYYY-MM-DD format");
    }
    const response = await this.request(
      {
        service: "CardSteps",
        operation: "CreateCardStep",
        resourceType: "card_step",
        isMutation: true,
        projectId,
        resourceId: cardId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/card_tables/cards/{cardId}/steps.json", {
          params: {
            path: { projectId, cardId },
          },
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
   * Update an existing step
   * @param projectId - The project ID
   * @param stepId - The step ID
   * @param req - Card_step update parameters
   * @returns The CardStep
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.cardSteps.update(123, 123, { });
   * ```
   */
  async update(projectId: number, stepId: number, req: UpdateCardStepRequest): Promise<CardStep> {
    if (req.dueOn && !/^\d{4}-\d{2}-\d{2}$/.test(req.dueOn)) {
      throw Errors.validation("Due on must be in YYYY-MM-DD format");
    }
    const response = await this.request(
      {
        service: "CardSteps",
        operation: "UpdateCardStep",
        resourceType: "card_step",
        isMutation: true,
        projectId,
        resourceId: stepId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/card_tables/steps/{stepId}", {
          params: {
            path: { projectId, stepId },
          },
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
   * Set card step completion status (PUT with completion: "on" to complete, "" to uncomplete)
   * @param projectId - The project ID
   * @param stepId - The step ID
   * @param req - Card_step_completion request parameters
   * @returns The CardStep
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.cardSteps.setCompletion(123, 123, { completion: "example" });
   * ```
   */
  async setCompletion(projectId: number, stepId: number, req: SetCompletionCardStepRequest): Promise<CardStep> {
    if (!req.completion) {
      throw Errors.validation("Completion is required");
    }
    const response = await this.request(
      {
        service: "CardSteps",
        operation: "SetCardStepCompletion",
        resourceType: "card_step_completion",
        isMutation: true,
        projectId,
        resourceId: stepId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/card_tables/steps/{stepId}/completions.json", {
          params: {
            path: { projectId, stepId },
          },
          body: {
            completion: req.completion,
          },
        })
    );
    return response;
  }
}