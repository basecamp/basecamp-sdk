/**
 * Tests for updateAccountLogo (hand-written multipart upload)
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import { createBasecampClient } from "../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("updateAccountLogo", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should send a multipart PUT and succeed on 204", async () => {
    let capturedRequest: Request | null = null;

    server.use(
      http.put(`${BASE_URL}/account/logo.json`, async ({ request }) => {
        capturedRequest = request;
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });

    const blob = new Blob(["fake-png-data"], { type: "image/png" });
    await client.account.updateAccountLogo(blob, "logo.png");

    expect(capturedRequest).not.toBeNull();
    expect(capturedRequest!.method).toBe("PUT");
    expect(capturedRequest!.headers.get("Authorization")).toBe("Bearer test-token");

    // Verify multipart body contains the file
    const formData = await capturedRequest!.formData();
    const file = formData.get("logo");
    expect(file).toBeInstanceOf(File);
    expect((file as File).name).toBe("logo.png");
  });

  it("should throw on non-204 response", async () => {
    server.use(
      http.put(`${BASE_URL}/account/logo.json`, () => {
        return HttpResponse.json(
          { error: "File too large" },
          { status: 422 },
        );
      }),
    );

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });

    const blob = new Blob(["data"], { type: "image/png" });
    await expect(client.account.updateAccountLogo(blob)).rejects.toThrow();
  });

  it("should fire operation hooks", async () => {
    server.use(
      http.put(`${BASE_URL}/account/logo.json`, () => {
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const onOperationStart = vi.fn();
    const onOperationEnd = vi.fn();

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks: { onOperationStart, onOperationEnd },
    });

    const blob = new Blob(["data"], { type: "image/png" });
    await client.account.updateAccountLogo(blob, "logo.png");

    expect(onOperationStart).toHaveBeenCalledWith(
      expect.objectContaining({
        service: "Account",
        operation: "UpdateAccountLogo",
        isMutation: true,
      }),
    );
    expect(onOperationEnd).toHaveBeenCalledWith(
      expect.objectContaining({ operation: "UpdateAccountLogo" }),
      expect.objectContaining({ durationMs: expect.any(Number) }),
    );
  });

  /**
   * The multipart transport runs its own retry loop, separate from `retry.ts`,
   * and conformance cannot reach it — no fixture drives an upload through a
   * 429. So this is the only guard on that loop's Retry-After handling.
   *
   * `Retry-After: 0` is the case that matters. SPEC §6 step 1 returns a value
   * only when the integer is > 0, so a zero must fall through to the backoff
   * formula; this loop's own `parseInt` copy guarded with `>= 0` and honoured
   * it as a zero-millisecond delay, retrying with no wait at all (#564). The
   * assertion is therefore on the delay handed to `onRetry`, not merely on the
   * retry happening — and note that this test previously sent `Retry-After: 0`
   * precisely BECAUSE it collapsed the backoff and kept the test fast, which is
   * how a defect ends up load-bearing in its own coverage.
   */
  it("rejects Retry-After: 0 on the upload path and backs off instead", async () => {
    let attempts = 0;

    server.use(
      http.put(`${BASE_URL}/account/logo.json`, () => {
        attempts++;
        if (attempts === 1) {
          return new HttpResponse(null, {
            status: 429,
            headers: { "Retry-After": "0" },
          });
        }
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const onRetry = vi.fn();

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks: { onRetry },
    });

    const blob = new Blob(["data"], { type: "image/png" });
    const started = performance.now();
    await client.account.updateAccountLogo(blob, "logo.png");
    const elapsed = performance.now() - started;

    expect(attempts).toBe(2);
    expect(onRetry).toHaveBeenCalledWith(
      expect.objectContaining({ method: "PUT", url: expect.stringContaining("/account/logo.json") }),
      expect.any(Number),
      expect.any(Error),
      // The upload loop's backoff term for attempt 0 — no jitter on this path.
      1000,
    );
    // The hook argument alone would pass if the loop announced 1000ms and then
    // slept on a separately computed value, so the wall clock is checked too.
    expect(elapsed).toBeGreaterThanOrEqual(950);
  });

  it("honours a positive Retry-After on the upload path", async () => {
    let attempts = 0;

    server.use(
      http.put(`${BASE_URL}/account/logo.json`, () => {
        attempts++;
        if (attempts === 1) {
          return new HttpResponse(null, {
            status: 429,
            headers: { "Retry-After": "2" },
          });
        }
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const onRetry = vi.fn();

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks: { onRetry },
    });

    const blob = new Blob(["data"], { type: "image/png" });
    await client.account.updateAccountLogo(blob, "logo.png");

    expect(attempts).toBe(2);
    // 2000ms, not the 1000ms backoff: the header still wins when it is valid.
    expect(onRetry).toHaveBeenCalledWith(
      expect.objectContaining({ method: "PUT" }),
      expect.any(Number),
      expect.any(Error),
      2000,
    );
  }, 10_000);

  /**
   * The integer branch alone would still pass if this loop reverted to an
   * integer-only parser, so the HTTP-date branch is proven here too — once per
   * formerly duplicated retry loop.
   *
   * This is the coverage the shared conformance fixture cannot supply, and it
   * is worth naming why the same date works here and not there: a unit test has
   * a clock. It computes the header at run time, three seconds ahead of THIS
   * run, where a fixture is a static JSON literal that would have to choose
   * between expiring and asking for a multi-year sleep (#780). So this is the
   * cheap half of that coverage, available now.
   *
   * `toUTCString()` truncates to the second, so the remaining interval is
   * somewhere in (2s, 3s] and rounds up to either 2 or 3 — the assertion is a
   * range. Its floor is still double the 1000ms backoff, which is the only
   * value it has to be distinguishable from.
   */
  it("honours a future HTTP-date Retry-After on the upload path", async () => {
    let attempts = 0;

    server.use(
      http.put(`${BASE_URL}/account/logo.json`, () => {
        attempts++;
        if (attempts === 1) {
          // Computed HERE, at response-serving time, not at test setup. A
          // deadline fixed before the handler is installed and the client is
          // built is measured from a moment that may be a second or more stale
          // on a loaded worker, and `toUTCString()` truncates to whole seconds
          // on top of that — enough to drive the parsed delay under the floor
          // asserted below with production behaviour entirely correct. That is
          // the flake shape of #783; no point filing it and then writing one.
          return new HttpResponse(null, {
            status: 429,
            headers: { "Retry-After": new Date(Date.now() + 3000).toUTCString() },
          });
        }
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const onRetry = vi.fn();

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks: { onRetry },
    });

    const blob = new Blob(["data"], { type: "image/png" });
    await client.account.updateAccountLogo(blob, "logo.png");

    expect(attempts).toBe(2);
    const delay = onRetry.mock.calls[0]?.[3] as number;
    expect(delay).toBeGreaterThanOrEqual(2000);
    expect(delay).toBeLessThanOrEqual(3000);
  }, 10_000);
});
