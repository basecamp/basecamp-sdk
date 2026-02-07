/**
 * MessageTypes service for the Basecamp API.
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

/** MessageType entity from the Basecamp API. */
export type MessageType = components["schemas"]["MessageType"];

/**
 * Options for list.
 */
export interface ListMessageTypeOptions extends PaginationOptions {
}

/**
 * Request parameters for create.
 */
export interface CreateMessageTypeRequest {
  /** Display name */
  name: string;
  /** Icon identifier */
  icon: string;
}

/**
 * Request parameters for update.
 */
export interface UpdateMessageTypeRequest {
  /** Display name */
  name?: string;
  /** Icon identifier */
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
   * @param options - Optional query parameters
   * @returns All MessageType across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.messageTypes.list(123);
   * ```
   */
  async list(projectId: number, options?: ListMessageTypeOptions): Promise<ListResult<MessageType>> {
    return this.requestPaginated(
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
      , options
    );
  }

  /**
   * Create a new message type in a project
   * @param projectId - The project ID
   * @param req - Message_type creation parameters
   * @returns The MessageType
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.messageTypes.create(123, { name: "My example", icon: "example" });
   * ```
   */
  async create(projectId: number, req: CreateMessageTypeRequest): Promise<MessageType> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    if (!req.icon) {
      throw Errors.validation("Icon is required");
    }
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
          body: {
            name: req.name,
            icon: req.icon,
          },
        })
    );
    return response;
  }

  /**
   * Get a single message type by id
   * @param projectId - The project ID
   * @param typeId - The type ID
   * @returns The MessageType
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.messageTypes.get(123, 123);
   * ```
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
   * @param req - Message_type update parameters
   * @returns The MessageType
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.messageTypes.update(123, 123, { });
   * ```
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
          body: {
            name: req.name,
            icon: req.icon,
          },
        })
    );
    return response;
  }

  /**
   * Delete a message type
   * @param projectId - The project ID
   * @param typeId - The type ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.messageTypes.delete(123, 123);
   * ```
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