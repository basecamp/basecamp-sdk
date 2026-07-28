/**
 * RFC 8628 device authorization grant tests.
 *
 * Timing is made deterministic with an injected sleep (records the interval
 * schedule, resolves immediately) and an injected monotonic clock.
 */

import { describe, it, expect, vi } from "vitest";
import { http as mswHttp, HttpResponse } from "msw";
import { server } from "../setup.js";
import {
  requestDeviceAuthorization,
  pollDeviceToken,
  performDeviceLogin,
  DeviceFlowError,
  DEVICE_CODE_GRANT_TYPE,
} from "../../src/oauth/index.js";
import { parseRetryAfterSeconds } from "../../src/oauth/device.js";
import type { OAuthConfig, DeviceAuthorization } from "../../src/oauth/types.js";
import { BasecampError } from "../../src/errors.js";

const ORIGIN = "https://issuer.device-test.example";
const DEVICE_ENDPOINT = `${ORIGIN}/oauth/device`;
const TOKEN_ENDPOINT = `${ORIGIN}/oauth/token`;

const deviceAuthResponse = {
  device_code: "dev-code-123",
  user_code: "WDJB-MJHT",
  verification_uri: `${ORIGIN}/device`,
  verification_uri_complete: `${ORIGIN}/device?user_code=WDJB-MJHT`,
  expires_in: 900,
  interval: 5,
};

const tokenResponse = {
  access_token: "device_access_token",
  refresh_token: "device_refresh_token",
  token_type: "Bearer",
  expires_in: 3600,
};

/** A sleep that records requested waits and resolves immediately. */
function recordingSleep() {
  const waits: number[] = [];
  const fn = (ms: number): Promise<void> => {
    waits.push(ms);
    return Promise.resolve();
  };
  return { waits, fn };
}

/** Serves a fixed sequence of token-endpoint responses, one per poll. */
function queueTokenResponses(responses: Array<{ status: number; body: object }>) {
  let i = 0;
  server.use(
    mswHttp.post(TOKEN_ENDPOINT, () => {
      const r = responses[Math.min(i, responses.length - 1)];
      i += 1;
      return HttpResponse.json(r.body, { status: r.status });
    })
  );
  return () => i;
}

const config: OAuthConfig = {
  issuer: ORIGIN,
  tokenEndpoint: TOKEN_ENDPOINT,
  deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
  grantTypesSupported: [DEVICE_CODE_GRANT_TYPE, "refresh_token"],
};

describe("requestDeviceAuthorization", () => {
  it("omits scope when unset and validates the response", async () => {
    let sentScope: string | null = "unset";
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, async ({ request }) => {
        const params = new URLSearchParams(await request.text());
        sentScope = params.get("scope");
        expect(params.get("client_id")).toBe("basecamp-cli");
        return HttpResponse.json(deviceAuthResponse);
      })
    );

    const auth = await requestDeviceAuthorization({
      deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
      clientId: "basecamp-cli",
    });

    expect(sentScope).toBeNull(); // scope omitted → server default (read)
    expect(auth.deviceCode).toBe("dev-code-123");
    expect(auth.userCode).toBe("WDJB-MJHT");
    expect(auth.interval).toBe(5);
  });

  it("defaults interval to 5 when the server omits it", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, interval: undefined })
      )
    );
    const auth = await requestDeviceAuthorization({
      deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
      clientId: "basecamp-cli",
    });
    expect(auth.interval).toBe(5);
  });

  it("rejects a non-positive expires_in (carrying the status)", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, expires_in: 0 })
      )
    );
    // A malformed 2xx device-auth field carries the HTTP status, like the
    // token-poll raises and the other SDKs.
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error", httpStatus: 200 });
  });

  it("rejects a fractional expires_in", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, expires_in: 0.5 })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("rejects a fractional interval", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, interval: 2.5 })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("rejects an oversized expires_in (1e100 is integer-valued but not schedulable)", async () => {
    // Number.isInteger(1e100) is true, so whole-second checking alone would
    // admit it; the shared cross-SDK ceiling (2147483 s) makes it api_error
    // before any ms-timer arithmetic can overflow or clamp.
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, expires_in: 1e100 })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("rejects an oversized interval", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, interval: 1e100 })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("rejects the first duration past the ceiling but accepts the ceiling itself", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, expires_in: 2_147_484 })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });

    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, expires_in: 2_147_483, interval: 2_147_483 })
      )
    );
    const auth = await requestDeviceAuthorization({
      deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
      clientId: "basecamp-cli",
    });
    expect(auth.expiresIn).toBe(2_147_483);
    expect(auth.interval).toBe(2_147_483);
  });

  it("rejects a valid-JSON-but-non-object body as api_error, not a TypeError", async () => {
    for (const body of ["null", "[]", '"a string"', "42"]) {
      server.use(
        mswHttp.post(DEVICE_ENDPOINT, () =>
          new HttpResponse(body, { status: 200, headers: { "Content-Type": "application/json" } })
        )
      );
      await expect(
        requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
      ).rejects.toMatchObject({ code: "api_error" });
    }
  });

  it("rejects a boolean expires_in (typeof guard, not just Number.isInteger)", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, expires_in: true })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("rejects a non-positive interval", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, interval: 0 })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("accepts an integer-valued float expires_in (e.g. 900.0)", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, expires_in: 900.0 })
      )
    );
    const auth = await requestDeviceAuthorization({
      deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
      clientId: "basecamp-cli",
    });
    expect(auth.expiresIn).toBe(900);
  });

  it("rejects a missing field", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ user_code: "X", verification_uri: ORIGIN, expires_in: 900 })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("rejects a non-string device_code (typeof guard, not just truthiness)", async () => {
    // A numeric device_code is truthy but not a usable code — it must fail as
    // api_error rather than flow into the poll loop.
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, device_code: 123456 })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("treats a JSON null interval as absent → default 5 (cross-SDK contract)", async () => {
    // Go and Kotlin decoders cannot distinguish null from absent, so a null
    // duration is treated as absent everywhere.
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, interval: null })
      )
    );
    const auth = await requestDeviceAuthorization({
      deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
      clientId: "basecamp-cli",
    });
    expect(auth.interval).toBe(5);
  });

  it("still rejects a JSON null expires_in (required, no default)", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, expires_in: null })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("rejects a non-string verification_uri_complete", async () => {
    // Optional, but a non-string value would return a malformed shape to callers.
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, verification_uri_complete: 42 })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("treats a JSON null verification_uri_complete as absent → undefined (cross-SDK contract)", async () => {
    // Go/Kotlin decoders cannot distinguish null from absent for optional strings,
    // so null must be accepted as absent here, not rejected — and normalized to undefined.
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        HttpResponse.json({ ...deviceAuthResponse, verification_uri_complete: null })
      )
    );
    const auth = await requestDeviceAuthorization({
      deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
      clientId: "basecamp-cli",
    });
    expect(auth.verificationUriComplete).toBeUndefined();
  });

  it("normalizes an invalid timeoutMs instead of aborting immediately", async () => {
    // Infinity would clamp setTimeout to ~1ms → an immediate abort masquerading
    // as a transport failure. The resolver falls back to the default, so the
    // request completes normally.
    server.use(mswHttp.post(DEVICE_ENDPOINT, () => HttpResponse.json(deviceAuthResponse)));
    const auth = await requestDeviceAuthorization({
      deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
      clientId: "basecamp-cli",
      timeoutMs: Infinity,
    });
    expect(auth.deviceCode).toBe("dev-code-123");
  });
});

