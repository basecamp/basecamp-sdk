/**
 * Service for TodolistGroups operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for TodolistGroups operations
 */
export class TodolistGroupsService extends BaseService {

  /**
   * Reposition a todolist group
   */
  async reposition(projectId: number, groupId: number, req: components["schemas"]["RepositionTodolistGroupRequestContent"]): Promise<void> {
    await this.request(
      {
        service: "TodolistGroups",
        operation: "RepositionTodolistGroup",
        resourceType: "todolist_group",
        isMutation: true,
        projectId,
        resourceId: groupId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/todolists/{groupId}/position.json", {
          params: {
            path: { projectId, groupId },
          },
          body: req,
        })
    );
  }

  /**
   * List groups in a todolist
   */
  async list(projectId: number, todolistId: number): Promise<components["schemas"]["ListTodolistGroupsResponseContent"]> {
    const response = await this.request(
      {
        service: "TodolistGroups",
        operation: "ListTodolistGroups",
        resourceType: "todolist_group",
        isMutation: false,
        projectId,
        resourceId: todolistId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todolists/{todolistId}/groups.json", {
          params: {
            path: { projectId, todolistId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new group in a todolist
   */
  async create(projectId: number, todolistId: number, req: components["schemas"]["CreateTodolistGroupRequestContent"]): Promise<components["schemas"]["CreateTodolistGroupResponseContent"]> {
    const response = await this.request(
      {
        service: "TodolistGroups",
        operation: "CreateTodolistGroup",
        resourceType: "todolist_group",
        isMutation: true,
        projectId,
        resourceId: todolistId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/todolists/{todolistId}/groups.json", {
          params: {
            path: { projectId, todolistId },
          },
          body: req,
        })
    );
    return response;
  }
}