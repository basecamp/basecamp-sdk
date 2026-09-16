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
import { BasecampError, errorFromParsedBody, errorFromResponse, parseRetryAfter, truncateErrorMessage } from "../errors.js";
import metadata from "../generated/metadata.js";
import { ListResult, parseTotalCount, type PaginationOptions } from "../pagination.js";
import { personIdNumber, scanPersonId } from "../person-id.js";
import { parseNextLink, resolveURL, isSameOrigin, DEFAULT_MAX_PAGES, assertValidMaxPages } from "../pagination-utils.js";
import { saturatingBackoff, timerSafeDelayMs } from "../retry.js";
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

/**
 * True when the caller pinned a single page (SPEC section 8).
 *
 * A positive `page` is a selector, not a starting offset: the operation issues
 * exactly one request and never follows `Link: rel="next"`. Absent, 0, and
 * negative all mean "walk the collection".
 */
function isPageSelected(opts?: PaginationOptions): boolean {
  return typeof opts?.page === "number" && opts.page > 0;
}

/**
 * Builds the ListResult for a pinned page.
 *
 * `truncated` still answers "were there more items than these": true when the
 * selected page carried a next link, or when the maxItems cap dropped items
 * from it.
 */
function selectedPageResult<T>(
  response: Response,
  items: T[],
  totalCount: number,
  maxItems: number | undefined,
): ListResult<T> {
  const capped = maxItems !== undefined && maxItems > 0 && items.length > maxItems;
  const truncated = capped || parseNextLink(response.headers.get("Link")) !== null;
  return new ListResult(
    capped ? items.slice(0, maxItems) : items,
    { totalCount, truncated },
  );
}

/**
 * Rewrites one Person-shaped object's string `id` in place.
 *
 * This is `coercePersonID` (`go/pkg/basecamp/normalize.go:38-68`), outcome for
 * outcome, over the one rule in `../person-id.js` — Go's
 * `strconv.ParseInt(s, 10, 64)`:
 *
 * - it reads a number: write the number, no `system_label` (`:58`)
 * - SYNTAX ("basecamp", " 7", "12.0", "1e3"): id `0`, the raw string kept as
 *   `system_label` (`:66-67`) — Go's non-numeric sentinel
 * - RANGE ("9223372036854775808"): leave the string exactly as it arrived
 *   (`:62-63`), so whoever reads it refuses rather than inventing an id
 *
 * An object whose `id` is already a number, or absent, is left alone — which is
 * also what makes this IDEMPOTENT, and both passes below depend on that.
 *
 * The `^-?\d+$` + `Number.isSafeInteger` pair this replaces was wrong at both
 * ends. It refused the leading `+` that `ParseInt` takes, so `"+7"` — a real
 * person — became the sentinel. And it collapsed EVERY id past
 * `Number.MAX_SAFE_INTEGER` to `id: 0` with a `system_label`: range errors and
 * genuine large ids alike, which is to say it turned people into LocalPerson,
 * silently, in the shape callers trust. Measured against the reference, 26 of
 * the 74 corpus rows in `tests/helpers/person-id-corpus.ts` disagreed with Go
 * (25 of them on the id itself; `"+0"` agreed on `0` and added a spurious
 * label). The 11 that remain are the unrepresentable-value rows
 * {@link personIdNumber} argues about.
 */
function coercePersonId(rec: Record<string, unknown>): void {
  if (typeof rec.id !== "string") return;
  const idStr = rec.id;
  const scan = scanPersonId(idStr);
  if (scan.kind === "syntax") {
    // Go's non-numeric sentinel, with the label the id came in as.
    rec.system_label = idStr;
    rec.id = 0;
  } else if (scan.kind === "value") {
    const id = personIdNumber(scan.value);
    // `undefined` is an id Go read and a `number` cannot hold. Leaving the
    // string is the RANGE treatment, for the reason at personIdNumber: a
    // rounded id names a different person and `0` names the system actor,
    // while the digits, left alone, name whoever they always named.
    if (id !== undefined) rec.id = id;
  }
  // RANGE: nothing. The string stays put and the caller refuses it.
}

