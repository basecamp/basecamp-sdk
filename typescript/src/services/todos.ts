/**
 * Todos service for the Basecamp API.
 *
 * Todos are individual tasks within a todolist. They can have
 * assignees, due dates, descriptions, and completion status.
 *
 * @example
 * ```ts
 * const todos = await client.todos.list(projectId, todolistId);
 * const todo = await client.todos.get(projectId, todoId);
 * await client.todos.complete(projectId, todoId);
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp todo item.
 */
export type Todo = components["schemas"]["Todo"];

/**
 * A person associated with a todo (assignee, creator, etc.).
 */
export type Person = components["schemas"]["Person"];

/**
 * Options for listing todos.
 */
export interface TodoListOptions {
  /**
   * Filter by completion status.
   * "completed" returns completed todos, "pending" returns pending todos.
   * Empty returns all todos.
   */
  status?: "completed" | "pending";
}

/**
 * Request to create a new todo.
 */
export interface CreateTodoRequest {
  /** Todo text (required) */
  content: string;
  /** Extended description in HTML (optional) */
  description?: string;
  /** Person IDs to assign this todo to (optional) */
  assigneeIds?: number[];
  /** Person IDs to notify on completion (optional) */
  completionSubscriberIds?: number[];
  /** Notify assignees when true (optional) */
  notify?: boolean;
  /** Due date in ISO 8601 format (YYYY-MM-DD) (optional) */
  dueOn?: string;
  /** Start date in ISO 8601 format (YYYY-MM-DD) (optional) */
  startsOn?: string;
}

/**
 * Request to update an existing todo.
 */
export interface UpdateTodoRequest {
  /** Todo text (optional) */
  content?: string;
  /** Extended description in HTML (optional) */
  description?: string;
  /** Person IDs to assign this todo to (optional) */
  assigneeIds?: number[];
  /** Person IDs to notify on completion (optional) */
  completionSubscriberIds?: number[];
  /** Notify assignees when true (optional) */
  notify?: boolean;
  /** Due date in ISO 8601 format (YYYY-MM-DD) (optional) */
  dueOn?: string;
  /** Start date in ISO 8601 format (YYYY-MM-DD) (optional) */
  startsOn?: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp todos.
 */
export class TodosService extends BaseService {
  /**
   * Lists all todos in a todolist.
   *
   * @param projectId - The project (bucket) ID
   * @param todolistId - The todolist ID
   * @param options - Optional filters
   * @returns Array of todos
   *
   * @example
   * ```ts
   * // List all todos
   * const todos = await client.todos.list(projectId, todolistId);
   *
   * // List only completed todos
   * const completed = await client.todos.list(projectId, todolistId, { status: "completed" });
   * ```
   */
  async list(projectId: number, todolistId: number, options?: TodoListOptions): Promise<Todo[]> {
    const response = await this.request(
      {
        service: "Todos",
        operation: "List",
        resourceType: "todo",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todolists/{todolistId}/todos.json", {
          params: {
            path: { projectId, todolistId },
            query: options?.status ? { status: options.status } : undefined,
          },
        })
    );

    return response?.todos ?? [];
  }

  /**
   * Gets a todo by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param todoId - The todo ID
   * @returns The todo
   * @throws BasecampError with code "not_found" if todo doesn't exist
   *
   * @example
   * ```ts
   * const todo = await client.todos.get(projectId, todoId);
   * console.log(todo.content, todo.completed);
   * ```
   */
  async get(projectId: number, todoId: number): Promise<Todo> {
    const response = await this.request(
      {
        service: "Todos",
        operation: "Get",
        resourceType: "todo",
        isMutation: false,
        projectId,
        resourceId: todoId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todos/{todoId}", {
          params: { path: { projectId, todoId } },
        })
    );

