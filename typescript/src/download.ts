import type { AuthStrategy } from "./auth-strategy.js";
import type { BasecampHooks, OperationInfo, RequestInfo } from "./hooks.js";
import { BasecampError, Errors, errorFromResponse } from "./errors.js";
import { safeInvoke } from "./hooks.js";
import {
  TerminalRetryError,
  executeWithRetry,
  type RetryConfig,
  type RetryEmit,
} from "./retry.js";

/**
 * The fixed hop-1 retry policy (SPEC §14): three total attempts when retry is
 * enabled, retrying network errors plus {429, 502, 503, 504} — never 500 —
 * with exponential backoff, honoring Retry-After on 429. DownloadURL is
 * deliberately absent from behavior-model.json, so the policy is passed to
 * the retry primitive directly rather than looked up by operation. There is
 * no public knob for the attempt count.
 */
const DOWNLOAD_MAX_ATTEMPTS = 3;
const DOWNLOAD_RETRY_ON = [429, 502, 503, 504];
const DOWNLOAD_RETRY_BASE_DELAY_MS = 1000;

/** The redirects hop 1 dispatches on (SPEC §14 step 3d) and hop 2 refuses. */
const REDIRECT_STATUSES = [301, 302, 303, 307, 308];

/**
 * Result of downloading file content from a URL.
 */
export interface DownloadResult {
  /** File content stream — caller must consume or cancel */
  body: ReadableStream<Uint8Array>;
  /** MIME type of the file */
  contentType: string;
  /** Size in bytes, or -1 if unknown */
  contentLength: number;
  /** Filename extracted from the URL */
  filename: string;
}

/**
 * Extracts a filename from the last path segment of a URL.
 * Falls back to "download" if the URL is unparseable or has no path segments.
 */
export function filenameFromURL(rawURL: string): string {
  try {
    const u = new URL(rawURL);
    const segments = u.pathname.split("/").filter(Boolean);
    if (segments.length === 0) return "download";
    const last = segments[segments.length - 1]!;
    if (last === "" || last === "." || last === "/") return "download";
    try {
      return decodeURIComponent(last);
    } catch {
      return last;
    }
  } catch {
    return "download";
  }
}

/** Parse Content-Length header defensively, returning -1 for missing/invalid values. */
function parseContentLength(headers: Headers): number {
  const raw = headers.get("Content-Length");
  if (!raw) return -1;
  const parsed = parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : -1;
}

/** Dependencies for createDownloadURL factory */
interface DownloadDeps {
  authStrategy: AuthStrategy;
  userAgent: string;
  baseUrl: string;
  hooks?: BasecampHooks;
  requestTimeoutMs: number;
  /** false collapses hop 1 to exactly one attempt (SPEC §14). */
  enableRetry: boolean;
  /**
   * Test seam for the fixed policy's backoff base. Not wired to any client
   * option — production callers omit it and get the 1-second base.
   */
  retryBaseDelayMs?: number;
}

/**
 * Creates a downloadURL function bound to the client's auth and configuration.
 *
 * Handles the full download flow: URL rewriting to the configured API host,
 * authenticated first hop (which typically 302s to a signed download URL),
 * and unauthenticated second hop to fetch the actual file content. Common
 * inputs include storage blob URLs from <bc-attachment> elements and any
 * other signed-download URL that routes through the API. Neither hop follows
 * a redirect on its own: hop 1's is the dispatch to hop 2, and a redirect on
 * hop 2 is an error (SPEC §14 "Hop-2 Redirect Policy").
 */
