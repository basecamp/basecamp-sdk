/**
 * RFC 8628 device authorization grant — request + poll.
 *
 * `requestDeviceAuthorization` obtains a device/user code pair;
 * `pollDeviceToken` runs the §3.5 polling loop against the token endpoint. Both
 * are TLS-guarded. The polling clock is monotonic and injectable for tests.
 */

import { BasecampError } from "../errors.js";
import { requireSecureEndpoint } from "../security.js";
import { readBodyBounded } from "./discovery.js";
import { DeviceFlowError } from "./device-errors.js";
import type { DeviceAuthorization, OAuthToken, RawTokenResponse, OAuthErrorResponse } from "./types.js";

/** URN grant type for the device authorization grant. */
export const DEVICE_CODE_GRANT_TYPE = "urn:ietf:params:oauth:grant-type:device_code";

/** Default polling interval when the server omits `interval` (RFC 8628 §3.2). */
const DEFAULT_INTERVAL_SECONDS = 5;

/**
 * Upper bound (seconds) for `expires_in` / `interval`: 2_147_483 s (~24.8 days),
 * the largest whole-second duration whose millisecond form (2_147_483_000 ms)
 * fits a 32-bit signed timer. Beyond it a `setTimeout` silently clamps an
 * out-of-range delay to 1 ms (hot poll loop) and deadline arithmetic can
 * overflow. Far above any real device-code lifetime. Shared across all five SDKs
 * (SPEC.md §16).
 */
const MAX_DEVICE_SECONDS = 2_147_483;

/** Default per-request timeout (ms) for every device-flow HTTP round-trip. */
const DEFAULT_DEVICE_TIMEOUT_MS = 30_000;

/**
 * Coerce a caller-supplied request timeout (ms) to a finite, positive, timer-safe
 * value. `setTimeout` silently coerces a non-finite delay (NaN/Infinity) or one
 * beyond its 32-bit range to ~1 ms — an immediate abort that would masquerade as a
 * `DeviceFlowError("transport")` (and, in the poll loop, as repeated timeout
 * backoffs). Fall back to the default instead, mirroring how Python/Ruby normalize
 * an invalid device timeout. `MAX_DEVICE_SECONDS * 1000` stays safely under the
 * 2^31 ms `setTimeout` ceiling.
 */
function resolveDeviceTimeoutMs(timeoutMs: number): number {
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0 || timeoutMs > MAX_DEVICE_SECONDS * 1000) {
    return DEFAULT_DEVICE_TIMEOUT_MS;
  }
  return timeoutMs;
}

/**
 * Upper bound (seconds) for an OAuth token's `expires_in`: 2_147_483_647 s
 * (~68 years) — cross-runtime safe and vastly beyond any realistic token
 * lifetime. Unlike {@link MAX_DEVICE_SECONDS} this bounds `expiresAt` Date
 * arithmetic rather than a timer: a very large finite value (or a non-finite one
 * from `1e400`) makes `new Date(Date.now() + expires_in * 1000)` an Invalid Date
 * whose `getTime()` is NaN, so downstream expiry checks would treat the token as
 * never expiring. A value past this ceiling is a malformed response. Shared
 * across all five SDKs.
 */
const MAX_TOKEN_LIFETIME_SECONDS = 2_147_483_647;

/** slow_down bumps the interval by this many seconds, sustained (RFC 8628 §3.5). */
const SLOW_DOWN_INCREMENT_SECONDS = 5;

/** Cap on exponential backoff after connection timeouts. */
const MAX_BACKOFF_SECONDS = 60;

/** Cap on a device-auth / token response body (1 MiB) — these docs are tiny. */
const MAX_DEVICE_BODY_BYTES = 1 * 1024 * 1024;

/** Monotonic clock in milliseconds. Injectable so tests can advance time. */
export type MonotonicClock = () => number;

