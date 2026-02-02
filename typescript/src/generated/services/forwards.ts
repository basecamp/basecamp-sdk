/**
 * Forwards service for the Basecamp API.
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

/** Forward entity from the Basecamp API. */
export type Forward = components["schemas"]["Forward"];
/** ForwardReply entity from the Basecamp API. */
export type ForwardReply = components["schemas"]["ForwardReply"];
/** Inbox entity from the Basecamp API. */
export type Inbox = components["schemas"]["Inbox"];

/**
 * Options for listReplies.
 */
export interface ListRepliesForwardOptions extends PaginationOptions {
}

/**
 * Request parameters for createReply.
 */
export interface CreateReplyForwardRequest {
  /** Text content */
  content: string;
}

/**
 * Options for list.
 */
export interface ListForwardOptions extends PaginationOptions {
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
   * @param forwardId - The forward ID
   * @returns The Forward
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.forwards.get(123);
   * ```
   */
  async get(forwardId: number): Promise<Forward> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "GetForward",
        resourceType: "forward",
        isMutation: false,
        resourceId: forwardId,
      },
      () =>
        this.client.GET("/inbox_forwards/{forwardId}", {
          params: {
            path: { forwardId },
          },
        })
    );
    return response;
  }

  /**
   * List all replies to a forward
   * @param forwardId - The forward ID
   * @param options - Optional query parameters
   * @returns All ForwardReply across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.forwards.listReplies(123);
   * ```
   */
  async listReplies(forwardId: number, options?: ListRepliesForwardOptions): Promise<ListResult<ForwardReply>> {
    return this.requestPaginated(
      {
        service: "Forwards",
        operation: "ListForwardReplies",
        resourceType: "forward_replie",
        isMutation: false,
        resourceId: forwardId,
      },
      () =>
        this.client.GET("/inbox_forwards/{forwardId}/replies.json", {
          params: {
            path: { forwardId },
          },
        })
      , options
    );
  }

  /**
   * Create a reply to a forward
   * @param forwardId - The forward ID
   * @param req - Forward_reply creation parameters
   * @returns The ForwardReply
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.forwards.createReply(123, { content: "Hello world" });
   * ```
   */
  async createReply(forwardId: number, req: CreateReplyForwardRequest): Promise<ForwardReply> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    const response = await this.request(
      {
        service: "Forwards",
        operation: "CreateForwardReply",
        resourceType: "forward_reply",
        isMutation: true,
        resourceId: forwardId,
      },
      () =>
        this.client.POST("/inbox_forwards/{forwardId}/replies.json", {
          params: {
            path: { forwardId },
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
   * @param forwardId - The forward ID
   * @param replyId - The reply ID
   * @returns The ForwardReply
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.forwards.getReply(123, 123);
   * ```
   */
  async getReply(forwardId: number, replyId: number): Promise<ForwardReply> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "GetForwardReply",
        resourceType: "forward_reply",
        isMutation: false,
        resourceId: forwardId,
      },
      () =>
        this.client.GET("/inbox_forwards/{forwardId}/replies/{replyId}", {
          params: {
            path: { forwardId, replyId },
          },
        })
    );
    return response;
  }

  /**
   * Get an inbox by ID
   * @param inboxId - The inbox ID
   * @returns The Inbox
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.forwards.getInbox(123);
   * ```
   */
  async getInbox(inboxId: number): Promise<Inbox> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "GetInbox",
        resourceType: "inbox",
        isMutation: false,
        resourceId: inboxId,
      },
      () =>
        this.client.GET("/inboxes/{inboxId}", {
          params: {
            path: { inboxId },
          },
        })
    );
    return response;
  }

  /**
   * List all forwards in an inbox
   * @param inboxId - The inbox ID
   * @param options - Optional query parameters
   * @returns All Forward across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.forwards.list(123);
   * ```
   */
  async list(inboxId: number, options?: ListForwardOptions): Promise<ListResult<Forward>> {
    return this.requestPaginated(
      {
        service: "Forwards",
        operation: "ListForwards",
        resourceType: "forward",
        isMutation: false,
        resourceId: inboxId,
      },
      () =>
        this.client.GET("/inboxes/{inboxId}/forwards.json", {
          params: {
            path: { inboxId },
          },
        })
      , options
    );
  }
}