export function createDownloadURL(deps: DownloadDeps): (rawURL: string) => Promise<DownloadResult> {
  const { authStrategy, userAgent, baseUrl, hooks, requestTimeoutMs, enableRetry, retryBaseDelayMs } = deps;

  return async (rawURL: string): Promise<DownloadResult> => {
    // Validation
    if (!rawURL) {
      throw new BasecampError("usage", "download URL is required");
    }
    let parsed: URL;
    try {
      parsed = new URL(rawURL);
    } catch {
      throw new BasecampError("usage", "download URL must be an absolute URL");
    }
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      throw new BasecampError("usage", "download URL must be an absolute URL");
    }

    // Operation hooks
    const op: OperationInfo = {
      service: "Client",
      operation: "DownloadURL",
      resourceType: "download",
      isMutation: false,
    };

    const start = performance.now();
    safeInvoke(hooks, "onOperationStart", op);

    let operationError: Error | undefined;
    try {
      // URL rewriting: replace origin with baseUrl origin, preserve path+query
      const base = new URL(baseUrl);
      const rewrittenURL = `${base.origin}${parsed.pathname}${parsed.search}${parsed.hash}`;

      // Hop 1: Authenticated API request (capture redirect), under the fixed
      // hop-1 retry policy. Attempt 1's auth runs here, outside the loop, so
      // a failing strategy surfaces raw without request hooks — matching the
      // client path, where the middleware authenticates before the loop.
      const headers = new Headers({
        "User-Agent": userAgent,
      });
      await authStrategy.authenticate(headers);

      const downloadRetryConfig: RetryConfig = {
        maxAttempts: enableRetry ? DOWNLOAD_MAX_ATTEMPTS : 1,
        baseDelayMs: retryBaseDelayMs ?? DOWNLOAD_RETRY_BASE_DELAY_MS,
        backoff: "exponential",
        retryOn: DOWNLOAD_RETRY_ON,
      };

      const requestInfoFor = (attempt: number): RequestInfo => ({
        method: "GET",
        url: rewrittenURL,
        attempt,
      });

      // The download path has no lifecycle middleware, so the emit seams fire
      // the hooks directly. executeWithRetry finalizes only the attempts it
      // abandons; the terminal outcome is finalized after the loop below.
      let currentAttempt = 1;
      let attemptStart = performance.now();
      const emit: RetryEmit = {
        begin: (attempt) => {
          currentAttempt = attempt;
          attemptStart = performance.now();
          safeInvoke(hooks, "onRequestStart", requestInfoFor(attempt));
        },
        finalize: (outcome) => {
          const durationMs = Math.round(performance.now() - attemptStart);
          safeInvoke(hooks, "onRequestEnd", requestInfoFor(currentAttempt), {
            statusCode: outcome.statusCode,
            durationMs,
            fromCache: false,
            ...(outcome.error ? { error: outcome.error } : {}),
          });
        },
        retrying: (failedAttempt, error, delayMs) => {
          safeInvoke(hooks, "onRetry", requestInfoFor(failedAttempt), failedAttempt + 1, error, delayMs);
        },
      };

      const makeAttempt = async (attempt: number): Promise<Response> => {
        if (attempt > 1) {
          // Refresh auth (the token may have rotated since the last attempt),
          // so EVERY hop-1 attempt goes out authenticated. A throwing refresh
          // is terminal: the marker carries it raw past retry classification.
          try {
            await authStrategy.authenticate(headers);
          } catch (error) {
            throw new TerminalRetryError(error);
          }
        }
        // Per-attempt timeout: the controller aborts only its own fetch, and
        // an abort-shaped rejection is terminal in the loop — a request that
        // consumed its whole time budget is a slowness shape a retry tends to
        // repeat, not a transient blip.
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), requestTimeoutMs);
        try {
          return await fetch(rewrittenURL, {
            method: "GET",
            headers,
            redirect: "manual",
            signal: controller.signal,
          });
        } finally {
          clearTimeout(timeoutId);
        }
      };

      let response: Response;
      try {
        response = await executeWithRetry(makeAttempt, downloadRetryConfig, emit);
      } catch (err) {
        if (err instanceof TerminalRetryError) {
          // A retry's auth refresh failed: finalize the live attempt, then
          // surface the strategy's own error raw — auth faults are neither
          // transport failures nor API errors.
          const reason = err.reason;
          const error = reason instanceof Error ? reason : new Error(String(reason));
          emit.finalize({ statusCode: 0, error });
          throw reason;
        }
        const error = err instanceof Error ? err : new Error(String(err));
        emit.finalize({ statusCode: 0, error });
        throw Errors.network(error.message, error);
      }

      // Terminal attempt's end — the loop deliberately leaves it to us.
      emit.finalize({ statusCode: response.status });

      // Dispatch on response status
      const isRedirect = REDIRECT_STATUSES.includes(response.status);
      if (isRedirect) {
        // Redirect — extract Location, cancel body, proceed to hop 2
        const location = response.headers.get("Location");
        response.body?.cancel();
        if (!location) {
          throw new BasecampError(
            "api_error",
            `redirect ${response.status} with no Location header`,
          );
        }
        // Resolve relative Location against the rewritten API URL
        const resolvedLocation = new URL(location, rewrittenURL).href;

        // Hop 2: fetch from signed URL (no auth, no timeout, no request hooks).
        // `redirect: "manual"`, as on hop 1: the signed URL is the one
        // destination the API host named, and a redirect from it is refused below
        // rather than followed wherever the storage host points (#805). Node's
        // fetch hands the redirect back with its status; a browser's yields an
        // opaqueredirect (status 0), which the !ok branch refuses the same way.
        let signedResponse: Response;
        try {
          signedResponse = await fetch(resolvedLocation, { redirect: "manual" });
        } catch (err) {
          const error = err instanceof Error ? err : new Error(String(err));
          throw Errors.network(error.message, error);
        }

        if (REDIRECT_STATUSES.includes(signedResponse.status)) {
          signedResponse.body?.cancel();
          throw new BasecampError(
            "api_error",
            `redirect ${signedResponse.status} on the signed download hop is not followed`,
            { httpStatus: signedResponse.status },
          );
        }

        if (!signedResponse.ok) {
          signedResponse.body?.cancel();
          throw new BasecampError(
            "api_error",
            `download failed with status ${signedResponse.status}`,
            { httpStatus: signedResponse.status },
          );
        }

        if (!signedResponse.body) {
          throw new BasecampError("api_error", "download response has no body");
        }

        return {
          body: signedResponse.body,
          contentType: signedResponse.headers.get("Content-Type") ?? "",
          contentLength: parseContentLength(signedResponse.headers),
          filename: filenameFromURL(rawURL),
        };
      }

      if (response.status >= 200 && response.status < 300) {
        // Direct download — no second hop
        if (!response.body) {
          throw new BasecampError("api_error", "download response has no body");
        }

        return {
          body: response.body,
          contentType: response.headers.get("Content-Type") ?? "",
          contentLength: parseContentLength(response.headers),
          filename: filenameFromURL(rawURL),
        };
      }

      // Error response
      throw await errorFromResponse(response);
    } catch (err) {
      if (err instanceof Error) operationError = err;
      throw err;
    } finally {
      safeInvoke(hooks, "onOperationEnd", op, {
        durationMs: Math.round(performance.now() - start),
        error: operationError,
      });
    }
  };
}
