/**
 * Service for MessageTypes operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for MessageTypes operations
 */
export class MessageTypesService extends BaseService {

  /**
   * List message types in a project
   */
  async list(projectId: number): Promise<components["schemas"]["ListMessageTypesResponseContent"]> {
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
   */
  async create(projectId: number, req: components["schemas"]["CreateMessageTypeRequestContent"]): Promise<components["schemas"]["CreateMessageTypeResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a single message type by id
   */
  async get(projectId: number, typeId: number): Promise<components["schemas"]["GetMessageTypeResponseContent"]> {
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
   */
  async update(projectId: number, typeId: number, req: components["schemas"]["UpdateMessageTypeRequestContent"]): Promise<components["schemas"]["UpdateMessageTypeResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Delete a message type
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