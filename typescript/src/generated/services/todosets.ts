/**
 * Service for Todosets operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Todosets operations
 */
export class TodosetsService extends BaseService {

  /**
   * Get a todoset (container for todolists in a project)
   */
  async get(projectId: number, todosetId: number): Promise<components["schemas"]["GetTodosetResponseContent"]> {
    const response = await this.request(
      {
        service: "Todosets",
        operation: "GetTodoset",
        resourceType: "todoset",
        isMutation: false,
        projectId,
        resourceId: todosetId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todosets/{todosetId}", {
          params: {
            path: { projectId, todosetId },
          },
        })
    );
    return response;
  }
}