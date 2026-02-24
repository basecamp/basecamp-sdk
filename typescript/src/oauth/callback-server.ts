/**
 * Local HTTP callback server for OAuth 2.0 authorization code flow.
 *
 * Starts a temporary server to receive the authorization callback,
 * validates the state parameter, and extracts the authorization code.
 */

import { createServer, type Server } from "node:http";
import { BasecampError } from "../errors.js";

/**
 * Result from a successful OAuth callback.
 */
export interface CallbackResult {
  /** The authorization code */
  code: string;
  /** The state parameter (already validated) */
  state: string;
}

/**
 * Options for the callback server.
 */
export interface CallbackServerOptions {
  /** Port to listen on (default: 14923) */
  port?: number;
  /** Host to bind to (default: "localhost") */
  host?: string;
  /** Timeout in milliseconds (default: 120000) */
  timeoutMs?: number;
  /** Expected state parameter for CSRF validation */
  expectedState: string;
}

const SUCCESS_HTML = `<!DOCTYPE html>
<html><head><title>Authorization Complete</title></head>
<body style="font-family:system-ui;text-align:center;padding:40px">
<h1>Authorization complete</h1>
<p>You can close this window.</p>
</body></html>`;

const ERROR_HTML = `<!DOCTYPE html>
<html><head><title>Authorization Failed</title></head>
<body style="font-family:system-ui;text-align:center;padding:40px">
<h1>Authorization failed</h1>
<p>Please try again.</p>
</body></html>`;

/**
 * Starts a local HTTP server to receive the OAuth callback.
 *
 * The server listens for a single GET /callback request, validates
 * the state parameter, and extracts the authorization code.
 * It auto-closes after receiving the callback or on timeout.
 *
 * @param options - Server configuration
 * @returns The callback URL, a promise for the result, and a close function
 * @throws BasecampError if the port is already in use
 *
 * @example
 * ```ts
 * const { url, waitForCallback, close } = await startCallbackServer({
 *   expectedState: myState,
 * });
 *
 * // Use `url` as the redirect_uri in the authorization request
 * // Then wait for the user to complete the flow:
 * const { code } = await waitForCallback();
 * ```
 */
export async function startCallbackServer(
  options: CallbackServerOptions,
): Promise<{ url: string; waitForCallback: () => Promise<CallbackResult>; close: () => void }> {
  const {
    port = 14923,
    host = "localhost",
    timeoutMs = 120_000,
    expectedState,
  } = options;

  let resolve: (result: CallbackResult) => void;
  let reject: (err: Error) => void;
  const promise = new Promise<CallbackResult>((res, rej) => {
    resolve = res;
    reject = rej;
  });

  // Prevent unhandled rejection warnings when reject() fires before
  // the consumer has called waitForCallback(). The consumer's call
  // still receives the rejection.
  promise.catch(() => {});

  let settled = false;
  let timeoutId: ReturnType<typeof setTimeout> | undefined;

  const httpServer: Server = createServer((req, res) => {
    if (settled) {
      res.writeHead(404);
      res.end();
      return;
    }

    let reqUrl: URL;
    try {
      const reqHost = req.headers.host ?? `${host}:${port}`;
      reqUrl = new URL(req.url ?? "/", `http://${reqHost}`);
    } catch {
      res.writeHead(400, { "Content-Type": "text/html" });
      res.end(ERROR_HTML);
      return;
    }

    if (reqUrl.pathname !== "/callback") {
      res.writeHead(404);
      res.end();
      return;
    }

    const code = reqUrl.searchParams.get("code");
    const state = reqUrl.searchParams.get("state");
    const error = reqUrl.searchParams.get("error");

    if (error) {
      settled = true;
      const description = reqUrl.searchParams.get("error_description") ?? error;
      res.writeHead(400, { "Content-Type": "text/html" });
      res.end(ERROR_HTML);
      reject!(new BasecampError("auth", `OAuth callback error: ${description}`));
      scheduleClose();
      return;
    }

    if (!code || !state) {
      res.writeHead(400, { "Content-Type": "text/html" });
      res.end(ERROR_HTML);
      return;
    }

    if (state !== expectedState) {
      settled = true;
      res.writeHead(400, { "Content-Type": "text/html" });
      res.end(ERROR_HTML);
      reject!(new BasecampError("auth", "OAuth state mismatch — possible CSRF attack"));
      scheduleClose();
      return;
    }

    settled = true;
    res.writeHead(200, { "Content-Type": "text/html" });
    res.end(SUCCESS_HTML);
    resolve!({ code, state });
    scheduleClose();
  });

  function scheduleClose() {
    if (timeoutId) clearTimeout(timeoutId);
    // Close on next tick to allow the response to flush
    setImmediate(() => httpServer.close());
  }

  function close() {
    if (timeoutId) clearTimeout(timeoutId);
    if (!settled) {
      settled = true;
      reject!(new BasecampError("auth", "Callback server closed before receiving callback"));
    }
    httpServer.close();
  }

  // Start listening
  await new Promise<void>((listenResolve, listenReject) => {
    httpServer.on("error", (err: NodeJS.ErrnoException) => {
      if (err.code === "EADDRINUSE") {
        listenReject(
          new BasecampError("network", `Port ${port} is already in use — cannot start OAuth callback server`)
        );
      } else {
        listenReject(err);
      }
    });
    httpServer.listen(port, host, listenResolve);
  });

  // Resolve actual bound port (important when port=0 for OS-assigned ports)
  const addr = httpServer.address();
  const actualPort = typeof addr === "object" && addr ? addr.port : port;

  // Start timeout
  timeoutId = setTimeout(() => {
    if (!settled) {
      settled = true;
      reject!(new BasecampError("auth", `OAuth callback timed out after ${timeoutMs}ms`));
      httpServer.close();
    }
  }, timeoutMs);

  const url = `http://${host}:${actualPort}/callback`;
  return { url, waitForCallback: () => promise, close };
}
