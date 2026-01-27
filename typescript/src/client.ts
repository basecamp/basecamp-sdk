/**
 * Basecamp TypeScript SDK Client
 *
 * Creates a type-safe client for the Basecamp 3 API using openapi-fetch.
 * Includes middleware for authentication, retry with exponential backoff,
 * and ETag-based caching.
 */

import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./generated/schema.js";

// Re-export types for consumer convenience
export type { paths };
export type BasecampClient = ReturnType<typeof createClient<paths>>;

/**
 * Token provider - either a static token string or an async function that returns a token.
 * Use an async function for token refresh scenarios.
 */
export type TokenProvider = string | (() => Promise<string>);

/**
 * Configuration options for creating a Basecamp client.
 */
export interface BasecampClientOptions {
  /** Basecamp account ID (found in your Basecamp URL) */
  accountId: string;
  /** OAuth access token or async function that returns one */
  accessToken: TokenProvider;
  /** Base URL override (defaults to https://3.basecampapi.com/{accountId}) */
  baseUrl?: string;
  /** User-Agent header (defaults to basecamp-sdk-ts/VERSION) */
  userAgent?: string;
  /** Enable ETag-based caching (defaults to true) */
  enableCache?: boolean;
  /** Enable automatic retry on 429/503 (defaults to true) */
  enableRetry?: boolean;
}

const VERSION = "0.1.0";
const DEFAULT_USER_AGENT = `basecamp-sdk-ts/${VERSION}`;

/**
 * Creates a type-safe Basecamp API client with built-in middleware for:
 * - Authentication (Bearer token)
 * - Retry with exponential backoff (respects Retry-After header)
 * - ETag-based HTTP caching
 *
 * @example
 * ```ts
 * import { createBasecampClient } from "@basecamp/sdk";
 *
 * const client = createBasecampClient({
 *   accountId: "12345",
 *   accessToken: process.env.BASECAMP_TOKEN!,
 * });
 *
 * const { data, error } = await client.GET("/projects.json");
 * ```
 */
export function createBasecampClient(options: BasecampClientOptions): BasecampClient {
  const {
    accountId,
    accessToken,
    baseUrl = `https://3.basecampapi.com/${accountId}`,
    userAgent = DEFAULT_USER_AGENT,
    enableCache = true,
    enableRetry = true,
  } = options;

  const client = createClient<paths>({ baseUrl });

  // Apply middleware in order: auth first, then cache, then retry
  client.use(createAuthMiddleware(accessToken, userAgent));

  if (enableCache) {
    client.use(createCacheMiddleware());
  }

  if (enableRetry) {
    client.use(createRetryMiddleware());
  }

  return client;
}

// =============================================================================
// Auth Middleware
// =============================================================================

function createAuthMiddleware(tokenProvider: TokenProvider, userAgent: string): Middleware {
  return {
    async onRequest({ request }) {
      const token =
        typeof tokenProvider === "function" ? await tokenProvider() : tokenProvider;

      request.headers.set("Authorization", `Bearer ${token}`);
      request.headers.set("User-Agent", userAgent);
      request.headers.set("Content-Type", "application/json");
      request.headers.set("Accept", "application/json");

      return request;
    },
  };
}

// =============================================================================
// Cache Middleware (ETag-based)
// =============================================================================

interface CacheEntry {
  etag: string;
  body: string;
}

const MAX_CACHE_ENTRIES = 1000;

function createCacheMiddleware(): Middleware {
  // Use Map for insertion-order iteration (approximates LRU)
  const cache = new Map<string, CacheEntry>();

  const evictOldest = () => {
    if (cache.size >= MAX_CACHE_ENTRIES) {
      // Delete oldest entry (first key in insertion order)
      const firstKey = cache.keys().next().value;
      if (firstKey) cache.delete(firstKey);
    }
  };

  return {
    async onRequest({ request }) {
      if (request.method !== "GET") return request;

      const cacheKey = getCacheKey(request.url);
      const entry = cache.get(cacheKey);

      if (entry?.etag) {
        request.headers.set("If-None-Match", entry.etag);
      }

      return request;
    },

    async onResponse({ request, response }) {
      if (request.method !== "GET") return response;

      const cacheKey = getCacheKey(request.url);

      // Handle 304 Not Modified - return cached body
      if (response.status === 304) {
        const entry = cache.get(cacheKey);
        if (entry) {
          return new Response(entry.body, {
            status: 200,
            headers: response.headers,
          });
        }
      }

      // Cache successful responses with ETag
      if (response.ok) {
        const etag = response.headers.get("ETag");
        if (etag) {
          const body = await response.clone().text();
          evictOldest();
          cache.set(cacheKey, { etag, body });
        }
      }

      return response;
    },
  };
}

function getCacheKey(url: string): string {
  // Simple hash using URL - in production, could use crypto.subtle
  let hash = 0;
  for (let i = 0; i < url.length; i++) {
    const char = url.charCodeAt(i);
    hash = (hash << 5) - hash + char;
    hash |= 0; // Convert to 32bit integer
  }
  return String(hash);
}

// =============================================================================
// Retry Middleware
// =============================================================================

/**
 * Retry configuration matching x-basecamp-retry extension schema.
 */
interface RetryConfig {
  maxAttempts: number;
  baseDelayMs: number;
  backoff: "exponential" | "linear" | "constant";
  retryOn: number[];
}

/** Default retry config used when no operation-specific config is available */
const DEFAULT_RETRY_CONFIG: RetryConfig = {
  maxAttempts: 3,
  baseDelayMs: 1000,
  backoff: "exponential",
  retryOn: [429, 503],
};

const MAX_JITTER_MS = 100;

