/**
 * RFC 8628 device authorization grant — request + poll.
 *
 * `requestDeviceAuthorization` obtains a device/user code pair;
 * `pollDeviceToken` runs the §3.5 polling loop against the token endpoint. Both
 * are TLS-guarded. The polling clock is monotonic and injectable for tests.
 */

import { BasecampError, truncateErrorMessage } from "../errors.js";
import { requireSecureEndpoint } from "../security.js";
import { readBodyBounded } from "./discovery.js";
import { DeviceFlowError } from "./device-errors.js";
import { MAX_TOKEN_LIFETIME_SECONDS, abortError, raceAbort, resolveRequestTimeoutMs } from "./limits.js";

// Re-exported for existing importers; the declaration lives in limits.ts so
// non-device consumers need not pull this module.
export { MAX_TOKEN_LIFETIME_SECONDS };
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
 * Coerce a caller-supplied request timeout to the shared clamp
 * (`resolveRequestTimeoutMs`, limits.ts), falling back to the device flow's
 * own 30 s default — an invalid value would otherwise become an immediate
 * abort masquerading as a `DeviceFlowError("transport")` (and, in the poll
 * loop, as repeated timeout backoffs).
 */
function resolveDeviceTimeoutMs(timeoutMs: number): number {
  return resolveRequestTimeoutMs(timeoutMs, DEFAULT_DEVICE_TIMEOUT_MS);
}


/** slow_down bumps the interval by this many seconds, sustained (RFC 8628 §3.5). */
const SLOW_DOWN_INCREMENT_SECONDS = 5;

/** Cap on exponential backoff after connection timeouts. */
const MAX_BACKOFF_SECONDS = 60;

/** Cap on a device-auth / token response body (1 MiB) — these docs are tiny. */
const MAX_DEVICE_BODY_BYTES = 1 * 1024 * 1024;

/** Monotonic clock in milliseconds. Injectable so tests can advance time. */
export type MonotonicClock = () => number;

/**
 * Default monotonic clock (ms): `performance.now()`, which every supported
 * Node/browser runtime provides. The `Date.now()` branch is a last resort for
 * exotic runtimes without `performance` — wall-clock, so not strictly monotonic;
 * inject a real monotonic clock there if NTP steps matter.
 */
export const defaultClock: MonotonicClock = () =>
  typeof performance !== "undefined" ? performance.now() : Date.now();

/**
 * Validate one sample from an injected clock seam. `Number.isFinite` performs
 * no coercion, so a bigint, string, or Symbol sample fails exactly like a
 * NaN/Infinity one — the typed usage error, never a raw TypeError out of later
 * deadline arithmetic. Shared by the poller and the login orchestrator so both
 * validate EVERY sample the same way.
 */
export function validatedClockSample(sample: unknown, entry: string): number {
  if (typeof sample !== "number" || !Number.isFinite(sample)) {
    throw new BasecampError("usage", `${entry}: clock must return a finite number of milliseconds`);
  }
  return sample;
}

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
  /** Cancellation signal — aborting rejects with DeviceFlowError("cancelled"). */
  signal?: AbortSignal;
}

/**
 * Requests a device/user code pair (RFC 8628 §3.1–3.2).
 *
 * @throws DeviceFlowError("cancelled") when the signal aborts;
 *   DeviceFlowError("transport") on a network failure; BasecampError on
 *   validation / non-2xx.
 */
