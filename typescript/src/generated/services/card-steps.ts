/**
 * Service for CardSteps operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for CardSteps operations
 */
export class CardStepsService extends BaseService {

  /**
   * Reposition a step within a card
   */
  async reposition(projectId: number, cardId: number, req: components["schemas"]["RepositionCardStepRequestContent"]): Promise<void> {
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
          body: req,
        })
    );
  }

  /**
   * Create a step on a card
   */
  async create(projectId: number, cardId: number, req: components["schemas"]["CreateCardStepRequestContent"]): Promise<components["schemas"]["CreateCardStepResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Update an existing step
   */
  async update(projectId: number, stepId: number, req: components["schemas"]["UpdateCardStepRequestContent"]): Promise<components["schemas"]["UpdateCardStepResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Mark a step as completed
   */
  async complete(projectId: number, stepId: number): Promise<components["schemas"]["CompleteCardStepResponseContent"]> {
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
   */
  async uncomplete(projectId: number, stepId: number): Promise<components["schemas"]["UncompleteCardStepResponseContent"]> {
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