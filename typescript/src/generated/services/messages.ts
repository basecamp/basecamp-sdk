/**
 * Service for Messages operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Messages operations
 */
export class MessagesService extends BaseService {

  /**
   * List messages on a message board
   */
  async list(projectId: number, boardId: number): Promise<components["schemas"]["ListMessagesResponseContent"]> {
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
   */
  async create(projectId: number, boardId: number, req: components["schemas"]["CreateMessageRequestContent"]): Promise<components["schemas"]["CreateMessageResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a single message by id
   */
  async get(projectId: number, messageId: number): Promise<components["schemas"]["GetMessageResponseContent"]> {
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
   */
  async update(projectId: number, messageId: number, req: components["schemas"]["UpdateMessageRequestContent"]): Promise<components["schemas"]["UpdateMessageResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Pin a message to the top of the message board
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