describe("pollDeviceToken", () => {
  it("pending → slow_down → token, with sustained +5s interval and hook flow", async () => {
    queueTokenResponses([
      { status: 400, body: { error: "authorization_pending" } },
      { status: 400, body: { error: "slow_down" } },
      { status: 400, body: { error: "authorization_pending" } },
      { status: 200, body: tokenResponse },
    ]);
    const { waits, fn } = recordingSleep();

    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    });

    expect(token.accessToken).toBe("device_access_token");
    // Waits: 5s (pending), 5s (before slow_down), then +5 sustained → 10s, 10s.
    expect(waits).toEqual([5000, 5000, 10000, 10000]);
  });

  it("doubles the interval after a connection timeout, then recovers", async () => {
    server.use(mswHttp.post(TOKEN_ENDPOINT, () => HttpResponse.json(tokenResponse)));
    const { waits, fn } = recordingSleep();

    // A custom fetch that turns the first attempt into an AbortError (timeout),
    // then delegates to the real (MSW-mocked) fetch.
    let attempts = 0;
    const fakeFetch: typeof globalThis.fetch = async (input, init) => {
      attempts += 1;
      if (attempts === 1) {
        const e = new Error("timed out");
        e.name = "AbortError";
        throw e;
      }
      return globalThis.fetch(input, init);
    };

    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
      fetch: fakeFetch,
    });

    expect(token.accessToken).toBe("device_access_token");
    // First wait 5s, timeout → backoff doubles to 10s for the next wait.
    expect(waits[0]).toBe(5000);
    expect(waits[1]).toBe(10000);
  });

  it("resets the timeout backoff after any completed round-trip (waits return to the server interval)", async () => {
    // timeout, timeout, pending, pending → token. The two timeouts double the
    // backoff (10s, 20s); the first completed round-trip (the pending) resets
    // it to the server interval, so later waits return to 5s — never staying
    // inflated by earlier transient timeouts.
    queueTokenResponses([
      { status: 400, body: { error: "authorization_pending" } },
      { status: 400, body: { error: "authorization_pending" } },
      { status: 200, body: tokenResponse },
    ]);
    const { waits, fn } = recordingSleep();
    let attempts = 0;
    const fakeFetch: typeof globalThis.fetch = async (input, init) => {
      attempts += 1;
      if (attempts <= 2) {
        const e = new Error("timed out");
        e.name = "AbortError";
        throw e;
      }
      return globalThis.fetch(input, init);
    };

    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
      fetch: fakeFetch,
    });

    expect(token.accessToken).toBe("device_access_token");
    expect(waits).toEqual([5000, 10000, 20000, 5000, 5000]);
  });

  it.each([
    ["NaN", NaN],
    ["Infinity", Infinity],
    ["zero", 0],
    ["negative", -1],
    ["oversized", 1e100],
  ])("rejects a nonsense caller expiresIn (%s) as usage before any request", async (_label, expiresIn) => {
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn,
      sleepFn: () => Promise.resolve(),
    }).catch((e) => e);
    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("usage");
  });

  it.each([
    ["NaN", NaN],
    ["Infinity", Infinity],
    ["zero", 0],
    ["negative", -1],
    ["oversized", 1e100],
    // Whole seconds (RFC 8628): a fractional interval would permit ~1000
    // polls per second.
    ["fractional", 0.001],
  ])("rejects a nonsense caller interval (%s) as usage before any request", async (_label, interval) => {
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval,
      expiresIn: 900,
      sleepFn: () => Promise.resolve(),
    }).catch((e) => e);
    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("usage");
  });

  it.each([
    ["NaN", NaN],
    ["Infinity", Infinity],
    ["-Infinity", -Infinity],
    ["absurdly far future", Number.MAX_SAFE_INTEGER],
  ])("rejects a nonsense caller deadlineAtMs (%s) as usage before any request", async (_label, deadlineAtMs) => {
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      deadlineAtMs,
      sleepFn: () => Promise.resolve(),
    }).catch((e) => e);
    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("usage");
  });

  it.each([NaN, Infinity])("rejects an injected clock returning %s as usage", async (sample) => {
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      clock: () => sample,
      sleepFn: () => Promise.resolve(),
    }).catch((e) => e);
    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("usage");
  });

  it("rejects a deadlineAtMs beyond the code lifetime (cannot extend polling past expiresIn)", async () => {
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      deadlineAtMs: 900_001,
      clock: () => 0,
      sleepFn: () => Promise.resolve(),
    }).catch((e) => e);
    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("usage");
  });

  it("accepts a deadlineAtMs exactly at the code lifetime (issuance-anchored equality edge)", async () => {
    queueTokenResponses([{ status: 200, body: tokenResponse }]);
    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      deadlineAtMs: 900_000,
      clock: () => 0,
      sleepFn: () => Promise.resolve(),
    });
    expect(token.accessToken).toBe("device_access_token");
  });

  it("treats a finite deadlineAtMs already in the past as expiry, not usage", async () => {
    // An issuance-anchored deadline fully consumed by a slow display hook is a
    // legitimate runtime outcome — it must surface as the expired flow, not a
    // caller-input rejection.
    queueTokenResponses([{ status: 400, body: { error: "authorization_pending" } }]);
    const { fn } = recordingSleep();
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      deadlineAtMs: 500,
      clock: () => 1_000,
      sleepFn: fn,
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("expired");
    expect(err.code).toBe("auth_required");
  });

  it("accepts a fractional caller expiresIn (remaining-lifetime deduction produces it)", async () => {
    // performDeviceLogin passes a fractional remaining lifetime after deducting
    // display-hook time — caller sanity must not impose whole seconds THERE.
    // The interval, by contrast, is whole seconds (RFC 8628).
    queueTokenResponses([{ status: 200, body: tokenResponse }]);
    const { waits, fn } = recordingSleep();
    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 2,
      expiresIn: 42.5,
      sleepFn: fn,
    });
    expect(token.accessToken).toBe("device_access_token");
    expect(waits).toEqual([2000]);
  });

  it("expires against the injected monotonic clock (parent category auth)", async () => {
    queueTokenResponses([{ status: 400, body: { error: "authorization_pending" } }]);
    const { fn } = recordingSleep();
    // Clock: base at 0, then jumps past the 900s deadline on the first check.
    const times = [0, 1_000_000];
    let idx = 0;
    const clock = () => times[Math.min(idx++, times.length - 1)];

    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
      clock,
    }).catch((e) => e);

    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("expired");
    expect(err.code).toBe("auth_required");
  });

  it("raises access_denied (parent category auth)", async () => {
    queueTokenResponses([{ status: 400, body: { error: "access_denied" } }]);
    const { fn } = recordingSleep();
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("access_denied");
    expect(err.code).toBe("auth_required");
  });

  it("raises transport (network, retryable) on a non-timeout failure", async () => {
    server.use(mswHttp.post(TOKEN_ENDPOINT, () => HttpResponse.error()));
    const { fn } = recordingSleep();
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("transport");
    expect(err.code).toBe("network");
    expect(err.retryable).toBe(true);
  });

  it("raises cancelled when the signal aborts (parent category usage)", async () => {
    queueTokenResponses([{ status: 400, body: { error: "authorization_pending" } }]);
    const controller = new AbortController();
    // Abort on the first sleep.
    const fn = (_ms: number): Promise<void> => {
      controller.abort();
      return Promise.resolve();
    };
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
      signal: controller.signal,
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("cancelled");
    expect(err.code).toBe("usage");
  });

  it("does not accept a late 200 from a fetch that outlives the per-request timeout", async () => {
    // A non-cooperative fetch that ignores its AbortSignal and settles with a
    // valid 200 AFTER the per-request timeout fired: the race must discard the
    // late result (timeout → transient backoff), and the flow must end by its
    // own deadline — never by accepting the late token.
    let calls = 0;
    const lateFetch = (async () => {
      calls += 1;
      await new Promise((r) => setTimeout(r, 50));
      return new Response(JSON.stringify(tokenResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }) as unknown as typeof fetch;
    // Clock: deadline anchored at 0; after round 1's timeout the clock jumps
    // past the deadline so round 2 surfaces expired.
    const times = [0, 0, 0, 1_000_000];
    let idx = 0;
    const clock = () => times[Math.min(idx++, times.length - 1)];
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      timeoutMs: 1,
      fetch: lateFetch,
      clock,
      sleepFn: () => Promise.resolve(),
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("expired");
    expect(calls).toBe(1);
  });

  it("releases the call at its timeout when a fetch never settles", async () => {
    // The starkest non-cooperative fetch: a promise that NEVER settles. The
    // public call must still return on its own schedule — the race, not the
    // fetch, owns the timeout.
    const neverFetch = (() =>
      new Promise<Response>(() => {})) as unknown as typeof fetch;
    const err = await requestDeviceAuthorization({
      deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
      clientId: "basecamp-cli",
      fetch: neverFetch,
      timeoutMs: 20,
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("transport");
  });

  it("does not return a token when a signal-ignoring fetch completes after abort", async () => {
    // A custom fetch that ignores its AbortSignal can complete a 200 AFTER the
    // caller aborted. The success branch must re-check the signal before
    // handing back the credential — never a token returned after the caller
    // asked to stop.
    const controller = new AbortController();
    const ignoringFetch = (async () => {
      controller.abort();
      return new Response(JSON.stringify(tokenResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }) as unknown as typeof fetch;
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      signal: controller.signal,
      fetch: ignoringFetch,
      sleepFn: () => Promise.resolve(),
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("cancelled");
    expect(err.code).toBe("usage");
  });

  it("does not return a token when the signal was aborted before the token POST", async () => {
    // Regression: a signal aborted during a sleep that RESOLVES (rather than
    // rejecting) hands postDeviceToken an already-aborted signal. Without an
    // explicit already-aborted check the once-listener never fires, the fetch
    // proceeds, and a 200 would return a token AFTER cancellation. It must cancel.
    queueTokenResponses([{ status: 200, body: tokenResponse }]);
    const controller = new AbortController();
    const fn = (_ms: number): Promise<void> => {
      controller.abort();
      return Promise.resolve();
    };
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
      signal: controller.signal,
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("cancelled");
  });

  it("raises cancelled when the sleep itself rejects with an AbortError", async () => {
    // The default sleep() rejects with an AbortError when the caller aborts
    // mid-wait; that rejection must surface as cancelled, never leak raw.
    queueTokenResponses([{ status: 400, body: { error: "authorization_pending" } }]);
    const controller = new AbortController();
    const fn = (): Promise<void> => {
      const e = new Error("The operation was aborted");
      e.name = "AbortError";
      return Promise.reject(e);
    };
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
      signal: controller.signal,
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("cancelled");
  });

  it("never passes a negative wait to the sleep seam when the clock crosses the deadline mid-iteration", async () => {
    // A single cached clock read per iteration keeps remainingMs > 0. With two
    // separate reads straddling the deadline, the second could yield a negative
    // wait for the injected sleeper.
    queueTokenResponses([{ status: 400, body: { error: "authorization_pending" } }]);
    const waits: number[] = [];
    // deadline = times[0] + 10_000 = 10_000. The check reads just-below, the
    // (pre-fix) second read would cross → negative; post-sleep read expires.
    const times = [0, 9_999, 10_001, 10_050];
    let i = 0;
    const clock = (): number => times[Math.min(i++, times.length - 1)];
    const sleepFn = (ms: number): Promise<void> => {
      waits.push(ms);
      return Promise.resolve();
    };
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 10,
      clock,
      sleepFn,
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("expired");
    expect(waits.every((w) => w >= 0)).toBe(true);
  });

  it("rejects a 2xx token response with a non-numeric expires_in as api_error", async () => {
    // "soon" would otherwise flow into Date arithmetic → invalid Date (NaN),
    // making downstream expiry checks treat the token as never expiring.
    queueTokenResponses([{ status: 200, body: { ...tokenResponse, expires_in: "soon" } }]);
    const { fn } = recordingSleep();
    await expect(
      pollDeviceToken({
        tokenEndpoint: TOKEN_ENDPOINT,
        clientId: "basecamp-cli",
        deviceCode: "dev-code-123",
        interval: 5,
        expiresIn: 900,
        sleepFn: fn,
      })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("accepts a 2xx token response without expires_in (no expiry)", async () => {
    const { expires_in: _omitted, ...noExpiry } = tokenResponse;
    queueTokenResponses([{ status: 200, body: noExpiry }]);
    const { fn } = recordingSleep();
    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    });
    expect(token.accessToken).toBe("device_access_token");
    expect(token.expiresIn).toBeUndefined();
    expect(token.expiresAt).toBeUndefined();
  });

  it("rejects an infinite expires_in (1e400) as api_error", async () => {
    // JSON.parse("1e400") → Infinity. Number.isFinite rejects it before it can
    // become new Date(Infinity) whose getTime() is NaN (token never expires).
    // Sent as a raw body since JSON.stringify(Infinity) would emit null.
    server.use(
      mswHttp.post(TOKEN_ENDPOINT, () =>
        new HttpResponse(
          '{"access_token":"device_access_token","refresh_token":"r","token_type":"Bearer","expires_in":1e400}',
          { status: 200, headers: { "Content-Type": "application/json" } }
        )
      )
    );
    const { fn } = recordingSleep();
    await expect(
      pollDeviceToken({
        tokenEndpoint: TOKEN_ENDPOINT,
        clientId: "basecamp-cli",
        deviceCode: "dev-code-123",
        interval: 5,
        expiresIn: 900,
        sleepFn: fn,
      })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("rejects an expires_in past the 2_147_483_647 s ceiling as api_error", async () => {
    queueTokenResponses([{ status: 200, body: { ...tokenResponse, expires_in: 2_147_483_648 } }]);
    const { fn } = recordingSleep();
    await expect(
      pollDeviceToken({
        tokenEndpoint: TOKEN_ENDPOINT,
        clientId: "basecamp-cli",
        deviceCode: "dev-code-123",
        interval: 5,
        expiresIn: 900,
        sleepFn: fn,
      })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("accepts the token-lifetime ceiling (2_147_483_647 s)", async () => {
    queueTokenResponses([{ status: 200, body: { ...tokenResponse, expires_in: 2_147_483_647 } }]);
    const { fn } = recordingSleep();
    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    });
    expect(token.expiresIn).toBe(2_147_483_647);
  });

  it("rejects a fractional expires_in on a 2xx token response as api_error", async () => {
    // Whole-second contract: Go/Kotlin reject a fractional lifetime by int/Long
    // typing; TS/Python/Ruby reject it explicitly so all five behave uniformly.
    queueTokenResponses([{ status: 200, body: { ...tokenResponse, expires_in: 1.5 } }]);
    const { fn } = recordingSleep();
    await expect(
      pollDeviceToken({
        tokenEndpoint: TOKEN_ENDPOINT,
        clientId: "basecamp-cli",
        deviceCode: "dev-code-123",
        interval: 5,
        expiresIn: 900,
        sleepFn: fn,
      })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("accepts an integer-valued float expires_in on a 2xx token response (3600.0)", async () => {
    queueTokenResponses([{ status: 200, body: { ...tokenResponse, expires_in: 3600.0 } }]);
    const { fn } = recordingSleep();
    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    });
    expect(token.expiresIn).toBe(3600);
  });

  it.each([
    ["zero", 0],
    ["negative", -1],
  ])("rejects a %s expires_in on a 2xx token response as api_error", async (_label, expiresIn) => {
    queueTokenResponses([{ status: 200, body: { ...tokenResponse, expires_in: expiresIn } }]);
    const { fn } = recordingSleep();
    await expect(
      pollDeviceToken({
        tokenEndpoint: TOKEN_ENDPOINT,
        clientId: "basecamp-cli",
        deviceCode: "dev-code-123",
        interval: 5,
        expiresIn: 900,
        sleepFn: fn,
      })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it.each(["refresh_token", "token_type", "scope"])(
    "rejects a non-string %s on a 2xx token response as api_error (carrying the status)",
    async (field) => {
      queueTokenResponses([{ status: 200, body: { ...tokenResponse, [field]: 123 } }]);
      const { fn } = recordingSleep();
      await expect(
        pollDeviceToken({
          tokenEndpoint: TOKEN_ENDPOINT,
          clientId: "basecamp-cli",
          deviceCode: "dev-code-123",
          interval: 5,
          expiresIn: 900,
          sleepFn: fn,
        })
        // A malformed 2xx token field must carry the HTTP status (SPEC §16),
        // consistent with the other token-poll raises and the other SDKs.
      ).rejects.toMatchObject({ code: "api_error", httpStatus: 200 });
    }
  );

  it("rejects an explicit empty token_type on a 2xx token response as api_error", async () => {
    // An explicit "" token_type is malformed token metadata — uniform across
    // all five SDKs (absent/null defaults to Bearer instead).
    queueTokenResponses([{ status: 200, body: { ...tokenResponse, token_type: "" } }]);
    const { fn } = recordingSleep();
    await expect(
      pollDeviceToken({
        tokenEndpoint: TOKEN_ENDPOINT,
        clientId: "basecamp-cli",
        deviceCode: "dev-code-123",
        interval: 5,
        expiresIn: 900,
        sleepFn: fn,
      })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("defaults a null token_type to Bearer (JSON null is treated as absent)", async () => {
    queueTokenResponses([{ status: 200, body: { ...tokenResponse, token_type: null } }]);
    const { fn } = recordingSleep();
    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    });
    expect(token.tokenType).toBe("Bearer");
  });

  it("rejects a 2xx token response whose access_token is non-string as api_error", async () => {
    // A numeric access_token is truthy but not a usable credential — fail fast
    // as api_error rather than return an unusable token downstream.
    queueTokenResponses([{ status: 200, body: { access_token: 12345, token_type: "Bearer" } }]);
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: () => Promise.resolve(),
    }).catch((e) => e);
    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("api_error");
  });
});

describe("performDeviceLogin display guard", () => {
  it("rejects a non-function display as usage before any request", async () => {
    let requests = 0;
    const recordingFetch = (async () => {
      requests += 1;
      return new Response("{}", { status: 500 });
    }) as unknown as typeof fetch;
    const err = await performDeviceLogin({
      config,
      clientId: "basecamp-cli",
      display: undefined as unknown as (auth: DeviceAuthorization) => void,
      fetch: recordingFetch,
    }).catch((e) => e);
    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("usage");
    expect(requests).toBe(0);
  });
});

describe("device transport hardening", () => {
  it("propagates a malformed 2xx token response as api_error, not retryable transport", async () => {
    // 200 OK whose body is not valid JSON.
    server.use(
      mswHttp.post(TOKEN_ENDPOINT, () =>
        new HttpResponse("not json", { status: 200, headers: { "Content-Type": "application/json" } })
      )
    );
    const { fn } = recordingSleep();
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    }).catch((e) => e);

    expect(err).toBeInstanceOf(BasecampError);
    expect(err).not.toBeInstanceOf(DeviceFlowError); // NOT re-wrapped as transport
    expect(err.code).toBe("api_error");
    expect(err.retryable).toBe(false);
  });

  it.each([
    ["null", "null"],
    ["array", "[]"],
    ["number", "42"],
    ["string", '"a-bare-string"'],
  ])("propagates a valid-JSON-but-non-object 2xx token body (%s) as api_error, no raw deref", async (_label, bodyJson) => {
    server.use(
      mswHttp.post(TOKEN_ENDPOINT, () =>
        new HttpResponse(bodyJson, { status: 200, headers: { "Content-Type": "application/json" } })
      )
    );
    const { fn } = recordingSleep();
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    }).catch((e) => e);

    expect(err).toBeInstanceOf(BasecampError);
    expect(err).not.toBeInstanceOf(DeviceFlowError); // NOT re-wrapped as transport
    expect(err.code).toBe("api_error");
    expect(err.retryable).toBe(false);
  });

  it("propagates a 2xx missing access_token as api_error, not transport", async () => {
    server.use(
      mswHttp.post(TOKEN_ENDPOINT, () => HttpResponse.json({ token_type: "Bearer" }, { status: 200 }))
    );
    const { fn } = recordingSleep();
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    }).catch((e) => e);

    expect(err).toBeInstanceOf(BasecampError);
    expect(err).not.toBeInstanceOf(DeviceFlowError);
    expect(err.code).toBe("api_error");
    expect(err.retryable).toBe(false);
  });

  it("rejects an oversized device authorization body via the bounded read", async () => {
    // A valid-JSON body padded far past the 1 MiB cap; the streaming reader
    // aborts before the whole body is buffered.
    const oversized =
      `{"device_code":"d","user_code":"u","verification_uri":"${ORIGIN}/v",` +
      `"expires_in":900,"pad":"${"x".repeat(2 * 1024 * 1024)}"}`;
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        new HttpResponse(oversized, { status: 200, headers: { "Content-Type": "application/json" } })
      )
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("maps a body stream failure on the device authorization read to transport, not a raw error", async () => {
    // Headers arrive fine, then the body stream errors mid-read (connection
    // reset). The failure must surface as DeviceFlowError("transport"), never
    // escape as the raw stream error.
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () => {
        const stream = new ReadableStream<Uint8Array>({
          start(controller) {
            controller.enqueue(new TextEncoder().encode('{"device_code":"d'));
            controller.error(new Error("connection reset mid-body"));
          },
        });
        return new HttpResponse(stream, { status: 200, headers: { "Content-Type": "application/json" } });
      })
    );
    const err = await requestDeviceAuthorization({
      deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
      clientId: "basecamp-cli",
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("transport");
    expect(err.code).toBe("network");
  });

  it("times out a stalled device authorization body stream (abort timer covers the read)", async () => {
    // Headers arrive, one chunk arrives, then the stream stalls forever. The
    // request timeout must stay armed through the body read: its abort errors
    // the read, mapping to transport instead of hanging indefinitely.
    //
    // msw's interceptor does not propagate a signal abort into an in-flight
    // body stream, so this uses a custom fetch that wires the abort the way a
    // real runtime (undici) does: aborting the signal errors the reader. With
    // the pre-fix code (timer cleared before the read) the abort never fires
    // and this test hangs.
    const stalledFetch: typeof globalThis.fetch = async (_input, init) => {
      const signal = init?.signal ?? undefined;
      const stream = new ReadableStream<Uint8Array>({
        start(controller) {
          controller.enqueue(new TextEncoder().encode('{"device_code":"d'));
          // Never close, never enqueue again — a stalled stream that only
          // ends when the request signal aborts.
          signal?.addEventListener(
            "abort",
            () => controller.error(new DOMException("Aborted", "AbortError")),
            { once: true }
          );
        },
      });
      return new Response(stream, { status: 200, headers: { "Content-Type": "application/json" } });
    };
    const err = await requestDeviceAuthorization({
      deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
      clientId: "basecamp-cli",
      fetch: stalledFetch,
      timeoutMs: 50,
    }).catch((e) => e);
    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("transport");
  }, 2000);

  it("does not follow a redirect on the device authorization POST", async () => {
    let attackerContacted = false;
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () =>
        new HttpResponse(null, { status: 302, headers: { Location: "https://attacker.example/device" } })
      ),
      mswHttp.post("https://attacker.example/device", () => {
        attackerContacted = true;
        return HttpResponse.json(deviceAuthResponse);
      })
    );
    await expect(
      requestDeviceAuthorization({ deviceAuthorizationEndpoint: DEVICE_ENDPOINT, clientId: "basecamp-cli" })
    ).rejects.toMatchObject({ code: "api_error" });
    expect(attackerContacted).toBe(false);
  });

  it("does not follow a redirect on the token poll (api_error, not transport)", async () => {
    let attackerContacted = false;
    server.use(
      mswHttp.post(TOKEN_ENDPOINT, () =>
        new HttpResponse(null, { status: 302, headers: { Location: "https://attacker.example/token" } })
      ),
      mswHttp.post("https://attacker.example/token", () => {
        attackerContacted = true;
        return HttpResponse.json(tokenResponse);
      })
    );
    const { fn } = recordingSleep();
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    }).catch((e) => e);

    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("api_error");
    expect(attackerContacted).toBe(false);
  });

  it("treats a non-string token error as http_<status> (cross-SDK parity)", async () => {
    queueTokenResponses([{ status: 400, body: { error: 123 } }]);
    const { fn } = recordingSleep();
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    }).catch((e) => e);

    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("api_error");
    // 123 is not a valid error code → falls back to http_400, never surfaced as-is.
    expect(err.message).toContain("http_400");
  });

  it("fails a redirecting token poll by status without draining a huge body", async () => {
    // A 3xx must surface as api_error by STATUS before the body is read — else a
    // slow or oversized redirect body could time out (retried) or trip the size cap.
    server.use(
      mswHttp.post(
        TOKEN_ENDPOINT,
        () => new HttpResponse("x".repeat(2 * 1024 * 1024), { status: 302, headers: { Location: "https://x/" } })
      )
    );
    const { fn } = recordingSleep();
    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    }).catch((e) => e);

    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("api_error");
    expect(err.message).toContain("redirect"); // by status, not "too large"
  });

  it("clamps the backoff wait to the monotonic deadline (never overshoots expiry)", async () => {
    // Every poll times out (connection timeout), forcing sustained backoff.
    const fakeFetch: typeof globalThis.fetch = async () => {
      const e = new Error("timed out");
      e.name = "AbortError";
      throw e;
    };
    // Virtual clock advanced only by the injected sleep, so waits map to elapsed time.
    let t = 0;
    const clock = () => t;
    const waits: number[] = [];
    const sleepFn = (ms: number): Promise<void> => {
      waits.push(ms);
      t += ms;
      return Promise.resolve();
    };

    const err = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 25,
      expiresIn: 30, // 30s total lifetime
      clock,
      sleepFn,
      fetch: fakeFetch,
    }).catch((e) => e);

    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("expired");
    // Backoff would ask for 50s on the 2nd wait; the clamp caps it to the
    // remaining lifetime so no single wait — and the total — overshoots expiry.
    expect(Math.max(...waits)).toBeLessThanOrEqual(30000);
    expect(waits.reduce((a, b) => a + b, 0)).toBeLessThanOrEqual(30000);
  });
});

