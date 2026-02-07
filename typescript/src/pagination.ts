/**
 * Pagination types and utilities for the Basecamp SDK.
 *
 * Provides ListResult (an Array subclass with metadata), pagination options,
 * and header parsing utilities used by generated service methods.
 */

/**
 * Metadata about a paginated list response.
 */
export interface ListMeta {
  /** Total number of items across all pages (from X-Total-Count header). */
  readonly totalCount: number;
}

/**
 * Options for controlling pagination behavior.
 */
export interface PaginationOptions {
  /**
   * Maximum number of items to return across all pages.
   * When undefined or 0, all pages are fetched.
   */
  maxItems?: number;
}

/**
 * An array of results with pagination metadata.
 *
 * Extends Array<T> so it's fully backwards-compatible: works with
 * .forEach(), .map(), spread, .length, indexing, and Array.isArray().
 * Additional metadata is accessible via the `.meta` property.
 *
 * @example
 * ```ts
 * const todos = await client.todos.list(projectId, todolistId);
 * console.log(`Showing ${todos.length} of ${todos.meta.totalCount} todos`);
 * todos.forEach(todo => console.log(todo.content));
 * ```
 */
export class ListResult<T> extends Array<T> {
  readonly meta: ListMeta;

  constructor(items: T[], meta: ListMeta) {
    // Use super(0) + push to avoid both:
    // 1. The single-number-argument trap (super(5) creates 5 empty slots)
    // 2. The ...spread limit for large arrays (stack overflow on 100k+ items)
    super(0);
    if (items.length > 0) {
      this.push(...items);
    }
    this.meta = meta;
  }
}

/**
 * Parses the X-Total-Count header from a Response.
 * Returns 0 if the header is missing or invalid.
 */
export function parseTotalCount(response: Response): number {
  const header = response.headers.get("X-Total-Count");
  if (!header) return 0;
  const parsed = parseInt(header, 10);
  return isNaN(parsed) ? 0 : parsed;
}