/** Default monotonic clock (ms): `performance.now()` when present, else `Date.now()`. */
export const defaultClock: MonotonicClock = () =>
  typeof performance !== "undefined" ? performance.now() : Date.now();

/** Raw RFC 8628 device authorization response. */
interface RawDeviceAuthorization {
  device_code?: string;
  user_code?: string;
  verification_uri?: string;
  // May be a JSON null on the wire — treated as absent (normalized to undefined),
  // matching the Go/Kotlin decoders that cannot distinguish null from absent.
  verification_uri_complete?: string | null;
  expires_in?: number;
  interval?: number | null;
}

/**
 * Parameters for {@link requestDeviceAuthorization}.
 */
export interface RequestDeviceAuthorizationParams {
  /** The device_authorization_endpoint from discovery. */
  deviceAuthorizationEndpoint: string;
  /** The public client id (e.g. "basecamp-cli"). */
  clientId: string;
  /** Requested scope. Omitted from the request entirely when unset → server default `read`. */
  scope?: string;
  /** Custom fetch (testing). */
  fetch?: typeof globalThis.fetch;
  /** Request timeout in milliseconds (default: 30000). */
  timeoutMs?: number;
}

/**
 * Requests a device/user code pair (RFC 8628 §3.1–3.2).
 *
 * @throws DeviceFlowError("transport") on a network failure; BasecampError on
 *   validation / non-2xx.
 */
export async function requestDeviceAuthorization(
  params: RequestDeviceAuthorizationParams
): Promise<DeviceAuthorization> {
  const { deviceAuthorizationEndpoint, clientId, scope, fetch: customFetch = globalThis.fetch, timeoutMs = DEFAULT_DEVICE_TIMEOUT_MS } = params;

  requireSecureEndpoint(deviceAuthorizationEndpoint, "device authorization endpoint");
  if (!clientId) {
    throw new BasecampError("validation", "Client ID is required for device authorization");
  }

  const body = new URLSearchParams();
  body.set("client_id", clientId);
  // Omit scope entirely when unset so the server applies its default (`read`).
  if (scope) body.set("scope", scope);

  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), resolveDeviceTimeoutMs(timeoutMs));
  let response: Response;
  let text: string;
  try {
    try {
      response = await customFetch(deviceAuthorizationEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded", Accept: "application/json" },
        body: body.toString(),
        signal: controller.signal,
        // Never chase an attacker-influenced Location: a 3xx surfaces below as a
        // non-2xx api_error rather than a followed request.
        redirect: "manual",
      });
    } catch (err) {
      throw new DeviceFlowError("transport", `Device authorization request failed: ${errMessage(err)}`, {
        cause: err instanceof Error ? err : undefined,
      });
    }
    // Bounded/streaming read: an oversized body aborts before it is fully
    // buffered. The abort timer stays armed until the read completes, so a
    // stalled response STREAM times out just like a stalled request; an
    // oversized body is already api_error, and any other stream failure
    // (including the timeout's AbortError) maps to transport rather than
    // escaping raw.
    try {
      text = await readBodyBounded(response, MAX_DEVICE_BODY_BYTES, "device authorization");
    } catch (err) {
      if (err instanceof BasecampError) throw err;
      throw new DeviceFlowError("transport", `Device authorization response read failed: ${errMessage(err)}`, {
        cause: err instanceof Error ? err : undefined,
      });
    }
  } finally {
    clearTimeout(timeoutId);
  }

  // Reject any non-2xx (including a suppressed 3xx) before parsing.
  if (response.status < 200 || response.status >= 300) {
    throw new BasecampError("api_error", `Device authorization failed with status ${response.status}`, {
      httpStatus: response.status,
    });
  }
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    throw new BasecampError("api_error", "Failed to parse device authorization response", {
      httpStatus: response.status,
    });
  }
  // A valid-JSON-but-non-object body (null, array, number, string) is malformed —
  // fail as api_error before any property deref, never a raw TypeError.
  if (typeof data !== "object" || data === null || Array.isArray(data)) {
    throw new BasecampError("api_error", "Device authorization response is not a JSON object", {
      httpStatus: response.status,
    });
  }

  return validateDeviceAuthorization(data as RawDeviceAuthorization, response.status);
}