export async function requestDeviceAuthorization(
  params: RequestDeviceAuthorizationParams
): Promise<DeviceAuthorization> {
  const { deviceAuthorizationEndpoint, clientId, scope, fetch: customFetch = globalThis.fetch, timeoutMs = DEFAULT_DEVICE_TIMEOUT_MS, signal } = params;

  // An already-aborted signal makes no request at all — reject before the
  // fetch path so a direct caller cannot send (or a recording customFetch
  // observe) a device-code request post-cancellation. performDeviceLogin
  // checks at its own entry too; this covers direct callers.
  throwIfAborted(signal);

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
  // Chain the caller's cancellation into the request like postDeviceToken:
  // an already-aborted signal must not fire the POST at all, and an abort
  // mid-request surfaces as the contractual cancelled — never transport.
  const onAbort = () => controller.abort();
  signal?.addEventListener("abort", onAbort, { once: true });
  if (signal?.aborted) controller.abort();
  let response: Response;
  let text: string;
  try {
    // The whole round trip runs INSIDE a race against the controller's signal:
    // a cooperative fetch rejects on abort anyway, but a custom fetch that
    // ignores its AbortSignal must not hold this call past its timeout or hand
    // back a code pair after cancellation — the race rejects the moment the
    // timeout or the caller's abort fires, and a late settlement is discarded.
    ({ response, text } = await raceAbort(controller.signal, async () => {
      let raced: Response;
      try {
        raced = await customFetch(deviceAuthorizationEndpoint, {
          method: "POST",
          headers: { "Content-Type": "application/x-www-form-urlencoded", Accept: "application/json" },
          body: body.toString(),
          signal: controller.signal,
          // Never chase an attacker-influenced Location: a 3xx surfaces below as a
          // non-2xx api_error rather than a followed request.
          redirect: "manual",
        });
      } catch (err) {
        if (isAbort(err) && signal?.aborted) {
          throw new DeviceFlowError("cancelled", "Device flow cancelled");
        }
        throw new DeviceFlowError("transport", `Device authorization request failed: ${errMessage(err)}`, {
          cause: err instanceof Error ? err : undefined,
        });
      }
      // A non-2xx (including a suppressed 3xx) is a hard failure whose body is
      // unused, so surface it by status BEFORE draining the body. Otherwise a
      // slow/never-ending error body could time out mid-read and be misclassified
      // as a retryable transport failure instead of the api_error it is.
      if (raced.status < 200 || raced.status >= 300) {
        // Release the unread stream (non-blocking) so repeated failures don't retain
        // sockets / connection-pool resources; the status error surfaces immediately.
        void raced.body?.cancel().catch(() => {});
        throw new BasecampError("api_error", `Device authorization failed with status ${raced.status}`, {
          httpStatus: raced.status,
        });
      }
      // Bounded/streaming read: an oversized body aborts before it is fully
      // buffered. The abort timer stays armed until the read completes, so a
      // stalled response STREAM times out just like a stalled request; an
      // oversized body is already api_error, and any other stream failure
      // (including the timeout's AbortError) maps to transport rather than
      // escaping raw.
      let racedText: string;
      try {
        racedText = await readBodyBounded(raced, MAX_DEVICE_BODY_BYTES, "device authorization");
      } catch (err) {
        if (err instanceof BasecampError) throw err;
        if (isAbort(err) && signal?.aborted) {
          throw new DeviceFlowError("cancelled", "Device flow cancelled");
        }
        throw new DeviceFlowError("transport", `Device authorization response read failed: ${errMessage(err)}`, {
          cause: err instanceof Error ? err : undefined,
        });
      }
      return { response: raced, text: racedText };
    }));
  } catch (err) {
    // Already-classified failures from the raced work pass through; a raw
    // AbortError here means the race itself lost — the caller aborted, or the
    // timeout fired while a non-cooperative fetch refused to settle.
    if (err instanceof DeviceFlowError || err instanceof BasecampError) throw err;
    if (isAbort(err) && signal?.aborted) {
      throw new DeviceFlowError("cancelled", "Device flow cancelled");
    }
    if (isAbort(err)) {
      throw new DeviceFlowError("transport", "Device authorization request failed: timed out");
    }
    throw err;
  } finally {
    clearTimeout(timeoutId);
    signal?.removeEventListener("abort", onAbort);
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

  // Re-check after the round-trip: a custom fetch that ignores its
  // AbortSignal can complete after the abort, and a direct caller must get
  // cancelled — never a code minted post-cancellation. (performDeviceLogin
  // re-checks too; this covers direct callers.)
  throwIfAborted(signal);

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
  /**
   * Absolute monotonic deadline (ms, same clock as `clock`) anchoring the
   * code's expiry at ISSUANCE. When set it overrides the `clock() + expiresIn`
   * anchoring so time elapsed before this call is never handed back to the
   * lifetime. Must be finite and no later than `expiresIn` seconds from now —
   * a deadline can only shorten the validated lifetime, never extend it.
   */
  deadlineAtMs?: number;
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
  // expiresIn MAY be fractional — performDeviceLogin legitimately passes a
  // fractional remaining lifetime after deducting display-hook time — but the
  // polling interval is whole seconds (RFC 8628), matching the response
  // validation and the integer-typed Go/Kotlin APIs: a fractional interval
  // (0.001) would otherwise permit ~1000 polls per second.
  if (!Number.isFinite(expiresIn) || expiresIn <= 0 || expiresIn > MAX_DEVICE_SECONDS) {
    throw new BasecampError(
      "usage",
      `pollDeviceToken: expiresIn must be a positive number of seconds no greater than ${MAX_DEVICE_SECONDS}`
    );
  }
  if (!Number.isInteger(params.interval) || params.interval <= 0 || params.interval > MAX_DEVICE_SECONDS) {
    throw new BasecampError(
      "usage",
      `pollDeviceToken: interval must be a positive whole number of seconds no greater than ${MAX_DEVICE_SECONDS}`
    );
  }

  // Same caller-input sanity for the optional absolute deadline: Infinity polls
  // forever, NaN defeats every deadline comparison and wait clamp, and a
  // deadline later than the VALIDATED expiresIn would extend polling past the
  // server-issued code lifetime. The bound is expiresIn from now — the
  // issuance-anchored deadline performDeviceLogin passes is always at or
  // before it (the clock only advances after issuance). A deadline at/below
  // "now" is legal — it surfaces as device_flow_expired, matching an
  // issuance-anchored deadline fully consumed by a slow display hook.
  // EVERY sample of the injected clock is validated, not just the first: a
  // NaN/Infinity sample poisons the deadline math, and setTimeout(NaN)
  // coerces to 0 — a tight poll loop instead of a fast usage failure. The
  // wrapper preserves scripted-clock step counts (one underlying call per
  // sample).
  const safeClock: MonotonicClock = () => validatedClockSample(clock(), "pollDeviceToken");
  const nowMs = safeClock();
  if (
    params.deadlineAtMs !== undefined &&
    (!Number.isFinite(params.deadlineAtMs) || params.deadlineAtMs > nowMs + expiresIn * 1000)
  ) {
    throw new BasecampError(
      "usage",
      "pollDeviceToken: deadlineAtMs must be a finite timestamp no later than expiresIn seconds from now"
    );
  }

  // Normalize the per-request timeout ONCE at entry: the remaining-lifetime
  // clamp below takes min() against it, and an invalid runtime value (NaN,
  // Infinity, non-positive) would otherwise poison the min into the 30s
  // default INSIDE postDeviceToken — letting a near-expiry poll run the full
  // default budget past the deadline.
  const effectiveTimeoutMs = resolveDeviceTimeoutMs(timeoutMs);

  // Server-driven poll interval (initial + sustained slow_down bumps), tracked
  // SEPARATELY from the transient-timeout backoff: the wait is the larger of the
  // two, so intermittent timeouts never permanently inflate the poll cadence.
  let intervalSeconds = params.interval;
  let backoffSeconds = intervalSeconds;
  // One-shot next-wait override from a 429 too_many_requests Retry-After
  // (SPEC §16): consumed by the next wait, never inflating the slow_down
  // interval. 0 = none.
  let overrideWaitSeconds = 0;
  const deadline = params.deadlineAtMs ?? nowMs + expiresIn * 1000;

  const body = new URLSearchParams();
  body.set("grant_type", DEVICE_CODE_GRANT_TYPE);
  body.set("device_code", deviceCode);
  body.set("client_id", clientId);

  for (;;) {
    throwIfAborted(signal);

    // Read the clock ONCE per iteration and reuse it for both the deadline check
    // and the remaining-lifetime clamp: two separate reads could straddle the
    // deadline and yield a negative wait for the (possibly injected) sleep seam.
    const now = safeClock();
    // Check the deadline before sleeping so a long display hook, a stalled prior
    // request, or a long backoff cannot carry us past expiry undetected.
    // Sub-millisecond remainders count as expired: performance.now() is
    // fractional, and a <1ms residue would truncate into 0ms timers — a tight
    // loop right at expiry.
    if (deadline - now < 1) {
      throw new DeviceFlowError("expired", "Device code expired before authorization completed");
    }
    // Wait the larger of the server interval and the timeout backoff, clamped
    // to the remaining lifetime (guaranteed >= 1ms here) so the wait never
    // overshoots the monotonic deadline. FLOOR to whole milliseconds: ceil
    // would round a fractional remainder past the deadline, and the <1ms
    // guard above keeps the floor at >= 1.
    const remainingMs = deadline - now;
    // max(1, floor(...)): floor stays inside the remaining lifetime, and the
    // 1ms floor keeps a caller-supplied sub-millisecond interval from
    // degrading into a 0ms hot loop.
    const waitMs = Math.max(1, Math.floor(Math.min(
      Math.max(intervalSeconds, backoffSeconds, overrideWaitSeconds) * 1000,
      remainingMs
    )));
    overrideWaitSeconds = 0; // one-shot: consumed by this wait, then gone
    try {
      // Race the injected sleep against the signal: a custom sleepFn that
      // ignores its signal argument must not hold a cancelled poll open until
      // its timer fires (or forever, for a never-settling seam).
      await (signal ? raceAbort(signal, () => sleepFn(waitMs, signal)) : sleepFn(waitMs, signal));
    } catch (err) {
      // The caller aborted the signal mid-wait: surface the contractual
      // cancellation, never let a raw AbortError/DOMException escape.
      if (isAbort(err)) {
        throw new DeviceFlowError("cancelled", "Device flow cancelled");
      }
      throw err;
    }

    const postRemainingMs = deadline - safeClock();
    if (postRemainingMs < 1) {
      throw new DeviceFlowError("expired", "Device code expired before authorization completed");
    }

    let result: TokenPollResult;
    try {
      // Bound the request by the REMAINING code lifetime as well as the
      // per-request timeout: near expiry, a stalled token POST must not hold
      // the flow past the monotonic deadline for the full request budget.
      result = await postDeviceToken(
        tokenEndpoint,
        body,
        customFetch,
        Math.min(effectiveTimeoutMs, postRemainingMs),
        signal
      );
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

    // A custom fetch that ignores its AbortSignal can complete a 200 after the
    // caller aborted: re-check before handing back the credential — never a
    // token returned after the caller asked to stop (Go's success branch does
    // the same via ctx.Err()).
    if (result.kind === "token") {
      throwIfAborted(signal);
      return result.token;
    }

    switch (result.error) {
      case "authorization_pending":
        continue;
      case "too_many_requests":
        // Retryable ONLY as the exact 429 + too_many_requests pair (SPEC §16).
        // The next wait honors a positive integral Retry-After delta via a
        // one-shot max(interval, Retry-After) override — a missing/malformed
        // header falls back to the current interval, and the override decays
        // after one wait.
        if (result.status !== 429) {
          throw new BasecampError("api_error", `Device token request failed: ${result.error}`, {
            httpStatus: result.status,
          });
        }
        overrideWaitSeconds = parseRetryAfterSeconds(result.retryAfter);
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
  | { kind: "error"; error: string; status: number; retryAfter: string | null };

/**
 * Validates a Retry-After delta for the 429 poll contract (SPEC §16): a
 * positive integral number of seconds. A representable delta beyond
 * MAX_DEVICE_SECONDS (the shared 32-bit-ms timer bound) CLAMPS to the
 * ceiling — the wait rule clips to the remaining code lifetime, honoring the
 * throttle. Anything else — missing, an HTTP-date, fractional, non-positive,
 * or unrepresentable (beyond 2^53) — returns 0 so the caller falls back to
 * the current interval. Trimming is ASCII SP/HTAB only (RFC 9110
 * OWS) — NOT String.prototype.trim(), whose Unicode whitespace (NBSP above
 * all) would trim a malformed value into validity.
 */
export function parseRetryAfterSeconds(header: string | null): number {
  if (!header) return 0;
  const trimmed = header.replace(/^[ \t]+|[ \t]+$/g, "");
  if (!/^\d+$/.test(trimmed)) return 0;
  // The shared 10-significant-digit bound (Python/Ruby mirror it; Go/Kotlin
  // get it from bounded int parses): strip leading zeros first so a padded
  // in-range delta ("00000000030") is honored, then treat longer strings as
  // unrepresentable — interval fallback — instead of feeding parseInt an
  // unbounded digit string. Comfortably covers MAX_DEVICE_SECONDS (7 digits).
  const significant = trimmed.replace(/^0+/, "") || "0";
  if (significant.length > 10) return 0;
  const parsed = parseInt(significant, 10);
  // Safe-integer, not merely integer: parseInt("9".repeat(20)) yields an
  // integer-valued double past 2^53 — unrepresentable → interval fallback. A
  // representable delta beyond the shared device ceiling CLAMPS instead: the
  // wait rule clamps to the remaining code lifetime anyway, so an over-ceiling
  // throttle waits out the rest of the lifetime rather than resending before
  // the server's throttle.
  if (!Number.isSafeInteger(parsed) || parsed <= 0) return 0;
  return Math.min(parsed, MAX_DEVICE_SECONDS);
}

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
    // The whole round trip runs INSIDE a race against the controller's signal
    // (which fires on the per-request timeout AND the caller's abort): a custom
    // fetch that ignores its AbortSignal must not hold the poll past its
    // timeout or hand back a token after cancellation. The race's AbortError
    // classifies in the poll loop exactly like a cooperative fetch's — caller
    // abort → cancelled, timeout → transient backoff.
    return await raceAbort(controller.signal, async () => {
      const response = await customFetch(tokenEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded", Accept: "application/json" },
        body: body.toString(),
        signal: controller.signal,
        // Never chase a Location: a redirected token poll is treated as an
        // api_error below rather than followed.
        redirect: "manual",
      });
      // A suppressed 3xx is never a valid OAuth response and its body is unused —
      // fail by status BEFORE draining the body. Otherwise a redirect that slowly
      // streams its body could time out mid-read and be misclassified as a
      // connection timeout, which the poll loop would back off and retry (until the
      // device code expires) instead of surfacing the api_error now.
      if (response.status >= 300 && response.status < 400) {
        // Release the unread stream (non-blocking) so a redirecting endpoint under a
        // long poll can't retain sockets / connection-pool resources.
        void response.body?.cancel().catch(() => {});
        throw new BasecampError("api_error", `Device token endpoint returned redirect status ${response.status}`, {
          httpStatus: response.status,
        });
      }
      // Every remaining status outside 200 and 4xx is terminal WITHOUT its body
      // (only a 200 carries the token and only a 4xx the OAuth error code) —
      // classify it before the read, like the 3xx above, so a 201/500 that
      // stalls while streaming its body cannot abort mid-read and be retried as
      // a transient timeout until the code expires.
      if (response.status !== 200 && !(response.status >= 400 && response.status < 500)) {
        void response.body?.cancel().catch(() => {});
        throw new BasecampError("api_error", `Device token request failed with status ${response.status}`, {
          httpStatus: response.status,
        });
      }
      // Bounded/streaming read: an oversized body aborts before it is fully buffered.
      // A 4xx still reads the body — that is how authorization_pending/slow_down and
      // other OAuth errors are carried.
      const text = await readBodyBounded(response, MAX_DEVICE_BODY_BYTES, "device token");
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
      // Exactly HTTP 200, not response.ok (any 2xx): RFC 8628/6749 token
      // responses are 200, and SPEC §16 pins the contract. A nonstandard 201/202
      // carrying an access_token must not prematurely complete polling — it
      // falls through to the OAuth-error path and terminates as api_error.
      if (response.status === 200) {
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
        // resource: absent and JSON null are unset; when present it must be a
        // non-empty string (SPEC §16) — an empty binding is not a binding.
        if (token.resource != null && !isNonEmptyString(token.resource)) {
          throw new BasecampError("api_error", "Device token response resource must be a non-empty string when present", {
            httpStatus: response.status,
          });
        }
        return {
          kind: "token",
          token: {
            accessToken: token.access_token,
            // `?? undefined`: validation admits JSON null (absent per SPEC),
            // but the public type is `string | undefined` — never leak null.
            refreshToken: token.refresh_token ?? undefined,
            tokenType: token.token_type || "Bearer",
            expiresIn: token.expires_in ?? undefined,
            expiresAt: token.expires_in != null ? new Date(Date.now() + token.expires_in * 1000) : undefined,
            scope: token.scope ?? undefined,
            resource: token.resource ?? undefined,
          },
        };
      }
      // Recognize OAuth protocol error codes ONLY on a 4xx (RFC 8628 §3.5 error
      // responses are 400-class): a nonstandard 2xx (201/202) or a 5xx carrying
      // a crafted authorization_pending body must not keep the loop polling —
      // only a 200 can produce a token and only a 4xx a protocol state. The
      // `error` must also be a non-empty string: a non-string (`{"error": 123}`)
      // is not an OAuth error code. Everything else falls back to http_<status>,
      // which the loop terminates as api_error.
      const rawError = (data as { error?: unknown }).error;
      // truncateErrorMessage at extraction (SPEC §9's 500-unit cap): the server
      // controls this string and an unrecognized value is interpolated into the
      // api_error message. Real protocol codes are short, so classification is
      // unaffected.
      let error =
        response.status >= 400 && response.status < 500 && typeof rawError === "string" && rawError !== ""
          ? truncateErrorMessage(rawError)
          : `http_${response.status}`;
      // A 429 recognizes ONLY too_many_requests (the exact retryable pair): a
      // throttling endpoint whose body parrots authorization_pending/slow_down
      // must not keep the loop polling until code expiry.
      if (response.status === 429 && error !== "too_many_requests") {
        error = `http_${response.status}`;
      }
      return { kind: "error", error, status: response.status, retryAfter: response.headers.get("Retry-After") };
    });
  } finally {
    clearTimeout(timeoutId);
    signal?.removeEventListener("abort", onAbort);
  }
}

// abortError and raceAbort live in limits.ts (shared with the token-exchange
// path); the poll loop and sleep below use them unchanged.

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
    // Race: the signal can abort between the `signal?.aborted` check above and
    // attaching this listener. With { once }, that abort event is already spent, so
    // the listener would never fire and the promise would only settle at the full
    // timeout. Re-check and handle it manually, dropping the now-dead listener.
    if (signal?.aborted) {
      signal.removeEventListener("abort", onAbort);
      onAbort();
    }
  });
}

export function throwIfAborted(signal?: AbortSignal): void {
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
