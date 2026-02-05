/**
 * Todos service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Todo entity from the Basecamp API. */
export type Todo = components["schemas"]["Todo"];

/**
 * Options for list.
 */
export interface ListTodoOptions {
  /** active|archived|trashed */
  status?: string;
  /** completed */
  completed?: boolean;
}

/**
 * Request parameters for create.
 */
export interface CreateTodoRequest {
  /** content */
  content: string;
  /** description */
  description?: string;
  /** assignee ids */
  assigneeIds?: number[];
  /** completion subscriber ids */
  completionSubscriberIds?: number[];
  /** notify */
  notify?: boolean;
  /** due on (YYYY-MM-DD) */
  dueOn?: string;
  /** starts on (YYYY-MM-DD) */
  startsOn?: string;
}

/**
 * Request parameters for update.
 */
export interface UpdateTodoRequest {
  /** content */
  content?: string;
  /** description */
  description?: string;
  /** assignee ids */
  assigneeIds?: number[];
  /** completion subscriber ids */
  completionSubscriberIds?: number[];
  /** notify */
  notify?: boolean;
  /** due on (YYYY-MM-DD) */
  dueOn?: string;
  /** starts on (YYYY-MM-DD) */
  startsOn?: string;
}

/**
 * Request parameters for reposition.
 */
export interface RepositionTodoRequest {
  /** position */
  position: number;
  /** Optional todolist ID to move the todo to a different parent */
  parentId?: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Todos operations.
 */
export class TodosService extends BaseService {

  /**
   * List todos in a todolist
   * @param projectId - The project ID
   * @param todolistId - The todolist ID
   * @param options - Optional parameters
   * @returns Array of Todo
   */
  async list(projectId: number, todolistId: number, options?: ListTodoOptions): Promise<Todo[]> {
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
   * @param projectId - The project ID
   * @param todolistId - The todolist ID
   * @param req - Request parameters
   * @returns The Todo
   *
   * @example
   * ```ts
   * const result = await client.todos.create(123, 123, { ... });
   * ```
   */
  async create(projectId: number, todolistId: number, req: CreateTodoRequest): Promise<Todo> {
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
          body: {
            content: req.content,
            description: req.description,
            assignee_ids: req.assigneeIds,
            completion_subscriber_ids: req.completionSubscriberIds,
            notify: req.notify,
            due_on: req.dueOn,
            starts_on: req.startsOn,
          },
        })
    );
    return response;
  }

  /**
   * Get a single todo by id
   * @param projectId - The project ID
   * @param todoId - The todo ID
   * @returns The Todo
   */
  async get(projectId: number, todoId: number): Promise<Todo> {
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
   * @param projectId - The project ID
   * @param todoId - The todo ID
   * @param req - Request parameters
   * @returns The Todo
   */
  async update(projectId: number, todoId: number, req: UpdateTodoRequest): Promise<Todo> {
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
          body: {
            content: req.content,
            description: req.description,
            assignee_ids: req.assigneeIds,
            completion_subscriber_ids: req.completionSubscriberIds,
            notify: req.notify,
            due_on: req.dueOn,
            starts_on: req.startsOn,
          },
        })
    );
    return response;
  }

  /**
   * Trash a todo (returns 204 No Content)
   * @param projectId - The project ID
   * @param todoId - The todo ID
   * @returns void
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
   * @param projectId - The project ID
   * @param todoId - The todo ID
   * @returns void
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
   * @param projectId - The project ID
   * @param todoId - The todo ID
   * @returns void
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
   * @param projectId - The project ID
   * @param todoId - The todo ID
   * @param req - Request parameters
   * @returns void
   */
  async reposition(projectId: number, todoId: number, req: RepositionTodoRequest): Promise<void> {
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
          body: {
            position: req.position,
            parent_id: req.parentId,
          },
        })
    );
  }
}