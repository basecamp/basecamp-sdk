/**
 * Service for Forwards operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Forwards operations
 */
export class ForwardsService extends BaseService {

  /**
   * Get a forward by ID
   */
  async get(projectId: number, forwardId: number): Promise<components["schemas"]["GetForwardResponseContent"]> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "GetForward",
        resourceType: "forward",
        isMutation: false,
        projectId,
        resourceId: forwardId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/inbox_forwards/{forwardId}", {
          params: {
            path: { projectId, forwardId },
          },
        })
    );
    return response;
  }

  /**
   * List all replies to a forward
   */
  async listReplies(projectId: number, forwardId: number): Promise<components["schemas"]["ListForwardRepliesResponseContent"]> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "ListForwardReplies",
        resourceType: "forward_replie",
        isMutation: false,
        projectId,
        resourceId: forwardId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json", {
          params: {
            path: { projectId, forwardId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a reply to a forward
   */
  async createReply(projectId: number, forwardId: number, req: components["schemas"]["CreateForwardReplyRequestContent"]): Promise<components["schemas"]["CreateForwardReplyResponseContent"]> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "CreateForwardReply",
        resourceType: "forward_reply",
        isMutation: true,
        projectId,
        resourceId: forwardId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json", {
          params: {
            path: { projectId, forwardId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a forward reply by ID
   */
  async getReply(projectId: number, forwardId: number, replyId: number): Promise<components["schemas"]["GetForwardReplyResponseContent"]> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "GetForwardReply",
        resourceType: "forward_reply",
        isMutation: false,
        projectId,
        resourceId: forwardId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/inbox_forwards/{forwardId}/replies/{replyId}", {
          params: {
            path: { projectId, forwardId, replyId },
          },
        })
    );
    return response;
  }

  /**
   * Get an inbox by ID
   */
  async getInbox(projectId: number, inboxId: number): Promise<components["schemas"]["GetInboxResponseContent"]> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "GetInbox",
        resourceType: "inbox",
        isMutation: false,
        projectId,
        resourceId: inboxId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/inboxes/{inboxId}", {
          params: {
            path: { projectId, inboxId },
          },
        })
    );
    return response;
  }

  /**
   * List all forwards in an inbox
   */
  async list(projectId: number, inboxId: number): Promise<components["schemas"]["ListForwardsResponseContent"]> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "ListForwards",
        resourceType: "forward",
        isMutation: false,
        projectId,
        resourceId: inboxId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/inboxes/{inboxId}/forwards.json", {
          params: {
            path: { projectId, inboxId },
          },
        })
    );
    return response ?? [];
  }
}