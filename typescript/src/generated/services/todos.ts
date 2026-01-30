/**
 * Service for Todos operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Todos operations
 */
export class TodosService extends BaseService {

  /**
   * List todos in a todolist
   */
  async list(projectId: number, todolistId: number, options?: { status?: string; completed?: boolean }): Promise<components["schemas"]["ListTodosResponseContent"]> {
    const response = await this.request(
      {
        service: "Todos",
        operation: "ListTodos",
        resourceType: "todo",
        isMutation: false,
        projectId,
        resourceId: todolistId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todolists/{todolistId}/todos.json", {
          params: {
            path: { projectId, todolistId },
            query: { status: options?.status, completed: options?.completed },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new todo in a todolist
   */
  async create(projectId: number, todolistId: number, req: components["schemas"]["CreateTodoRequestContent"]): Promise<components["schemas"]["CreateTodoResponseContent"]> {
    const response = await this.request(
      {
        service: "Todos",
        operation: "CreateTodo",
        resourceType: "todo",
        isMutation: true,
        projectId,
        resourceId: todolistId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/todolists/{todolistId}/todos.json", {
          params: {
            path: { projectId, todolistId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a single todo by id
   */
  async get(projectId: number, todoId: number): Promise<components["schemas"]["GetTodoResponseContent"]> {
    const response = await this.request(
      {
        service: "Todos",
        operation: "GetTodo",
        resourceType: "todo",
        isMutation: false,
        projectId,
        resourceId: todoId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todos/{todoId}", {
          params: {
            path: { projectId, todoId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing todo
   */
  async update(projectId: number, todoId: number, req: components["schemas"]["UpdateTodoRequestContent"]): Promise<components["schemas"]["UpdateTodoResponseContent"]> {
    const response = await this.request(
      {
        service: "Todos",
        operation: "UpdateTodo",
        resourceType: "todo",
        isMutation: true,
        projectId,
        resourceId: todoId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/todos/{todoId}", {
          params: {
            path: { projectId, todoId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Trash a todo (returns 204 No Content)
   */
  async trash(projectId: number, todoId: number): Promise<void> {
    await this.request(
      {
        service: "Todos",
        operation: "TrashTodo",
        resourceType: "todo",
        isMutation: true,
        projectId,
        resourceId: todoId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/todos/{todoId}", {
          params: {
            path: { projectId, todoId },
          },
        })
    );
  }

  /**
   * Mark a todo as complete
   */
  async complete(projectId: number, todoId: number): Promise<void> {
    await this.request(
      {
        service: "Todos",
        operation: "CompleteTodo",
        resourceType: "todo",
        isMutation: true,
        projectId,
        resourceId: todoId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/todos/{todoId}/completion.json", {
          params: {
            path: { projectId, todoId },
          },
        })
    );
  }

  /**
   * Mark a todo as incomplete
   */
  async uncomplete(projectId: number, todoId: number): Promise<void> {
    await this.request(
      {
        service: "Todos",
        operation: "UncompleteTodo",
        resourceType: "todo",
        isMutation: true,
        projectId,
        resourceId: todoId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/todos/{todoId}/completion.json", {
          params: {
            path: { projectId, todoId },
          },
        })
    );
  }

  /**
   * Reposition a todo within its todolist
   */
  async reposition(projectId: number, todoId: number, req: components["schemas"]["RepositionTodoRequestContent"]): Promise<void> {
    await this.request(
      {
        service: "Todos",
        operation: "RepositionTodo",
        resourceType: "todo",
        isMutation: true,
        projectId,
        resourceId: todoId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/todos/{todoId}/position.json", {
          params: {
            path: { projectId, todoId },
          },
          body: req,
        })
    );
  }
}