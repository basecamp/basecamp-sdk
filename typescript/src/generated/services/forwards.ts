/**
 * Forwards service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";

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
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for list.
 */
export interface ListForwardOptions extends PaginationOptions {
  /** Filter by sort */
  sort?: "created_at" | "updated_at";
  /** Filter by direction */
  direction?: "asc" | "desc";
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
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
   *
   * // With options
   * const filtered = await client.forwards.listReplies(123, { page: 1 });
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
            query: { page: options?.page },
          },
        })
      , options
    );
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
        resourceId: replyId,
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
   *
   * // With options
   * const filtered = await client.forwards.list(123, { sort: "created_at" });
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
        this.client.GET("/inboxes/{inboxId}/inbox_forwards.json", {
          params: {
            path: { inboxId },
            query: { sort: options?.sort, direction: options?.direction, page: options?.page },
          },
        })
      , options
    );
  }
}