/**
 * Tests for the BaseService class
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { BaseService } from "../../src/services/base.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";
import { ListResult } from "../../src/pagination.js";
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

  async testPaginatedGet<T>(path: string, info: OperationInfo): Promise<ListResult<T>> {
    return this.requestPaginated(info, () =>
      (this.client as any).GET(path)
    );
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
        expect((err as BasecampError).code).toBe("auth");
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
      expect(result[2]).toEqual({ id: 3 });
      expect(pageRequests).toBe(2);
    });

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
