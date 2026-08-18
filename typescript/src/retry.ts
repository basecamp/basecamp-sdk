/**
 * The SDK's retry primitive: the attempt loop extracted from the client's
 * retrying fetch so that both the middleware-chained JSON path (client.ts)
 * and the raw-fetch download hop 1 (download.ts) share one budget/backoff/
 * abort policy. The loop is transport-agnostic — callers own attempt
 * preparation (auth, request construction) via `makeAttempt` and hook
 * emission via `RetryEmit`.
 */

// errors.ts imports nothing, so this edge introduces no cycle.
import { parseRetryAfter } from "./errors.js";

/**
 * Retry configuration matching x-basecamp-retry extension schema.
 */
export interface RetryConfig {
  maxAttempts: number;
  baseDelayMs: number;
  backoff: "exponential" | "linear" | "constant";
  retryOn: number[];
}

/** Default retry config used when no operation-specific config is available */
export const DEFAULT_RETRY_CONFIG: RetryConfig = {
  maxAttempts: 3,
  baseDelayMs: 1000,
  backoff: "exponential",
  retryOn: [429, 503],
};

/** No-retry config for non-idempotent POST operations */
export const NO_RETRY_CONFIG: RetryConfig = {
  maxAttempts: 1,
  baseDelayMs: 0,
  backoff: "constant",
  retryOn: [],
};

const MAX_JITTER_MS = 100;

/**
 * Ceiling on the backoff term (SPEC §7, "Backoff Ceiling"). Jitter is added
 * after the clamp, so the longest single backoff sleep is this plus
 * `MAX_JITTER_MS`.
 */
export const MAX_BACKOFF_DELAY_MS = 30_000;

/**
 * Lifecycle seams the retry loop emits through. The loop begins EVERY attempt
 * and finalizes the attempts it abandons (before the backoff sleep); the
 * terminal outcome — the response it returns, or the error it throws — is
 * deliberately NOT finalized here. The caller owns the terminal attempt's end
 * so it can record post-transform results (the client's cache middleware may
 * rewrite a 304 into a cached 200 before its lifecycle middleware finalizes).
 */
export interface RetryEmit {
  /** An attempt is beginning (fired for attempt 1 and every retry). */
  begin(attempt: number): void;
  /** An attempt was abandoned in favor of a retry (never the terminal one). */
  finalize(outcome: { statusCode: number; error?: Error }): void;
  /** A retry is coming: `failedAttempt` just ended, `failedAttempt + 1` is next. */
  retrying(failedAttempt: number, error: Error, delayMs: number): void;
}

/**
 * Marks an error thrown during attempt PREPARATION (e.g. a retry's auth
 * refresh) rather than by the transport. The retry loop rethrows it as-is —
 * no retry classification, no budget spent — and the caller unwraps `reason`
 * so the original error's identity survives to its error path.
 */
export class TerminalRetryError extends Error {
  constructor(readonly reason: unknown) {
    super("attempt preparation failed terminally");
    this.name = "TerminalRetryError";
  }
}

/**
 * Runs `makeAttempt` under the retry policy in `config`.
 *
 * `config.maxAttempts` is a total attempt count — the caller passes the
 * effective budget (e.g. 1 when retry is disabled). Status retry is gated on
 * the declared `retryOn` set; 429 honors Retry-After. Transport errors retry
 * on the same budget, except aborts, which are terminal no matter what the
 * budget says: a caller cancellation must not re-send, and a request-timeout
 * budget is shared by every attempt and backoff — once it fires, a retry
 * would instantly re-reject. Terminal errors are rethrown as-is so their
 * identity survives to the caller.
 *
 * `signal`, when given, is the authoritative abort test: a caller can abort
 * with a CUSTOM reason — AbortController.abort(reason) — and fetch then
 * rejects with that reason, not a DOMException named AbortError. The
 * DOMException check remains for abort-shaped rejections that arrive without
 * an aborted signal (e.g. a per-attempt timeout controller).
 */