function createRetryMiddleware(): Middleware {
  // Store request body clones keyed by a request identifier
  // This is needed because Request.body can only be read once
  const bodyCache = new Map<string, ArrayBuffer | null>();

  return {
    async onRequest({ request }) {
      // For methods that may have a body, clone it before the initial fetch
      // so we can use it for retries. Request.body can only be consumed once.
      const method = request.method.toUpperCase();
      if (method === "POST" || method === "PUT" || method === "PATCH") {
        const requestId = `${method}:${request.url}:${Date.now()}`;
        request.headers.set("X-Request-Id", requestId);

        if (request.body) {
          // Clone the body before it gets consumed
          const cloned = request.clone();
          bodyCache.set(requestId, await cloned.arrayBuffer());
        } else {
          bodyCache.set(requestId, null);
        }
      }

      return request;
    },

    async onResponse({ request, response }) {
      // Use default retry config (operation-specific config would come from metadata)
      const retryConfig = DEFAULT_RETRY_CONFIG;

      const requestId = request.headers.get("X-Request-Id");

      // Helper to clean up cached body
      const cleanupBody = () => {
        if (requestId) bodyCache.delete(requestId);
      };

      // Check if status code should trigger retry
      if (!retryConfig.retryOn.includes(response.status)) {
        cleanupBody();
        return response;
      }

      // Extract current retry attempt from custom header
      const attemptHeader = request.headers.get("X-Retry-Attempt");
      const attempt = attemptHeader ? parseInt(attemptHeader, 10) : 0;

      // Check if we've exhausted retries (maxAttempts is total attempts, not retries)
      // With maxAttempts=3: attempt 0 (initial), 1 (retry 1), 2 (retry 2) = 3 total
      if (attempt >= retryConfig.maxAttempts - 1) {
        cleanupBody();
        return response;
      }

      // Calculate delay
      let delay: number;

      // For 429, respect Retry-After header
      if (response.status === 429) {
        const retryAfter = response.headers.get("Retry-After");
        if (retryAfter) {
          const seconds = parseInt(retryAfter, 10);
          if (!isNaN(seconds)) {
            delay = seconds * 1000;
          } else {
            delay = calculateBackoffDelay(retryConfig, attempt);
          }
        } else {
          delay = calculateBackoffDelay(retryConfig, attempt);
        }
      } else {
        delay = calculateBackoffDelay(retryConfig, attempt);
      }

      // Wait before retry
      await sleep(delay);

      // Get cached body for methods that may have one
      let body: ArrayBuffer | null = null;
      if (requestId && bodyCache.has(requestId)) {
        const cachedBody = bodyCache.get(requestId);
        if (cachedBody) {
          body = cachedBody;
        }
      }

      // Create retry request with fresh body
      const retryRequest = new Request(request.url, {
        method: request.method,
        headers: new Headers(request.headers),
        body,
        signal: request.signal,
      });
      retryRequest.headers.set("X-Retry-Attempt", String(attempt + 1));

      // Retry using native fetch
      return fetch(retryRequest);
    },
  };
}

function calculateBackoffDelay(config: RetryConfig, attempt: number): number {
  const base = config.baseDelayMs;
  let delay: number;

  switch (config.backoff) {
    case "exponential":
      delay = base * Math.pow(2, attempt);
      break;
    case "linear":
      delay = base * (attempt + 1);
      break;
    case "constant":
    default:
      delay = base;
  }

  // Add jitter (0-100ms)
  const jitter = Math.random() * MAX_JITTER_MS;
  return delay + jitter;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// =============================================================================
// Pagination Helper
// =============================================================================

/**
 * Fetches all pages of a paginated resource using Link header pagination.
 * Automatically follows rel="next" links until no more pages exist.
 *
 * @example
 * ```ts
 * const response = await client.GET("/projects.json");
 *
 * const allProjects = await fetchAllPages(
 *   response.response,
 *   (r) => r.json()
 * );
 * ```
 */
export async function fetchAllPages<T>(
  initialResponse: Response,
  parse: (response: Response) => Promise<T[]>,
  authHeader?: string
): Promise<T[]> {
  const results: T[] = [];
  let response = initialResponse;

  while (true) {
    const items = await parse(response.clone());
    results.push(...items);

    const nextUrl = parseNextLink(response.headers.get("Link"));
    if (!nextUrl) break;

    const headers: Record<string, string> = { Accept: "application/json" };
    if (authHeader) {
      headers["Authorization"] = authHeader;
    }

    response = await fetch(nextUrl, { headers });
  }

  return results;
}

/**
 * Async generator that yields pages of results one at a time.
 * Useful for processing large datasets without loading everything into memory.
 *
 * @example
 * ```ts
 * for await (const page of paginateAll(response.response, (r) => r.json())) {
 *   console.log(`Processing ${page.length} items`);
 * }
 * ```
 */
export async function* paginateAll<T>(
  initialResponse: Response,
  parse: (response: Response) => Promise<T[]>,
  authHeader?: string
): AsyncGenerator<T[], void, unknown> {
  let response = initialResponse;

  while (true) {
    const items = await parse(response.clone());
    yield items;

    const nextUrl = parseNextLink(response.headers.get("Link"));
    if (!nextUrl) break;

    const headers: Record<string, string> = { Accept: "application/json" };
    if (authHeader) {
      headers["Authorization"] = authHeader;
    }

    response = await fetch(nextUrl, { headers });
  }
}

function parseNextLink(linkHeader: string | null): string | null {
  if (!linkHeader) return null;

  for (const part of linkHeader.split(",")) {
    const trimmed = part.trim();
    if (trimmed.includes('rel="next"')) {
      const match = trimmed.match(/<([^>]+)>/);
      return match?.[1] ?? null;
    }
  }

  return null;
}
