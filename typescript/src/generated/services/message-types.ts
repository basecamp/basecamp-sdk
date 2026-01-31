/**
 * MessageTypes service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** MessageType entity from the Basecamp API. */
export type MessageType = components["schemas"]["MessageType"];

/**
 * Request parameters for create.
 */
export interface CreateMessageTypeRequest {
  /** name */
  name: string;
  /** icon */
  icon: string;
}

/**
 * Request parameters for update.
 */
export interface UpdateMessageTypeRequest {
  /** name */
  name?: string;
  /** icon */
  icon?: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for MessageTypes operations.
 */
export class MessageTypesService extends BaseService {

  /**
   * List message types in a project
   * @param projectId - The project ID
   * @returns Array of MessageType
   */
  async list(projectId: number): Promise<MessageType[]> {
    const response = await this.request(
      {
        service: "MessageTypes",
        operation: "ListMessageTypes",
        resourceType: "message_type",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/categories.json", {
          params: {
            path: { projectId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new message type in a project
   * @param projectId - The project ID
   * @param req - Request parameters
   * @returns The MessageType
   *
   * @example
   * ```ts
   * const result = await client.messageTypes.create(123, { ... });
   * ```
   */
  async create(projectId: number, req: CreateMessageTypeRequest): Promise<MessageType> {
    const response = await this.request(
      {
        service: "MessageTypes",
        operation: "CreateMessageType",
        resourceType: "message_type",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/categories.json", {
          params: {
            path: { projectId },
          },
          body: req as any,
        })
    );
    return response;
  }

  /**
   * Get a single message type by id
   * @param projectId - The project ID
   * @param typeId - The type ID
   * @returns The MessageType
   */
  async get(projectId: number, typeId: number): Promise<MessageType> {
    const response = await this.request(
      {
        service: "MessageTypes",
        operation: "GetMessageType",
        resourceType: "message_type",
        isMutation: false,
        projectId,
        resourceId: typeId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/categories/{typeId}", {
          params: {
            path: { projectId, typeId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing message type
   * @param projectId - The project ID
   * @param typeId - The type ID
   * @param req - Request parameters
   * @returns The MessageType
   */
  async update(projectId: number, typeId: number, req: UpdateMessageTypeRequest): Promise<MessageType> {
    const response = await this.request(
      {
        service: "MessageTypes",
        operation: "UpdateMessageType",
        resourceType: "message_type",
        isMutation: true,
        projectId,
        resourceId: typeId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/categories/{typeId}", {
          params: {
            path: { projectId, typeId },
          },
          body: req as any,
        })
    );
    return response;
  }

  /**
   * Delete a message type
   * @param projectId - The project ID
   * @param typeId - The type ID
   * @returns void
   */
  async delete(projectId: number, typeId: number): Promise<void> {
    await this.request(
      {
        service: "MessageTypes",
        operation: "DeleteMessageType",
        resourceType: "message_type",
        isMutation: true,
        projectId,
        resourceId: typeId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/categories/{typeId}", {
          params: {
            path: { projectId, typeId },
          },
        })
    );
  }
}