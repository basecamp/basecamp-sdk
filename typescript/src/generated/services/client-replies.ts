/**
 * ClientReplies service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** ClientReply entity from the Basecamp API. */
export type ClientReply = components["schemas"]["ClientReply"];

// =============================================================================
// Service
// =============================================================================

/**
 * Service for ClientReplies operations.
 */
export class ClientRepliesService extends BaseService {

  /**
   * List all client replies for a recording (correspondence or approval)
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @returns Array of ClientReply
   */
  async list(projectId: number, recordingId: number): Promise<ClientReply[]> {
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
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param replyId - The reply ID
   * @returns The ClientReply
   */
  async get(projectId: number, recordingId: number, replyId: number): Promise<ClientReply> {
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