/**
 * Service for Comments operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Comments operations
 */
export class CommentsService extends BaseService {

  /**
   * Get a single comment by id
   */
  async get(projectId: number, commentId: number): Promise<components["schemas"]["GetCommentResponseContent"]> {
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
   */
  async update(projectId: number, commentId: number, req: components["schemas"]["UpdateCommentRequestContent"]): Promise<components["schemas"]["UpdateCommentResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * List comments on a recording
   */
  async list(projectId: number, recordingId: number): Promise<components["schemas"]["ListCommentsResponseContent"]> {
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
   */
  async create(projectId: number, recordingId: number, req: components["schemas"]["CreateCommentRequestContent"]): Promise<components["schemas"]["CreateCommentResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }
}