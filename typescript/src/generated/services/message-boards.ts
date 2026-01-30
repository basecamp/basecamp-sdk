/**
 * Service for MessageBoards operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for MessageBoards operations
 */
export class MessageBoardsService extends BaseService {

  /**
   * Get a message board
   */
  async get(projectId: number, boardId: number): Promise<components["schemas"]["GetMessageBoardResponseContent"]> {
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