/**
 * Everything service for the Basecamp API.
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

/** EverythingBoost entity from the Basecamp API. */
export type EverythingBoost = components["schemas"]["EverythingBoost"];
/** Card entity from the Basecamp API. */
export type Card = components["schemas"]["Card"];
/** Recording entity from the Basecamp API. */
export type Recording = components["schemas"]["Recording"];
/** EverythingFile entity from the Basecamp API. */
export type EverythingFile = components["schemas"]["EverythingFile"];
/** Todo entity from the Basecamp API. */
export type Todo = components["schemas"]["Todo"];

/**
 * Options for everythingBoosts.
 */
export interface EverythingBoostsEverythingOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. */
  page?: number;
}

/**
 * Options for everythingCheckins.
 */
export interface EverythingCheckinsEverythingOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. */
  page?: number;
}

/**
 * Options for everythingComments.
 */
export interface EverythingCommentsEverythingOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. */
  page?: number;
}

/**
 * Options for everythingFiles.
 */
export interface EverythingFilesEverythingOptions extends PaginationOptions {
  /** Filter by file kind: all (default), images, pdfs, documents, or videos. */
  kind?: string;
  /** Restrict to files created by the given people (repeatable). */
  peopleIds?: number[];
  /** Page number for paginating through results. Defaults to 1. */
  page?: number;
}

/**
 * Options for everythingForwards.
 */
export interface EverythingForwardsEverythingOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. */
  page?: number;
}

/**
 * Options for everythingMessages.
 */
export interface EverythingMessagesEverythingOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. */
  page?: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Everything operations.
 */
export class EverythingService extends BaseService {

  /**
   * Get every boost across all accessible projects, newest-first (paginated).
   * @param options - Optional query parameters
   * @returns All EverythingBoost across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingBoosts();
   * ```
   */
  async everythingBoosts(options?: EverythingBoostsEverythingOptions): Promise<ListResult<EverythingBoost>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingBoosts",
        resourceType: "everything_boost",
        isMutation: false,
      },
      () =>
        this.client.GET("/boosts.json", {
          params: {
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get every overdue card across all accessible projects, oldest-due-date-first.
   * @returns Array of Card
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingOverdueCards();
   * ```
   */
  async everythingOverdueCards(): Promise<Card[]> {
    const response = await this.request(
      {
        service: "Everything",
        operation: "GetEverythingOverdueCards",
        resourceType: "everything_overdue_card",
        isMutation: false,
      },
      () =>
        this.client.GET("/cards/overdue.json", {
        })
    );
    return response ?? [];
  }

  /**
   * Get every automatic check-in answer across all accessible projects, newest-first.
   * @param options - Optional query parameters
   * @returns All Recording across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingCheckins();
   * ```
   */
  async everythingCheckins(options?: EverythingCheckinsEverythingOptions): Promise<ListResult<Recording>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingCheckins",
        resourceType: "everything_checkin",
        isMutation: false,
      },
      () =>
        this.client.GET("/checkins.json", {
          params: {
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get every comment across all accessible projects, newest-first (paginated).
   * @param options - Optional query parameters
   * @returns All Recording across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingComments();
   * ```
   */
  async everythingComments(options?: EverythingCommentsEverythingOptions): Promise<ListResult<Recording>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingComments",
        resourceType: "everything_comment",
        isMutation: false,
      },
      () =>
        this.client.GET("/comments.json", {
          params: {
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get every file recording across all accessible projects, newest-first (paginated).
   * @param options - Optional query parameters
   * @returns All EverythingFile across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingFiles();
   * ```
   */
  async everythingFiles(options?: EverythingFilesEverythingOptions): Promise<ListResult<EverythingFile>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingFiles",
        resourceType: "everything_file",
        isMutation: false,
      },
      () =>
        this.client.GET("/files.json", {
          params: {
            query: { kind: options?.kind, "people_ids[]": options?.peopleIds, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get every inbox forward across all accessible projects, newest-first (paginated).
   * @param options - Optional query parameters
   * @returns All Recording across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingForwards();
   * ```
   */
  async everythingForwards(options?: EverythingForwardsEverythingOptions): Promise<ListResult<Recording>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingForwards",
        resourceType: "everything_forward",
        isMutation: false,
      },
      () =>
        this.client.GET("/forwards.json", {
          params: {
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get every message across all accessible projects, newest-first (paginated).
   * @param options - Optional query parameters
   * @returns All Recording across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingMessages();
   * ```
   */
  async everythingMessages(options?: EverythingMessagesEverythingOptions): Promise<ListResult<Recording>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingMessages",
        resourceType: "everything_message",
        isMutation: false,
      },
      () =>
        this.client.GET("/messages.json", {
          params: {
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get every overdue to-do across all accessible projects, oldest-due-date-first.
   * @returns Array of Todo
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingOverdueTodos();
   * ```
   */
  async everythingOverdueTodos(): Promise<Todo[]> {
    const response = await this.request(
      {
        service: "Everything",
        operation: "GetEverythingOverdueTodos",
        resourceType: "everything_overdue_todo",
        isMutation: false,
      },
      () =>
        this.client.GET("/todos/overdue.json", {
        })
    );
    return response ?? [];
  }
}