    return response.todo!;
  }

  /**
   * Creates a new todo in a todolist.
   *
   * @param projectId - The project (bucket) ID
   * @param todolistId - The todolist ID
   * @param req - Todo creation parameters
   * @returns The created todo
   * @throws BasecampError with code "validation" if content is missing
   *
   * @example
   * ```ts
   * const todo = await client.todos.create(projectId, todolistId, {
   *   content: "Complete the project",
   *   dueOn: "2024-12-31",
   *   assigneeIds: [1234],
   * });
   * ```
   */
  async create(projectId: number, todolistId: number, req: CreateTodoRequest): Promise<Todo> {
    if (!req.content) {
      throw Errors.validation("Todo content is required");
    }

    // Validate date formats if provided
    if (req.dueOn && !isValidDateFormat(req.dueOn)) {
      throw Errors.validation("Todo due_on must be in YYYY-MM-DD format");
    }
    if (req.startsOn && !isValidDateFormat(req.startsOn)) {
      throw Errors.validation("Todo starts_on must be in YYYY-MM-DD format");
    }

    const response = await this.request(
      {
        service: "Todos",
        operation: "Create",
        resourceType: "todo",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/todolists/{todolistId}/todos.json", {
          params: { path: { projectId, todolistId } },
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

    return response.todo!;
  }

  /**
   * Updates an existing todo.
   *
   * @param projectId - The project (bucket) ID
   * @param todoId - The todo ID
   * @param req - Todo update parameters
   * @returns The updated todo
   *
   * @example
   * ```ts
   * const todo = await client.todos.update(projectId, todoId, {
   *   content: "Updated task",
   *   dueOn: "2024-12-15",
   * });
   * ```
   */
  async update(projectId: number, todoId: number, req: UpdateTodoRequest): Promise<Todo> {
    // Validate date formats if provided
    if (req.dueOn && !isValidDateFormat(req.dueOn)) {
      throw Errors.validation("Todo due_on must be in YYYY-MM-DD format");
    }
    if (req.startsOn && !isValidDateFormat(req.startsOn)) {
      throw Errors.validation("Todo starts_on must be in YYYY-MM-DD format");
    }

    const response = await this.request(
      {
        service: "Todos",
        operation: "Update",
        resourceType: "todo",
        isMutation: true,
        projectId,
        resourceId: todoId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/todos/{todoId}", {
          params: { path: { projectId, todoId } },
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

    return response.todo!;
  }

  /**
   * Marks a todo as completed.
   *
   * @param projectId - The project (bucket) ID
   * @param todoId - The todo ID
   *
   * @example
   * ```ts
   * await client.todos.complete(projectId, todoId);
   * ```
   */
  async complete(projectId: number, todoId: number): Promise<void> {
    await this.request(
      {
        service: "Todos",
        operation: "Complete",
        resourceType: "todo",
        isMutation: true,
        projectId,
        resourceId: todoId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/todos/{todoId}/completion.json", {
          params: { path: { projectId, todoId } },
        })
    );
  }

  /**
   * Marks a completed todo as incomplete (reopens it).
   *
   * @param projectId - The project (bucket) ID
   * @param todoId - The todo ID
   *
   * @example
   * ```ts
   * await client.todos.uncomplete(projectId, todoId);
   * ```
   */
  async uncomplete(projectId: number, todoId: number): Promise<void> {
    await this.request(
      {
        service: "Todos",
        operation: "Uncomplete",
        resourceType: "todo",
        isMutation: true,
        projectId,
        resourceId: todoId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/todos/{todoId}/completion.json", {
          params: { path: { projectId, todoId } },
        })
    );
  }

  /**
   * Moves a todo to the trash.
   * Trashed todos can be recovered from the trash.
   *
   * @param projectId - The project (bucket) ID
   * @param todoId - The todo ID
   *
   * @example
   * ```ts
   * await client.todos.trash(projectId, todoId);
   * ```
   */
  async trash(projectId: number, todoId: number): Promise<void> {
    await this.request(
      {
        service: "Todos",
        operation: "Trash",
        resourceType: "todo",
        isMutation: true,
        projectId,
        resourceId: todoId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: { path: { projectId, recordingId: todoId } },
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