describe("performDeviceLogin", () => {
  it("guards capability: endpoint present but no device grant → unavailable (validation), no poll", async () => {
    let polled = false;
    server.use(mswHttp.post(TOKEN_ENDPOINT, () => { polled = true; return HttpResponse.json(tokenResponse); }));

    const err = await performDeviceLogin({
      config: { ...config, grantTypesSupported: ["refresh_token"] }, // no device_code grant
      clientId: "basecamp-cli",
      display: () => {},
    }).catch((e) => e);

    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("unavailable");
    expect(err.code).toBe("validation");
    expect(polled).toBe(false);
  });

  it("guards capability: a string grantTypesSupported does not substring-match → unavailable", async () => {
    // A manually-constructed config could supply grantTypesSupported as a string;
    // String.prototype.includes would substring-match and wrongly pass the guard.
    // The Array.isArray check rejects it (Python/Ruby defend the same way).
    let polled = false;
    server.use(mswHttp.post(TOKEN_ENDPOINT, () => { polled = true; return HttpResponse.json(tokenResponse); }));

    const err = await performDeviceLogin({
      config: { ...config, grantTypesSupported: DEVICE_CODE_GRANT_TYPE as unknown as string[] },
      clientId: "basecamp-cli",
      display: () => {},
    }).catch((e) => e);

    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("unavailable");
    expect(polled).toBe(false);
  });

  it("fires the display hook then completes", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () => HttpResponse.json(deviceAuthResponse)),
      mswHttp.post(TOKEN_ENDPOINT, () => HttpResponse.json(tokenResponse))
    );
    const display = vi.fn();
    const { fn: sleepFn } = recordingSleep();

    const token = await performDeviceLogin({
      config,
      clientId: "basecamp-cli",
      display,
      sleepFn,
    });

    expect(display).toHaveBeenCalledOnce();
    expect(display.mock.calls[0][0].userCode).toBe("WDJB-MJHT");
    expect(token.accessToken).toBe("device_access_token");
  });

  it("anchors expiry at response receipt (SPEC §16): request-leg latency does not shrink the window", async () => {
    // The deadline is clock.now() + expiresIn taken AFTER
    // requestDeviceAuthorization returns — a 6s request leg with
    // expires_in: 5 must NOT expire the fresh code client-side; expiry past
    // receipt is arbitrated by the server (expired_token).
    let now = 0;
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () => {
        now += 6_000;
        return HttpResponse.json({ ...deviceAuthResponse, expires_in: 5 });
      }),
      mswHttp.post(TOKEN_ENDPOINT, () => HttpResponse.json(tokenResponse))
    );
    const { fn: sleepFn } = recordingSleep();

    const token = await performDeviceLogin({
      config,
      clientId: "basecamp-cli",
      display: () => {},
      clock: () => now,
      sleepFn,
    });

    expect(token.accessToken).toBe("device_access_token");
  });

  it("rejects a malformed login clock sample as usage before the code is surfaced", async () => {
    // The orchestrator validates its own samples like the poller does: a NaN
    // clock is the typed usage error at the issuance anchor — never a raw
    // TypeError out of the deadline arithmetic, and never a code shown on a
    // flow that cannot compute its own deadline.
    server.use(mswHttp.post(DEVICE_ENDPOINT, () => HttpResponse.json(deviceAuthResponse)));
    const display = vi.fn();

    const err = await performDeviceLogin({
      config,
      clientId: "basecamp-cli",
      display,
      clock: () => NaN,
    }).catch((e) => e);

    expect(err).toBeInstanceOf(BasecampError);
    expect(err.code).toBe("usage");
    expect(display).not.toHaveBeenCalled();
  });

  it("deducts display time: a hook that burns the whole expires_in → expired, never polls", async () => {
    let polled = false;
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () => HttpResponse.json(deviceAuthResponse)), // expires_in: 900
      mswHttp.post(TOKEN_ENDPOINT, () => {
        polled = true;
        return HttpResponse.json(tokenResponse);
      })
    );

    // Injected monotonic clock (ms). The display hook consumes the entire 900s
    // lifetime, so the remaining lifetime is 0 and polling must never start.
    let now = 0;
    const clock = () => now;

    const err = await performDeviceLogin({
      config,
      clientId: "basecamp-cli",
      clock,
      display: () => {
        now += deviceAuthResponse.expires_in * 1000;
      },
    }).catch((e) => e);

    expect(err).toBeInstanceOf(DeviceFlowError);
    expect(err.reason).toBe("expired");
    expect(polled).toBe(false);
  });

  it("polls with the remaining lifetime after a fast display", async () => {
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () => HttpResponse.json(deviceAuthResponse)),
      mswHttp.post(TOKEN_ENDPOINT, () => HttpResponse.json(tokenResponse))
    );

    // A fast display consumes only 10ms of the 900s lifetime → polling proceeds.
    let now = 0;
    const clock = () => now;
    const { waits, fn: sleepFn } = recordingSleep();

    const token = await performDeviceLogin({
      config,
      clientId: "basecamp-cli",
      clock,
      display: () => {
        now += 10;
      },
      sleepFn,
    });

    expect(token.accessToken).toBe("device_access_token");
    // First wait is the interval (5s), clamped to the remaining lifetime.
    expect(waits[0]).toBe(5000);
  });
});

