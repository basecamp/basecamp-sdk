/**
 * MessageTypes service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** MessageType entity from the Basecamp API. */
export type MessageType = components["schemas"]["MessageType"];

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
   * @returns Array of MessageType
   *
   * @example
   * ```ts
   * const result = await client.messageTypes.list();
   * ```
   */
  async list(): Promise<MessageType[]> {
    const response = await this.request(
      {
        service: "MessageTypes",
        operation: "ListMessageTypes",
        resourceType: "message_type",
        isMutation: false,
      },
      () =>
        this.client.GET("/categories.json", {
        })
    );
    return response ?? [];
  }

  /**
   * Create a new message type in a project
   * @param req - Message_type creation parameters
   * @returns The MessageType
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.messageTypes.create({ name: "My example", icon: "example" });
   * ```
   */
  async create(req: CreateMessageTypeRequest): Promise<MessageType> {
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
      },
      () =>
        this.client.POST("/categories.json", {
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
   * @param typeId - The type ID
   * @returns The MessageType
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.messageTypes.get(123);
   * ```
   */
  async get(typeId: number): Promise<MessageType> {
    const response = await this.request(
      {
        service: "MessageTypes",
        operation: "GetMessageType",
        resourceType: "message_type",
        isMutation: false,
        resourceId: typeId,
      },
      () =>
        this.client.GET("/categories/{typeId}", {
          params: {
            path: { typeId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing message type
   * @param typeId - The type ID
   * @param req - Message_type update parameters
   * @returns The MessageType
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.messageTypes.update(123, { });
   * ```
   */
  async update(typeId: number, req: UpdateMessageTypeRequest): Promise<MessageType> {
    const response = await this.request(
      {
        service: "MessageTypes",
        operation: "UpdateMessageType",
        resourceType: "message_type",
        isMutation: true,
        resourceId: typeId,
      },
      () =>
        this.client.PUT("/categories/{typeId}", {
          params: {
            path: { typeId },
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
   * @param typeId - The type ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.messageTypes.delete(123);
   * ```
   */
  async delete(typeId: number): Promise<void> {
    await this.request(
      {
        service: "MessageTypes",
        operation: "DeleteMessageType",
        resourceType: "message_type",
        isMutation: true,
        resourceId: typeId,
      },
      () =>
        this.client.DELETE("/categories/{typeId}", {
          params: {
            path: { typeId },
          },
        })
    );
  }
}