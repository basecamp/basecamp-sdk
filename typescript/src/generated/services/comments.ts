/**
 * Comments service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Comment entity from the Basecamp API. */
export type Comment = components["schemas"]["Comment"];

/**
 * Request parameters for update.
 */
export interface UpdateCommentRequest {
  /** Text content */
  content: string;
}

/**
 * Request parameters for create.
 */
export interface CreateCommentRequest {
  /** Text content */
  content: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Comments operations.
 */
export class CommentsService extends BaseService {

  /**
   * Get a single comment by id
   * @param projectId - The project ID
   * @param commentId - The comment ID
   * @returns The Comment
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.comments.get(123, 123);
   * ```
   */
  async get(projectId: number, commentId: number): Promise<Comment> {
    const response = await this.request(
      {
        service: "Comments",
        operation: "GetComment",
        resourceType: "comment",
        isMutation: false,
        projectId,
        resourceId: commentId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/comments/{commentId}", {
          params: {
            path: { projectId, commentId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing comment
   * @param projectId - The project ID
   * @param commentId - The comment ID
   * @param req - Comment update parameters
   * @returns The Comment
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.comments.update(123, 123, { content: "Hello world" });
   * ```
   */
  async update(projectId: number, commentId: number, req: UpdateCommentRequest): Promise<Comment> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    const response = await this.request(
      {
        service: "Comments",
        operation: "UpdateComment",
        resourceType: "comment",
        isMutation: true,
        projectId,
        resourceId: commentId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/comments/{commentId}", {
          params: {
            path: { projectId, commentId },
          },
          body: {
            content: req.content,
          },
        })
    );
    return response;
  }

  /**
   * List comments on a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @returns Array of Comment
   *
   * @example
   * ```ts
   * const result = await client.comments.list(123, 123);
   * ```
   */
  async list(projectId: number, recordingId: number): Promise<Comment[]> {
    const response = await this.request(
      {
        service: "Comments",
        operation: "ListComments",
        resourceType: "comment",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}/comments.json", {
          params: {
            path: { projectId, recordingId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new comment on a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param req - Comment creation parameters
   * @returns The Comment
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.comments.create(123, 123, { content: "Hello world" });
   * ```
   */
  async create(projectId: number, recordingId: number, req: CreateCommentRequest): Promise<Comment> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    const response = await this.request(
      {
        service: "Comments",
        operation: "CreateComment",
        resourceType: "comment",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/recordings/{recordingId}/comments.json", {
          params: {
            path: { projectId, recordingId },
          },
          body: {
            content: req.content,
          },
        })
    );
    return response;
  }
}