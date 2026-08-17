/**
 * Request lifecycle regressions for the client middleware.
 *
 * Four defects motivated these, all in the since-replaced retry/hooks middleware
 * (the retry loop now lives in createRetryingFetch, beneath the chain):
 *
 *   1. bodyCache was keyed on `${method}:${url}:${Date.now()}`, so two concurrent
 *      mutations to the same URL in the same millisecond shared one cache slot —
 *      a retry replayed the other request's body, or none at all.
 *   2. onRequestEnd never fired when the initial fetch rejected (network error or
 *      timeout), because openapi-fetch skips onResponse entirely on that path and
 *      no middleware implemented onError.
 *   3. bodyCache/timings were pruned only in onResponse, so retried and
 *      network-failed requests leaked their serialized bodies for the process
 *      lifetime.
 *   4. The raw `fetch(retryRequest)` retry bypassed middleware, so attempt 2 got
 *      no onRequestStart/onRequestEnd at all, and onRetry reported the attempt
 *      that just failed (1) where SPEC section 7 requires the upcoming one (2).
 *
 * SPEC section 7 attempt semantics, which these pin:
 *   - RequestInfo.attempt  — the failed/current attempt, 1-based
 *   - onRetry's 2nd arg    — the UPCOMING attempt, so 2 on the first retry
 * Go, Python, Ruby and Kotlin all pass (1, 2); TypeScript passed (1, 1).
 *
 * The retry loop now runs beneath the middleware chain as the client's custom
 * fetch (closing SPEC waiver 2B.1), so attempts run to each operation's
 * declared maxAttempts and network errors retry under the idempotency gate.
 * The multi-attempt tests below pin that; the two-attempt tests remain as the
 * minimal shape of each original defect.
 */