/**
 * True iff `value` is a positive integer. Rejects fractional numbers, booleans
 * (`typeof true === "boolean"`), NaN, Infinity, and undefined — narrowing to
 * `number` so validated fields carry a definite type downstream.
 */
function isPositiveInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value > 0;
}

/** True iff `value` is a non-empty string. Rejects numbers/booleans/null so a
 * malformed server response can't smuggle a non-string code/URI past validation. */
function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}

function validateDeviceAuthorization(data: RawDeviceAuthorization, status: number): DeviceAuthorization {
  // Every validation error carries the (2xx) status so a malformed success body
  // is diagnosable as such — uniform with the token-poll raises and the other SDKs.
  // Type-check, not just truthiness: a JSON number is truthy but is not a usable
  // code/URI, so it must fail as api_error rather than flow into the poll loop.
  if (
    !isNonEmptyString(data.device_code) ||
    !isNonEmptyString(data.user_code) ||
    !isNonEmptyString(data.verification_uri)
  ) {
    throw new BasecampError("api_error", "Invalid device authorization response: missing or non-string required fields", {
      httpStatus: status,
    });
  }
  // expires_in drives a monotonic deadline; interval drives poll delays. Both
  // must be positive integers within MAX_DEVICE_SECONDS — a fractional value
  // (0.5) yields a sub-second delay and a huge value overflows the ms timer
  // (Node clamps an out-of-range setTimeout to 1 ms → hot poll loop). Booleans
  // and non-int-valued numbers are likewise rejected.
  if (!isPositiveInteger(data.expires_in) || data.expires_in > MAX_DEVICE_SECONDS) {
    throw new BasecampError(
      "api_error",
      `Invalid device authorization response: expires_in must be a positive integer no greater than ${MAX_DEVICE_SECONDS}`,
      { httpStatus: status }
    );
  }
  let interval = DEFAULT_INTERVAL_SECONDS;
  // A JSON `null` interval is treated as ABSENT (cross-SDK contract: the Go and
  // Kotlin decoders cannot distinguish null from absent), so it takes the default.
  if (data.interval !== undefined && data.interval !== null) {
    if (!isPositiveInteger(data.interval) || data.interval > MAX_DEVICE_SECONDS) {
      throw new BasecampError(
        "api_error",
        `Invalid device authorization response: interval must be a positive integer no greater than ${MAX_DEVICE_SECONDS}`,
        { httpStatus: status }
      );
    }
    interval = data.interval;
  }
  // Optional; when present it must be a string. A JSON `null` is treated as
  // ABSENT (cross-SDK contract: the Go and Kotlin decoders cannot distinguish
  // null from absent) and normalized to undefined below — a non-string value
  // (number/array) is still rejected as a malformed shape.
  if (
    data.verification_uri_complete !== undefined &&
    data.verification_uri_complete !== null &&
    typeof data.verification_uri_complete !== "string"
  ) {
    throw new BasecampError("api_error", "Invalid device authorization response: verification_uri_complete must be a string", {
      httpStatus: status,
    });
  }
  return {
    deviceCode: data.device_code,
    userCode: data.user_code,
    verificationUri: data.verification_uri,
    verificationUriComplete: data.verification_uri_complete ?? undefined,
    expiresIn: data.expires_in,
    interval,
  };
}

/**
 * Parameters for {@link pollDeviceToken}.
 */
