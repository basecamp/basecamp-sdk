/**
 * Base service class for Basecamp API services.
 *
 * Provides shared functionality for all service classes including:
 * - Error handling with typed BasecampError
 * - Hooks integration for observability
 * - Request/response processing
 * - Automatic pagination via Link headers
 *
 * @example
 * ```ts
 * export class TodosService extends BaseService {
 *   async list(projectId: number, todolistId: number): Promise<ListResult<Todo>> {
 *     return this.requestPaginated(
 *       { service: "Todos", operation: "List", resourceType: "todo", isMutation: false, projectId },
 *       () => this.client.GET("/buckets/{projectId}/todolists/{todolistId}/todos.json", {
 *         params: { path: { projectId, todolistId } },
 *       })
 *     );
 *   }
 * }
 * ```
 */

import type { BasecampHooks, OperationInfo, OperationResult } from "../hooks.js";
import { BasecampError, errorFromResponse } from "../errors.js";
import { ListResult, parseTotalCount, type PaginationOptions } from "../pagination.js";
import { parseNextLink, resolveURL, isSameOrigin } from "../pagination-utils.js";
import type { paths } from "../generated/schema.js";
import type createClient from "openapi-fetch";

/**
 * Raw client type from openapi-fetch.
 */
export type RawClient = ReturnType<typeof createClient<paths>>;

/**
 * Response type from openapi-fetch methods.
 */
export interface FetchResponse<T> {
  data?: T;
  error?: unknown;
  response: Response;
}

/** Maximum pages to follow as a safety cap against infinite loops. */
const MAX_PAGES = 10_000;

/**
 * Abstract base class for all Basecamp API services.
 *
 * Services extend this class to inherit common functionality
 * for making API requests, handling errors, and integrating
 * with the hooks system.
 */
export abstract class BaseService {
  /** The underlying openapi-fetch client */
  protected readonly client: RawClient;

  /** Optional hooks for observability */
  protected readonly hooks?: BasecampHooks;

  /**
   * Authenticated fetch for pagination follow-up requests.
   *
   * Note: Subsequent pages use this raw fetch rather than the openapi-fetch
   * middleware stack (retry, cache, hooks). This is intentional — Link header
   * URLs are absolute and don't map to openapi-fetch path patterns. The
   * createBasecampClient() factory provides an authenticated fetchPage closure
   * with Bearer token and User-Agent. When services are instantiated directly
   * (without the factory), the fallback is unauthenticated — page 1 will
   * succeed via the authenticated raw client, but page 2+ may 401.
   */
  protected readonly fetchPage: (url: string) => Promise<Response>;

  constructor(
    client: RawClient,
    hooks?: BasecampHooks,
    fetchPage?: (url: string) => Promise<Response>,
  ) {
    this.client = client;
    this.hooks = hooks;
    this.fetchPage = fetchPage ?? ((url) => fetch(url, { headers: { Accept: "application/json" } }));
  }

  /**
   * Executes an API request with error handling and hooks integration.
   *
   * @param info - Operation metadata for hooks
   * @param fn - The function that performs the actual API call
   * @returns The response data
   * @throws BasecampError on API errors
   */
  protected async request<T>(
    info: OperationInfo,
    fn: () => Promise<FetchResponse<T>>
  ): Promise<T> {
    const start = performance.now();
    let result: OperationResult = { durationMs: 0 };

    // Notify hooks of operation start (wrapped to prevent hook failures from breaking operations)
    try {
      this.hooks?.onOperationStart?.(info);
    } catch {
      // Hooks should not interrupt operations
    }

    try {
      const { data, error, response } = await fn();
      result.durationMs = Math.round(performance.now() - start);

      // Check for errors
      if (!response.ok || error) {
        const basecampError = await this.handleError(response, error);
        result.error = basecampError;
        throw basecampError;
      }

      // For void responses (204, etc.), return undefined as T
      if (response.status === 204 || data === undefined) {
        return undefined as T;
      }

      return data;
    } catch (err) {
      result.durationMs = Math.round(performance.now() - start);

      if (err instanceof BasecampError) {
        result.error = err;
      } else if (err instanceof Error) {
        result.error = err;
      }

      throw err;
    } finally {
      // Always notify hooks of operation end (wrapped to prevent hook failures from breaking operations)
      try {
        this.hooks?.onOperationEnd?.(info, result);
      } catch {
        // Hooks should not interrupt operations
      }
    }
  }

