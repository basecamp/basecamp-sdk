/**
 * Forwards service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Forward entity from the Basecamp API. */
export type Forward = components["schemas"]["Forward"];
/** ForwardReply entity from the Basecamp API. */
export type ForwardReply = components["schemas"]["ForwardReply"];
/** Inbox entity from the Basecamp API. */
export type Inbox = components["schemas"]["Inbox"];

/**
 * Request parameters for createReply.
 */
export interface CreateReplyForwardRequest {
  /** Text content */
  content: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Forwards operations.
 */
export class ForwardsService extends BaseService {

  /**
   * Get a forward by ID
   * @param projectId - The project ID
   * @param forwardId - The forward ID
   * @returns The Forward
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.forwards.get(123, 123);
   * ```
   */
  async get(projectId: number, forwardId: number): Promise<Forward> {
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
   * @param projectId - The project ID
   * @param forwardId - The forward ID
   * @returns Array of ForwardReply
   *
   * @example
   * ```ts
   * const result = await client.forwards.listReplies(123, 123);
   * ```
   */
  async listReplies(projectId: number, forwardId: number): Promise<ForwardReply[]> {
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
   * @param projectId - The project ID
   * @param forwardId - The forward ID
   * @param req - Forward_reply creation parameters
   * @returns The ForwardReply
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.forwards.createReply(123, 123, { content: "Hello world" });
   * ```
   */
  async createReply(projectId: number, forwardId: number, req: CreateReplyForwardRequest): Promise<ForwardReply> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
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
          body: {
            content: req.content,
          },
        })
    );
    return response;
  }

  /**
   * Get a forward reply by ID
   * @param projectId - The project ID
   * @param forwardId - The forward ID
   * @param replyId - The reply ID
   * @returns The ForwardReply
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.forwards.getReply(123, 123, 123);
   * ```
   */
  async getReply(projectId: number, forwardId: number, replyId: number): Promise<ForwardReply> {
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
   * @param projectId - The project ID
   * @param inboxId - The inbox ID
   * @returns The Inbox
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.forwards.getInbox(123, 123);
   * ```
   */
  async getInbox(projectId: number, inboxId: number): Promise<Inbox> {
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
   * @param projectId - The project ID
   * @param inboxId - The inbox ID
   * @returns Array of Forward
   *
   * @example
   * ```ts
   * const result = await client.forwards.list(123, 123);
   * ```
   */
  async list(projectId: number, inboxId: number): Promise<Forward[]> {
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