/**
 * Wire-capture utilities for the live canary.
 *
 * Both fetch sites in the SDK (openapi-fetch's default fetch for page 1 and
 * the fetchPage closure used for pagination follow-up) ultimately call
 * globalThis.fetch. Wrapping the global gives us a single chokepoint where
 * we can clone the response and record its raw bytes/headers without
 * affecting SDK behavior.
 *
 * Snapshot format per test (persisted by live-runner):
 *   { operation: "GetProject", pages: [{status, headers, body, ...}, ...], pages_count: N }
 *
 * The top-level `operation` field is added by the runner when serializing,
 * so cross-language replay runners (PR 3) can dispatch without re-parsing
 * the test fixture. Backwards compatible — existing readers ignore unknown
 * top-level keys.
 *
 * Schema validation runs per page; extras-observed reporting unions extras
 * across all pages.
 */

export interface WirePage {
  status: number;
  headers: Record<string, string>;
  /**
   * Parsed JSON body when the response body parses cleanly; raw text otherwise.
   * Always preserved as raw text in `bodyText` so callers can re-parse if needed.
   */
  body: unknown;
  bodyText: string;
  url: string;
}

export interface WireSnapshot {
  pages: WirePage[];
  pages_count: number;
}

/** Snapshot as persisted to disk: WireSnapshot plus the operation that produced it. */
export interface PersistedWireSnapshot extends WireSnapshot {
  operation: string;
}

export interface WireCaptureSession {
  /** Restore the original fetch. Idempotent. */
  restore(): void;
  /** Take the captured pages and reset the buffer for the next test. */
  drain(): WireSnapshot;
}

/**
 * Install a global fetch wrapper that records each call's raw response.
 * Returns a session object to drain captured pages and restore the original fetch.
 *
 * Caveats:
 * - Records every fetch made while installed — caller is responsible for
 *   draining between tests so pages don't bleed across operations.
 * - Body is read from a clone, so the original Response stream remains
 *   consumable by the SDK.
 * - Non-JSON response bodies are recorded as text and `body` falls back to
 *   the raw text.
 */
export function installWireCapture(): WireCaptureSession {
  const original = globalThis.fetch;
  let pages: WirePage[] = [];

  const wrapped: typeof fetch = async (input, init) => {
    const response = await original(input as RequestInfo | URL, init);
    try {
      const clone = response.clone();
      const text = await clone.text();
      const headers: Record<string, string> = {};
      response.headers.forEach((value, key) => {
        headers[key] = value;
      });
      let body: unknown = text;
      if (text.length > 0) {
        try {
          body = JSON.parse(text);
        } catch {
          // Leave as text — not JSON.
        }
      } else {
        body = null;
      }
      pages.push({
        status: response.status,
        headers,
        body,
        bodyText: text,
        url: response.url || (typeof input === "string" ? input : ""),
      });
    } catch {
      // Capture is best-effort; never break the SDK if cloning fails.
    }
    return response;
  };

  globalThis.fetch = wrapped;

  return {
    restore(): void {
      if (globalThis.fetch === wrapped) {
        globalThis.fetch = original;
      }
    },
    drain(): WireSnapshot {
      const captured = pages;
      pages = [];
      return { pages: captured, pages_count: captured.length };
    },
  };
}
