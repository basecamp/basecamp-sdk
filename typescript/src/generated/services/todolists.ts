/**
 * Service for Todolists operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Todolists operations
 */
export class TodolistsService extends BaseService {

  /**
   * Get a single todolist or todolist group by id
   */
  async get(projectId: number, id: number): Promise<components["schemas"]["GetTodolistOrGroupResponseContent"]> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "GetTodolistOrGroup",
        resourceType: "todolist_or_group",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todolists/{id}", {
          params: {
            path: { projectId, id },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing todolist or todolist group
   */
  async update(projectId: number, id: number, req: components["schemas"]["UpdateTodolistOrGroupRequestContent"]): Promise<components["schemas"]["UpdateTodolistOrGroupResponseContent"]> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "UpdateTodolistOrGroup",
        resourceType: "todolist_or_group",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/todolists/{id}", {
          params: {
            path: { projectId, id },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * List todolists in a todoset
   */
  async list(projectId: number, todosetId: number, options?: { status?: string }): Promise<components["schemas"]["ListTodolistsResponseContent"]> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "ListTodolists",
        resourceType: "todolist",
        isMutation: false,
        projectId,
        resourceId: todosetId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todosets/{todosetId}/todolists.json", {
          params: {
            path: { projectId, todosetId },
            query: { status: options?.status },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new todolist in a todoset
   */
  async create(projectId: number, todosetId: number, req: components["schemas"]["CreateTodolistRequestContent"]): Promise<components["schemas"]["CreateTodolistResponseContent"]> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "CreateTodolist",
        resourceType: "todolist",
        isMutation: true,
        projectId,
        resourceId: todosetId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/todosets/{todosetId}/todolists.json", {
          params: {
            path: { projectId, todosetId },
          },
          body: req,
        })
    );
    return response;
  }
}