import { describe, it, expect, vi, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import { createBasecampClient } from "../src/client.js";
import type { BasecampHooks, RequestInfo, RequestResult } from "../src/hooks.js";

const BASE_URL = "https://3.basecampapi.com/12345";

/** Records every lifecycle callback in order, so we can assert balance and args. */
function recordingHooks() {
  const events: Array<{
    kind: "start" | "end" | "retry";
    attempt: number;
    statusCode?: number;
    fromCache?: boolean;
    error?: unknown;
  }> = [];
  const hooks: BasecampHooks = {
    onRequestStart(info: RequestInfo) {
      events.push({ kind: "start", attempt: info.attempt });
    },
    onRequestEnd(info: RequestInfo, result: RequestResult) {
      events.push({
        kind: "end",
        attempt: info.attempt,
        statusCode: result.statusCode,
        fromCache: result.fromCache,
        error: (result as { error?: unknown }).error,
      });
    },
    onRetry(_info: RequestInfo, upcomingAttempt: number, error: Error) {
      events.push({ kind: "retry", attempt: upcomingAttempt, error });
    },
  };
  return { hooks, events };
}

const starts = (e: ReturnType<typeof recordingHooks>["events"]) =>
  e.filter((x) => x.kind === "start");
const ends = (e: ReturnType<typeof recordingHooks>["events"]) =>
  e.filter((x) => x.kind === "end");

describe("middleware request lifecycle", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  // Defect 1. Two concurrent same-method/same-URL mutations with Date.now()
  // pinned. Under the old timestamp key both computed the SAME cache key, so one
  // body overwrote the other and at least one retry replayed the wrong bytes.
  it("replays each concurrent same-URL mutation's own body on retry", async () => {
    vi.spyOn(Date, "now").mockReturnValue(1_700_000_000_000);

    const seen: Array<{ phase: "initial" | "retry"; body: string }> = [];
    let release: (() => void) | undefined;
    const barrier = new Promise<void>((resolve) => {
      release = resolve;
    });
    let arrived = 0;

    server.use(
      http.put(`${BASE_URL}/todos/2`, async ({ request }) => {
        const body = await request.text();
        const n = ++arrived;

        // The first two arrivals are the initial attempts. The barrier holds both
        // of them until both have arrived, so their onRequest handlers provably
        // interleave — and it makes the initial/retry split deterministic without
        // relying on any wire header.
        if (n <= 2) {
          seen.push({ phase: "initial", body });
          if (n === 2) release?.();
          await barrier;
          // Retry-After: 0 keeps the test off the real backoff clock.
          return new HttpResponse(null, {
            status: 429,
            headers: { "Retry-After": "0" },
          });
        }

        seen.push({ phase: "retry", body });
        return HttpResponse.json({ ok: true });
      })
    );

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });

    await Promise.all([
      client.PUT("/todos/{todoId}", {
        params: { path: { todoId: 2 } },
        body: { content: "AAA" },
      }),
      client.PUT("/todos/{todoId}", {
        params: { path: { todoId: 2 } },
        body: { content: "BBB" },
      }),
    ]);

    const firstBodies = seen.filter((s) => s.phase === "initial").map((s) => s.body).sort();
    const retryBodies = seen.filter((s) => s.phase === "retry").map((s) => s.body).sort();

    expect(firstBodies).toHaveLength(2);
    expect(retryBodies).toHaveLength(2);
    // The retries must carry the same multiset of bodies as the originals.
    // The old key made both retries send one request's body (or an empty one).
    expect(retryBodies).toEqual(firstBodies);
  });

  // Defect 2, updated for network-error retry. A GET whose fetch keeps
  // rejecting is retried to the operation's maxAttempts, and every attempt gets
  // a balanced start/end pair with statusCode 0 and the error preserved.
  it("retries a GET whose fetch fails, with balanced hooks for every attempt", async () => {
    server.use(
      http.get(`${BASE_URL}/projects.json`, () => HttpResponse.error())
    );

    const { hooks, events } = recordingHooks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
    });

    await expect(client.GET("/projects.json")).rejects.toBeTruthy();

    expect(starts(events).map((e) => e.attempt)).toEqual([1, 2, 3]);
    expect(ends(events).map((e) => e.attempt)).toEqual([1, 2, 3]);
    for (const end of ends(events)) {
      expect(end.statusCode).toBe(0);
      expect(end.error).toBeInstanceOf(Error);
    }
  }, 10_000);

  // Network-error retry's success path: two fetch rejections, then a 200.
  it("recovers when a retried GET's network error clears", async () => {
    let attempts = 0;
    server.use(
      http.get(`${BASE_URL}/projects.json`, () => {
        attempts++;
        if (attempts <= 2) {
          return HttpResponse.error();
        }
        return HttpResponse.json([{ id: 1 }]);
      })
    );

    const { hooks, events } = recordingHooks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
    });

    const { data } = await client.GET("/projects.json");

    expect(attempts).toBe(3);
    expect(data).toEqual([{ id: 1 }]);
    expect(starts(events).map((e) => e.attempt)).toEqual([1, 2, 3]);
    expect(ends(events).map((e) => e.statusCode)).toEqual([0, 0, 200]);
  }, 10_000);

  // Defect 2, timeout variant. The auth middleware installs
  // AbortSignal.timeout, so a stalled request rejects the same way.
  it("fires a balanced start/end with statusCode 0 when the request times out", async () => {
    server.use(
      http.get(`${BASE_URL}/projects.json`, async () => {
        await new Promise((r) => setTimeout(r, 200));
        return HttpResponse.json([]);
      })
    );

    const { hooks, events } = recordingHooks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
      requestTimeoutMs: 20,
    });

    await expect(client.GET("/projects.json")).rejects.toBeTruthy();

    expect(starts(events)).toHaveLength(1);
    expect(ends(events)).toHaveLength(1);
    const end = ends(events)[0]!;
    expect(end.statusCode).toBe(0);
    expect(end.error).toBeInstanceOf(Error);
  });

  // Review follow-up (Codex). A caller can abort with a CUSTOM reason —
  // AbortController.abort(reason) — and fetch then rejects with that reason,
  // not a DOMException named AbortError. Cancellation must stay terminal on
  // that path too: no retry, no backoff, and the caller's reason surfaces
  // untouched.
  it("treats a caller abort with a custom reason as terminal", async () => {
    server.use(
      http.get(`${BASE_URL}/projects.json`, async () => {
        await new Promise((r) => setTimeout(r, 1000));
        return HttpResponse.json([]);
      })
    );

    const { hooks, events } = recordingHooks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
    });

    const reason = new Error("caller cancelled");
    const controller = new AbortController();
    const abortTimer = setTimeout(() => controller.abort(reason), 50);

    try {
      const err = await client
        .GET("/projects.json", { signal: controller.signal } as never)
        .then(
          () => undefined,
          (e: unknown) => e
        );

      expect(err).toBe(reason);
      // Terminal on attempt 1: no retry started, no onRetry announced.
      expect(starts(events).map((e) => e.attempt)).toEqual([1]);
      expect(ends(events).map((e) => e.attempt)).toEqual([1]);
      expect(events.filter((e) => e.kind === "retry")).toHaveLength(0);
    } finally {
      clearTimeout(abortTimer);
    }
  }, 10_000);

  // Review follow-up (Codex, round 2). An abort that fires DURING the backoff
  // sleep must be terminal immediately: without a signal-aware sleep the
  // request stays pending for the full delay, then begins another attempt
  // (start + auth refresh) against an already-aborted signal. The same seam
  // guards the request-timeout budget, which shares this signal.
  it("rejects promptly when the caller aborts during a retry backoff", async () => {
    server.use(
      http.get(`${BASE_URL}/projects.json`, () =>
        // Retry-After: 2 puts the loop into a 2s backoff we can abort inside.
        new HttpResponse(null, { status: 429, headers: { "Retry-After": "2" } })
      )
    );

    const { hooks, events } = recordingHooks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
    });

    const reason = new Error("caller cancelled during backoff");
    const controller = new AbortController();
    const abortTimer = setTimeout(() => controller.abort(reason), 100);

    try {
      const startedAt = Date.now();
      const err = await client
        .GET("/projects.json", { signal: controller.signal } as never)
        .then(
          () => undefined,
          (e: unknown) => e
        );

      expect(err).toBe(reason);
      // Prompt: nowhere near the 2s Retry-After backoff.
      expect(Date.now() - startedAt).toBeLessThan(1000);
      // Attempt 1 was started and ended (429) before the backoff; attempt 2
      // must never start. onRetry had already announced it — that is the
      // inherent race of cancelling between announce and begin — but starts
      // and ends stay balanced.
      expect(starts(events).map((e) => e.attempt)).toEqual([1]);
      expect(ends(events).map((e) => e.attempt)).toEqual([1]);
      expect(ends(events)[0]!.statusCode).toBe(429);
    } finally {
      clearTimeout(abortTimer);
    }
  }, 10_000);

  // Defect 4, updated for network-error retry. A 503 followed by fetch
  // rejections keeps retrying to maxAttempts, with balanced hooks per attempt
  // and nothing dangling.
  it("fires balanced lifecycle hooks for every attempt when retry fetches fail", async () => {
    let attempts = 0;
    server.use(
      http.get(`${BASE_URL}/projects.json`, () => {
        attempts++;
        if (attempts === 1) {
          return new HttpResponse(null, {
            status: 503,
            headers: { "Retry-After": "0" },
          });
        }
        return HttpResponse.error();
      })
    );

    const { hooks, events } = recordingHooks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
    });

    await expect(client.GET("/projects.json")).rejects.toBeTruthy();

    expect(attempts).toBe(3);
    // One start and one end per attempt — nothing dangling.
    expect(starts(events).map((e) => e.attempt)).toEqual([1, 2, 3]);
    expect(ends(events).map((e) => e.attempt)).toEqual([1, 2, 3]);
    expect(ends(events).map((e) => e.statusCode)).toEqual([503, 0, 0]);
    expect(ends(events)[2]!.error).toBeInstanceOf(Error);
  }, 10_000);

  // Defect 4, attempt numbering. SPEC section 7: RequestInfo.attempt is the
  // attempt that just failed (1); onRetry's second argument is the UPCOMING
  // attempt (2). TypeScript passed 1 for both.
  it("reports the failed attempt in RequestInfo and the upcoming attempt to onRetry", async () => {
    let attempts = 0;
    server.use(
      http.get(`${BASE_URL}/projects.json`, () => {
        attempts++;
        if (attempts === 1) {
          return new HttpResponse(null, {
            status: 503,
            headers: { "Retry-After": "0" },
          });
        }
        return HttpResponse.json([]);
      })
    );

    const seen: Array<{ infoAttempt: number; upcoming: number }> = [];
    const hooks: BasecampHooks = {
      onRetry(info: RequestInfo, upcoming: number) {
        seen.push({ infoAttempt: info.attempt, upcoming });
      },
    };
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
    });

    await client.GET("/projects.json");

    expect(seen).toHaveLength(1);
    expect(seen[0]!.infoAttempt).toBe(1);
    expect(seen[0]!.upcoming).toBe(2);
  });

  // Review follow-up. The lifecycle middleware is registered even with no hooks
  // configured, because it owns releasing per-request state and the retry
  // middleware records an attempt regardless of whether anyone is listening.
  // Without it, every retried request stranded one entry for the client's life.
  it("retries correctly when no hooks are configured", async () => {
    let attempts = 0;
    server.use(
      http.get(`${BASE_URL}/projects.json`, () => {
        attempts++;
        if (attempts % 2 === 1) {
          return new HttpResponse(null, {
            status: 429,
            headers: { "Retry-After": "0" },
          });
        }
        return HttpResponse.json([]);
      })
    );

    // No `hooks` option at all — the path where state was stranded, because the
    // middleware that releases it used to be registered only when hooks existed.
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });

    // Exercises the no-hooks retry path end to end. The release itself is
    // structural (the lifecycle middleware is now unconditional) and not directly
    // observable through the public API, so this pins the behaviour around it:
    // retries still work, and each logical request completes.
    for (let i = 0; i < 3; i++) {
      const { data } = await client.GET("/projects.json");
      expect(data).toEqual([]);
    }
    expect(attempts).toBe(6);
  });

  // The two 304 paths, as a pair. Here: fromCache means "served out of the ETag
  // cache", and a bare 304 does not prove that — it reaches the lifecycle when the
  // cache is disabled, or is enabled but holds no entry, and in both cases the
  // caller's own conditional request went to the server for a real round trip.
  // Reporting it as a cache hit overstates the cache's effectiveness in anyone's
  // metrics. The test below covers the opposite path, where the cache does serve.
  it("does not report a bare 304 as served from cache", async () => {
    // Reproduce the real path rather than just the status: the CALLER owns the
    // conditional request. The server only answers 304 because it received their
    // If-None-Match, which is what makes this a genuine round trip and not a
    // cache hit.
    let sawConditional = false;
    server.use(
      http.get(`${BASE_URL}/projects.json`, ({ request }) => {
        if (request.headers.get("If-None-Match") === 'W/"caller-etag"') {
          sawConditional = true;
          return new HttpResponse(null, { status: 304 });
        }
        return HttpResponse.json([{ id: 1 }]);
      })
    );

    const { hooks, events } = recordingHooks();
    // Cache disabled, so nothing can rewrite the 304 and no X-From-Cache is set.
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
      enableCache: false,
    });

    await client.GET("/projects.json", {
      headers: { "If-None-Match": 'W/"caller-etag"' },
    } as never);

    // The 304 was earned by the caller's own conditional request.
    expect(sawConditional).toBe(true);

    const last = ends(events).at(-1)!;
    expect({ statusCode: last.statusCode, fromCache: last.fromCache }).toEqual({
      statusCode: 304,
      fromCache: false,
    });
  });

  // The other 304 path. A cached conditional GET whose retry returns 304 must be
  // reported as the cache middleware finally resolved it — 200 and fromCache true,
  // not the raw 304 the retry saw. The retry deliberately leaves finalization to
  // the downstream lifecycle pass so the cache can transform the response first.
  it("reports the post-cache outcome when a retried conditional GET returns 304", async () => {
    let attempts = 0;
    server.use(
      http.get(`${BASE_URL}/projects.json`, ({ request }) => {
        attempts++;
        if (attempts === 1) {
          return HttpResponse.json([{ id: 1 }], { headers: { ETag: 'W/"v1"' } });
        }
        if (attempts === 2) {
          return new HttpResponse(null, {
            status: 429,
            headers: { "Retry-After": "0" },
          });
        }
        // The retry carries the conditional header and the server confirms.
        expect(request.headers.get("If-None-Match")).toBe('W/"v1"');
        return new HttpResponse(null, { status: 304 });
      })
    );

    const { hooks, events } = recordingHooks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
      enableCache: true,
    });

    await client.GET("/projects.json"); // populates the cache
    await client.GET("/projects.json"); // 429 -> retry -> 304 -> cached 200

    // Asserted as one object so BOTH halves of the transformation are proven:
    // eager finalization froze the status at 304 *and* fromCache at false, and a
    // sequence of separate expects would stop at the first and leave the second
    // unproven.
    const last = ends(events).at(-1)!;
    expect({
      attempt: last.attempt,
      statusCode: last.statusCode,
      fromCache: last.fromCache,
    }).toEqual({ attempt: 2, statusCode: 200, fromCache: true });
  });

  // Review follow-up. Attempt 2 begins before the auth refresh, so a throw from
  // that refresh still lands on a live attempt. Otherwise onRetry announces the
  // upcoming attempt and nothing ever accounts for it — the same blind spot this
  // PR exists to close, just moved one step later.
  it("accounts for attempt 2 when the retry's auth refresh throws", async () => {
    server.use(
      http.get(`${BASE_URL}/projects.json`, () =>
        new HttpResponse(null, { status: 429, headers: { "Retry-After": "0" } })
      )
    );

    // Succeeds for the initial request, fails when the retry refreshes.
    let calls = 0;
    const failingAuth = {
      async authenticate(headers: Headers) {
        calls += 1;
        if (calls > 1) throw new Error("token refresh failed");
        headers.set("Authorization", "Bearer test-token");
      },
    };

    const { hooks, events } = recordingHooks();
    const client = createBasecampClient({
      accountId: "12345",
      auth: failingAuth,
      hooks,
    });

    await expect(client.GET("/projects.json")).rejects.toThrow("token refresh failed");

    // A retry to attempt 2 was announced...
    const retries = events.filter((e) => e.kind === "retry");
    expect(retries.map((r) => r.attempt)).toEqual([2]);

    // ...and attempt 2 is accounted for, not silently dropped.
    expect(starts(events).map((e) => e.attempt)).toEqual([1, 2]);
    const last = ends(events).at(-1)!;
    expect({ attempt: last.attempt, statusCode: last.statusCode }).toEqual({
      attempt: 2,
      statusCode: 0,
    });
    expect(last.error).toBeInstanceOf(Error);
  });

  // Review follow-up. A failed response carrying a body has its stream cancelled
  // before the backoff, so a throttled client does not hold a connection per
  // in-flight retry. The other retry tests all use null-body responses, so this is
  // the only one that reaches that branch.
  //
  // Connection reuse is not observable here, but the cancellation itself is:
  // without the spy this test passed unchanged when the cancel was deleted, which
  // made it no regression at all.
  it("cancels the failed response's stream before retrying", async () => {
    const cancelSpy = vi.spyOn(ReadableStream.prototype, "cancel");
    let attempts = 0;
    server.use(
      http.get(`${BASE_URL}/projects.json`, () => {
        attempts++;
        if (attempts === 1) {
          // A body, unlike the null-body 429s used elsewhere in this file.
          return HttpResponse.json(
            { error: "Rate limited", details: "x".repeat(2048) },
            { status: 429, headers: { "Retry-After": "0" } }
          );
        }
        return HttpResponse.json([{ id: 1 }]);
      })
    );

    const { hooks, events } = recordingHooks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
    });

    const { data } = await client.GET("/projects.json");

    expect(attempts).toBe(2);
    expect(data).toEqual([{ id: 1 }]);
    expect(ends(events).map((e) => e.statusCode)).toEqual([429, 200]);
    // The point of the test: the discarded body's stream was actually cancelled.
    expect(cancelSpy).toHaveBeenCalled();
  });

  // Waiver 2B.1's kill shot. The retry loop lives beneath the middleware chain
  // as the client's custom fetch, so a second 503 no longer exhausts the
  // architecture — attempts run to the operation's declared maxAttempts (3 by
  // default), each with balanced hooks and an onRetry announcing the next.
  it("chains status retries to the operation's full maxAttempts", async () => {
    let attempts = 0;
    server.use(
      http.get(`${BASE_URL}/projects.json`, () => {
        attempts++;
        if (attempts <= 2) {
          return new HttpResponse(null, { status: 503 });
        }
        return HttpResponse.json([{ id: 1 }]);
      })
    );

    const events: Array<{
      kind: "start" | "end" | "retry";
      attempt: number;
      statusCode?: number;
      upcoming?: number;
    }> = [];
    const hooks: BasecampHooks = {
      onRequestStart(info: RequestInfo) {
        events.push({ kind: "start", attempt: info.attempt });
      },
      onRequestEnd(info: RequestInfo, result: RequestResult) {
        events.push({ kind: "end", attempt: info.attempt, statusCode: result.statusCode });
      },
      onRetry(info: RequestInfo, upcoming: number) {
        events.push({ kind: "retry", attempt: info.attempt, upcoming });
      },
    };
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
    });

    const { data } = await client.GET("/projects.json");

    expect(attempts).toBe(3);
    expect(data).toEqual([{ id: 1 }]);
    expect(starts(events).map((e) => e.attempt)).toEqual([1, 2, 3]);
    expect(ends(events).map((e) => e.attempt)).toEqual([1, 2, 3]);
    expect(ends(events).map((e) => e.statusCode)).toEqual([503, 503, 200]);
    // SPEC section 7 pairs: (failed attempt, upcoming attempt).
    const retries = events.filter((e) => e.kind === "retry");
    expect(retries.map((r) => [r.attempt, r.upcoming])).toEqual([
      [1, 2],
      [2, 3],
    ]);
  }, 10_000);

  // Body replay across MULTIPLE retries: the buffer captured before attempt 1
  // must feed every subsequent attempt, byte-identical. The concurrency test
  // above only exercises one retry per request.
  it("replays the same body bytes on every retry attempt", async () => {
    const bodies: string[] = [];
    let attempts = 0;
    server.use(
      http.put(`${BASE_URL}/todos/2`, async ({ request }) => {
        attempts++;
        bodies.push(await request.text());
        if (attempts <= 2) {
          return new HttpResponse(null, {
            status: 429,
            headers: { "Retry-After": "0" },
          });
        }
        return HttpResponse.json({ ok: true });
      })
    );

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });

    await client.PUT("/todos/{todoId}", {
      params: { path: { todoId: 2 } },
      body: { content: "same-bytes" },
    });

    expect(attempts).toBe(3);
    expect(bodies).toHaveLength(3);
    expect(new Set(bodies).size).toBe(1);
    expect(bodies[0]).toContain("same-bytes");
  });

  // Repeated retried requests each report their own two attempts, in order, with
  // no cross-talk between logical requests. This says nothing about state being
  // released — that is not observable from here — only that attempt attribution
  // stays correct across many sequential retries.
  it("attributes attempts correctly across repeated retried requests", async () => {
    let attempts = 0;
    server.use(
      http.get(`${BASE_URL}/projects.json`, () => {
        attempts++;
        if (attempts % 2 === 1) {
          // 429 rather than 503: Retry-After is honoured only for 429, so this
          // keeps the loop off the real exponential-backoff clock.
          return new HttpResponse(null, {
            status: 429,
            headers: { "Retry-After": "0" },
          });
        }
        return HttpResponse.json([]);
      })
    );

    const { hooks, events } = recordingHooks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks,
    });

    for (let i = 0; i < 5; i++) {
      await client.GET("/projects.json");
    }

    // 5 logical requests x 2 attempts each.
    expect(starts(events)).toHaveLength(10);
    expect(ends(events)).toHaveLength(10);
    expect(starts(events).map((e) => e.attempt)).toEqual([1, 2, 1, 2, 1, 2, 1, 2, 1, 2]);
    expect(ends(events).map((e) => e.attempt)).toEqual([1, 2, 1, 2, 1, 2, 1, 2, 1, 2]);
  });
});
