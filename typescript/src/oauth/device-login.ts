/**
 * RFC 8628 device login orchestration.
 *
 * `performDeviceLogin` accepts an ALREADY-SELECTED OAuthConfig (from discovery),
 * guards the device capability, requests a device code, surfaces it through a
 * display hook, and polls for the token.
 */

import { BasecampError } from "../errors.js";
import { DeviceFlowError } from "./device-errors.js";
import {
  requestDeviceAuthorization,
  pollDeviceToken,
  throwIfAborted,
  DEVICE_CODE_GRANT_TYPE,
  defaultClock,
  type MonotonicClock,
} from "./device.js";
import type { OAuthConfig, OAuthToken, DeviceAuthorization } from "./types.js";

/**
 * Options for {@link performDeviceLogin}.
 */
export interface DeviceLoginOptions {
  /** The already-selected authorization-server config (from discovery). */
  config: OAuthConfig;
  /** The public client id (e.g. "basecamp-cli"). */
  clientId: string;
  /** Requested scope. Omitted → server default (`read`). */
  scope?: string;
  /**
   * Display hook: show the user the verification URI + user code. Called once,
   * after the device code is obtained and before polling begins.
   */
  display: (auth: DeviceAuthorization) => void | Promise<void>;
  /**
   * Cancellation signal for the WHOLE flow: checked before any work, threaded
   * into the device-code request (an abort cancels it in flight), re-checked
   * before the display hook, and honored throughout the polling loop. An
   * abort anywhere rejects with DeviceFlowError("cancelled") — a code is
   * never surfaced after cancellation.
   */
  signal?: AbortSignal;
  /** Injectable monotonic clock (ms) for the polling deadline. */
  clock?: MonotonicClock;
  /** Custom fetch (testing). */
  fetch?: typeof globalThis.fetch;
  /**
   * Injectable sleep (testing). Forwarded to the poll loop so tests can drive the
   * interval schedule without real delays; defaults to a real, abortable timer.
   */
  sleepFn?: (ms: number, signal?: AbortSignal) => Promise<void>;
}

/**
 * Runs the full device authorization grant against a selected config.
 *
 * @throws DeviceFlowError("unavailable") when the config cannot do device flow;
 *   other DeviceFlowError reasons on denial/expiry/transport/cancellation.
 */
export async function performDeviceLogin(options: DeviceLoginOptions): Promise<OAuthToken> {
  const { config, clientId, scope, display, signal, clock = defaultClock, fetch: customFetch, sleepFn } = options;

  // Capability guard requires BOTH the endpoint and the advertised grant type.
  // Require a genuine array: a manually-constructed config could supply
  // grantTypesSupported as a string, and String.prototype.includes would
  // substring-match and wrongly pass the guard (the other SDKs defend the same
  // way — Python/Ruby check the collection type, Go/Kotlin are statically typed).
  const supportsDeviceGrant =
    Array.isArray(config.grantTypesSupported) && config.grantTypesSupported.includes(DEVICE_CODE_GRANT_TYPE);
  if (!config.deviceAuthorizationEndpoint || !supportsDeviceGrant) {
    throw new DeviceFlowError(
      "unavailable",
      "The selected authorization server does not support the device authorization grant"
    );
  }

  // A non-function display is a usage error, not a late TypeError: it is the
  // only mechanism surfacing the verification code, so dereferencing it AFTER
  // the request would mint a code nobody can approve. Reject before any
  // network activity (matching Go) — a JS caller can pass anything.
  if (typeof display !== "function") {
    throw new BasecampError("usage", "performDeviceLogin requires a display callback function");
  }

  // Honor a cancellation raised before any work: an already-aborted signal
  // must not fire the authorization request or surface a code via display.
  throwIfAborted(signal);

  const auth = await requestDeviceAuthorization({
    deviceAuthorizationEndpoint: config.deviceAuthorizationEndpoint,
    clientId,
    scope,
    fetch: customFetch,
    // Threaded so an abort DURING the request cancels it in flight and
    // surfaces as the contractual cancelled — never a code shown post-cancel.
    signal,
  });

  // Re-check before surfacing the code: an abort racing the request's
  // completion must not reach the display hook.
  throwIfAborted(signal);

  // The code's lifetime starts at issuance, not after display: a slow display
  // hook must eat into the deadline, never reset it. Capture the monotonic clock
  // (ms) at issuance, then deduct the elapsed display time so polling anchors its
  // deadline against the REMAINING lifetime. `expiresIn` is seconds; `clock()` is
  // ms, so convert the elapsed span before subtracting.
  const issuedAt = clock();
  // Race an async display hook against the abort signal: a hook awaiting user
  // interaction must not hold a cancelled flow open until it settles. On
  // abort the hook's promise is left to settle on its own (JS cannot cancel
  // it) — the flow rejects promptly and never proceeds to polling.
  await new Promise<void>((resolve, reject) => {
    const onAbort = () => reject(new DeviceFlowError("cancelled", "Device flow cancelled"));
    signal?.addEventListener("abort", onAbort, { once: true });
    if (signal?.aborted) {
      // The abort won between the entry throwIfAborted and the registration
      // above: the event is already spent, so the once-listener would never
      // fire — and never auto-remove. Drop it before rejecting manually
      // (same race handling as the sleep helper).
      signal.removeEventListener("abort", onAbort);
      onAbort();
      return;
    }
    Promise.resolve()
      .then(() => {
        // The abort can win between scheduling and this deferred callback —
        // the outer promise has already rejected then, but this callback still
        // runs. Re-check so a cancelled flow never surfaces the code.
        if (signal?.aborted) return undefined;
        return display(auth);
      })
      .then(
        () => {
          signal?.removeEventListener("abort", onAbort);
          resolve();
        },
        (err) => {
          signal?.removeEventListener("abort", onAbort);
          reject(err);
        }
      );
  });
  const remainingSeconds = auth.expiresIn - (clock() - issuedAt) / 1000;
  // Re-check after the clock read: an abort landing after the display race
  // settled (its listener is already removed) must still win over expiry —
  // cancellation beats every other classification.
  throwIfAborted(signal);
  if (remainingSeconds <= 0) {
    throw new DeviceFlowError("expired", "Device code expired before authorization completed");
  }

  return pollDeviceToken({
    tokenEndpoint: config.tokenEndpoint,
    clientId,
    deviceCode: auth.deviceCode,
    interval: auth.interval,
    expiresIn: remainingSeconds,
    // Absolute issuance-anchored deadline: clock time elapsing between the
    // remainingSeconds computation above and pollDeviceToken's own clock()
    // anchor would otherwise be handed BACK to the code's lifetime.
    deadlineAtMs: issuedAt + auth.expiresIn * 1000,
    signal,
    clock,
    fetch: customFetch,
    sleepFn,
  });
}