  /**
   * Executes a paginated API request, automatically following Link headers.
   *
   * Returns a ListResult<T> which extends Array<T> — fully backwards-compatible
   * with array operations, plus `.meta.totalCount` for total item count.
   *
   * @param info - Operation metadata for hooks
   * @param fn - The function that performs the initial API call
   * @param paginationOpts - Optional pagination control (maxItems)
   * @returns A ListResult containing all items across pages
   * @throws BasecampError on API errors or cross-origin Link headers
   */
  protected async requestPaginated<T>(
    info: OperationInfo,
    fn: () => Promise<FetchResponse<T[]>>,
    paginationOpts?: PaginationOptions,
  ): Promise<ListResult<T>> {
    const start = performance.now();
    let result: OperationResult = { durationMs: 0 };

    // Notify hooks of operation start
    try {
      this.hooks?.onOperationStart?.(info);
    } catch {
      // Hooks should not interrupt operations
    }

    try {
      const { data, error, response } = await fn();
      result.durationMs = Math.round(performance.now() - start);

      // Check for errors
      if (!response.ok || error) {
        const basecampError = await this.handleError(response, error);
        result.error = basecampError;
        throw basecampError;
      }

      const firstPageItems: T[] = data ?? [];
      const totalCount = parseTotalCount(response);
      const maxItems = paginationOpts?.maxItems;

      // If maxItems is set and first page satisfies it, return early
      if (maxItems && maxItems > 0 && firstPageItems.length >= maxItems) {
        // Only mark truncated if there are actually more items beyond maxItems
        // (either more items on this page than maxItems, or a Link header for more pages)
        const hasMore = firstPageItems.length > maxItems
          || parseNextLink(response.headers.get("Link")) !== null;
        result.durationMs = Math.round(performance.now() - start);
        return new ListResult(firstPageItems.slice(0, maxItems), { totalCount, truncated: hasMore });
      }

      // Follow pagination
      const { items: allItems, truncated } = await this.followPagination(
        response,
        firstPageItems,
        maxItems,
      );

      // Update duration to reflect total time across all pages
      result.durationMs = Math.round(performance.now() - start);

      return new ListResult(allItems, { totalCount, truncated });
    } catch (err) {
      result.durationMs = Math.round(performance.now() - start);

      if (err instanceof BasecampError) {
        result.error = err;
      } else if (err instanceof Error) {
        result.error = err;
      }

      throw err;
    } finally {
      try {
        this.hooks?.onOperationEnd?.(info, result);
      } catch {
        // Hooks should not interrupt operations
      }
    }
  }

  /**
   * Follows Link header pagination, accumulating items across pages.
   * Returns items and whether results were truncated (by maxItems or page cap).
   */
  private async followPagination<T>(
    initialResponse: Response,
    firstPageItems: T[],
    maxItems: number | undefined,
  ): Promise<{ items: T[]; truncated: boolean }> {
    const allItems = [...firstPageItems];
    let response = initialResponse;
    const initialUrl = initialResponse.url;

    for (let page = 1; page < MAX_PAGES; page++) {
      const rawNextUrl = parseNextLink(response.headers.get("Link"));
      if (!rawNextUrl) break;

      const nextUrl = resolveURL(response.url, rawNextUrl);

      // Validate same-origin to prevent SSRF / token leakage
      if (!isSameOrigin(nextUrl, initialUrl)) {
        throw new BasecampError(
          "api_error",
          `Pagination Link header points to different origin: ${nextUrl}`,
        );
      }

      response = await this.fetchPage(nextUrl);

      if (!response.ok) {
        throw await errorFromResponse(response, response.headers.get("X-Request-Id") ?? undefined);
      }

      const pageItems: T[] = (await response.json()) as T[];
      allItems.push(...pageItems);

      // Check maxItems cap
      if (maxItems && maxItems > 0 && allItems.length >= maxItems) {
        return { items: allItems.slice(0, maxItems), truncated: true };
      }
    }

    // If we exited the loop because page >= MAX_PAGES and there's still a next link,
    // the results are truncated by the safety cap
    const hasMore = parseNextLink(response.headers.get("Link")) !== null;
    return { items: allItems, truncated: hasMore };
  }

  /**
   * Converts an HTTP error response to a typed BasecampError.
   *
   * @param response - The HTTP response
   * @param error - Optional error object from openapi-fetch
   * @returns A BasecampError with appropriate code and metadata
   */
  protected async handleError(response: Response, error?: unknown): Promise<BasecampError> {
    // If already a BasecampError, just return it
    if (error instanceof BasecampError) {
      return error;
    }

    // Extract request ID from response headers if available
    const requestId = response.headers.get("X-Request-Id") ?? undefined;

    // Use the errorFromResponse helper to create the appropriate error
    return errorFromResponse(response, requestId);
  }
}
