/**
 * Todos service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Todo entity from the Basecamp API. */
export type Todo = components["schemas"]["Todo"];

/**
 * Options for list.
 */
export interface ListTodoOptions extends PaginationOptions {
  /** Filter by status */
  status?: "active" | "archived" | "trashed";
  /** Completed */
  completed?: boolean;
}

/**
 * Request parameters for create.
 */
export interface CreateTodoRequest {
  /** Text content */
  content: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Person IDs to assign to */
  assigneeIds?: number[];
  /** Person IDs to notify on completion */
  completionSubscriberIds?: number[];
  /** Whether to send notifications to relevant people */
  notify?: boolean;
  /** Due date (YYYY-MM-DD) */
  dueOn?: string;
  /** Start date (YYYY-MM-DD) */
  startsOn?: string;
}

/**
 * Request parameters for update.
 */
export interface UpdateTodoRequest {
  /** Text content */
  content?: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Person IDs to assign to */
  assigneeIds?: number[];
  /** Person IDs to notify on completion */
  completionSubscriberIds?: number[];
  /** Whether to send notifications to relevant people */
  notify?: boolean;
  /** Due date (YYYY-MM-DD) */
  dueOn?: string;
  /** Start date (YYYY-MM-DD) */
  startsOn?: string;
}

/**
 * Request parameters for reposition.
 */
export interface RepositionTodoRequest {
  /** Position for ordering (1-based) */
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
   * @param options - Optional query parameters
   * @returns All Todo across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.todos.list(123, 123);
   *
   * // With options
   * const filtered = await client.todos.list(123, 123, { status: "active" });
   * ```
   */
  async list(projectId: number, todolistId: number, options?: ListTodoOptions): Promise<ListResult<Todo>> {
    return this.requestPaginated(
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
      , options
    );
  }

  /**
   * Create a new todo in a todolist
   * @param projectId - The project ID
   * @param todolistId - The todolist ID
   * @param req - Todo creation parameters
   * @returns The Todo
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.todos.create(123, 123, { content: "Hello world" });
   * ```
   */
  async create(projectId: number, todolistId: number, req: CreateTodoRequest): Promise<Todo> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    if (req.dueOn && !/^\d{4}-\d{2}-\d{2}$/.test(req.dueOn)) {
      throw Errors.validation("Due on must be in YYYY-MM-DD format");
    }
    if (req.startsOn && !/^\d{4}-\d{2}-\d{2}$/.test(req.startsOn)) {
      throw Errors.validation("Starts on must be in YYYY-MM-DD format");
    }
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
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.todos.get(123, 123);
   * ```
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
   * @param req - Todo update parameters
   * @returns The Todo
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.todos.update(123, 123, { });
   * ```
   */
  async update(projectId: number, todoId: number, req: UpdateTodoRequest): Promise<Todo> {
    if (req.dueOn && !/^\d{4}-\d{2}-\d{2}$/.test(req.dueOn)) {
      throw Errors.validation("Due on must be in YYYY-MM-DD format");
    }
    if (req.startsOn && !/^\d{4}-\d{2}-\d{2}$/.test(req.startsOn)) {
      throw Errors.validation("Starts on must be in YYYY-MM-DD format");
    }
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
   * Trash a todo. Trashed items can be recovered.
   * @param projectId - The project ID
   * @param todoId - The todo ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.todos.trash(123, 123);
   * ```
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
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.todos.complete(123, 123);
   * ```
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
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.todos.uncomplete(123, 123);
   * ```
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
   * @param req - Todo request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.todos.reposition(123, 123, { position: 1 });
   * ```
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