describe("DeviceFlowError retryability", () => {
  it("derives retryable from the reason and ignores a caller-supplied override", () => {
    // transport → retryable, even if the caller passes retryable: false.
    const transport = new DeviceFlowError("transport", "x", { retryable: false });
    expect(transport.retryable).toBe(true);
    expect(transport.code).toBe("network");

    // Non-transport reasons are never retryable, even if the caller passes true.
    for (const reason of ["access_denied", "expired", "unavailable", "cancelled"] as const) {
      const err = new DeviceFlowError(reason, "x", { retryable: true });
      expect(err.retryable).toBe(false);
    }
  });

  it("falls back to an api_error category for an unknown reason (JS caller safety)", () => {
    // DeviceFlowReason is an exhaustive TS union, but this is a public runtime
    // class: a JS caller can construct an unknown reason. categoryFor must not
    // leave BasecampError with an undefined code; the cross-SDK default for an
    // unknown reason is api_error (matching Go/Ruby/Python/Kotlin).
    const err = new DeviceFlowError("bogus_reason" as never, "x");
    expect(err.code).toBe("api_error");
    expect(err.retryable).toBe(false);
  });

  it("serializes the reason via toJSON (and through JSON.stringify)", () => {
    const err = new DeviceFlowError("access_denied", "denied");
    const json = err.toJSON();
    expect(json.reason).toBe("access_denied");
    expect(json.name).toBe("DeviceFlowError");
    expect(json.code).toBe("auth_required");
    expect(json.message).toBe("denied");
    expect(JSON.parse(JSON.stringify(err)).reason).toBe("access_denied");
  });
});

