/**
 * Client Replies service for the Basecamp API.
 *
 * Client replies are responses to client correspondences or approvals
 * within a project's client portal.
 *
 * @example
 * ```ts
 * const replies = await client.clientReplies.list(projectId, recordingId);
 * const reply = await client.clientReplies.get(projectId, recordingId, replyId);
 * ```
 */

import { BaseService } from "./base.js";

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
 * A reply to a client correspondence or approval.
 */
export interface ClientReply {
  id: number;
  status: string;
  visible_to_clients: boolean;
  created_at: string;
  updated_at: string;
  title: string;
  inherits_status: boolean;
  type: string;
  url: string;
  app_url: string;
  bookmark_url: string;
  content: string;
  parent?: ParentRef;
  bucket?: BucketRef;
  creator?: PersonRef;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing client replies in Basecamp.
 */
export class ClientRepliesService extends BaseService {
  /**
   * Lists all replies for a client recording (correspondence or approval).
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The parent correspondence/approval ID
   * @returns Array of client replies
   *
   * @example
   * ```ts
   * const replies = await client.clientReplies.list(projectId, correspondenceId);
   * replies.forEach(r => console.log(r.creator?.name, r.content));
   * ```
   */
  async list(projectId: number, recordingId: number): Promise<ClientReply[]> {
    const response = await this.request(
      {
        service: "ClientReplies",
        operation: "List",
        resourceType: "client_reply",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/recordings/{recordingId}/replies.json", {
          params: { path: { projectId, recordingId } },
        })
    );

    return (response ?? []) as ClientReply[];
  }

  /**
   * Gets a specific client reply.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The parent correspondence/approval ID
   * @param replyId - The client reply ID
   * @returns The client reply
   * @throws BasecampError with code "not_found" if reply doesn't exist
   *
   * @example
   * ```ts
   * const reply = await client.clientReplies.get(projectId, recordingId, replyId);
   * console.log(reply.content, reply.creator?.name);
   * ```
   */
  async get(projectId: number, recordingId: number, replyId: number): Promise<ClientReply> {
    const response = await this.request(
      {
        service: "ClientReplies",
        operation: "Get",
        resourceType: "client_reply",
        isMutation: false,
        projectId,
        resourceId: replyId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/recordings/{recordingId}/replies/{replyId}", {
          params: { path: { projectId, recordingId, replyId } },
        })
    );

    return response as unknown as ClientReply;
  }
}
