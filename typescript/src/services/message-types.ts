/**
 * Message Types service for the Basecamp API.
 *
 * Message types (also called categories) are used to categorize messages
 * on a message board. Each message type has a name and icon.
 *
 * @example
 * ```ts
 * const types = await client.messageTypes.list(projectId);
 * const type = await client.messageTypes.create(projectId, {
 *   name: "Announcement",
 *   icon: "📢",
 * });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp message type (category).
 */
export interface MessageType {
  id: number;
  name: string;
  icon: string;
  created_at: string;
  updated_at: string;
}

/**
 * Request to create a new message type.
 */
export interface CreateMessageTypeRequest {
  /** Message type name (required) */
  name: string;
  /** Message type icon (required) */
  icon: string;
}

/**
 * Request to update an existing message type.
 */
export interface UpdateMessageTypeRequest {
  /** Message type name (optional) */
  name?: string;
  /** Message type icon (optional) */
  icon?: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp message types.
 */
export class MessageTypesService extends BaseService {
  /**
   * Lists all message types in a project.
   *
   * @param projectId - The project (bucket) ID
   * @returns Array of message types
   *
   * @example
   * ```ts
   * const types = await client.messageTypes.list(projectId);
   * types.forEach(t => console.log(t.icon, t.name));
   * ```
   */
  async list(projectId: number): Promise<MessageType[]> {
    const response = await this.request(
      {
        service: "MessageTypes",
        operation: "List",
        resourceType: "message_type",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/categories.json", {
          params: { path: { projectId } },
        })
    );

    return (response ?? []) as MessageType[];
  }

  /**
   * Gets a message type by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param typeId - The message type ID
   * @returns The message type
   * @throws BasecampError with code "not_found" if type doesn't exist
   *
   * @example
   * ```ts
   * const type = await client.messageTypes.get(projectId, typeId);
   * console.log(type.name, type.icon);
   * ```
   */
  async get(projectId: number, typeId: number): Promise<MessageType> {
    const response = await this.request(
      {
        service: "MessageTypes",
        operation: "Get",
        resourceType: "message_type",
        isMutation: false,
        projectId,
        resourceId: typeId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/categories/{typeId}", {
          params: { path: { projectId, typeId } },
        })
    );

    return response as unknown as MessageType;
  }

  /**
   * Creates a new message type in a project.
   *
   * @param projectId - The project (bucket) ID
   * @param req - Message type creation parameters
   * @returns The created message type
   * @throws BasecampError with code "validation" if name or icon is missing
   *
   * @example
   * ```ts
   * const type = await client.messageTypes.create(projectId, {
   *   name: "Announcement",
   *   icon: "📢",
   * });
   * ```
   */
  async create(projectId: number, req: CreateMessageTypeRequest): Promise<MessageType> {
    if (!req.name) {
      throw Errors.validation("Message type name is required");
    }
    if (!req.icon) {
      throw Errors.validation("Message type icon is required");
    }

    const response = await this.request(
      {
        service: "MessageTypes",
        operation: "Create",
        resourceType: "message_type",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/categories.json", {
          params: { path: { projectId } },
          body: {
            name: req.name,
            icon: req.icon,
          },
        })
    );

    return response as unknown as MessageType;
  }

  /**
   * Updates an existing message type.
   *
   * @param projectId - The project (bucket) ID
   * @param typeId - The message type ID
   * @param req - Message type update parameters
   * @returns The updated message type
   *
   * @example
   * ```ts
   * const type = await client.messageTypes.update(projectId, typeId, {
   *   name: "Updated Name",
   *   icon: "🎉",
   * });
   * ```
   */
  async update(
    projectId: number,
    typeId: number,
    req: UpdateMessageTypeRequest
  ): Promise<MessageType> {
    const response = await this.request(
      {
        service: "MessageTypes",
        operation: "Update",
        resourceType: "message_type",
        isMutation: true,
        projectId,
        resourceId: typeId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/categories/{typeId}", {
          params: { path: { projectId, typeId } },
          body: {
            name: req.name,
            icon: req.icon,
          },
        })
    );

    return response as unknown as MessageType;
  }

  /**
   * Deletes a message type from a project.
   *
   * @param projectId - The project (bucket) ID
   * @param typeId - The message type ID
   *
   * @example
   * ```ts
   * await client.messageTypes.delete(projectId, typeId);
   * ```
   */
  async delete(projectId: number, typeId: number): Promise<void> {
    await this.request(
      {
        service: "MessageTypes",
        operation: "Delete",
        resourceType: "message_type",
        isMutation: true,
        projectId,
        resourceId: typeId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/categories/{typeId}", {
          params: { path: { projectId, typeId } },
        })
    );
  }
}
