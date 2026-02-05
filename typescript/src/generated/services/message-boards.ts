/**
 * MessageBoards service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** MessageBoard entity from the Basecamp API. */
export type MessageBoard = components["schemas"]["MessageBoard"];

// =============================================================================
// Service
// =============================================================================

/**
 * Service for MessageBoards operations.
 */
export class MessageBoardsService extends BaseService {

  /**
   * Get a message board
   * @param projectId - The project ID
   * @param boardId - The board ID
   * @returns The MessageBoard
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.messageBoards.get(123, 123);
   * ```
   */
  async get(projectId: number, boardId: number): Promise<MessageBoard> {
    const response = await this.request(
      {
        service: "MessageBoards",
        operation: "GetMessageBoard",
        resourceType: "message_board",
        isMutation: false,
        projectId,
        resourceId: boardId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/message_boards/{boardId}", {
          params: {
            path: { projectId, boardId },
          },
        })
    );
    return response;
  }
}