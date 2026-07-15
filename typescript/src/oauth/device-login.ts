/**
 * RFC 8628 device login orchestration.
 *
 * `performDeviceLogin` accepts an ALREADY-SELECTED OAuthConfig (from discovery),
 * guards the device capability, requests a device code, surfaces it through a
 * display hook, and polls for the token.
 */

import { DeviceFlowError } from "./device-errors.js";
import {
  requestDeviceAuthorization,
  pollDeviceToken,
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
  /** Cancellation signal for the polling loop. */
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
  const supportsDeviceGrant = config.grantTypesSupported?.includes(DEVICE_CODE_GRANT_TYPE) ?? false;
  if (!config.deviceAuthorizationEndpoint || !supportsDeviceGrant) {
    throw new DeviceFlowError(
      "unavailable",
      "The selected authorization server does not support the device authorization grant"
    );
  }

  const auth = await requestDeviceAuthorization({
    deviceAuthorizationEndpoint: config.deviceAuthorizationEndpoint,
    clientId,
    scope,
    fetch: customFetch,
  });

  // The code's lifetime starts at issuance, not after display: a slow display
  // hook must eat into the deadline, never reset it. Capture the monotonic clock
  // (ms) at issuance, then deduct the elapsed display time so polling anchors its
  // deadline against the REMAINING lifetime. `expiresIn` is seconds; `clock()` is
  // ms, so convert the elapsed span before subtracting.
  const issuedAt = clock();
  await display(auth);
  const remainingSeconds = auth.expiresIn - (clock() - issuedAt) / 1000;
  if (remainingSeconds <= 0) {
    throw new DeviceFlowError("expired", "Device code expired before authorization completed");
  }

  return pollDeviceToken({
    tokenEndpoint: config.tokenEndpoint,
    clientId,
    deviceCode: auth.deviceCode,
    interval: auth.interval,
    expiresIn: remainingSeconds,
    signal,
    clock,
    fetch: customFetch,
    sleepFn,
  });
}