export interface PollDeviceTokenParams {
  /** The token_endpoint from discovery. */
  tokenEndpoint: string;
  /** The public client id. */
  clientId: string;
  /** The device_code from {@link requestDeviceAuthorization}. */
  deviceCode: string;
  /** Polling interval in seconds. */
  interval: number;
  /** Code lifetime in seconds (monotonic deadline). */
  expiresIn: number;
  /** Cancellation signal — aborting rejects with DeviceFlowError("cancelled"). */
  signal?: AbortSignal;
  /** Injectable monotonic clock (ms). Default performance.now(). */
  clock?: MonotonicClock;
  /** Custom fetch (testing). */
  fetch?: typeof globalThis.fetch;
  /** Per-request timeout in milliseconds (default: 30000). */
  timeoutMs?: number;
  /**
   * Injectable sleep (testing). Receives the wait in ms and the cancellation
   * signal; defaults to a real, abortable timer. Lets tests assert the interval
   * schedule (slow_down, backoff) without real delays.
   */
  sleepFn?: (ms: number, signal?: AbortSignal) => Promise<void>;
}

/**
 * Polls the token endpoint until the user approves, denies, or the codes expire
 * (RFC 8628 §3.4–3.5). Handles authorization_pending, sustained slow_down (+5s),
 * a monotonic expiry deadline, exponential backoff on connection timeouts, and
 * cooperative cancellation.
 */
export async function pollDeviceToken(params: PollDeviceTokenParams): Promise<OAuthToken> {
  const {
    tokenEndpoint,
    clientId,
    deviceCode,
    expiresIn,
    signal,
    clock = defaultClock,
    fetch: customFetch = globalThis.fetch,
    timeoutMs = DEFAULT_DEVICE_TIMEOUT_MS,
    sleepFn = sleep,
  } = params;

  requireSecureEndpoint(tokenEndpoint, "token endpoint");

  // Caller-input sanity (usage, not the RFC response validation): a non-finite
  // or oversized duration builds a broken deadline or an unschedulable wait.
  // Fractional values are ACCEPTED — performDeviceLogin legitimately passes a
  // fractional remaining lifetime after deducting display-hook time; whole-second
  // enforcement applies only to raw server responses (validateDeviceAuthorization).
  for (const [name, value] of [["expiresIn", expiresIn], ["interval", params.interval]] as const) {
    if (!Number.isFinite(value) || value <= 0 || value > MAX_DEVICE_SECONDS) {
      throw new BasecampError(
        "usage",
        `pollDeviceToken: ${name} must be a positive number of seconds no greater than ${MAX_DEVICE_SECONDS}`
      );
    }
  }

  // Server-driven poll interval (initial + sustained slow_down bumps), tracked
  // SEPARATELY from the transient-timeout backoff: the wait is the larger of the
  // two, so intermittent timeouts never permanently inflate the poll cadence.
  let intervalSeconds = params.interval;
  let backoffSeconds = intervalSeconds;
  const deadline = clock() + expiresIn * 1000;

  const body = new URLSearchParams();
  body.set("grant_type", DEVICE_CODE_GRANT_TYPE);
  body.set("device_code", deviceCode);
  body.set("client_id", clientId);

  for (;;) {
    throwIfAborted(signal);

    // Read the clock ONCE per iteration and reuse it for both the deadline check
    // and the remaining-lifetime clamp: two separate reads could straddle the
    // deadline and yield a negative wait for the (possibly injected) sleep seam.
    const now = clock();
    // Check the deadline before sleeping so a long display hook, a stalled prior
    // request, or a long backoff cannot carry us past expiry undetected.
    if (now >= deadline) {
      throw new DeviceFlowError("expired", "Device code expired before authorization completed");
    }
    // Wait the larger of the server interval and the timeout backoff, clamped
    // to the remaining lifetime (guaranteed > 0 here) so the wait never
    // overshoots the monotonic deadline.
    const remainingMs = deadline - now;
    const waitMs = Math.min(Math.max(intervalSeconds, backoffSeconds) * 1000, remainingMs);
    try {
      await sleepFn(waitMs, signal);
    } catch (err) {
      // The caller aborted the signal mid-wait: surface the contractual
      // cancellation, never let a raw AbortError/DOMException escape.
      if (isAbort(err)) {
        throw new DeviceFlowError("cancelled", "Device flow cancelled");
      }
      throw err;
    }

    if (clock() >= deadline) {
      throw new DeviceFlowError("expired", "Device code expired before authorization completed");
    }

    let result: TokenPollResult;
    try {
      result = await postDeviceToken(tokenEndpoint, body, customFetch, timeoutMs, signal);
    } catch (err) {
      if (isAbort(err) && signal?.aborted) {
        throw new DeviceFlowError("cancelled", "Device flow cancelled");
      }
      if (isAbort(err)) {
        // Our own per-request timeout fired → connection timeout: back off and
        // retry. The server interval is left untouched so recovery is instant.
        backoffSeconds = Math.min(backoffSeconds * 2, MAX_BACKOFF_SECONDS);
        continue;
      }
      // An already-typed BasecampError (e.g. a malformed 2xx token response, an
      // oversized body, or a redirect) is a server/API fault — propagate it
      // unchanged rather than mislabeling it a retryable transport failure.
      if (err instanceof BasecampError) throw err;
      // Any other transport failure ends the flow.
      throw new DeviceFlowError("transport", `Device token poll failed: ${errMessage(err)}`, {
        cause: err instanceof Error ? err : undefined,
      });
    }

    // ANY completed HTTP round-trip (token, authorization_pending, slow_down,
    // other OAuth error) resets the timeout backoff to the server interval.
    backoffSeconds = intervalSeconds;

    if (result.kind === "token") return result.token;

    switch (result.error) {
      case "authorization_pending":
        continue;
      case "slow_down":
        intervalSeconds += SLOW_DOWN_INCREMENT_SECONDS;
        backoffSeconds = intervalSeconds;
        continue;
      case "access_denied":
        throw new DeviceFlowError("access_denied", "The authorization request was denied");
      case "expired_token":
        throw new DeviceFlowError("expired", "Device code expired before authorization completed");
      default:
        throw new BasecampError("api_error", `Device token request failed: ${result.error}`, {
          httpStatus: result.status,
        });
    }
  }
}