/** A JSON object — what Go's `.(map[string]any)` assertion accepts, and no array. */
function isJSONObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * Normalizes Person-shaped objects in API responses.
 *
 * The BC3 API conflates real Person records (numeric id) with system actors
 * like LocalPerson (symbolic id: "basecamp", "campfire"), and it serializes
 * person ids as strings in some payloads. This stands where
 * `normalizeEmbeddedPeopleJSON` stands (`go/pkg/basecamp/normalize.go:117-134`),
 * so it owns BOTH of the passes that function runs, and finds a person two ways:
 *
 * 1. **By `personable_type`** (`normalizePersonIds`, `:16-29`) — any object at
 *    any depth carrying that key.
 * 2. **By structural position** (`normalizeEmbeddedPersonIds`, `:83-104`) — the
 *    `creator` object and each element of the `participants` array, on any
 *    object at any depth, WHETHER OR NOT it has a `personable_type`.
 *
 * The second pass is not redundant and Go's comment says why (`:78-82`):
 * embedded creator/participants people frequently omit `personable_type`, so
 * the first pass skips exactly the payloads the second exists to fix. Missing it
 * was observable in this SDK and in no other: every other port has a runtime
 * decoder behind the normalizer — Kotlin's `FlexibleLongSerializer`, Swift's
 * `FlexibleInt`, Rust's `flexible_i64`, Python's `_decoded_flexible_int64`,
 * Ruby's `person_from_wire` — that converts the string at read time whichever
 * pass did or did not touch it. TypeScript has none, so an un-normalized
 * `creator.id` reached the caller as the STRING `"007"` in a field typed
 * `number`, where the reference and the other five give `7`. It also made the
 * one honest string this normalizer does leave behind — the unrepresentable
 * large id — unreadable as a signal, because `typeof person.id === "string"`
 * could equally mean "nobody looked at this".
 *
 * ONE walk here where Go runs two, and the two are equivalent because
 * {@link coercePersonId} is idempotent and both passes apply it unchanged.
 * After JSON.parse the body is a tree, so each node is reached once per pass;
 * the set of coerced nodes is the union of "has personable_type" and "is a
 * creator/participant of its parent", identical either way; and a node in both
 * sets is coerced twice in both schemes, which is a no-op the second time
 * (`id` is no longer a string) or the same leave-in-place decision on the same
 * string. Only the order differs, and idempotence is what makes order not
 * matter. Doing it in one pass rather than two keeps this off a second full
 * walk of every response body.
 *
 * One deliberate breadth difference from the reference, pre-dating the second
 * pass: Go runs `normalizeEmbeddedPeopleJSON` on the notification, gauge and
 * todo paths specifically, while this runs on every response body. That is
 * wider, never narrower, and it is wider in the accepting direction — a string
 * id Go would only coerce on those paths is coerced here everywhere, rather
 * than reaching a `number`-typed field as a string.
 */
function normalizePersonIds(obj: unknown): void {
  if (!obj || typeof obj !== "object") return;
  if (Array.isArray(obj)) {
    for (const item of obj) normalizePersonIds(item);
    return;
  }
  const rec = obj as Record<string, unknown>;
  // Pass 1's test: the key's presence, whatever its value (`:19`).
  if ("personable_type" in rec) coercePersonId(rec);
  // Pass 2's: a `creator` that is an object and a `participants` that is an
  // array, each element of it an object. Go's type assertions skip anything
  // else — a `creator: "me"` or a `participants: {}` is not a person (`:85-95`).
  if (isJSONObject(rec.creator)) coercePersonId(rec.creator);
  if (Array.isArray(rec.participants)) {
    for (const participant of rec.participants) {
      if (isJSONObject(participant)) coercePersonId(participant);
    }
  }
  // Both passes then recurse through every child, so a creator nested under a
  // comment under an event is reached at whatever depth it sits (`:96-102`).
  for (const val of Object.values(rec)) {
    if (typeof val === "object" && val !== null) normalizePersonIds(val);
  }
}

/**
 * Whether a rejection denotes invalid JSON rather than a failed read.
 *
 * `response.json()` rejects for two unrelated reasons, and only one of them is
 * a malformed body: the bytes arrived and would not parse (SyntaxError), or the
 * bytes never finished arriving — a socket reset or a decompression failure
 * (TypeError), an aborted read (AbortError). Those are transport failures and
 * keep their own semantics.
 *
 * Matched on `name`, not `instanceof`, and the engines force that rather than
 * merely permitting it: Node/undici rejects with a real `SyntaxError`, but
 * WebKit rejects with a **DOMException** named "SyntaxError", which
 * `instanceof SyntaxError` answers `false` for. Narrowing by constructor would
 * classify nothing in Safari and let every malformed body escape there — and
 * excluding DOMException, to keep a DOMException named "SyntaxError" from
 * matching, would break exactly the engine that produces one. A cross-realm
 * error is the third case the name survives and `instanceof` does not, which is
 * why the OAuth device flow matches AbortError the same way. Nothing else
 * `response.json()` can reject with is named "SyntaxError".
 */
