/**
 * Messages service for the Basecamp API.
 *
 * Messages are posts on a project's message board. They have a subject,
 * content, and can be categorized with message types.
 *
 * @example
 * ```ts
 * const messages = await client.messages.list(projectId, boardId);
 * const message = await client.messages.get(projectId, messageId);
 * await client.messages.pin(projectId, messageId);
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp message on a message board.
 */
export type Message = components["schemas"]["Message"];

/**
 * A message type (category) for messages.
 */
export type MessageType = components["schemas"]["MessageType"];

/**
 * Request to create a new message.
 */
export interface CreateMessageRequest {
  /** Message title (required) */
  subject: string;
  /** Message body in HTML (optional) */
  content?: string;
  /** Status: "drafted" or "active" (optional, defaults to active) */
  status?: "drafted" | "active";
  /** Message type ID (optional) */
  categoryId?: number;
}

/**
 * Request to update an existing message.
 */
export interface UpdateMessageRequest {
  /** Message title (optional) */
  subject?: string;
  /** Message body in HTML (optional) */
  content?: string;
  /** Status: "drafted" or "active" (optional) */
  status?: "drafted" | "active";
  /** Message type ID (optional) */
  categoryId?: number;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp messages.
 */
export class MessagesService extends BaseService {
  /**
   * Lists all messages on a message board.
   *
   * @param projectId - The project (bucket) ID
   * @param boardId - The message board ID
   * @returns Array of messages
   *
   * @example
   * ```ts
   * const messages = await client.messages.list(projectId, boardId);
   * ```
   */
  async list(projectId: number, boardId: number): Promise<Message[]> {
    const response = await this.request(
      {
        service: "Messages",
        operation: "List",
        resourceType: "message",
        isMutation: false,
        projectId,
        resourceId: boardId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/message_boards/{boardId}/messages.json", {
          params: { path: { projectId, boardId } },
        })
    );

    return response?.messages ?? [];
  }

  /**
   * Gets a message by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param messageId - The message ID
   * @returns The message
   * @throws BasecampError with code "not_found" if message doesn't exist
   *
   * @example
   * ```ts
   * const message = await client.messages.get(projectId, messageId);
   * console.log(message.subject, message.content);
   * ```
   */
  async get(projectId: number, messageId: number): Promise<Message> {
    const response = await this.request(
      {
        service: "Messages",
        operation: "Get",
        resourceType: "message",
        isMutation: false,
        projectId,
        resourceId: messageId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/messages/{messageId}", {
          params: { path: { projectId, messageId } },
        })
    );

    return response.message!;
  }

  /**
   * Creates a new message on a message board.
   *
   * @param projectId - The project (bucket) ID
   * @param boardId - The message board ID
   * @param req - Message creation parameters
   * @returns The created message
   * @throws BasecampError with code "validation" if subject is missing
   *
   * @example
   * ```ts
   * const message = await client.messages.create(projectId, boardId, {
   *   subject: "Project Update",
   *   content: "<p>Here's what happened this week...</p>",
   * });
   * ```
   */
  async create(projectId: number, boardId: number, req: CreateMessageRequest): Promise<Message> {
    if (!req.subject) {
      throw Errors.validation("Message subject is required");
    }

    const response = await this.request(
      {
        service: "Messages",
        operation: "Create",
        resourceType: "message",
        isMutation: true,
        projectId,
        resourceId: boardId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/message_boards/{boardId}/messages.json", {
          params: { path: { projectId, boardId } },
          body: {
            subject: req.subject,
            content: req.content,
            status: req.status,
            category_id: req.categoryId,
          },
        })
    );

    return response.message!;
  }

  /**
   * Updates an existing message.
   *
   * @param projectId - The project (bucket) ID
   * @param messageId - The message ID
   * @param req - Message update parameters
   * @returns The updated message
   *
   * @example
   * ```ts
   * const message = await client.messages.update(projectId, messageId, {
   *   subject: "Updated Subject",
   * });
   * ```
   */
  async update(projectId: number, messageId: number, req: UpdateMessageRequest): Promise<Message> {
    const response = await this.request(
      {
        service: "Messages",
        operation: "Update",
        resourceType: "message",
        isMutation: true,
        projectId,
        resourceId: messageId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/messages/{messageId}", {
          params: { path: { projectId, messageId } },
          body: {
            subject: req.subject,
            content: req.content,
            status: req.status,
            category_id: req.categoryId,
          },
        })
    );

    return response.message!;
  }

  /**
   * Pins a message to the top of the message board.
   *
   * @param projectId - The project (bucket) ID
   * @param messageId - The message ID
   *
   * @example
   * ```ts
   * await client.messages.pin(projectId, messageId);
   * ```
   */
  async pin(projectId: number, messageId: number): Promise<void> {
    await this.request(
      {
        service: "Messages",
        operation: "Pin",
        resourceType: "message",
        isMutation: true,
        projectId,
        resourceId: messageId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/recordings/{messageId}/pin.json", {
          params: { path: { projectId, messageId } },
        })
    );
  }

  /**
   * Unpins a message from the top of the message board.
   *
   * @param projectId - The project (bucket) ID
   * @param messageId - The message ID
   *
   * @example
   * ```ts
   * await client.messages.unpin(projectId, messageId);
   * ```
   */
  async unpin(projectId: number, messageId: number): Promise<void> {
    await this.request(
      {
        service: "Messages",
        operation: "Unpin",
        resourceType: "message",
        isMutation: true,
        projectId,
        resourceId: messageId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/recordings/{messageId}/pin.json", {
          params: { path: { projectId, messageId } },
        })
    );
  }

  /**
   * Moves a message to the trash.
   * Trashed messages can be recovered from the trash.
   *
   * @param projectId - The project (bucket) ID
   * @param messageId - The message ID
   *
   * @example
   * ```ts
   * await client.messages.trash(projectId, messageId);
   * ```
   */
  async trash(projectId: number, messageId: number): Promise<void> {
    await this.request(
      {
        service: "Messages",
        operation: "Trash",
        resourceType: "message",
        isMutation: true,
        projectId,
        resourceId: messageId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: { path: { projectId, recordingId: messageId } },
        })
    );
  }

  /**
   * Archives a message.
   * Archived messages can be unarchived.
   *
   * @param projectId - The project (bucket) ID
   * @param messageId - The message ID
   *
   * @example
   * ```ts
   * await client.messages.archive(projectId, messageId);
   * ```
   */
  async archive(projectId: number, messageId: number): Promise<void> {
    await this.request(
      {
        service: "Messages",
        operation: "Archive",
        resourceType: "message",
        isMutation: true,
        projectId,
        resourceId: messageId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/archived.json", {
          params: { path: { projectId, recordingId: messageId } },
        })
    );
  }

  /**
   * Restores an archived message to active status.
   *
   * @param projectId - The project (bucket) ID
   * @param messageId - The message ID
   *
   * @example
   * ```ts
   * await client.messages.unarchive(projectId, messageId);
   * ```
   */
  async unarchive(projectId: number, messageId: number): Promise<void> {
    await this.request(
      {
        service: "Messages",
        operation: "Unarchive",
        resourceType: "message",
        isMutation: true,
        projectId,
        resourceId: messageId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/active.json", {
          params: { path: { projectId, recordingId: messageId } },
        })
    );
  }
}
