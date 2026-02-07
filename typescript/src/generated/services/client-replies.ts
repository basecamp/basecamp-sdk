/**
 * ClientReplies service for the Basecamp API.
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

/** ClientReply entity from the Basecamp API. */
export type ClientReply = components["schemas"]["ClientReply"];

/**
 * Options for list.
 */
export interface ListClientReplyOptions extends PaginationOptions {
}


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
   * @param options - Optional query parameters
   * @returns All ClientReply across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.clientReplies.list(123, 123);
   * ```
   */
  async list(projectId: number, recordingId: number, options?: ListClientReplyOptions): Promise<ListResult<ClientReply>> {
    return this.requestPaginated(
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
      , options
    );
  }

  /**
   * Get a single client reply by id
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param replyId - The reply ID
   * @returns The ClientReply
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.clientReplies.get(123, 123, 123);
   * ```
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