type TokenPollResult =
  | { kind: "token"; token: OAuthToken }
  | { kind: "error"; error: string; status: number };

async function postDeviceToken(
  tokenEndpoint: string,
  body: URLSearchParams,
  customFetch: typeof globalThis.fetch,
  timeoutMs: number,
  signal: AbortSignal | undefined
): Promise<TokenPollResult> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), resolveDeviceTimeoutMs(timeoutMs));
  const onAbort = () => controller.abort();
  signal?.addEventListener("abort", onAbort, { once: true });
  // If the signal was ALREADY aborted, the "abort" event has fired and the
  // once-listener never runs — abort the controller directly so the fetch below
  // does not proceed (and even return a token) after cancellation.
  if (signal?.aborted) controller.abort();
  try {
    const response = await customFetch(tokenEndpoint, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded", Accept: "application/json" },
      body: body.toString(),
      signal: controller.signal,
      // Never chase a Location: a redirected token poll is treated as an
      // api_error below rather than followed.
      redirect: "manual",
    });
    // Bounded/streaming read: an oversized body aborts before it is fully buffered.
    const text = await readBodyBounded(response, MAX_DEVICE_BODY_BYTES, "device token");
    // A suppressed 3xx is never a valid OAuth response — fail as api_error.
    if (response.status >= 300 && response.status < 400) {
      throw new BasecampError("api_error", `Device token endpoint returned redirect status ${response.status}`, {
        httpStatus: response.status,
      });
    }
    let parsed: unknown;
    try {
      parsed = JSON.parse(text);
    } catch {
      throw new BasecampError("api_error", "Failed to parse device token response", {
        httpStatus: response.status,
      });
    }
    // A valid-JSON-but-non-object body (null, array, number, string) is a
    // malformed OAuth response — fail as api_error before any property deref,
    // never a raw crash on `data.access_token`/`data.error`.
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
      throw new BasecampError("api_error", "Device token response is not a JSON object", {
        httpStatus: response.status,
      });
    }
    const data = parsed as RawTokenResponse | OAuthErrorResponse;
    if (response.ok) {
      const token = data as RawTokenResponse;
      // Non-empty string, not merely truthy: a numeric access_token is not a
      // usable credential and must fail as api_error, not be returned downstream.
      if (!isNonEmptyString(token.access_token)) {
        throw new BasecampError("api_error", "Device token response missing or non-string access_token", {
          httpStatus: response.status,
        });
      }
      // expires_in is optional (RFC 6749 §5.1), but when present it must be a
      // finite positive WHOLE number within MAX_TOKEN_LIFETIME_SECONDS. A
      // non-finite value (1e400 → Infinity) or a very large finite one would flow
      // into Date arithmetic and yield an Invalid Date whose getTime() is NaN, so
      // expiry checks downstream would treat the token as never expiring. Whole
      // seconds match the device-duration rule — every SDK validates the decoded
      // numeric value explicitly to reject a fractional lifetime; an
      // integer-valued float (3600.0) is still accepted.
      if (token.expires_in != null &&
          (typeof token.expires_in !== "number" || !Number.isInteger(token.expires_in) ||
            token.expires_in <= 0 || token.expires_in > MAX_TOKEN_LIFETIME_SECONDS)) {
        throw new BasecampError(
          "api_error",
          `Device token response expires_in must be a finite positive whole number no greater than ${MAX_TOKEN_LIFETIME_SECONDS} seconds`,
          { httpStatus: response.status }
        );
      }
      // token_type/refresh_token/scope are optional strings — a non-string value
      // is a malformed response, not a usable credential field.
      if (token.token_type != null && !isNonEmptyString(token.token_type)) {
        throw new BasecampError("api_error", "Device token response token_type must be a non-empty string", {
          httpStatus: response.status,
        });
      }
      if (token.refresh_token != null && typeof token.refresh_token !== "string") {
        throw new BasecampError("api_error", "Device token response refresh_token must be a string", {
          httpStatus: response.status,
        });
      }
      if (token.scope != null && typeof token.scope !== "string") {
        throw new BasecampError("api_error", "Device token response scope must be a string", {
          httpStatus: response.status,
        });
      }
      return {
        kind: "token",
        token: {
          accessToken: token.access_token,
          refreshToken: token.refresh_token,
          tokenType: token.token_type || "Bearer",
          expiresIn: token.expires_in ?? undefined,
          expiresAt: token.expires_in != null ? new Date(Date.now() + token.expires_in * 1000) : undefined,
          scope: token.scope,
        },
      };
    }
    const error = (data as OAuthErrorResponse).error || `http_${response.status}`;
    return { kind: "error", error, status: response.status };
  } finally {
    clearTimeout(timeoutId);
    signal?.removeEventListener("abort", onAbort);
  }
}

// A plain Error tagged "AbortError" rather than `new DOMException(...)`:
// DOMException is not guaranteed in every JS runtime that can run this SDK
// (referencing it there throws ReferenceError). isAbort() matches on
// `name === "AbortError"`, so cancellation stays runtime-agnostic.
function abortError(): Error {
  const err = new Error("Aborted");
  err.name = "AbortError";
  return err;
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(abortError());
      return;
    }
    // Declare the handler before the timer so neither forward-references the
    // other's binding (avoids "used before declaration"); `timer` is assigned
    // immediately below, before either callback can run.
    let timer: ReturnType<typeof setTimeout>;
    const onAbort = () => {
      clearTimeout(timer);
      reject(abortError());
    };
    timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

function throwIfAborted(signal?: AbortSignal): void {
  if (signal?.aborted) {
    throw new DeviceFlowError("cancelled", "Device flow cancelled");
  }
}

function isAbort(err: unknown): boolean {
  return err instanceof Error && err.name === "AbortError";
}

function errMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