describe("pollDeviceToken exact-200 contract", () => {
  it.each([201, 202])("treats a %d carrying an access_token as terminal api_error", async (status) => {
    // RFC 8628/6749 token responses are exactly 200 (SPEC §16): a nonstandard
    // 2xx must not prematurely complete polling.
    queueTokenResponses([{ status, body: tokenResponse }]);

    await expect(
      pollDeviceToken({
        tokenEndpoint: TOKEN_ENDPOINT,
        clientId: "basecamp-cli",
        deviceCode: "dev-code-123",
        interval: 5,
        expiresIn: 900,
        sleepFn: recordingSleep().fn,
      })
    ).rejects.toMatchObject({ code: "api_error" });
  });
});

describe("pollDeviceToken protocol errors only on 4xx", () => {
  it.each([201, 202, 500])(
    "terminates on a %d carrying authorization_pending instead of polling",
    async (status) => {
      // OAuth protocol states are recognized only on a 4xx: a nonstandard 2xx
      // or a 5xx carrying a crafted pending body must not extend the flow.
      const polled = queueTokenResponses([
        { status, body: { error: "authorization_pending" } },
      ]);

      await expect(
        pollDeviceToken({
          tokenEndpoint: TOKEN_ENDPOINT,
          clientId: "basecamp-cli",
          deviceCode: "dev-code-123",
          interval: 5,
          expiresIn: 900,
          sleepFn: recordingSleep().fn,
        })
      ).rejects.toMatchObject({ code: "api_error" });
      expect(polled()).toBe(1);
    }
  );
});

