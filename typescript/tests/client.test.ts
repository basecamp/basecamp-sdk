/**
 * Client tests using MSW for mocking
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import { createBasecampClient, normalizeUrlPath } from "../src/client.js";
import type { BasecampHooks } from "../src/hooks.js";
import { BasecampError } from "../src/errors.js";
import { ProjectsService } from "../src/generated/services/projects.js";
import { DEFAULT_MAX_PAGES } from "../src/pagination-utils.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("BasecampClient", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("authentication", () => {
    it("should add Authorization header to requests", async () => {
      let capturedRequest: Request | null = null;

      server.use(
        http.get(`${BASE_URL}/projects.json`, ({ request }) => {
          capturedRequest = request;
          return HttpResponse.json([]);
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      await client.GET("/projects.json");

      expect(capturedRequest?.headers.get("Authorization")).toBe(
        "Bearer test-token"
      );
    });

    it("should support async token provider", async () => {
      let capturedRequest: Request | null = null;

      server.use(
        http.get(`${BASE_URL}/projects.json`, ({ request }) => {
          capturedRequest = request;
          return HttpResponse.json([]);
        })
      );

      const tokenProvider = vi.fn().mockResolvedValue("dynamic-token");

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: tokenProvider,
      });

      await client.GET("/projects.json");

      expect(tokenProvider).toHaveBeenCalled();
      expect(capturedRequest?.headers.get("Authorization")).toBe(
        "Bearer dynamic-token"
      );
    });
  });

  describe("content type", () => {
    it("should not set Content-Type on bodyless GET requests", async () => {
      // bc3 silently discards query params on GET requests that carry a
      // Content-Type header, so bodyless requests must not send one.
      let capturedRequest: Request | null = null;

      server.use(
        http.get(`${BASE_URL}/projects.json`, ({ request }) => {
          capturedRequest = request;
          return HttpResponse.json([]);
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      await client.GET("/projects.json");

      expect(capturedRequest?.headers.get("Content-Type")).toBeNull();
      expect(capturedRequest?.headers.get("Accept")).toBe("application/json");
    });

    it("should set Content-Type to application/json for JSON bodies", async () => {
      let capturedRequest: Request | null = null;

      server.use(
        http.post(`${BASE_URL}/todolists/456/todos.json`, ({ request }) => {
          capturedRequest = request;
          return HttpResponse.json({ id: 1, content: "Test todo" }, { status: 201 });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      await client.POST("/todolists/{todolistId}/todos.json", {
        params: { path: { todolistId: 456 } },
        body: { content: "Test todo" },
      });

      expect(capturedRequest?.headers.get("Content-Type")).toBe("application/json");
    });

    it("should preserve an explicitly set Content-Type on requests with a body", async () => {
      let capturedRequest: Request | null = null;

      server.use(
        http.post(`${BASE_URL}/todolists/456/todos.json`, ({ request }) => {
          capturedRequest = request;
          return HttpResponse.json({ id: 1, content: "Test todo" }, { status: 201 });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      await client.POST("/todolists/{todolistId}/todos.json", {
        params: { path: { todolistId: 456 } },
        body: { content: "Test todo" },
        headers: { "Content-Type": "application/json; charset=utf-8" },
      });

      expect(capturedRequest?.headers.get("Content-Type")).toBe(
        "application/json; charset=utf-8"
      );
    });
  });

  describe("retry behavior", () => {
    it("should retry on 429 with Retry-After header", async () => {
      let attempts = 0;

      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          attempts++;
          if (attempts === 1) {
            return new HttpResponse(null, {
              status: 429,
              headers: { "Retry-After": "1" },
            });
          }
          return HttpResponse.json([{ id: 1, name: "Test Project" }]);
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      const { data } = await client.GET("/projects.json");

      expect(attempts).toBe(2);
      expect(data).toHaveLength(1);
    });

    it("should retry on 503 with exponential backoff", async () => {
      let attempts = 0;

      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          attempts++;
          if (attempts === 1) {
            return new HttpResponse(null, { status: 503 });
          }
          return HttpResponse.json([]);
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      const { data } = await client.GET("/projects.json");

      expect(attempts).toBe(2); // Initial request + 1 retry
      expect(data).toEqual([]);
    });

    it("should not retry non-idempotent POST requests", async () => {
      // POST operations are NOT retried unless explicitly marked idempotent
      // in metadata (idempotent.natural === true). CreateTodo is not marked
      // idempotent, so it should make exactly 1 request.
      let attempts = 0;

      server.use(
        http.post(`${BASE_URL}/todolists/456/todos.json`, () => {
          attempts++;
          return new HttpResponse(null, { status: 503 });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      const { error } = await client.POST(
        "/todolists/{todolistId}/todos.json",
        {
          params: { path: { todolistId: 456 } },
          body: { content: "Test todo" },
        }
      );

      expect(attempts).toBe(1); // Single request, no retry
      expect(error).toBeDefined();
    });

    it("should exhaust the operation's maxAttempts and surface the final 503", async () => {
      let attempts = 0;

      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          attempts++;
          return new HttpResponse(null, { status: 503 });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      const { error, response } = await client.GET("/projects.json");

      // The declared maxAttempts (3) is a total attempt count — and a ceiling,
      // so no fourth request goes out.
      expect(attempts).toBe(3);
      expect(error).toBeDefined();
      expect(response.status).toBe(503);
    }, 10_000);

    it("should honor a per-operation maxAttempts below the default", async () => {
      // UpdateAccountName declares maxAttempts: 2 in behavior-model metadata,
      // so the loop must stop below the default of 3.
      let attempts = 0;

      server.use(
        http.put(`${BASE_URL}/account/name.json`, () => {
          attempts++;
          return new HttpResponse(null, { status: 503 });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      const { error, response } = await client.PUT("/account/name.json", {
        body: { name: "Renamed" },
      } as never);

      expect(attempts).toBe(2);
      expect(error).toBeDefined();
      expect(response.status).toBe(503);
    }, 10_000);

    it("should resolve retry config for timesheet_entries paths", () => {
      // Regression test: normalizeUrlPath must map timesheet_entries/{id} → {entryId}
      // so PATH_TO_OPERATION lookup finds GetTimesheetEntry/UpdateTimesheetEntry.
      // Without timesheet_entries in idMapping, the ID falls back to {id} and lookup misses.
      const getPath = normalizeUrlPath(`${BASE_URL}/timesheet_entries/789`);
      expect(getPath).toBe("/{accountId}/timesheet_entries/{entryId}");

      const putPath = normalizeUrlPath(`${BASE_URL}/timesheet_entries/456`);
      expect(putPath).toBe("/{accountId}/timesheet_entries/{entryId}");
    });

    it("should resolve webhook paths with bucketId not projectId", () => {
      // Regression test: normalizeUrlPath must produce {bucketId} for /buckets/{id}/webhooks
      // because PATH_TO_OPERATION uses {bucketId} for webhook routes.
      const path = normalizeUrlPath(`${BASE_URL}/buckets/123/webhooks.json`);
      expect(path).toBe("/{accountId}/buckets/{bucketId}/webhooks.json");
    });

    it("should resolve message type paths with bucketId not projectId", () => {
      // Regression test: same class as webhooks above. Message types (categories) are
      // bucket-scoped (#368), and PATH_TO_OPERATION keys them under {bucketId}, but
      // idMapping.buckets defaults to {projectId} — so without the contextOverrides
      // entry all five keys silently become unreachable and getRetryConfigForRequest
      // stops seeing their metadata. For the four non-POST ops that means falling
      // through to DEFAULT_RETRY_CONFIG; CreateMessageType is a POST without
      // idempotent.natural, so it returns NO_RETRY_CONFIG either way and is unaffected
      // by reachability. Both outcomes happen to match today's declared metadata, so
      // the miss is currently inert — this test exists so it stays that way if any of
      // these ops is later given a non-default retry or idempotency.
      const collection = normalizeUrlPath(`${BASE_URL}/buckets/123/categories.json`);
      expect(collection).toBe("/{accountId}/buckets/{bucketId}/categories.json");

      const member = normalizeUrlPath(`${BASE_URL}/buckets/123/categories/456`);
      expect(member).toBe("/{accountId}/buckets/{bucketId}/categories/{typeId}");
    });

    it("should not retry a network error on a non-idempotent POST and preserve the error's identity", async () => {
      // The sentinel must surface unwrapped: the conformance runner (and any
      // caller) classifies transport failures by the raw error, so wrapping or
      // rethrowing a copy would break that contract.
      const sentinel = new TypeError("Failed to fetch");
      const fetchSpy = vi
        .spyOn(globalThis, "fetch")
        .mockRejectedValue(sentinel);

      try {
        const client = createBasecampClient({
          accountId: "12345",
          accessToken: "test-token",
        });

        const err = await client
          .POST("/todolists/{todolistId}/todos.json", {
            params: { path: { todolistId: 456 } },
            body: { content: "Test todo" },
          })
          .then(
            () => undefined,
            (e: unknown) => e
          );

        // CreateTodo is not idempotent, so exactly one attempt goes out.
        expect(fetchSpy).toHaveBeenCalledTimes(1);
        expect(err).toBe(sentinel);
      } finally {
        fetchSpy.mockRestore();
      }
    });

    it("should retry a network error on an idempotent POST", async () => {
      // CompleteTodo is flagged idempotent.natural in metadata, so a transport
      // failure is retried under the same gate as a status retry.
      let attempts = 0;

      server.use(
        http.post(`${BASE_URL}/todos/456/completion.json`, () => {
          attempts++;
          if (attempts === 1) {
            return HttpResponse.error();
          }
          return new HttpResponse(null, { status: 204 });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      const { error, response } = await client.POST(
        "/todos/{todoId}/completion.json",
        { params: { path: { todoId: 456 } } }
      );

      expect(attempts).toBe(2);
      expect(error).toBeUndefined();
      expect(response.status).toBe(204);
    }, 10_000);

    it("should refresh auth token on retry", async () => {
      let attempts = 0;
      const capturedTokens: string[] = [];

      server.use(
        http.get(`${BASE_URL}/projects.json`, ({ request }) => {
          attempts++;
          capturedTokens.push(request.headers.get("Authorization") ?? "");
          if (attempts === 1) {
            return new HttpResponse(null, {
              status: 429,
              headers: { "Retry-After": "0" },
            });
          }
          return HttpResponse.json([{ id: 1, name: "Test" }]);
        })
      );

      let callCount = 0;
      const tokenProvider = async () => {
        callCount++;
        return callCount === 1 ? "stale-token" : "fresh-token";
      };

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: tokenProvider,
        enableRetry: true,
      });

      await client.GET("/projects.json");

      expect(attempts).toBe(2);
      expect(capturedTokens[0]).toBe("Bearer stale-token");
      expect(capturedTokens[1]).toBe("Bearer fresh-token");
    });
  });

  describe("caching", () => {
    it("should cache responses with ETag", async () => {
      let requestCount = 0;

      server.use(
        http.get(`${BASE_URL}/projects.json`, ({ request }) => {
          requestCount++;
          const ifNoneMatch = request.headers.get("If-None-Match");

          if (ifNoneMatch === '"abc123"') {
            return new HttpResponse(null, { status: 304 });
          }

          return HttpResponse.json([{ id: 1, name: "Test" }], {
            headers: { ETag: '"abc123"' },
          });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      // First request - should cache
      const { data: data1 } = await client.GET("/projects.json");
      expect(data1).toHaveLength(1);

      // Second request - should use cache (304)
      const { data: data2 } = await client.GET("/projects.json");
      expect(data2).toHaveLength(1);

      expect(requestCount).toBe(2); // Both requests made, second got 304
    });
  });

  describe("error handling", () => {
    it("should return error for 401", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          return HttpResponse.json(
            { error: "Unauthorized" },
            { status: 401 }
          );
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "bad-token",
      });

      const { data, error } = await client.GET("/projects.json");

      expect(data).toBeUndefined();
      expect(error).toBeDefined();
    });

    it("should return error for 404", async () => {
      server.use(
        http.get(`${BASE_URL}/todolists/999.json`, () => {
          return HttpResponse.json(
            { error: "Not found" },
            { status: 404 }
          );
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      const { data, error } = await client.GET(
        "/todolists/{todolistId}.json",
        {
          params: { path: { todolistId: 999 } },
        }
      );

      expect(data).toBeUndefined();
      expect(error).toBeDefined();
    });
  });

  describe("request timeout", () => {
    it("should timeout slow requests", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, async () => {
          // Delay longer than timeout
          await new Promise(resolve => setTimeout(resolve, 500));
          return HttpResponse.json([]);
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
        requestTimeoutMs: 100,
      });

      await expect(client.GET("/projects.json")).rejects.toThrow();
    });

    it("should use default timeout of 30000ms", async () => {
      // Just verify the option is accepted and the client works normally
      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
        // No requestTimeoutMs - should use default
      });

      const result = await client.GET("/projects.json");
      expect(result.data).toEqual([]);
    });

    // The timeout signal and any caller-supplied signal are combined with
    // AbortSignal.any (Node >= 20.3, guaranteed by the >=22.12.0 engines
    // floor). Both inputs have to stay live: the timeout must still fire when
    // the caller passes no signal, and a caller abort must still win when it
    // fires first. Test both directions so a regression in either input of the
    // combined signal is caught.
    it("aborts on timeout when the caller supplies no signal", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, async () => {
          await new Promise(resolve => setTimeout(resolve, 1000));
          return HttpResponse.json([]);
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
        requestTimeoutMs: 50,
      });

      const startedAt = Date.now();
      // Assert the error identity, not just "it rejected": AbortSignal.timeout
      // aborts with TimeoutError, and this pins that contract so a regression
      // back to a generic AbortError is caught.
      await expect(client.GET("/projects.json")).rejects.toMatchObject({
        name: "TimeoutError",
      });

      // Must abort on the timeout, not by waiting out the 1000ms handler.
      expect(Date.now() - startedAt).toBeLessThan(900);
    });

    it("propagates a caller-supplied signal through the combined signal", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, async () => {
          await new Promise(resolve => setTimeout(resolve, 1000));
          return HttpResponse.json([]);
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
        // Long enough that the request timeout cannot be what aborts us.
        requestTimeoutMs: 30000,
      });

      const controller = new AbortController();
      const abortTimer = setTimeout(() => controller.abort(), 50);

      try {
        const startedAt = Date.now();
        // A caller abort surfaces as AbortError, distinct from the timeout's
        // TimeoutError — so this also proves it was the caller's signal, not
        // the (30s) timeout, that won.
        await expect(
          client.GET("/projects.json", { signal: controller.signal })
        ).rejects.toMatchObject({ name: "AbortError" });

        // Aborting promptly proves the caller's signal reached the request; the
        // 30s timeout signal could not have fired.
        expect(Date.now() - startedAt).toBeLessThan(900);
      } finally {
        clearTimeout(abortTimer);
      }
    });
  });

  describe("requestTimeoutMs validation", () => {
    // AbortSignal.timeout only schedules a non-negative signed-32-bit integer
    // faithfully. Everything else either throws a bare RangeError per request or
    // is silently clamped to 1ms, so the bound is enforced at construction.
    // Table-driven so each rejected shape is named rather than merged into one
    // assertion.
    const invalid: Array<[string, number]> = [
      ["negative", -1],
      ["NaN", NaN],
      ["Infinity", Infinity],
      ["fractional", 1.5],
      ["above the signed 32-bit timer range", 2_147_483_648],
      ["above 2^32-1", 4_294_967_296],
    ];

    it.each(invalid)("rejects a %s timeout at construction", (_label, value) => {
      // Catch and toMatchObject rather than toThrowError(objectContaining(...)):
      // both assert the same thing, but this one names the mismatched field on
      // failure instead of reporting "expected error to match asymmetric matcher".
      let caught: unknown;
      try {
        createBasecampClient({
          accountId: "12345",
          accessToken: "test-token",
          requestTimeoutMs: value,
        });
      } catch (e: unknown) {
        caught = e;
      }

      expect(caught).toBeInstanceOf(BasecampError);
      expect(caught).toMatchObject({
        name: "BasecampError",
        code: "usage",
        message: expect.stringContaining("'requestTimeoutMs' must be an integer"),
      });
    });

    it.each([
      ["zero", 0],
      ["a typical value", 30000],
      ["the maximum", 2_147_483_647],
    ])("accepts %s", (_label, value) => {
      expect(() =>
        createBasecampClient({
          accountId: "12345",
          accessToken: "test-token",
          requestTimeoutMs: value,
        })
      ).not.toThrow();
    });
  });

  describe("maxPages validation", () => {
    // `maxPages` is not consumed here — it is handed to every service and read
    // much later, in BaseService's `page < this.maxPages` loops. Each rejected
    // value below breaks the bound in a different direction and did so silently:
    // `Infinity` removes the cap entirely, `2.5` consumes 2 pages then fetches
    // and discards a 3rd, and `0`/negative/`NaN` consume zero pages. Checked at
    // construction, where the mistake was written.
    //
    // `Number.MAX_VALUE` removes the cap the same way `Infinity` does, and is
    // why the predicate is `isSafeInteger`: `isInteger` returns `true` for it.
    // Past `2 ** 53` the counter stalls -- `page++` on `2 ** 53` yields
    // `2 ** 53` again -- so a bound like `2 ** 53 + 2` is never reached.
    // `2 ** 53` itself would terminate; rejecting it is deliberately
    // conservative, MAX_SAFE_INTEGER being the honest edge of the guarantee.
    const invalid: Array<[string, number]> = [
      ["zero", 0],
      ["negative", -1],
      ["NaN", NaN],
      ["Infinity", Infinity],
      ["fractional", 2.5],
      ["Number.MAX_VALUE", Number.MAX_VALUE],
      ["unsafe integer", 2 ** 53],
    ];

    it.each(invalid)("rejects a %s maxPages at construction", (_label, value) => {
      let caught: unknown;
      try {
        createBasecampClient({
          accountId: "12345",
          accessToken: "test-token",
          maxPages: value,
        });
      } catch (e: unknown) {
        caught = e;
      }

      expect(caught).toBeInstanceOf(BasecampError);
      expect(caught).toMatchObject({
        name: "BasecampError",
        code: "usage",
        // Pin the whole sentence, not just the offending value. An earlier
        // revision asserted only `got ${value}`, which a message about some
        // other usage error would satisfy just as well — and which would not
        // have noticed the bound changing from "a positive integer" to
        // MAX_SAFE_INTEGER at all.
        message: `maxPages must be a positive integer no larger than ${Number.MAX_SAFE_INTEGER}, got ${String(value)}`,
      });
    });

    it.each([
      ["one", 1],
      ["a typical value", 25],
      ["the default", DEFAULT_MAX_PAGES],
    ])("accepts %s and propagates it to services", (_label, value) => {
      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        maxPages: value,
      });

      // `maxPages` is protected on BaseService; read it structurally, since the
      // point of the assertion is that the validated value reached the services
      // rather than being validated and dropped.
      expect((client.projects as unknown as { maxPages: number }).maxPages).toBe(value);
    });

    it("leaves an omitted maxPages at the default", () => {
      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      expect((client.projects as unknown as { maxPages: number }).maxPages).toBe(DEFAULT_MAX_PAGES);
    });

    // The third door. BaseService and every generated service extending it are
    // exported from index.ts, so a service can be constructed directly without
    // going through createBasecampClient — and its `maxPages` lands straight in
    // `followPagination`'s `page < this.maxPages`. Validating at the factory and
    // in the standalone helpers, but not here, is a guard on two of three.
    it.each(invalid)("rejects a %s maxPages passed straight to a service", (_label, value) => {
      const raw = createBasecampClient({ accountId: "12345", accessToken: "test-token" }).raw;

      expect(() => new ProjectsService(raw, undefined, undefined, value)).toThrow(BasecampError);
      expect(() => new ProjectsService(raw, undefined, undefined, value)).toThrow(/maxPages/);
    });

    it("leaves an omitted maxPages at the default when a service is built directly", () => {
      const raw = createBasecampClient({ accountId: "12345", accessToken: "test-token" }).raw;
      const service = new ProjectsService(raw);

      expect((service as unknown as { maxPages: number }).maxPages).toBe(DEFAULT_MAX_PAGES);
    });
  });

  describe("hooks integration", () => {
    it("should call onRequestStart and onRequestEnd hooks", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          return HttpResponse.json([{ id: 1, name: "Test" }]);
        })
      );

      const hooks: BasecampHooks = {
        onRequestStart: vi.fn(),
        onRequestEnd: vi.fn(),
      };

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        hooks,
      });

      await client.GET("/projects.json");

      expect(hooks.onRequestStart).toHaveBeenCalledWith(
        expect.objectContaining({
          method: "GET",
          url: expect.stringContaining("/projects.json"),
          attempt: 1,
        })
      );

      expect(hooks.onRequestEnd).toHaveBeenCalledWith(
        expect.objectContaining({
          method: "GET",
          url: expect.stringContaining("/projects.json"),
          attempt: 1,
        }),
        expect.objectContaining({
          statusCode: 200,
          durationMs: expect.any(Number),
          fromCache: false,
        })
      );
    });

    it("should call onRetry hook when retrying", async () => {
      let attempts = 0;

      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          attempts++;
          if (attempts === 1) {
            return new HttpResponse(null, {
              status: 429,
              headers: { "Retry-After": "1" },
            });
          }
          return HttpResponse.json([]);
        })
      );

      const hooks: BasecampHooks = {
        onRetry: vi.fn(),
      };

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        hooks,
      });

      await client.GET("/projects.json");

      expect(hooks.onRetry).toHaveBeenCalledWith(
        expect.objectContaining({
          method: "GET",
          url: expect.stringContaining("/projects.json"),
          attempt: 1, // the attempt that just failed
        }),
        2, // SPEC section 7: the UPCOMING attempt
        expect.any(Error),
        expect.any(Number)
      );
    });

    it("should expose hooks on client", () => {
      const hooks: BasecampHooks = {
        onRequestStart: vi.fn(),
      };

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        hooks,
      });

      expect(client.hooks).toBe(hooks);
    });

    it("should report duration for all requests", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          return HttpResponse.json([{ id: 1, name: "Test" }]);
        })
      );

      const hooks: BasecampHooks = {
        onRequestEnd: vi.fn(),
      };

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        hooks,
      });

      await client.GET("/projects.json");

      const call = (hooks.onRequestEnd as ReturnType<typeof vi.fn>).mock.calls[0];
      expect(call[1].durationMs).toBeGreaterThanOrEqual(0);
      expect(call[1].statusCode).toBe(200);
    });
  });
});
