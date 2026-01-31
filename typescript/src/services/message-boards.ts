/**
 * Message Boards service for the Basecamp API.
 *
 * Message boards are containers for messages in a project. Each project
 * has one message board where team members can post messages.
 *
 * @example
 * ```ts
 * const board = await client.messageBoards.get(projectId, boardId);
 * console.log(board.messages_count);
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
 * A Basecamp message board.
 */
export interface MessageBoard {
  id: number;
  status: string;
  title: string;
  created_at: string;
  updated_at: string;
  type: string;
  url: string;
  app_url: string;
  messages_count: number;
  messages_url: string;
  bucket?: BucketRef;
  creator?: PersonRef;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for accessing Basecamp message boards.
 */
export class MessageBoardsService extends BaseService {
  /**
   * Gets a message board by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param boardId - The message board ID
   * @returns The message board
   * @throws BasecampError with code "not_found" if board doesn't exist
   *
   * @example
   * ```ts
   * const board = await client.messageBoards.get(projectId, boardId);
   * console.log(board.title, board.messages_count);
   * ```
   */
  async get(projectId: number, boardId: number): Promise<MessageBoard> {
    const response = await this.request(
      {
        service: "MessageBoards",
        operation: "Get",
        resourceType: "message_board",
        isMutation: false,
        projectId,
        resourceId: boardId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/message_boards/{boardId}", {
          params: { path: { projectId, boardId } },
        })
    );

    return response as unknown as MessageBoard;
  }
}
