/**
 * Shared OAuth request primitives (SPEC §16), split out so the token-exchange
 * path can import them without pulling the whole device-flow module: the
 * token-lifetime ceiling, the per-request timeout resolver, and the abort
 * race that bounds a round trip even when a custom fetch ignores its
 * AbortSignal.
 */

/**
 * Upper bound (seconds) for an OAuth token's `expires_in`: 2_147_483_647 s
 * (~68 years) — cross-runtime safe and vastly beyond any realistic token
 * lifetime. A very large finite value (or a non-finite one from `1e400`)
 * makes `new Date(Date.now() + expires_in * 1000)` an Invalid Date whose
 * `getTime()` is NaN, so downstream expiry checks would treat the token as
 * never expiring. A value past this ceiling is a malformed response. Shared
 * across all five SDKs.
 */
export const MAX_TOKEN_LIFETIME_SECONDS = 2_147_483_647;

/**
 * Ceiling (ms) for a caller-supplied per-request timeout: the shared 3600 s
 * bound (Go's maxDeviceRequestTimeout, Python's _MAX_DEVICE_REQUEST_TIMEOUT,
 * Ruby's Fetcher::MAX_REQUEST_TIMEOUT). A large finite value would hold a
 * stalled request open for weeks, defeating the bounded-request guarantee.
 */
export const MAX_REQUEST_TIMEOUT_MS = 3600 * 1000;

/**
 * Coerce a caller-supplied request timeout (ms) to a finite, positive,
 * timer-safe value no greater than the shared ceiling. `setTimeout` silently
 * coerces a non-finite delay (NaN/Infinity) or one beyond its 32-bit range to
 * ~1 ms — an immediate abort that would masquerade as a transport failure.
 * Fall back to the operation's own default instead, mirroring how the other
 * SDKs normalize an invalid OAuth request timeout.
 */
export function resolveRequestTimeoutMs(timeoutMs: number, defaultMs: number): number {
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0 || timeoutMs > MAX_REQUEST_TIMEOUT_MS) {
    return defaultMs;
  }
  // Whole milliseconds, at least 1: timers truncate fractional delays toward
  // 0, so 0.5 would become an immediate abort.
  return Math.max(1, Math.floor(timeoutMs));
}

// A plain Error tagged "AbortError" rather than `new DOMException(...)`:
// DOMException is not guaranteed in every JS runtime that can run this SDK
// (referencing it there throws ReferenceError). isAbort() matches on
// `name === "AbortError"`, so cancellation stays runtime-agnostic.
export function abortError(): Error {
  const err = new Error("Aborted");
  err.name = "AbortError";
  return err;
}

/**
 * Races `run` against `signal`: rejects with AbortError the moment the signal
 * fires, even if the underlying promise NEVER settles. A cooperative fetch
 * already rejects on abort — this enforces the same contract on a custom fetch
 * that ignores its AbortSignal, so a late 200 cannot hand back a result and a
 * never-settling fetch cannot hold the public call past its timeout. An
 * already-aborted signal rejects without invoking `run`. A late settlement is
 * discarded (settling an already-settled promise is a no-op), and its
 * rejection path stays handled — no unhandled rejection escapes.
 */
export function raceAbort<T>(signal: AbortSignal, run: () => Promise<T>): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const onAbort = () => reject(abortError());
    if (signal.aborted) {
      onAbort();
      return;
    }
    signal.addEventListener("abort", onAbort, { once: true });
    // Microtask wrapper: a user-provided seam (custom fetch/sleepFn) can
    // throw SYNCHRONOUSLY despite the TS type — without this, that throw
    // would escape before the handlers attach and strand the listener.
    Promise.resolve()
      .then(() => {
        // The abort can win between entry and this microtask (the outer
        // promise has already rejected) — never invoke the seam post-abort;
        // an AbortSignal-ignoring fetch would still send the POST.
        if (signal.aborted) throw abortError();
        return run();
      })
      .then(
        (value) => {
          signal.removeEventListener("abort", onAbort);
          resolve(value);
        },
        (err) => {
          signal.removeEventListener("abort", onAbort);
          reject(err);
        }
      );
  });
}