function isJsonSyntaxError(err: unknown): err is SyntaxError {
  return typeof err === "object" && err !== null && (err as { name?: unknown }).name === "SyntaxError";
}

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

  /**
   * Maximum pages to follow before stopping (safety cap).
   *
   * A native #private field, not `protected readonly`: TypeScript's
   * `readonly` is compile-time only, so `(svc as any).maxPages = Infinity`
   * would have replaced the validated cap after construction. The pagination
   * loops read the #field directly — an own-property shadow cannot reach
   * them — and the getter below preserves the supported subclass read.
   */
  readonly #maxPages: number;

  /** Read-only view of the cap; the loops bypass it and read #maxPages. */
  protected get maxPages(): number {
    return this.#maxPages;
  }

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
    // BaseService is exported, and so is every generated service extending it,
    // so `new ProjectsService(client, hooks, fetchPage, Infinity)` is a
    // supported call that reaches followPagination's `page < this.maxPages`
    // without passing through createBasecampClient. Validating only at the
    // client factory would leave this door open. Checked only when supplied, so
    // an omitted cap still falls through to the default.
    // `!= null` rather than `!== undefined`, to agree with the `??` below:
    // that treats an explicit `null` as "not supplied" and falls back to the
    // default, so the guard must treat it the same way. A JS caller passing
    // `null` — outside the declared type, but reachable from untyped code —
    // would otherwise have started throwing where it previously defaulted.
    if (maxPages != null) {
      assertValidMaxPages(maxPages);
    }

    this.client = client;
    this.hooks = hooks;
    this.fetchPage = fetchPage ?? ((url) => fetch(url, { headers: { Accept: "application/json" } }));
    this.#maxPages = maxPages ?? DEFAULT_MAX_PAGES;
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
        const reqInfo = { method, url, attempt: attempt + 1 };
        try { this.hooks?.onRequestStart?.(reqInfo); } catch { /* hooks should not interrupt */ }

        const reqStart = performance.now();
        try {
          response = await this.authenticatedFetch(url, {
            method,
            body: formData,
          });
        } catch (fetchErr) {
          const durationMs = Math.round(performance.now() - reqStart);
          const error = fetchErr instanceof Error ? fetchErr : new Error(String(fetchErr));
          try { this.hooks?.onRequestEnd?.(reqInfo, { statusCode: 0, durationMs, fromCache: false, error }); } catch { /* */ }
          throw error;
        }

        const reqDurationMs = Math.round(performance.now() - reqStart);
        try { this.hooks?.onRequestEnd?.(reqInfo, { statusCode: response.status, durationMs: reqDurationMs, fromCache: false }); } catch { /* */ }

        if (!retryOn.includes(response.status) || attempt >= maxAttempts - 1) {
          break;
        }

        // Drain response body before retry to free resources and enable connection reuse
        response.body?.cancel();

        // Backoff before retry. A Retry-After replaces the curve at every
        // status this branch reaches (SPEC §6 "Retry-After Honouring"), and
        // the header goes through errors.ts's parseRetryAfter — the single
        // SPEC §6 implementation — rather than a local parseInt: the copy this
        // replaced had no HTTP-date branch and guarded with `>= 0`, so it
        // honoured `Retry-After: 0` as a zero-millisecond delay and retried
        // with no wait at all.
        const retryAfterSeconds = parseRetryAfter(response.headers.get("Retry-After"));
        // The locally-computed term is bounded by SPEC §7's ceiling; the
        // server-directed Retry-After is not, per the same section.
        const delay = retryAfterSeconds !== undefined
          ? timerSafeDelayMs(retryAfterSeconds)
          : saturatingBackoff(retryConfig.baseDelayMs ?? 1000, "exponential", attempt);

        try {
          // SPEC §7 step 3i: the status-mapped error, carrying the parsed
          // retryAfter that governs this sleep — the same value, not a
          // second parse — rather than a bare Error.
          const retryError = errorFromParsedBody(
            response,
            null,
            response.headers.get("X-Request-Id") ?? undefined,
            retryAfterSeconds,
          );
          // SPEC section 7: RequestInfo.attempt is the attempt that just failed
          // (1-based), while the standalone argument is the UPCOMING attempt.
          this.hooks?.onRetry?.(
            { method, url, attempt: attempt + 1 },
            attempt + 2,
            retryError,
            delay,
          );
        } catch { /* hooks should not interrupt */ }

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

      normalizePersonIds(data);
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
      normalizePersonIds(firstPageItems);
      const totalCount = parseTotalCount(response);
      const maxItems = paginationOpts?.maxItems;

      // A pinned page is the whole answer: return it without following links.
      if (isPageSelected(paginationOpts)) {
        result.durationMs = Math.round(performance.now() - start);
        return selectedPageResult(response, firstPageItems, totalCount, maxItems);
      }

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
      normalizePersonIds(firstPageData);
      const totalCount = parseTotalCount(response);

      // Extract wrapper fields (everything except the paginated key)
      const wrapper: Record<string, unknown> = {};
      for (const [k, v] of Object.entries(firstPageData)) {
        if (k !== key) wrapper[k] = v;
      }

      const firstPageItems: TItem[] = (firstPageData[key] as TItem[]) ?? [];
      const maxItems = paginationOpts?.maxItems;

      // A pinned page is the whole answer: return it without following links.
      if (isPageSelected(paginationOpts)) {
        result.durationMs = Math.round(performance.now() - start);
        const listResult = selectedPageResult(response, firstPageItems, totalCount, maxItems);
        return { ...wrapper, [key]: listResult } as Omit<Record<string, unknown>, K> & Record<K, ListResult<TItem>>;
      }

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
   * Decodes a followed page's body, refusing one that is not JSON.
   *
   * The raw-fetch pagination path is the only place this SDK decodes a body
   * itself — every other response comes back through `openapi-fetch` — and an
   * unwrapped `await response.json()` let a `SyntaxError` escape with no code,
   * no hint and nothing to distinguish "the server sent garbage on page 4" from
   * a bug in the caller's own code. Ruby and Python already classify this exact
   * failure with this exact message; TypeScript was the outlier.
   *
   * `cause` carries the decoder's own error, which is the #750 contract: the
   * message says which page failed, the slot says whether the body was truncated
   * mid-object or was never JSON, and neither answer is parsed back out of the
   * other. Statusless per SPEC §6 — the transport returned 2xx — and
   * non-retryable, because re-requesting cannot repair a malformed body.
   *
   * Only a syntax failure is classified. A read that dies partway through — a
   * reset socket, a corrupt gzip stream, an aborted request — rejects here too,
   * and it is transient: relabelling it non-retryable would be a worse answer
   * than the bare error this replaced. It propagates with its own type.
   */
  private async parsePage<T>(response: Response, page: number): Promise<T> {
    try {
      return (await response.json()) as T;
    } catch (err) {
      throw isJsonSyntaxError(err)
        ? new BasecampError(
            "api_error",
            truncateErrorMessage(`Failed to parse paginated response (page ${page}): ${err.message}`),
            { retryable: false, cause: err },
          )
        : err;
    }
  }

  /**
   * Follows Link header pagination, accumulating items across pages.
   * Returns items and whether results were truncated: true only when items
   * beyond maxItems were dropped, or a next-page link was left unfetched
   * (maxItems met at a page boundary, or the page safety cap).
   */
  private async followPagination<T>(
    initialResponse: Response,
    firstPageItems: T[],
    maxItems: number | undefined,
  ): Promise<{ items: T[]; truncated: boolean }> {
    const allItems = [...firstPageItems];
    let response = initialResponse;
    const initialUrl = initialResponse.url;

    for (let page = 1; page < this.#maxPages; page++) {
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

      const pageItems: T[] = await this.parsePage<T[]>(response, page + 1);
      normalizePersonIds(pageItems);
      allItems.push(...pageItems);

      // Check maxItems cap. Only mark truncated when items were actually
      // dropped, or the just-fetched page links to a further page.
      if (maxItems && maxItems > 0 && allItems.length >= maxItems) {
        const hasMore = allItems.length > maxItems
          || parseNextLink(response.headers.get("Link")) !== null;
        return { items: allItems.slice(0, maxItems), truncated: hasMore };
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

    for (let page = 1; page < this.#maxPages; page++) {
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

      const pageData = await this.parsePage<Record<string, unknown>>(response, page + 1);
      normalizePersonIds(pageData);
      const pageItems: T[] = (pageData[key] as T[]) ?? [];
      allItems.push(...pageItems);

      // Check maxItems cap. Only mark truncated when items were actually
      // dropped, or the just-fetched page links to a further page.
      if (maxItems && maxItems > 0 && allItems.length >= maxItems) {
        const hasMore = allItems.length > maxItems
          || parseNextLink(response.headers.get("Link")) !== null;
        return { items: allItems.slice(0, maxItems), truncated: hasMore };
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

    // openapi-fetch has already consumed and parsed the error body into
    // `error`; re-reading the response would throw and lose the server's
    // message (and any field-keyed 422 errors map).
    if (error !== undefined) {
      return errorFromParsedBody(response, error, requestId);
    }

    // Use the errorFromResponse helper to create the appropriate error
    return errorFromResponse(response, requestId);
  }
}