export async function executeWithRetry(
  makeAttempt: (attempt: number) => Promise<Response>,
  config: RetryConfig,
  emit: RetryEmit,
  signal?: AbortSignal | null,
): Promise<Response> {
  let attempt = 1;
  emit.begin(attempt);

  for (;;) {
    let response: Response;
    try {
      response = await makeAttempt(attempt);
    } catch (error) {
      // Attempt preparation failed — the caller marked it terminal. Rethrow
      // the marker itself; the caller unwraps it at its own boundary.
      if (error instanceof TerminalRetryError) {
        throw error;
      }

      const isAbort =
        signal?.aborted === true ||
        (error instanceof DOMException &&
          (error.name === "AbortError" || error.name === "TimeoutError"));
      if (isAbort || attempt >= config.maxAttempts) {
        throw error;
      }

      // Network-error retry rides the same budget as status retry: a caller
      // that resolves a no-retry policy (maxAttempts 1) is rethrown above
      // after its single attempt.
      const cause = error instanceof Error ? error : new Error(String(error));
      const delay = calculateBackoffDelay(config, attempt - 1);

      emit.finalize({ statusCode: 0, error: cause });
      emit.retrying(attempt, cause, delay);

      await sleep(delay, signal ?? undefined);

      // Same placement rationale as the status path below.
      attempt += 1;
      emit.begin(attempt);
      continue;
    }

    // Terminal: a status outside the declared retryOn set, or a spent
    // budget — maxAttempts is a total attempt count, so attempt N is
    // terminal when it equals the cap.
    if (!config.retryOn.includes(response.status) || attempt >= config.maxAttempts) {
      return response;
    }

    // For 429, respect Retry-After; otherwise back off. The header goes through
    // errors.ts's parseRetryAfter — the single SPEC §6 implementation — rather
    // than a local parseInt: 0, a negative value and an unparseable one all
    // come back undefined and fall through to backoff, where the local copy
    // this replaced turned them into a zero or negative sleep.
    const retryAfterSeconds =
      response.status === 429
        ? parseRetryAfter(response.headers.get("Retry-After"))
        : undefined;
    const delay =
      retryAfterSeconds !== undefined
        ? retryAfterSeconds * 1000
        : calculateBackoffDelay(config, attempt - 1);

    const statusError = new Error(
      `HTTP ${response.status}: ${response.statusText || "Request failed"}`,
    );

    // End the failed attempt before sleeping, so a slow backoff cannot leave
    // an attempt open, then announce the upcoming one.
    emit.finalize({ statusCode: response.status });
    emit.retrying(attempt, statusError, delay);

    // This response is being discarded, so release its stream before we sleep
    // rather than leaving it open across the backoff — otherwise a throttled
    // client holds a connection per in-flight retry and cannot reuse any of
    // them. The multipart transport in services/base.ts already does this.
    // Errors are ignored: the body may already be consumed or closed.
    void response.body?.cancel().catch(() => {});

    await sleep(delay, signal ?? undefined);

    // Begun after the backoff but before any work that can throw. After, so
    // the attempt's duration measures the request rather than the sleep;
    // before, so that if the caller's attempt preparation or the fetch
    // throws, its error path still finds a live attempt to finalize.
    // Starting it later would let onRetry announce an attempt and then never
    // account for it.
    attempt += 1;
    emit.begin(attempt);
  }
}

/**
 * `base * 2**exponent`, computed without ever forming `2**exponent`.
 *
 * JS has no `ldexp`, and `Math.pow(2, e)` is `Infinity` for `e > 1023` — so for
 * a denormal-small base the *product* is still an ordinary number long after the
 * multiplier has overflowed. Scaling in bounded steps keeps every intermediate
 * finite, which is what lets the exponent bound come from the base alone.
 */
function scaleByPowerOfTwo(base: number, exponent: number): number {
  let result = base;
  for (let remaining = exponent; remaining > 0; ) {
    const step = Math.min(remaining, 1000);
    result *= Math.pow(2, step);
    if (!Number.isFinite(result)) return Infinity;
    remaining -= step;
  }
  return result;
}

