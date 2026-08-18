/**
 * Tests for the BaseService class
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { BaseService } from "../../src/services/base.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";
import { ListResult, type PaginationOptions } from "../../src/pagination.js";
import type { BasecampHooks, OperationInfo } from "../../src/hooks.js";

const BASE_URL = "https://3.basecampapi.com/12345";

// Concrete implementation for testing
class TestService extends BaseService {
  async testGet<T>(path: string, info: OperationInfo): Promise<T> {
    return this.request(info, () =>
      // Use type assertion since we're testing with a mock path
      (this.client as any).GET(path)
    );
  }

  async testPost<T>(path: string, body: unknown, info: OperationInfo): Promise<T> {
    return this.request(info, () =>
      (this.client as any).POST(path, { body })
    );
  }

  async testPaginatedGet<T>(path: string, info: OperationInfo, opts?: PaginationOptions): Promise<ListResult<T>> {
    return this.requestPaginated(info, () =>
      (this.client as any).GET(path)
    , opts);
  }

  async testPaginatedWrappedGet<K extends string, TItem>(
    path: string, info: OperationInfo, key: K, opts?: PaginationOptions,
  ): Promise<Omit<Record<string, unknown>, K> & Record<K, ListResult<TItem>>> {
    return this.requestPaginatedWrapped<K, TItem>(info, () =>
      (this.client as any).GET(path)
    , key, opts);
  }
}

describe("BaseService", () => {
  let service: TestService;
  let mockHooks: BasecampHooks;

  beforeEach(() => {
    vi.clearAllMocks();
    mockHooks = {
      onOperationStart: vi.fn(),
      onOperationEnd: vi.fn(),
    };

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks: mockHooks,
    });

    service = new TestService(client.raw, mockHooks);
  });

  describe("request method", () => {
    it("should call hooks on successful request", async () => {
      server.use(
        http.get(`${BASE_URL}/test`, () => {
          return HttpResponse.json({ id: 1, name: "Test" });
        })
      );

      const info: OperationInfo = {
        service: "Test",
        operation: "Get",
        resourceType: "test",
        isMutation: false,
      };

      await service.testGet("/test", info);

      expect(mockHooks.onOperationStart).toHaveBeenCalledWith(info);
      expect(mockHooks.onOperationEnd).toHaveBeenCalledWith(
        info,
        expect.objectContaining({
          durationMs: expect.any(Number),
        })
      );

      // Should not have error in result
      const endCall = (mockHooks.onOperationEnd as ReturnType<typeof vi.fn>).mock.calls[0];
      expect(endCall[1].error).toBeUndefined();
    });

    it("should call hooks with error on failed request", async () => {
      server.use(
        http.get(`${BASE_URL}/test`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      const info: OperationInfo = {
        service: "Test",
        operation: "Get",
        resourceType: "test",
        isMutation: false,
        resourceId: 123,
      };

      await expect(service.testGet("/test", info)).rejects.toThrow(BasecampError);

      expect(mockHooks.onOperationEnd).toHaveBeenCalledWith(
        info,
        expect.objectContaining({
          error: expect.any(BasecampError),
          durationMs: expect.any(Number),
        })
      );
    });

    it("should convert 401 to auth error", async () => {
      server.use(
        http.get(`${BASE_URL}/test`, () => {
          return HttpResponse.json({ error: "Unauthorized" }, { status: 401 });
        })
      );

      const info: OperationInfo = {
        service: "Test",
        operation: "Get",
        resourceType: "test",
        isMutation: false,
      };

      try {
        await service.testGet("/test", info);
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("auth_required");
        expect((err as BasecampError).httpStatus).toBe(401);
      }
    });

    it("should convert 403 to forbidden error", async () => {
      server.use(
        http.get(`${BASE_URL}/test`, () => {
          return HttpResponse.json({ error: "Forbidden" }, { status: 403 });
        })
      );

      const info: OperationInfo = {
        service: "Test",
        operation: "Get",
        resourceType: "test",
        isMutation: false,
      };

      try {
        await service.testGet("/test", info);
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("forbidden");
        expect((err as BasecampError).httpStatus).toBe(403);
      }
    });

    it("should convert 404 to not_found error", async () => {
      server.use(
        http.get(`${BASE_URL}/test`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      const info: OperationInfo = {
        service: "Test",
        operation: "Get",
        resourceType: "test",
        isMutation: false,
      };

      try {
        await service.testGet("/test", info);
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("not_found");
        expect((err as BasecampError).httpStatus).toBe(404);
      }
    });

    it("should convert 429 to rate_limit error", async () => {
      server.use(
        http.get(`${BASE_URL}/test`, () => {
          return new HttpResponse(null, {
            status: 429,
            headers: { "Retry-After": "60" },
          });
        })
      );

      const info: OperationInfo = {
        service: "Test",
        operation: "Get",
        resourceType: "test",
        isMutation: false,
      };

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false, // Disable retry to test error handling
      });
      const serviceNoRetry = new TestService(client.raw);

      try {
        await serviceNoRetry.testGet("/test", info);
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("rate_limit");
        expect((err as BasecampError).httpStatus).toBe(429);
        expect((err as BasecampError).retryable).toBe(true);
        expect((err as BasecampError).retryAfter).toBe(60);
      }
    });

    it("should convert 5xx to retryable api_error", async () => {
      server.use(
        http.get(`${BASE_URL}/test`, () => {
          return HttpResponse.json({ error: "Internal error" }, { status: 500 });
        })
      );

      const info: OperationInfo = {
        service: "Test",
        operation: "Get",
        resourceType: "test",
        isMutation: false,
      };

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
      });
      const serviceNoRetry = new TestService(client.raw);

      try {
        await serviceNoRetry.testGet("/test", info);
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("api_error");
        expect((err as BasecampError).httpStatus).toBe(500);
        expect((err as BasecampError).retryable).toBe(true);
      }
    });
  });

  describe("requestPaginated", () => {
    const listInfo: OperationInfo = {
      service: "Test",
      operation: "List",
      resourceType: "test",
      isMutation: false,
    };

    it("should return ListResult with all items on single page (no Link header)", async () => {
      server.use(
        http.get(`${BASE_URL}/test-list`, () => {
          return HttpResponse.json([{ id: 1 }, { id: 2 }], {
            headers: { "X-Total-Count": "2" },
          });
        })
      );

      const result = await service.testPaginatedGet<{ id: number }>("/test-list", listInfo);

      expect(result).toBeInstanceOf(ListResult);
      expect(result.length).toBe(2);
      expect(result[0]).toEqual({ id: 1 });
      expect(result[1]).toEqual({ id: 2 });
      expect(result.meta.totalCount).toBe(2);
      expect(result.meta.truncated).toBe(false);
    });

    it("should follow Link headers and accumulate across pages", async () => {
      let pageRequests = 0;

      server.use(
        http.get(`${BASE_URL}/test-list`, ({ request }) => {
          pageRequests++;
          const url = new URL(request.url);
          const page = url.searchParams.get("page");

          if (page === "2") {
            // Second page — no Link header (last page)
            return HttpResponse.json([{ id: 3 }, { id: 4 }]);
          }

          // First page — includes Link to page 2
          return HttpResponse.json([{ id: 1 }, { id: 2 }], {
            headers: {
              "X-Total-Count": "4",
              Link: `<${BASE_URL}/test-list?page=2>; rel="next"`,
            },
          });
        })
      );

      // Create service with a real fetchPage that MSW can intercept
      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        hooks: mockHooks,
      });
      const paginatedService = new TestService(client.raw, mockHooks, async (url: string) => {
        return fetch(url, {
          headers: {
            Authorization: "Bearer test-token",
            Accept: "application/json",
          },
        });
      });

      const result = await paginatedService.testPaginatedGet<{ id: number }>("/test-list", listInfo);

      expect(result.length).toBe(4);
      expect(result.meta.totalCount).toBe(4);
      expect(result.meta.truncated).toBe(false);
      expect(result[2]).toEqual({ id: 3 });
      expect(pageRequests).toBe(2);
    });

    // A followed page whose body is not JSON is "the server sent something I
    // could not decode", and it used to escape as a bare SyntaxError: no code, no
    // hint, indistinguishable from a bug in the caller's own code. Ruby and
    // Python already classified it with this message; #750 gives all six the same
    // shape and the same reachable cause.
    it("should classify an unparseable followed page and keep the decoder error", async () => {
      server.use(
        http.get(`${BASE_URL}/test-list`, ({ request }) => {
          const url = new URL(request.url);
          if (url.searchParams.get("page") === "2") {
            return new HttpResponse('[{"id": 3', {
              headers: { "Content-Type": "application/json" },
            });
          }
          return HttpResponse.json([{ id: 1 }, { id: 2 }], {
            headers: { Link: `<${BASE_URL}/test-list?page=2>; rel="next"` },
          });
        })
      );

      const client = createBasecampClient({ accountId: "12345", accessToken: "test-token" });
      const paginatedService = new TestService(client.raw, undefined, async (url: string) => {
        return fetch(url, { headers: { Accept: "application/json" } });
      });

      const error = await paginatedService
        .testPaginatedGet<{ id: number }>("/test-list", listInfo)
        .then(() => null, (err: unknown) => err);

      expect(error).toBeInstanceOf(BasecampError);
      const basecampError = error as BasecampError;
      expect(basecampError.code).toBe("api_error");
      // Statusless per SPEC §6 — the transport returned 2xx — and re-requesting
      // cannot repair a malformed body.
      expect(basecampError.httpStatus).toBeUndefined();
      expect(basecampError.retryable).toBe(false);
      expect(basecampError.message).toContain("page 2");
      // The marker: the decoder's own error, not a phrase to match on.
      expect(basecampError.cause).toBeInstanceOf(SyntaxError);
    });

    // Vitest runs on Node, where a JSON syntax failure is a real SyntaxError, so
    // nothing above would notice the discriminator being "simplified" back to
    // `instanceof SyntaxError`. WebKit rejects the same malformed body with a
    // DOMException *named* SyntaxError, which that test answers false for — the
    // whole class would escape unclassified in Safari, and no Node test can see
    // it. This case is that engine's shape, pinned where it can fail.
    it("should classify a syntax failure the engine reports as a DOMException", async () => {
      server.use(
        http.get(`${BASE_URL}/test-list`, () => {
          return HttpResponse.json([{ id: 1 }, { id: 2 }], {
            headers: { Link: `<${BASE_URL}/test-list?page=2>; rel="next"` },
          });
        })
      );

      const thrown = new DOMException("JSON Parse error: Unexpected EOF", "SyntaxError");
      expect(thrown).not.toBeInstanceOf(SyntaxError);

      const client = createBasecampClient({ accountId: "12345", accessToken: "test-token" });
      const paginatedService = new TestService(client.raw, undefined, async () => {
        const body = new ReadableStream<Uint8Array>({
          start(controller) {
            controller.error(thrown);
          },
        });
        return new Response(body, {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      });

      const error = await paginatedService
        .testPaginatedGet<{ id: number }>("/test-list", listInfo)
        .then(() => null, (err: unknown) => err);

      expect(error).toBeInstanceOf(BasecampError);
      const basecampError = error as BasecampError;
      expect(basecampError.code).toBe("api_error");
      expect(basecampError.retryable).toBe(false);
      expect(basecampError.cause).toBe(thrown);
    });

    // `response.json()` rejects for two unrelated reasons: the bytes arrived and
    // were not JSON, or the bytes never finished arriving. Only the first is a
    // malformed body. A socket reset surfaces as TypeError, an aborted read as
    // AbortError, and both are transient — classifying either as a statusless
    // non-retryable api_error would tell a caller reading `retryable` that a
    // failure it should retry is permanent. They propagate untouched, with the
    // type the runtime gave them, so the SDK's own retry layer still sees them
    // for what they are.
    for (const { label, failure, expectedName } of [
      { label: "a socket reset", failure: () => new TypeError("terminated"), expectedName: "TypeError" },
      {
        label: "an aborted read",
        failure: () => new DOMException("The operation was aborted", "AbortError"),
        expectedName: "AbortError",
      },
    ]) {
      it(`should propagate ${label} during a followed page's body read`, async () => {
        server.use(
          http.get(`${BASE_URL}/test-list`, () => {
            return HttpResponse.json([{ id: 1 }, { id: 2 }], {
              headers: { Link: `<${BASE_URL}/test-list?page=2>; rel="next"` },
            });
          })
        );

        const thrown = failure();
        const client = createBasecampClient({ accountId: "12345", accessToken: "test-token" });
        // The page arrives with a 200 and a body stream that fails partway
        // through, which is what the transport failure looks like from here.
        const paginatedService = new TestService(client.raw, undefined, async () => {
          const body = new ReadableStream<Uint8Array>({
            start(controller) {
              controller.enqueue(new TextEncoder().encode('[{"id": 3}'));
              controller.error(thrown);
            },
          });
          return new Response(body, {
            status: 200,
            headers: { "Content-Type": "application/json" },
          });
        });

        const error = await paginatedService
          .testPaginatedGet<{ id: number }>("/test-list", listInfo)
          .then(() => null, (err: unknown) => err);

        expect(error).not.toBeInstanceOf(BasecampError);
        expect(error).toBe(thrown);
        expect((error as Error).name).toBe(expectedName);
      });
    }

    it("should throw BasecampError on cross-origin Link header", async () => {
      server.use(
        http.get(`${BASE_URL}/test-list`, () => {
          return HttpResponse.json([{ id: 1 }], {
            headers: {
              Link: '<https://evil.example.com/page2>; rel="next"',
            },
          });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });
      const paginatedService = new TestService(client.raw, undefined, async (url: string) => {
        return fetch(url, { headers: { Accept: "application/json" } });
      });

      await expect(
        paginatedService.testPaginatedGet("/test-list", listInfo)
      ).rejects.toThrow(BasecampError);

      try {
        await paginatedService.testPaginatedGet("/test-list", listInfo);
      } catch (err) {
        expect((err as BasecampError).code).toBe("api_error");
        expect((err as BasecampError).message).toContain("different origin");
      }
    });

    it("should return empty ListResult when no items", async () => {
      server.use(
        http.get(`${BASE_URL}/test-list`, () => {
          return HttpResponse.json([], {
            headers: { "X-Total-Count": "0" },
          });
        })
      );

      const result = await service.testPaginatedGet<{ id: number }>("/test-list", listInfo);

      expect(result).toBeInstanceOf(ListResult);
      expect(result.length).toBe(0);
      expect(result.meta.totalCount).toBe(0);
      expect(result.meta.truncated).toBe(false);
    });

    it("should set truncated=true when maxItems caps results mid-pagination", async () => {
      server.use(
        http.get(`${BASE_URL}/test-list`, ({ request }) => {
          const url = new URL(request.url);
          const page = url.searchParams.get("page");

          if (page === "2") {
            return HttpResponse.json([{ id: 3 }, { id: 4 }]);
          }

          return HttpResponse.json([{ id: 1 }, { id: 2 }], {
            headers: {
              "X-Total-Count": "4",
              Link: `<${BASE_URL}/test-list?page=2>; rel="next"`,
            },
          });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });
      const paginatedService = new TestService(client.raw, undefined, async (url: string) => {
        return fetch(url, { headers: { Accept: "application/json" } });
      });

      // maxItems=3: first page has 2, second page has 2, cap at 3
      const result = await paginatedService.testPaginatedGet<{ id: number }>(
        "/test-list", listInfo, { maxItems: 3 }
      );

      expect(result.length).toBe(3);
      expect(result.meta.truncated).toBe(true);
    });

    it("should set truncated=false when maxItems matches exact result count on first page", async () => {
      server.use(
        http.get(`${BASE_URL}/test-list`, () => {
          // No Link header — this is the only page
          return HttpResponse.json([{ id: 1 }, { id: 2 }], {
            headers: { "X-Total-Count": "2" },
          });
        })
      );

      // maxItems=2, and there are exactly 2 items with no next page
      const result = await service.testPaginatedGet<{ id: number }>(
        "/test-list", listInfo, { maxItems: 2 }
      );

      expect(result.length).toBe(2);
      expect(result.meta.truncated).toBe(false);
    });

    it("should set truncated=true when maxItems < first page and more pages exist", async () => {
      server.use(
        http.get(`${BASE_URL}/test-list`, () => {
          return HttpResponse.json([{ id: 1 }, { id: 2 }, { id: 3 }], {
            headers: {
              "X-Total-Count": "10",
              Link: `<${BASE_URL}/test-list?page=2>; rel="next"`,
            },
          });
        })
      );

      const result = await service.testPaginatedGet<{ id: number }>(
        "/test-list", listInfo, { maxItems: 2 }
      );

      expect(result.length).toBe(2);
      expect(result.meta.truncated).toBe(true);
    });

    it("should set truncated=false when maxItems is met exactly at a page boundary with no further pages", async () => {
      server.use(
        http.get(`${BASE_URL}/test-list`, ({ request }) => {
          const url = new URL(request.url);
          const page = url.searchParams.get("page");

          if (page === "2") {
            // Final page — exactly fills maxItems, no Link header
            return HttpResponse.json([{ id: 3 }]);
          }

          return HttpResponse.json([{ id: 1 }, { id: 2 }], {
            headers: {
              "X-Total-Count": "3",
              Link: `<${BASE_URL}/test-list?page=2>; rel="next"`,
            },
          });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });
      const paginatedService = new TestService(client.raw, undefined, async (url: string) => {
        return fetch(url, { headers: { Accept: "application/json" } });
      });

      // maxItems=3: page 1 has 2, page 2 has exactly 1 more and no next link —
      // nothing was dropped, so the result is complete
      const result = await paginatedService.testPaginatedGet<{ id: number }>(
        "/test-list", listInfo, { maxItems: 3 }
      );

      expect(result.length).toBe(3);
      expect(result.meta.truncated).toBe(false);
    });

    it("should set truncated=true when maxItems is met exactly at a page boundary and a next link exists", async () => {
      let pageRequests = 0;

      server.use(
        http.get(`${BASE_URL}/test-list`, ({ request }) => {
          pageRequests++;
          const url = new URL(request.url);
          const page = url.searchParams.get("page");

          if (page === "3") {
            return HttpResponse.json([{ id: 5 }, { id: 6 }]);
          }

          if (page === "2") {
            return HttpResponse.json([{ id: 3 }, { id: 4 }], {
              headers: {
                Link: `<${BASE_URL}/test-list?page=3>; rel="next"`,
              },
            });
          }

          return HttpResponse.json([{ id: 1 }, { id: 2 }], {
            headers: {
              "X-Total-Count": "6",
              Link: `<${BASE_URL}/test-list?page=2>; rel="next"`,
            },
          });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });
      const paginatedService = new TestService(client.raw, undefined, async (url: string) => {
        return fetch(url, { headers: { Accept: "application/json" } });
      });

      // maxItems=4: cap hit exactly after page 2, which advertises page 3
      const result = await paginatedService.testPaginatedGet<{ id: number }>(
        "/test-list", listInfo, { maxItems: 4 }
      );

      expect(result.length).toBe(4);
      expect(result.meta.truncated).toBe(true);
      // Page 3 must not be fetched — its Link header alone proves more pages exist
      expect(pageRequests).toBe(2);
    });

    it("should set truncated=true when maxItems matches exact first page and a next link exists", async () => {
      let pageRequests = 0;

      server.use(
        http.get(`${BASE_URL}/test-list`, () => {
          pageRequests++;
          return HttpResponse.json([{ id: 1 }, { id: 2 }], {
            headers: {
              "X-Total-Count": "10",
              Link: `<${BASE_URL}/test-list?page=2>; rel="next"`,
            },
          });
        })
      );

      // maxItems=2 exactly equals the first page, but a Link advertises more
      const result = await service.testPaginatedGet<{ id: number }>(
        "/test-list", listInfo, { maxItems: 2 }
      );

      expect(result.length).toBe(2);
      expect(result.meta.truncated).toBe(true);
      expect(pageRequests).toBe(1);
    });

    it("should set truncated=true when the page safety cap stops pagination", async () => {
      let pageRequests = 0;

      server.use(
        http.get(`${BASE_URL}/test-list`, ({ request }) => {
          pageRequests++;
          const url = new URL(request.url);
          const page = Number(url.searchParams.get("page") ?? "1");

          // Every page advertises another one
          return HttpResponse.json([{ id: page }], {
            headers: {
              Link: `<${BASE_URL}/test-list?page=${page + 1}>; rel="next"`,
            },
          });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });
      // maxPages=2: page 1 + one followed page, then the safety cap stops the loop
      const cappedService = new TestService(client.raw, undefined, async (url: string) => {
        return fetch(url, { headers: { Accept: "application/json" } });
      }, 2);

      const result = await cappedService.testPaginatedGet<{ id: number }>("/test-list", listInfo);

      expect(result.length).toBe(2);
      expect(result.meta.truncated).toBe(true);
      expect(pageRequests).toBe(2);
    });

    it("keeps the validated page cap when a caller assigns maxPages after construction", async () => {
      let pageRequests = 0;

      server.use(
        http.get(`${BASE_URL}/test-list`, ({ request }) => {
          pageRequests++;
          const url = new URL(request.url);
          const page = Number(url.searchParams.get("page") ?? "1");

          // Every page advertises another one
          return HttpResponse.json([{ id: page }], {
            headers: {
              Link: `<${BASE_URL}/test-list?page=${page + 1}>; rel="next"`,
            },
          });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });
      const cappedService = new TestService(client.raw, undefined, async (url: string) => {
        return fetch(url, { headers: { Accept: "application/json" } });
      }, 5);

      // TypeScript's `readonly` is compile-time only, so before the cap moved
      // into a #private field this cast REPLACED the validated value — the
      // pagination loops read the assigned 2 and stopped three pages early.
      // The assignment now hits a getter-only prototype property: it throws in
      // strict mode, and either way cannot shadow the #field the loops read.
      // (A lowered replacement, not Infinity — that would hang rather than
      // fail against the pre-#private code.)
      try {
        (cappedService as unknown as { maxPages: number }).maxPages = 2;
      } catch {
        // TypeError under strict mode: exactly the enforcement being pinned.
      }

      const result = await cappedService.testPaginatedGet<{ id: number }>("/test-list", listInfo);

      expect(result.length).toBe(5);
      expect(result.meta.truncated).toBe(true);
      expect(pageRequests).toBe(5);
    });
  });

  describe("requestPaginatedWrapped", () => {
    const listInfo: OperationInfo = {
      service: "Test",
      operation: "ListWrapped",
      resourceType: "test",
      isMutation: false,
    };

    it("should accumulate items across pages and preserve wrapper fields", async () => {
      server.use(
        http.get(`${BASE_URL}/test-wrapped`, ({ request }) => {
          const url = new URL(request.url);
          const page = url.searchParams.get("page");

          if (page === "2") {
            return HttpResponse.json({
              person: { id: 456, name: "Jane Doe" },
              events: [{ id: 3, action: "updated" }],
            });
          }

          return HttpResponse.json({
            person: { id: 456, name: "Jane Doe" },
            events: [{ id: 1, action: "created" }, { id: 2, action: "completed" }],
          }, {
            headers: {
              "X-Total-Count": "3",
              Link: `<${BASE_URL}/test-wrapped?page=2>; rel="next"`,
            },
          });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        hooks: mockHooks,
      });
      const wrappedService = new TestService(client.raw, mockHooks, async (url: string) => {
        return fetch(url, {
          headers: { Authorization: "Bearer test-token", Accept: "application/json" },
        });
      });

      const result = await wrappedService.testPaginatedWrappedGet<"events", { id: number; action: string }>(
        "/test-wrapped", listInfo, "events"
      );

      // Wrapper field preserved from page 1
      expect((result as any).person).toEqual({ id: 456, name: "Jane Doe" });

      // Events accumulated across both pages
      const events = result.events;
      expect(events).toBeInstanceOf(ListResult);
      expect(events.length).toBe(3);
      expect(events[0]).toEqual({ id: 1, action: "created" });
      expect(events[1]).toEqual({ id: 2, action: "completed" });
      expect(events[2]).toEqual({ id: 3, action: "updated" });
      expect(events.meta.totalCount).toBe(3);
      expect(events.meta.truncated).toBe(false);
    });

    it("should set truncated=false when maxItems is met exactly at a page boundary with no further pages", async () => {
      server.use(
        http.get(`${BASE_URL}/test-wrapped`, ({ request }) => {
          const url = new URL(request.url);
          const page = url.searchParams.get("page");

          if (page === "2") {
            // Final page — exactly fills maxItems, no Link header
            return HttpResponse.json({
              person: { id: 456, name: "Jane Doe" },
              events: [{ id: 3, action: "updated" }],
            });
          }

          return HttpResponse.json({
            person: { id: 456, name: "Jane Doe" },
            events: [{ id: 1, action: "created" }, { id: 2, action: "completed" }],
          }, {
            headers: {
              "X-Total-Count": "3",
              Link: `<${BASE_URL}/test-wrapped?page=2>; rel="next"`,
            },
          });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });
      const wrappedService = new TestService(client.raw, undefined, async (url: string) => {
        return fetch(url, { headers: { Accept: "application/json" } });
      });

      // maxItems=3: page 1 has 2 events, page 2 has exactly 1 more and no next
      // link — nothing was dropped, so the result is complete
      const result = await wrappedService.testPaginatedWrappedGet<"events", { id: number; action: string }>(
        "/test-wrapped", listInfo, "events", { maxItems: 3 }
      );

      const events = result.events;
      expect(events.length).toBe(3);
      expect(events.meta.truncated).toBe(false);
    });

    // The wrapped loop decodes its pages at its own call site, so it is a second
    // case rather than a second assertion about the first: routing both through
    // one helper is not what makes them both classified.
    it("should classify an unparseable followed page and keep the decoder error", async () => {
      server.use(
        http.get(`${BASE_URL}/test-wrapped`, ({ request }) => {
          const url = new URL(request.url);
          if (url.searchParams.get("page") === "2") {
            return new HttpResponse("not json at all", {
              headers: { "Content-Type": "application/json" },
            });
          }
          return HttpResponse.json({
            person: { id: 456, name: "Jane Doe" },
            events: [{ id: 1, action: "created" }],
          }, {
            headers: { Link: `<${BASE_URL}/test-wrapped?page=2>; rel="next"` },
          });
        })
      );

      const client = createBasecampClient({ accountId: "12345", accessToken: "test-token" });
      const wrappedService = new TestService(client.raw, undefined, async (url: string) => {
        return fetch(url, { headers: { Accept: "application/json" } });
      });

      const error = await wrappedService
        .testPaginatedWrappedGet<"events", { id: number; action: string }>("/test-wrapped", listInfo, "events")
        .then(() => null, (err: unknown) => err);

      expect(error).toBeInstanceOf(BasecampError);
      const basecampError = error as BasecampError;
      expect(basecampError.code).toBe("api_error");
      expect(basecampError.httpStatus).toBeUndefined();
      expect(basecampError.retryable).toBe(false);
      expect(basecampError.message).toContain("page 2");
      expect(basecampError.cause).toBeInstanceOf(SyntaxError);
    });
  });

  describe("hooks behavior", () => {
    it("should not let hook errors break operations", async () => {
      const throwingHooks: BasecampHooks = {
        onOperationStart: vi.fn().mockImplementation(() => {
          throw new Error("Hook error");
        }),
        onOperationEnd: vi.fn(),
      };

      server.use(
        http.get(`${BASE_URL}/test`, () => {
          return HttpResponse.json({ id: 1 });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });
      const serviceWithHooks = new TestService(client.raw, throwingHooks);

      const info: OperationInfo = {
        service: "Test",
        operation: "Get",
        resourceType: "test",
        isMutation: false,
      };

      // Hook errors should NOT break operations - they are caught and swallowed
      const result = await serviceWithHooks.testGet("/test", info);
      expect(result).toEqual({ id: 1 });
      // Hook was still called
      expect(throwingHooks.onOperationStart).toHaveBeenCalled();
    });

    it("should work without hooks", async () => {
      server.use(
        http.get(`${BASE_URL}/test`, () => {
          return HttpResponse.json({ id: 1, name: "Test" });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });
      const serviceNoHooks = new TestService(client.raw);

      const info: OperationInfo = {
        service: "Test",
        operation: "Get",
        resourceType: "test",
        isMutation: false,
      };

      // Should not throw
      const result = await serviceNoHooks.testGet("/test", info);
      expect(result).toBeDefined();
    });
  });
});