describe("pollDeviceToken terminal statuses without body reads", () => {
  it.each([201, 500])("classifies a %d by status without draining an oversized body", async (status) => {
    // An oversized body on a terminal non-4xx would trip the size cap if
    // drained — the early status check surfaces the status api_error instead.
    server.use(
      mswHttp.post(TOKEN_ENDPOINT, () =>
        new HttpResponse("x".repeat(2 * 1024 * 1024), { status })
      )
    );

    await expect(
      pollDeviceToken({
        tokenEndpoint: TOKEN_ENDPOINT,
        clientId: "basecamp-cli",
        deviceCode: "dev-code-123",
        interval: 5,
        expiresIn: 900,
        sleepFn: recordingSleep().fn,
      })
    ).rejects.toMatchObject({ code: "api_error", message: expect.stringContaining(`status ${status}`) });
  });

  it("normalizes an invalid timeoutMs before the remaining-lifetime clamp", async () => {
    // A NaN timeoutMs poisoned Math.min into NaN, which postDeviceToken then
    // resolved to the full 30s default — bypassing the near-expiry clamp. The
    // entry normalization keeps the flow working (and clamped) regardless.
    queueTokenResponses([{ status: 200, body: tokenResponse }]);

    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      timeoutMs: Number.NaN,
      sleepFn: recordingSleep().fn,
    });
    expect(token.accessToken).toBe("device_access_token");
  });
});