/**
 * Smallest exponent `e` with `base * 2**e >= MAX_BACKOFF_DELAY_MS`.
 *
 * Computed in the LOG domain rather than as `MAX_BACKOFF_DELAY_MS / base`. That
 * ratio is `Infinity` for any base below ~1.67e-304 ms, and falling back to a
 * fixed 1023 then saturates *early*: such a base reaches only a fraction of the
 * ceiling at exponent 1023, so returning the ceiling there overstates the
 * specified term instead of tracking it. The log form has no such cliff, so the
 * numeric backstop is gone entirely rather than merely made rarer.
 */
function saturatingExponent(base: number): number {
  // log2 is not correctly rounded here, so the estimate can land one either side
  // of the true boundary. Both corrections are bounded.
  let exponent = Math.max(Math.ceil(Math.log2(MAX_BACKOFF_DELAY_MS) - Math.log2(base)), 0);
  while (exponent > 0 && scaleByPowerOfTwo(base, exponent - 1) >= MAX_BACKOFF_DELAY_MS) exponent--;
  while (scaleByPowerOfTwo(base, exponent) < MAX_BACKOFF_DELAY_MS) exponent++;
  return exponent;
}

/**
 * The backoff term, saturating at {@link MAX_BACKOFF_DELAY_MS} (SPEC §7).
 *
 * Shared by both of the SDK's retry loops — this one and the raw multipart
 * loop in `services/base.ts` — so the ceiling cannot be honored on one path
 * and not the other.
 *
 * The clamp is load-bearing rather than defensive. `Math.pow(2, attempt)`
 * reaches `Infinity` at attempt 1024, and `setTimeout` clamps an out-of-range
 * delay to **1ms**: the failure mode of an unbounded backoff in JS is not a
 * long sleep, it is a tight retry loop against a server that is already
 * answering 429/503. Well before that, the computed delays run to millennia.
 *
 * The exponent is compared against the point where the term reaches the ceiling
 * *before* `Math.pow` is evaluated, and that point is derived from the
 * CONFIGURED base — the same contract Go, Kotlin and Swift get from comparing
 * their multiplier against `MAX_BACKOFF_DELAY_MS / base` before multiplying. A
 * fixed exponent cap plus a trailing `Math.min` looks equivalent and is not:
 * for a small enough base the capped product never reaches the ceiling, so the
 * delay plateaus below it forever instead of saturating at it.
 *
 * `attempt` is the 0-indexed retry count (first retry = 0), matching the SPEC
 * §7 `retry_index`.
 */
export function saturatingBackoff(
  baseDelayMs: number,
  backoff: RetryConfig["backoff"],
  attempt: number,
): number {
  const base = baseDelayMs > 0 ? baseDelayMs : 0;
  const index = attempt > 0 ? attempt : 0;
  if (base === 0) return 0;

  let delay: number;
  switch (backoff) {
    case "exponential":
      if (index >= saturatingExponent(base)) return MAX_BACKOFF_DELAY_MS;
      delay = scaleByPowerOfTwo(base, index);
      break;
    case "linear": {
      // Compared before multiplying, so `Infinity * 0`-shaped intermediates
      // never arise. `index + 1` is finite for any finite index.
      const multiplier = index + 1;
      if (multiplier >= MAX_BACKOFF_DELAY_MS / base) return MAX_BACKOFF_DELAY_MS;
      delay = base * multiplier;
      break;
    }
    case "constant":
    default:
      delay = base;
  }

  return Math.min(delay, MAX_BACKOFF_DELAY_MS);
}

export function calculateBackoffDelay(config: RetryConfig, attempt: number): number {
  const delay = saturatingBackoff(config.baseDelayMs, config.backoff, attempt);

  // Add jitter (0-100ms)
  const jitter = Math.random() * MAX_JITTER_MS;
  return delay + jitter;
}

/**
 * Signal-aware sleep for backoff waits: resolves after `ms`, or rejects with
 * the signal's abort reason the moment it fires. Without this, a caller abort
 * or the request-timeout budget expiring during a backoff would leave the
 * request pending for the full delay and then start another attempt — begin,
 * auth refresh, fetch — against an already-aborted signal.
 */
export function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(signal.reason as Error);
      return;
    }
    const onAbort = () => {
      clearTimeout(timer);
      reject(signal!.reason as Error);
    };
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}
