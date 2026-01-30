/**
 * CardSteps service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** CardStep entity from the Basecamp API. */
export type CardStep = components["schemas"]["CardStep"];

/**
 * Request parameters for reposition.
 */
export interface RepositionCardStepRequest {
  /** source id */
  sourceId: number;
  /** 0-indexed position */
  position: number;
}

/**
 * Request parameters for create.
 */
export interface CreateCardStepRequest {
  /** title */
  title: string;
  /** due on (YYYY-MM-DD) */
  dueOn?: string;
  /** assignees */
  assignees?: number[];
}

/**
 * Request parameters for update.
 */
export interface UpdateCardStepRequest {
  /** title */
  title?: string;
  /** due on (YYYY-MM-DD) */
  dueOn?: string;
  /** assignees */
  assignees?: number[];
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
   * @param req - Request parameters
   * @returns void
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
   * @param req - Request parameters
   * @returns The CardStep
   *
   * @example
   * ```ts
   * const result = await client.cardSteps.create(123, 123, { ... });
   * ```
   */
  async create(projectId: number, cardId: number, req: CreateCardStepRequest): Promise<CardStep> {
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
   * @param req - Request parameters
   * @returns The CardStep
   */
  async update(projectId: number, stepId: number, req: UpdateCardStepRequest): Promise<CardStep> {
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
   * Mark a step as completed
   * @param projectId - The project ID
   * @param stepId - The step ID
   * @returns The CardStep
   */
  async complete(projectId: number, stepId: number): Promise<CardStep> {
    const response = await this.request(
      {
        service: "CardSteps",
        operation: "CompleteCardStep",
        resourceType: "card_step",
        isMutation: true,
        projectId,
        resourceId: stepId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/card_tables/steps/{stepId}/completions.json", {
          params: {
            path: { projectId, stepId },
          },
        })
    );
    return response;
  }

  /**
   * Mark a step as incomplete
   * @param projectId - The project ID
   * @param stepId - The step ID
   * @returns The CardStep
   */
  async uncomplete(projectId: number, stepId: number): Promise<CardStep> {
    const response = await this.request(
      {
        service: "CardSteps",
        operation: "UncompleteCardStep",
        resourceType: "card_step",
        isMutation: true,
        projectId,
        resourceId: stepId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/card_tables/steps/{stepId}/completions.json", {
          params: {
            path: { projectId, stepId },
          },
        })
    );
    return response;
  }
}