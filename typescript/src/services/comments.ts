/**
 * Comments service for the Basecamp API.
 *
 * Comments can be added to most recordings (todos, messages, etc.)
 * in Basecamp. They support HTML content.
 *
 * @example
 * ```ts
 * const comments = await client.comments.list(projectId, recordingId);
 * const comment = await client.comments.get(projectId, commentId);
 * await client.comments.create(projectId, recordingId, { content: "Great work!" });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp comment on a recording.
 */
export type Comment = components["schemas"]["Comment"];

/**
 * Request to create a new comment.
 */
export interface CreateCommentRequest {
  /** Comment text in HTML (required) */
  content: string;
}

/**
 * Request to update an existing comment.
 */
export interface UpdateCommentRequest {
  /** Comment text in HTML (required) */
  content: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp comments.
 */
export class CommentsService extends BaseService {
  /**
   * Lists all comments on a recording.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The ID of the recording (todo, message, etc.)
   * @returns Array of comments
   *
   * @example
   * ```ts
   * const comments = await client.comments.list(projectId, todoId);
   * ```
   */
  async list(projectId: number, recordingId: number): Promise<Comment[]> {
    const response = await this.request(
      {
        service: "Comments",
        operation: "List",
        resourceType: "comment",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}/comments.json", {
          params: { path: { projectId, recordingId } },
        })
    );

    return response?.comments ?? [];
  }

  /**
   * Gets a comment by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param commentId - The comment ID
   * @returns The comment
   * @throws BasecampError with code "not_found" if comment doesn't exist
   *
   * @example
   * ```ts
   * const comment = await client.comments.get(projectId, commentId);
   * console.log(comment.content);
   * ```
   */
  async get(projectId: number, commentId: number): Promise<Comment> {
    const response = await this.request(
      {
        service: "Comments",
        operation: "Get",
        resourceType: "comment",
        isMutation: false,
        projectId,
        resourceId: commentId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/comments/{commentId}", {
          params: { path: { projectId, commentId } },
        })
    );

    return response.comment!;
  }

  /**
   * Creates a new comment on a recording.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The ID of the recording to comment on
   * @param req - Comment creation parameters
   * @returns The created comment
   * @throws BasecampError with code "validation" if content is missing
   *
   * @example
   * ```ts
   * const comment = await client.comments.create(projectId, todoId, {
   *   content: "<p>Great work on this task!</p>",
   * });
   * ```
   */
  async create(
    projectId: number,
    recordingId: number,
    req: CreateCommentRequest
  ): Promise<Comment> {
    if (!req.content) {
      throw Errors.validation("Comment content is required");
    }

    const response = await this.request(
      {
        service: "Comments",
        operation: "Create",
        resourceType: "comment",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/recordings/{recordingId}/comments.json", {
          params: { path: { projectId, recordingId } },
          body: {
            content: req.content,
          },
        })
    );

    return response.comment!;
  }

  /**
   * Updates an existing comment.
   *
   * @param projectId - The project (bucket) ID
   * @param commentId - The comment ID
   * @param req - Comment update parameters
   * @returns The updated comment
   * @throws BasecampError with code "validation" if content is missing
   *
   * @example
   * ```ts
   * const comment = await client.comments.update(projectId, commentId, {
   *   content: "<p>Updated comment text</p>",
   * });
   * ```
   */
  async update(
    projectId: number,
    commentId: number,
    req: UpdateCommentRequest
  ): Promise<Comment> {
    if (!req.content) {
      throw Errors.validation("Comment content is required");
    }

    const response = await this.request(
      {
        service: "Comments",
        operation: "Update",
        resourceType: "comment",
        isMutation: true,
        projectId,
        resourceId: commentId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/comments/{commentId}", {
          params: { path: { projectId, commentId } },
          body: {
            content: req.content,
          },
        })
    );

    return response.comment!;
  }

  /**
   * Moves a comment to the trash.
   * Trashed comments can be recovered from the trash.
   *
   * Note: Permanent deletion of comments is not supported by the Basecamp API.
   *
   * @param projectId - The project (bucket) ID
   * @param commentId - The comment ID
   *
   * @example
   * ```ts
   * await client.comments.trash(projectId, commentId);
   * ```
   */
  async trash(projectId: number, commentId: number): Promise<void> {
    await this.request(
      {
        service: "Comments",
        operation: "Trash",
        resourceType: "comment",
        isMutation: true,
        projectId,
        resourceId: commentId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: { path: { projectId, recordingId: commentId } },
        })
    );
  }
}
