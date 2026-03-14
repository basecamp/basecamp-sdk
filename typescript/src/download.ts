import type { AuthStrategy } from "./auth-strategy.js";
import type { BasecampHooks, OperationInfo, RequestInfo, RequestResult } from "./hooks.js";
import { BasecampError, Errors, errorFromResponse } from "./errors.js";
import { safeInvoke } from "./hooks.js";

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
}

/**
 * Creates a downloadURL function bound to the client's auth and configuration.
 *
 * Handles the full download flow: URL rewriting to the configured API host,
 * authenticated first hop (which typically 302s to a signed download URL),
 * and unauthenticated second hop to fetch the actual file content. Common
 * inputs include storage blob URLs from <bc-attachment> elements and any
 * other signed-download URL that routes through the API.
 */
export function createDownloadURL(deps: DownloadDeps): (rawURL: string) => Promise<DownloadResult> {
  const { authStrategy, userAgent, baseUrl, hooks, requestTimeoutMs } = deps;

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

      // Hop 1: Authenticated API request (capture redirect)
      const headers = new Headers({
        "User-Agent": userAgent,
      });
      await authStrategy.authenticate(headers);

      const requestInfo: RequestInfo = {
        method: "GET",
        url: rewrittenURL,
        attempt: 1,
      };
      safeInvoke(hooks, "onRequestStart", requestInfo);

      const reqStart = performance.now();
      let response: Response;
      try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), requestTimeoutMs);
        try {
          response = await fetch(rewrittenURL, {
            method: "GET",
            headers,
            redirect: "manual",
            signal: controller.signal,
          });
        } finally {
          clearTimeout(timeoutId);
        }
      } catch (err) {
        const durationMs = Math.round(performance.now() - reqStart);
        const error = err instanceof Error ? err : new Error(String(err));
        safeInvoke(hooks, "onRequestEnd", requestInfo, {
          statusCode: 0,
          durationMs,
          fromCache: false,
          error,
        });
        throw Errors.network(error.message, error);
      }

      const durationMs = Math.round(performance.now() - reqStart);
      safeInvoke(hooks, "onRequestEnd", requestInfo, {
        statusCode: response.status,
        durationMs,
        fromCache: false,
      });

      // Dispatch on response status
      const isRedirect = [301, 302, 303, 307, 308].includes(response.status);
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

        // Hop 2: fetch from signed URL (no auth, no timeout, no request hooks)
        let signedResponse: Response;
        try {
          signedResponse = await fetch(resolvedLocation);
        } catch (err) {
          const error = err instanceof Error ? err : new Error(String(err));
          throw Errors.network(error.message, error);
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
