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
  /** True when results were truncated (by maxItems or page safety cap). */
  readonly truncated: boolean;
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

  /**
   * Array species methods (map, filter, slice, etc.) should return plain
   * Arrays, not ListResult instances. Without this override, those methods
   * would call `new ListResult(length)` which fails because our constructor
   * expects (items[], meta).
   */
  static get [Symbol.species](): ArrayConstructor {
    return Array;
  }

  constructor(items: T[], meta: ListMeta) {
    // Use super(0) + length + indexed assignment to avoid:
    // 1. The single-number-argument trap (super(5) creates 5 empty slots)
    // 2. The ...spread limit (push(...items) throws RangeError on ~300k+ items)
    super(0);
    if (items.length > 0) {
      this.length = items.length;
      for (let i = 0; i < items.length; i++) {
        this[i] = items[i]!;
      }
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
