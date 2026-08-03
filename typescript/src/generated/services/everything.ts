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

/** BucketCardsGroup entity from the Basecamp API. */
export type BucketCardsGroup = components["schemas"]["BucketCardsGroup"];
/** Card entity from the Basecamp API. */
export type Card = components["schemas"]["Card"];
/** Recording entity from the Basecamp API. */
export type Recording = components["schemas"]["Recording"];
/** EverythingFile entity from the Basecamp API. */
export type EverythingFile = components["schemas"]["EverythingFile"];
/** BucketTodosGroup entity from the Basecamp API. */
export type BucketTodosGroup = components["schemas"]["BucketTodosGroup"];
/** Todo entity from the Basecamp API. */
export type Todo = components["schemas"]["Todo"];

/**
 * Options for everythingCompletedCards.
 */
export interface EverythingCompletedCardsEverythingOptions extends PaginationOptions {
  /** Restrict to tasks assigned to at least one of the given people (repeatable).
Assignees on nested steps are not considered. */
  assigneeIds?: number[];
  /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
  due?: string;
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingNoDueDateCards.
 */
export interface EverythingNoDueDateCardsEverythingOptions extends PaginationOptions {
  /** Restrict to tasks assigned to at least one of the given people (repeatable).
Assignees on nested steps are not considered. */
  assigneeIds?: number[];
  /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
  due?: string;
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingNotNowCards.
 */
export interface EverythingNotNowCardsEverythingOptions extends PaginationOptions {
  /** Restrict to tasks assigned to at least one of the given people (repeatable).
Assignees on nested steps are not considered. */
  assigneeIds?: number[];
  /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
  due?: string;
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingOpenCards.
 */
export interface EverythingOpenCardsEverythingOptions extends PaginationOptions {
  /** Restrict to tasks assigned to at least one of the given people (repeatable).
Assignees on nested steps are not considered. */
  assigneeIds?: number[];
  /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
  due?: string;
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingOverdueCards.
 */
export interface EverythingOverdueCardsEverythingOptions {
  /** Restrict to tasks assigned to at least one of the given people (repeatable).
Assignees on nested steps are not considered. */
  assigneeIds?: number[];
  /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
  due?: string;
}

/**
 * Options for everythingUnassignedCards.
 */
export interface EverythingUnassignedCardsEverythingOptions extends PaginationOptions {
  /** Restrict to tasks assigned to at least one of the given people (repeatable).
Assignees on nested steps are not considered. */
  assigneeIds?: number[];
  /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
  due?: string;
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingCheckins.
 */
export interface EverythingCheckinsEverythingOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingComments.
 */
export interface EverythingCommentsEverythingOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
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
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingForwards.
 */
export interface EverythingForwardsEverythingOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingMessages.
 */
export interface EverythingMessagesEverythingOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingCompletedTodos.
 */
export interface EverythingCompletedTodosEverythingOptions extends PaginationOptions {
  /** Restrict to tasks assigned to at least one of the given people (repeatable).
Assignees on nested steps are not considered. */
  assigneeIds?: number[];
  /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
  due?: string;
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingNoDueDateTodos.
 */
export interface EverythingNoDueDateTodosEverythingOptions extends PaginationOptions {
  /** Restrict to tasks assigned to at least one of the given people (repeatable).
Assignees on nested steps are not considered. */
  assigneeIds?: number[];
  /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
  due?: string;
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingOpenTodos.
 */
export interface EverythingOpenTodosEverythingOptions extends PaginationOptions {
  /** Restrict to tasks assigned to at least one of the given people (repeatable).
Assignees on nested steps are not considered. */
  assigneeIds?: number[];
  /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
  due?: string;
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for everythingOverdueTodos.
 */
export interface EverythingOverdueTodosEverythingOptions {
  /** Restrict to tasks assigned to at least one of the given people (repeatable).
Assignees on nested steps are not considered. */
  assigneeIds?: number[];
  /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
  due?: string;
}

/**
 * Options for everythingUnassignedTodos.
 */
export interface EverythingUnassignedTodosEverythingOptions extends PaginationOptions {
  /** Restrict to tasks assigned to at least one of the given people (repeatable).
Assignees on nested steps are not considered. */
  assigneeIds?: number[];
  /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
  due?: string;
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
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
   * Completed cards across all accessible projects, grouped by project (paginated).
   * @param options - Optional query parameters
   * @returns All BucketCardsGroup across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingCompletedCards();
   * ```
   */
  async everythingCompletedCards(options?: EverythingCompletedCardsEverythingOptions): Promise<ListResult<BucketCardsGroup>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingCompletedCards",
        resourceType: "everything_completed_card",
        isMutation: false,
      },
      () =>
        this.client.GET("/cards/completed.json", {
          params: {
            query: { "assignee_ids[]": options?.assigneeIds, due: options?.due, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Open cards with no due date across all accessible projects, grouped by project (paginated).
   * @param options - Optional query parameters
   * @returns All BucketCardsGroup across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingNoDueDateCards();
   * ```
   */
  async everythingNoDueDateCards(options?: EverythingNoDueDateCardsEverythingOptions): Promise<ListResult<BucketCardsGroup>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingNoDueDateCards",
        resourceType: "everything_no_due_date_card",
        isMutation: false,
      },
      () =>
        this.client.GET("/cards/no_due_date.json", {
          params: {
            query: { "assignee_ids[]": options?.assigneeIds, due: options?.due, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Cards parked in a project's "Not now" column across all accessible projects, grouped by project (paginated).
   * @param options - Optional query parameters
   * @returns All BucketCardsGroup across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingNotNowCards();
   * ```
   */
  async everythingNotNowCards(options?: EverythingNotNowCardsEverythingOptions): Promise<ListResult<BucketCardsGroup>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingNotNowCards",
        resourceType: "everything_not_now_card",
        isMutation: false,
      },
      () =>
        this.client.GET("/cards/not_now.json", {
          params: {
            query: { "assignee_ids[]": options?.assigneeIds, due: options?.due, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Incomplete cards in active columns across all accessible projects, grouped by project (paginated).
   * @param options - Optional query parameters
   * @returns All BucketCardsGroup across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingOpenCards();
   * ```
   */
  async everythingOpenCards(options?: EverythingOpenCardsEverythingOptions): Promise<ListResult<BucketCardsGroup>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingOpenCards",
        resourceType: "everything_open_card",
        isMutation: false,
      },
      () =>
        this.client.GET("/cards/open.json", {
          params: {
            query: { "assignee_ids[]": options?.assigneeIds, due: options?.due, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get every overdue card across all accessible projects, oldest-due-date-first.
   * @param options - Optional query parameters
   * @returns Array of Card
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingOverdueCards();
   * ```
   */
  async everythingOverdueCards(options?: EverythingOverdueCardsEverythingOptions): Promise<Card[]> {
    const response = await this.request(
      {
        service: "Everything",
        operation: "GetEverythingOverdueCards",
        resourceType: "everything_overdue_card",
        isMutation: false,
      },
      () =>
        this.client.GET("/cards/overdue.json", {
          params: {
            query: { "assignee_ids[]": options?.assigneeIds, due: options?.due },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Open, unassigned cards across all accessible projects, grouped by project (paginated).
   * @param options - Optional query parameters
   * @returns All BucketCardsGroup across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingUnassignedCards();
   * ```
   */
  async everythingUnassignedCards(options?: EverythingUnassignedCardsEverythingOptions): Promise<ListResult<BucketCardsGroup>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingUnassignedCards",
        resourceType: "everything_unassigned_card",
        isMutation: false,
      },
      () =>
        this.client.GET("/cards/unassigned.json", {
          params: {
            query: { "assignee_ids[]": options?.assigneeIds, due: options?.due, page: options?.page },
          },
        })
      , options
    );
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
   * Completed to-dos across all accessible projects, grouped by project (paginated).
   * @param options - Optional query parameters
   * @returns All BucketTodosGroup across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingCompletedTodos();
   * ```
   */
  async everythingCompletedTodos(options?: EverythingCompletedTodosEverythingOptions): Promise<ListResult<BucketTodosGroup>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingCompletedTodos",
        resourceType: "everything_completed_todo",
        isMutation: false,
      },
      () =>
        this.client.GET("/todos/completed.json", {
          params: {
            query: { "assignee_ids[]": options?.assigneeIds, due: options?.due, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Open to-dos with no due date across all accessible projects, grouped by project (paginated).
   * @param options - Optional query parameters
   * @returns All BucketTodosGroup across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingNoDueDateTodos();
   * ```
   */
  async everythingNoDueDateTodos(options?: EverythingNoDueDateTodosEverythingOptions): Promise<ListResult<BucketTodosGroup>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingNoDueDateTodos",
        resourceType: "everything_no_due_date_todo",
        isMutation: false,
      },
      () =>
        this.client.GET("/todos/no_due_date.json", {
          params: {
            query: { "assignee_ids[]": options?.assigneeIds, due: options?.due, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Active, incomplete to-dos across all accessible projects, grouped by project (paginated).
   * @param options - Optional query parameters
   * @returns All BucketTodosGroup across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingOpenTodos();
   * ```
   */
  async everythingOpenTodos(options?: EverythingOpenTodosEverythingOptions): Promise<ListResult<BucketTodosGroup>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingOpenTodos",
        resourceType: "everything_open_todo",
        isMutation: false,
      },
      () =>
        this.client.GET("/todos/open.json", {
          params: {
            query: { "assignee_ids[]": options?.assigneeIds, due: options?.due, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get every overdue to-do across all accessible projects, oldest-due-date-first.
   * @param options - Optional query parameters
   * @returns Array of Todo
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingOverdueTodos();
   * ```
   */
  async everythingOverdueTodos(options?: EverythingOverdueTodosEverythingOptions): Promise<Todo[]> {
    const response = await this.request(
      {
        service: "Everything",
        operation: "GetEverythingOverdueTodos",
        resourceType: "everything_overdue_todo",
        isMutation: false,
      },
      () =>
        this.client.GET("/todos/overdue.json", {
          params: {
            query: { "assignee_ids[]": options?.assigneeIds, due: options?.due },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Open, unassigned to-dos across all accessible projects, grouped by project (paginated).
   * @param options - Optional query parameters
   * @returns All BucketTodosGroup across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.everything.everythingUnassignedTodos();
   * ```
   */
  async everythingUnassignedTodos(options?: EverythingUnassignedTodosEverythingOptions): Promise<ListResult<BucketTodosGroup>> {
    return this.requestPaginated(
      {
        service: "Everything",
        operation: "GetEverythingUnassignedTodos",
        resourceType: "everything_unassigned_todo",
        isMutation: false,
      },
      () =>
        this.client.GET("/todos/unassigned.json", {
          params: {
            query: { "assignee_ids[]": options?.assigneeIds, due: options?.due, page: options?.page },
          },
        })
      , options
    );
  }
}