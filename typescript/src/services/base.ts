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
import metadata from "../generated/metadata.json" with { type: "json" };
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

/** Default maximum pages to follow as a safety cap against infinite loops. */
const DEFAULT_MAX_PAGES = 10_000;

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

  /** Maximum pages to follow before stopping (safety cap). */
  protected readonly maxPages: number;

  /** Base URL for building multipart upload URLs. */
  protected readonly baseUrl: string;

  /**
   * Authenticated fetch for multipart uploads and other raw requests.
   * Provided by the client factory with Bearer token and User-Agent.
   */
  protected readonly authenticatedFetch: (url: string, init: RequestInit) => Promise<Response>;

  constructor(
    client: RawClient,
    hooks?: BasecampHooks,
    fetchPage?: (url: string) => Promise<Response>,
    maxPages?: number,
    authenticatedFetch?: (url: string, init: RequestInit) => Promise<Response>,
    baseUrl?: string,
  ) {
    this.client = client;
    this.hooks = hooks;
    this.fetchPage = fetchPage ?? ((url) => fetch(url, { headers: { Accept: "application/json" } }));
    this.maxPages = maxPages ?? DEFAULT_MAX_PAGES;
    this.authenticatedFetch = authenticatedFetch ?? ((url, init) => fetch(url, init));
    this.baseUrl = baseUrl ?? "";
  }

  /**
   * Uploads a file as multipart/form-data with hooks integration.
   *
   * @param info - Operation metadata for hooks
   * @param url - The full API URL
   * @param method - HTTP method (PUT, POST)
   * @param file - File or Blob to upload
   * @param fieldName - The form field name
   * @param filename - Display name for the uploaded file
   */
  protected async requestMultipartUpload(
    info: OperationInfo,
    url: string,
    method: string,
    file: Blob | File,
    fieldName: string,
    filename?: string,
  ): Promise<void> {
    const start = performance.now();
    let result: OperationResult = { durationMs: 0 };

    try { this.hooks?.onOperationStart?.(info); } catch { /* hooks should not interrupt */ }

    try {
      const formData = new FormData();
      formData.append(fieldName, file, filename ?? (file instanceof File ? file.name : fieldName));

      // Look up retry config from operation metadata
      const opMeta = (metadata as any).operations?.[info.operation];
      const retryConfig = opMeta?.retry ?? { maxAttempts: 1, baseDelayMs: 1000, backoff: "exponential", retryOn: [] };
      const maxAttempts: number = retryConfig.maxAttempts ?? 1;
      const retryOn: number[] = retryConfig.retryOn ?? [];

      let response: Response | undefined;
      for (let attempt = 0; attempt < maxAttempts; attempt++) {
        response = await this.authenticatedFetch(url, {
          method,
          body: formData,
        });

        if (!retryOn.includes(response.status) || attempt >= maxAttempts - 1) {
          break;
        }

        // Backoff before retry
        const retryAfter = response.status === 429
          ? parseInt(response.headers.get("Retry-After") ?? "", 10) * 1000
          : NaN;
        const delay = !isNaN(retryAfter) && retryAfter > 0
          ? retryAfter
          : (retryConfig.baseDelayMs ?? 1000) * Math.pow(2, attempt);
        await new Promise((r) => setTimeout(r, delay));
      }

      result.durationMs = Math.round(performance.now() - start);

      if (!response!.ok) {
        const basecampError = await errorFromResponse(response!);
        result.error = basecampError;
        throw basecampError;
      }

      // Drain body for 204 No Content
      if (response!.status === 204) {
        response!.body?.cancel();
      }
    } catch (err) {
      result.durationMs = Math.round(performance.now() - start);
      if (err instanceof BasecampError || err instanceof Error) {
        result.error = err;
      }
      throw err;
    } finally {
      try { this.hooks?.onOperationEnd?.(info, result); } catch { /* hooks should not interrupt */ }
    }
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
   * Executes a paginated API request for wrapped responses.
   *
   * For endpoints that return `{ wrapper_field: ..., key: [items] }` on every page.
   * Follows Link headers, extracting items from the specified key on each page.
   * Returns wrapper fields from page 1 + all items across pages as a ListResult.
   */
  protected async requestPaginatedWrapped<K extends string, TItem>(
    info: OperationInfo,
    fn: () => Promise<FetchResponse<Record<string, unknown>>>,
    key: K,
    paginationOpts?: PaginationOptions,
  ): Promise<Omit<Record<string, unknown>, K> & Record<K, ListResult<TItem>>> {
    const start = performance.now();
    let result: OperationResult = { durationMs: 0 };

    try {
      this.hooks?.onOperationStart?.(info);
    } catch {
      // Hooks should not interrupt operations
    }

    try {
      const { data, error, response } = await fn();
      result.durationMs = Math.round(performance.now() - start);

      if (!response.ok || error) {
        const basecampError = await this.handleError(response, error);
        result.error = basecampError;
        throw basecampError;
      }

      const firstPageData = (data ?? {}) as Record<string, unknown>;
      const totalCount = parseTotalCount(response);

      // Extract wrapper fields (everything except the paginated key)
      const wrapper: Record<string, unknown> = {};
      for (const [k, v] of Object.entries(firstPageData)) {
        if (k !== key) wrapper[k] = v;
      }

      const firstPageItems: TItem[] = (firstPageData[key] as TItem[]) ?? [];
      const maxItems = paginationOpts?.maxItems;

      // If maxItems is set and first page satisfies it, return early
      if (maxItems && maxItems > 0 && firstPageItems.length >= maxItems) {
        const hasMore = firstPageItems.length > maxItems
          || parseNextLink(response.headers.get("Link")) !== null;
        result.durationMs = Math.round(performance.now() - start);
        const listResult = new ListResult(firstPageItems.slice(0, maxItems), { totalCount, truncated: hasMore });
        return { ...wrapper, [key]: listResult } as Omit<Record<string, unknown>, K> & Record<K, ListResult<TItem>>;
      }

      // Follow pagination, extracting items from key on each subsequent page
      const { items: allItems, truncated } = await this.followPaginationWrapped<TItem>(
        response,
        firstPageItems,
        key,
        maxItems,
      );

      result.durationMs = Math.round(performance.now() - start);
      const listResult = new ListResult(allItems, { totalCount, truncated });
      return { ...wrapper, [key]: listResult } as Omit<Record<string, unknown>, K> & Record<K, ListResult<TItem>>;
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

    for (let page = 1; page < this.maxPages; page++) {
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

    // If we exited the loop because page >= maxPages and there's still a next link,
    // the results are truncated by the safety cap
    const hasMore = parseNextLink(response.headers.get("Link")) !== null;
    return { items: allItems, truncated: hasMore };
  }

  /**
   * Follows Link header pagination for wrapped responses.
   * Each page is a wrapper object; items are extracted from data[key].
   */
  private async followPaginationWrapped<T>(
    initialResponse: Response,
    firstPageItems: T[],
    key: string,
    maxItems: number | undefined,
  ): Promise<{ items: T[]; truncated: boolean }> {
    const allItems = [...firstPageItems];
    let response = initialResponse;
    const initialUrl = initialResponse.url;

    for (let page = 1; page < this.maxPages; page++) {
      const rawNextUrl = parseNextLink(response.headers.get("Link"));
      if (!rawNextUrl) break;

      const nextUrl = resolveURL(response.url, rawNextUrl);

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

      const pageData = (await response.json()) as Record<string, unknown>;
      const pageItems: T[] = (pageData[key] as T[]) ?? [];
      allItems.push(...pageItems);

      if (maxItems && maxItems > 0 && allItems.length >= maxItems) {
        return { items: allItems.slice(0, maxItems), truncated: true };
      }
    }

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
