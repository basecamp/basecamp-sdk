/**
 * Forwards service for the Basecamp API.
 *
 * Forwards are emails that have been forwarded to a project's inbox.
 * Team members can reply to forwarded emails from within Basecamp.
 *
 * @example
 * ```ts
 * const inbox = await client.forwards.getInbox(projectId, inboxId);
 * const forwards = await client.forwards.list(projectId, inboxId);
 * const reply = await client.forwards.createReply(projectId, forwardId, {
 *   content: "<p>Thanks for reaching out!</p>",
 * });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A person reference (simplified).
 */
export interface PersonRef {
  id: number;
  name: string;
  email_address?: string;
  avatar_url?: string;
  admin?: boolean;
  owner?: boolean;
}

/**
 * A bucket (project) reference.
 */
export interface BucketRef {
  id: number;
  name: string;
  type: string;
}

/**
 * A parent reference.
 */
export interface ParentRef {
  id: number;
  title: string;
  type: string;
  url: string;
  app_url: string;
}

/**
 * A Basecamp inbox (forwards tool).
 */
export interface Inbox {
  id: number;
  status: string;
  created_at: string;
  updated_at: string;
  title: string;
  type: string;
  url: string;
  app_url: string;
  bucket?: BucketRef;
  creator?: PersonRef;
}

/**
 * A forwarded email in Basecamp.
 */
export interface Forward {
  id: number;
  status: string;
  created_at: string;
  updated_at: string;
  subject: string;
  content: string;
  from: string;
  type: string;
  url: string;
  app_url: string;
  parent?: ParentRef;
  bucket?: BucketRef;
  creator?: PersonRef;
}

/**
 * A reply to a forwarded email.
 */
export interface ForwardReply {
  id: number;
  status: string;
  created_at: string;
  updated_at: string;
  content: string;
  type: string;
  url: string;
  app_url: string;
  parent?: ParentRef;
  bucket?: BucketRef;
  creator?: PersonRef;
}

/**
 * Request to create a reply to a forwarded email.
 */
export interface CreateForwardReplyRequest {
  /** Reply body in HTML (required) */
  content: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing email forwards in Basecamp.
 */
export class ForwardsService extends BaseService {
  /**
   * Gets an inbox by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param inboxId - The inbox ID
   * @returns The inbox
   * @throws BasecampError with code "not_found" if inbox doesn't exist
   *
   * @example
   * ```ts
   * const inbox = await client.forwards.getInbox(projectId, inboxId);
   * console.log(inbox.title);
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
          params: { path: { projectId, inboxId } },
        })
    );

    return response as unknown as Inbox;
  }

  /**
   * Lists all forwards in an inbox.
   *
   * @param projectId - The project (bucket) ID
   * @param inboxId - The inbox ID
   * @returns Array of forwards
   *
   * @example
   * ```ts
   * const forwards = await client.forwards.list(projectId, inboxId);
   * forwards.forEach(f => console.log(f.subject, f.from));
   * ```
   */
  async list(projectId: number, inboxId: number): Promise<Forward[]> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "List",
        resourceType: "forward",
        isMutation: false,
        projectId,
        resourceId: inboxId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/inboxes/{inboxId}/forwards.json", {
          params: { path: { projectId, inboxId } },
        })
    );

    return (response ?? []) as Forward[];
  }

  /**
   * Gets a forward by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param forwardId - The forward ID
   * @returns The forward
   * @throws BasecampError with code "not_found" if forward doesn't exist
   *
   * @example
   * ```ts
   * const forward = await client.forwards.get(projectId, forwardId);
   * console.log(forward.subject, forward.content);
   * ```
   */
  async get(projectId: number, forwardId: number): Promise<Forward> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "Get",
        resourceType: "forward",
        isMutation: false,
        projectId,
        resourceId: forwardId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/inbox_forwards/{forwardId}", {
          params: { path: { projectId, forwardId } },
        })
    );

    return response as unknown as Forward;
  }

  /**
   * Lists all replies to a forward.
   *
   * @param projectId - The project (bucket) ID
   * @param forwardId - The forward ID
   * @returns Array of replies
   *
   * @example
   * ```ts
   * const replies = await client.forwards.listReplies(projectId, forwardId);
   * replies.forEach(r => console.log(r.content));
   * ```
   */
  async listReplies(projectId: number, forwardId: number): Promise<ForwardReply[]> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "ListReplies",
        resourceType: "forward_reply",
        isMutation: false,
        projectId,
        resourceId: forwardId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json", {
          params: { path: { projectId, forwardId } },
        })
    );

    return (response ?? []) as ForwardReply[];
  }

  /**
   * Gets a forward reply by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param forwardId - The forward ID
   * @param replyId - The reply ID
   * @returns The reply
   * @throws BasecampError with code "not_found" if reply doesn't exist
   *
   * @example
   * ```ts
   * const reply = await client.forwards.getReply(projectId, forwardId, replyId);
   * console.log(reply.content);
   * ```
   */
  async getReply(projectId: number, forwardId: number, replyId: number): Promise<ForwardReply> {
    const response = await this.request(
      {
        service: "Forwards",
        operation: "GetReply",
        resourceType: "forward_reply",
        isMutation: false,
        projectId,
        resourceId: replyId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/inbox_forwards/{forwardId}/replies/{replyId}", {
          params: { path: { projectId, forwardId, replyId } },
        })
    );

    return response as unknown as ForwardReply;
  }

  /**
   * Creates a reply to a forwarded email.
   *
   * @param projectId - The project (bucket) ID
   * @param forwardId - The forward ID
   * @param req - Reply creation parameters
   * @returns The created reply
   * @throws BasecampError with code "validation" if content is missing
   *
   * @example
   * ```ts
   * const reply = await client.forwards.createReply(projectId, forwardId, {
   *   content: "<p>Thanks for your email!</p>",
   * });
   * ```
   */
  async createReply(
    projectId: number,
    forwardId: number,
    req: CreateForwardReplyRequest
  ): Promise<ForwardReply> {
    if (!req.content) {
      throw Errors.validation("Reply content is required");
    }

    const response = await this.request(
      {
        service: "Forwards",
        operation: "CreateReply",
        resourceType: "forward_reply",
        isMutation: true,
        projectId,
        resourceId: forwardId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json", {
          params: { path: { projectId, forwardId } },
          body: {
            content: req.content,
          },
        })
    );

    return response as unknown as ForwardReply;
  }
}