describe("resolveDeviceTimeoutMs ceiling", () => {
  it("falls back to the default beyond the shared 3600s ceiling", async () => {
    // A large finite timeoutMs (up to the old ~24.8-day bound) would hold a
    // stalled request open for weeks — beyond 3600s it must resolve to the
    // default like NaN/non-positive values, matching Go/Python/Ruby.
    let requests = 0;
    server.use(
      mswHttp.post(TOKEN_ENDPOINT, () => {
        requests += 1;
        return HttpResponse.json(tokenResponse);
      })
    );

    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      timeoutMs: 24 * 24 * 3600 * 1000,
      sleepFn: recordingSleep().fn,
    });
    expect(token.accessToken).toBe("device_access_token");
    expect(requests).toBe(1);
  });
});

describe("requestDeviceAuthorization direct-caller cancellation", () => {
  it("makes no fetch when the signal is already aborted", async () => {
    let fetches = 0;
    const countingFetch: typeof globalThis.fetch = async (input, init) => {
      fetches += 1;
      return globalThis.fetch(input, init);
    };
    const controller = new AbortController();
    controller.abort();

    await expect(
      requestDeviceAuthorization({
        deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
        clientId: "basecamp-cli",
        fetch: countingFetch,
        signal: controller.signal,
      })
    ).rejects.toMatchObject({ reason: "cancelled" });
    expect(fetches).toBe(0);
  });
});

describe("performDeviceLogin abort during an async display hook", () => {
  it("rejects promptly instead of waiting for the hook to settle", async () => {
    queueTokenResponses([{ status: 200, body: tokenResponse }]);
    server.use(mswHttp.post(DEVICE_ENDPOINT, () => HttpResponse.json(deviceAuthResponse)));
    const controller = new AbortController();
    let hookSettled = false;

    const login = performDeviceLogin({
      config,
      clientId: "basecamp-cli",
      // A hook that never settles on its own — models a UI prompt awaiting
      // user interaction. The abort must win the race.
      display: () =>
        new Promise<void>((resolve) => {
          setTimeout(() => {
            hookSettled = true;
            resolve();
          }, 60_000);
        }),
      signal: controller.signal,
    });

    controller.abort();
    await expect(login).rejects.toMatchObject({ reason: "cancelled" });
    expect(hookSettled).toBe(false);
  });
});

describe("requestDeviceAuthorization post-completion abort", () => {
  it("returns cancelled when a signal-ignoring fetch completes after the abort", async () => {
    const controller = new AbortController();
    // A fetch that IGNORES its AbortSignal: aborts the caller mid-flight and
    // completes the response anyway.
    const ignoringFetch: typeof globalThis.fetch = async () => {
      controller.abort();
      return HttpResponse.json(deviceAuthResponse);
    };

    await expect(
      requestDeviceAuthorization({
        deviceAuthorizationEndpoint: DEVICE_ENDPOINT,
        clientId: "basecamp-cli",
        fetch: ignoringFetch,
        signal: controller.signal,
      })
    ).rejects.toMatchObject({ reason: "cancelled" });
  });
});

describe("performDeviceLogin issuance-anchored polling deadline", () => {
  it("does not hand display time back to the code lifetime", async () => {
    // Stepped clock: issuance at t=0; display consumes 890 of the 900s
    // lifetime; polls then advance past the 900s issuance deadline. Without
    // the absolute-deadline handoff, pollDeviceToken would re-anchor at its
    // own entry clock() and grant a fresh 10s from there PLUS the drift
    // between the two clock reads.
    const times = [0, 890_000, 890_000, 890_000, 901_000];
    let i = 0;
    const clock = () => times[Math.min(i++, times.length - 1)];
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () => HttpResponse.json(deviceAuthResponse)),
      mswHttp.post(TOKEN_ENDPOINT, () =>
        HttpResponse.json({ error: "authorization_pending" }, { status: 400 })
      )
    );

    await expect(
      performDeviceLogin({
        config,
        clientId: "basecamp-cli",
        display: () => {},
        clock,
        sleepFn: recordingSleep().fn,
      })
    ).rejects.toMatchObject({ reason: "expired" });
  });
});

describe("performDeviceLogin cancellation around the authorization request", () => {
  it("makes no request and never fires display when already aborted", async () => {
    let requests = 0;
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () => {
        requests += 1;
        return HttpResponse.json(deviceAuthResponse);
      })
    );
    const controller = new AbortController();
    controller.abort();
    const displayed: unknown[] = [];

    await expect(
      performDeviceLogin({
        config,
        clientId: "basecamp-cli",
        display: (auth) => { displayed.push(auth); },
        signal: controller.signal,
      })
    ).rejects.toMatchObject({ reason: "cancelled" });
    expect(requests).toBe(0);
    expect(displayed).toEqual([]);
  });

  it("an abort during the authorization request never reaches display", async () => {
    // The mock endpoint aborts the signal as it serves the code pair, so the
    // entry check passes and only the in-flight/post-request handling can
    // catch it — the display hook must never fire for a cancelled flow.
    const controller = new AbortController();
    server.use(
      mswHttp.post(DEVICE_ENDPOINT, () => {
        controller.abort();
        return HttpResponse.json(deviceAuthResponse);
      })
    );
    const displayed: unknown[] = [];

    await expect(
      performDeviceLogin({
        config,
        clientId: "basecamp-cli",
        display: (auth) => { displayed.push(auth); },
        signal: controller.signal,
      })
    ).rejects.toMatchObject({ reason: "cancelled" });
    expect(displayed).toEqual([]);
  });
});

