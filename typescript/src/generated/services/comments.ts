/**
 * Comments service for the Basecamp API.
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
 * Options for list.
 */
export interface ListCommentOptions extends PaginationOptions {
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
   * @param commentId - The comment ID
   * @returns The Comment
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.comments.get(123);
   * ```
   */
  async get(commentId: number): Promise<Comment> {
    const response = await this.request(
      {
        service: "Comments",
        operation: "GetComment",
        resourceType: "comment",
        isMutation: false,
        resourceId: commentId,
      },
      () =>
        this.client.GET("/comments/{commentId}", {
          params: {
            path: { commentId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing comment
   * @param commentId - The comment ID
   * @param req - Comment update parameters
   * @returns The Comment
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.comments.update(123, { content: "Hello world" });
   * ```
   */
  async update(commentId: number, req: UpdateCommentRequest): Promise<Comment> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    const response = await this.request(
      {
        service: "Comments",
        operation: "UpdateComment",
        resourceType: "comment",
        isMutation: true,
        resourceId: commentId,
      },
      () =>
        this.client.PUT("/comments/{commentId}", {
          params: {
            path: { commentId },
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
   * @param recordingId - The recording ID
   * @param options - Optional query parameters
   * @returns All Comment across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.comments.list(123);
   * ```
   */
  async list(recordingId: number, options?: ListCommentOptions): Promise<ListResult<Comment>> {
    return this.requestPaginated(
      {
        service: "Comments",
        operation: "ListComments",
        resourceType: "comment",
        isMutation: false,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/recordings/{recordingId}/comments.json", {
          params: {
            path: { recordingId },
          },
        })
      , options
    );
  }

  /**
   * Create a new comment on a recording
   * @param recordingId - The recording ID
   * @param req - Comment creation parameters
   * @returns The Comment
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.comments.create(123, { content: "Hello world" });
   * ```
   */
  async create(recordingId: number, req: CreateCommentRequest): Promise<Comment> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    const response = await this.request(
      {
        service: "Comments",
        operation: "CreateComment",
        resourceType: "comment",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/recordings/{recordingId}/comments.json", {
          params: {
            path: { recordingId },
          },
          body: {
            content: req.content,
          },
        })
    );
    return response;
  }
}