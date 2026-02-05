/**
 * Messages service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Message entity from the Basecamp API. */
export type Message = components["schemas"]["Message"];

/**
 * Request parameters for create.
 */
export interface CreateMessageRequest {
  /** Subject line */
  subject: string;
  /** Text content */
  content?: string;
  /** Status */
  status?: "active" | "drafted";
  /** Category id */
  categoryId?: number;
}

/**
 * Request parameters for update.
 */
export interface UpdateMessageRequest {
  /** Subject line */
  subject?: string;
  /** Text content */
  content?: string;
  /** Status */
  status?: "active" | "drafted";
  /** Category id */
  categoryId?: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Messages operations.
 */
export class MessagesService extends BaseService {

  /**
   * List messages on a message board
   * @param projectId - The project ID
   * @param boardId - The board ID
   * @returns Array of Message
   *
   * @example
   * ```ts
   * const result = await client.messages.list(123, 123);
   * ```
   */
  async list(projectId: number, boardId: number): Promise<Message[]> {
    const response = await this.request(
      {
        service: "Messages",
        operation: "ListMessages",
        resourceType: "message",
        isMutation: false,
        projectId,
        resourceId: boardId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/message_boards/{boardId}/messages.json", {
          params: {
            path: { projectId, boardId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new message on a message board
   * @param projectId - The project ID
   * @param boardId - The board ID
   * @param req - Message creation parameters
   * @returns The Message
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.messages.create(123, 123, { subject: "example" });
   * ```
   */
  async create(projectId: number, boardId: number, req: CreateMessageRequest): Promise<Message> {
    if (!req.subject) {
      throw Errors.validation("Subject is required");
    }
    const response = await this.request(
      {
        service: "Messages",
        operation: "CreateMessage",
        resourceType: "message",
        isMutation: true,
        projectId,
        resourceId: boardId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/message_boards/{boardId}/messages.json", {
          params: {
            path: { projectId, boardId },
          },
          body: {
            subject: req.subject,
            content: req.content,
            status: req.status,
            category_id: req.categoryId,
          },
        })
    );
    return response;
  }

  /**
   * Get a single message by id
   * @param projectId - The project ID
   * @param messageId - The message ID
   * @returns The Message
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.messages.get(123, 123);
   * ```
   */
  async get(projectId: number, messageId: number): Promise<Message> {
    const response = await this.request(
      {
        service: "Messages",
        operation: "GetMessage",
        resourceType: "message",
        isMutation: false,
        projectId,
        resourceId: messageId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/messages/{messageId}", {
          params: {
            path: { projectId, messageId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing message
   * @param projectId - The project ID
   * @param messageId - The message ID
   * @param req - Message update parameters
   * @returns The Message
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.messages.update(123, 123, { });
   * ```
   */
  async update(projectId: number, messageId: number, req: UpdateMessageRequest): Promise<Message> {
    const response = await this.request(
      {
        service: "Messages",
        operation: "UpdateMessage",
        resourceType: "message",
        isMutation: true,
        projectId,
        resourceId: messageId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/messages/{messageId}", {
          params: {
            path: { projectId, messageId },
          },
          body: {
            subject: req.subject,
            content: req.content,
            status: req.status,
            category_id: req.categoryId,
          },
        })
    );
    return response;
  }

  /**
   * Pin a message to the top of the message board
   * @param projectId - The project ID
   * @param messageId - The message ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.messages.pin(123, 123);
   * ```
   */
  async pin(projectId: number, messageId: number): Promise<void> {
    await this.request(
      {
        service: "Messages",
        operation: "PinMessage",
        resourceType: "message",
        isMutation: true,
        projectId,
        resourceId: messageId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/recordings/{messageId}/pin.json", {
          params: {
            path: { projectId, messageId },
          },
        })
    );
  }

  /**
   * Unpin a message from the message board
   * @param projectId - The project ID
   * @param messageId - The message ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.messages.unpin(123, 123);
   * ```
   */
  async unpin(projectId: number, messageId: number): Promise<void> {
    await this.request(
      {
        service: "Messages",
        operation: "UnpinMessage",
        resourceType: "message",
        isMutation: true,
        projectId,
        resourceId: messageId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/recordings/{messageId}/pin.json", {
          params: {
            path: { projectId, messageId },
          },
        })
    );
  }
}