describe("pollDeviceToken resource capture", () => {
  it("captures resource from the device token response and treats null as absent", async () => {
    queueTokenResponses([
      { status: 200, body: { ...tokenResponse, resource: "urn:bc:account:42" } },
    ]);
    const { fn } = recordingSleep();

    const token = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: fn,
    });
    expect(token.resource).toBe("urn:bc:account:42");

    queueTokenResponses([
      { status: 200, body: { ...tokenResponse, resource: null } },
    ]);
    const nullToken = await pollDeviceToken({
      tokenEndpoint: TOKEN_ENDPOINT,
      clientId: "basecamp-cli",
      deviceCode: "dev-code-123",
      interval: 5,
      expiresIn: 900,
      sleepFn: recordingSleep().fn,
    });
    expect(nullToken.resource).toBeUndefined();
  });

  it("rejects a present-but-empty or non-string resource as api_error", async () => {
    for (const resource of ["", 7]) {
      queueTokenResponses([
        { status: 200, body: { ...tokenResponse, resource } },
      ]);
      await expect(
        pollDeviceToken({
          tokenEndpoint: TOKEN_ENDPOINT,
          clientId: "basecamp-cli",
          deviceCode: "dev-code-123",
          interval: 5,
          expiresIn: 900,
          sleepFn: recordingSleep().fn,
        })
      ).rejects.toMatchObject({ code: "api_error" });
    }
  });
});

/** Serves a fixed response sequence with optional Retry-After headers. */
function queueTokenResponses429(
  responses: Array<{ status: number; body: object; retryAfter?: string }>
) {
  let i = 0;
  server.use(
    mswHttp.post(TOKEN_ENDPOINT, () => {
      const r = responses[Math.min(i, responses.length - 1)];
      i += 1;
      const headers = r.retryAfter != null ? { "Retry-After": r.retryAfter } : undefined;
      return HttpResponse.json(r.body, { status: r.status, headers });
    })
  );
}

const tooManyRequestsBody = { error: "too_many_requests" };

describe("pollDeviceToken 429 handling", () => {
  const pollParams = {
    tokenEndpoint: TOKEN_ENDPOINT,
    clientId: "basecamp-cli",
    deviceCode: "dev-code-123",
    interval: 5,
    expiresIn: 900,
  };

  it("retries after 429 too_many_requests with a one-shot max(interval, Retry-After) wait", async () => {
    queueTokenResponses429([
      { status: 429, body: tooManyRequestsBody, retryAfter: "30" },
      { status: 200, body: tokenResponse },
    ]);
    const { waits, fn } = recordingSleep();

    const token = await pollDeviceToken({ ...pollParams, sleepFn: fn });

    expect(token.accessToken).toBe("device_access_token");
    expect(waits).toEqual([5000, 30000]);
  });

  it.each(["abc", "1.5", "-1", "0", "99999999999999999999", undefined])(
    "falls back to the interval on a missing/malformed Retry-After (%s)",
    async (retryAfter) => {
      queueTokenResponses429([
        { status: 429, body: tooManyRequestsBody, retryAfter },
        { status: 200, body: tokenResponse },
      ]);
      const { waits, fn } = recordingSleep();

      await pollDeviceToken({ ...pollParams, sleepFn: fn });

      expect(waits).toEqual([5000, 5000]);
    }
  );

  it("trims only ASCII SP/HTAB around a Retry-After delta (RFC 9110 OWS)", () => {
    // Direct parser test: msw cannot carry NBSP header values (the Python
    // suite tests its parser directly for the same reason). Unicode
    // whitespace must never trim a malformed value into validity.
    expect(parseRetryAfterSeconds(" 30 ")).toBe(30);
    expect(parseRetryAfterSeconds("\t30\t")).toBe(30);
    expect(parseRetryAfterSeconds(" \t30\t ")).toBe(30);
    expect(parseRetryAfterSeconds("\u00a030")).toBe(0);
    expect(parseRetryAfterSeconds("30\u00a0")).toBe(0);
    expect(parseRetryAfterSeconds("\u200930")).toBe(0);
    expect(parseRetryAfterSeconds("\n30\n")).toBe(0);
  });

  it("clamps a representable over-ceiling Retry-After; unrepresentable falls back", () => {
    // The wait rule clips to the remaining code lifetime, so clamping honors
    // the throttle (wait out the lifetime) instead of resending on the
    // interval; a beyond-2^53 digit string is unrepresentable → fallback.
    expect(parseRetryAfterSeconds("2147484")).toBe(2_147_483);
    expect(parseRetryAfterSeconds("99999999999999999999")).toBe(0);
  });

  it("decays the Retry-After override after one wait", async () => {
    queueTokenResponses429([
      { status: 429, body: tooManyRequestsBody, retryAfter: "30" },
      { status: 400, body: { error: "authorization_pending" } },
      { status: 200, body: tokenResponse },
    ]);
    const { waits, fn } = recordingSleep();

    await pollDeviceToken({ ...pollParams, sleepFn: fn });

    expect(waits).toEqual([5000, 30000, 5000]);
  });

  it.each([
    { name: "429 without too_many_requests", status: 429, body: { error: "rate_limited" } },
    { name: "429 parroting authorization_pending", status: 429, body: { error: "authorization_pending" } },
    { name: "429 parroting slow_down", status: 429, body: { error: "slow_down" } },
    { name: "too_many_requests on 400", status: 400, body: tooManyRequestsBody },
  ])("stays terminal on $name", async ({ status, body }) => {
    queueTokenResponses429([{ status, body, retryAfter: "30" }]);

    await expect(
      pollDeviceToken({ ...pollParams, sleepFn: recordingSleep().fn })
    ).rejects.toMatchObject({ code: "api_error" });
  });

  it("clamps the 429 override wait to the expiry deadline", async () => {
    queueTokenResponses429([{ status: 429, body: tooManyRequestsBody, retryAfter: "3600" }]);
    const { waits, fn } = recordingSleep();
    // Scripted monotonic clock (ms): deadline anchors at t=0 with a 20s
    // lifetime; the second iteration's huge override clamps to the 14s
    // remaining and the post-wait check expires the flow.
    const times = [0, 0, 5000, 6000, 20000];
    let i = 0;
    const clock = () => times[Math.min(i++, times.length - 1)];

    await expect(
      pollDeviceToken({ ...pollParams, expiresIn: 20, sleepFn: fn, clock })
    ).rejects.toMatchObject({ reason: "expired" });
    expect(waits).toEqual([5000, 14000]);
  });

  it("preserves cancellation during the 429 override wait", async () => {
    queueTokenResponses429([{ status: 429, body: tooManyRequestsBody, retryAfter: "30" }]);
    const controller = new AbortController();
    let waitCount = 0;
    const fn = (ms: number, signal?: AbortSignal): Promise<void> => {
      waitCount += 1;
      if (waitCount === 2) {
        controller.abort(); // cancel during the post-429 override wait
        return Promise.reject(Object.assign(new Error("Aborted"), { name: "AbortError" }));
      }
      void ms;
      void signal;
      return Promise.resolve();
    };

    await expect(
      pollDeviceToken({ ...pollParams, signal: controller.signal, sleepFn: fn })
    ).rejects.toMatchObject({ reason: "cancelled" });
  });
});
