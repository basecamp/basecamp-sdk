/**
 * Service for ClientReplies operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for ClientReplies operations
 */
export class ClientRepliesService extends BaseService {

  /**
   * List all client replies for a recording (correspondence or approval)
   */
  async list(projectId: number, recordingId: number): Promise<components["schemas"]["ListClientRepliesResponseContent"]> {
    const response = await this.request(
      {
        service: "ClientReplies",
        operation: "ListClientReplies",
        resourceType: "client_replie",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/recordings/{recordingId}/replies.json", {
          params: {
            path: { projectId, recordingId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Get a single client reply by id
   */
  async get(projectId: number, recordingId: number, replyId: number): Promise<components["schemas"]["GetClientReplyResponseContent"]> {
    const response = await this.request(
      {
        service: "ClientReplies",
        operation: "GetClientReply",
        resourceType: "client_reply",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/recordings/{recordingId}/replies/{replyId}", {
          params: {
            path: { projectId, recordingId, replyId },
          },
        })
    );
    return response;
  }
}