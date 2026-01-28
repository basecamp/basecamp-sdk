/**
 * Tests for the BaseService class
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { BaseService } from "../../src/services/base.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";
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
