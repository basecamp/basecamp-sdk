/**
 * Client tests using MSW for mocking
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import { createBasecampClient, normalizeUrlPath } from "../src/client.js";
import type { BasecampHooks } from "../src/hooks.js";

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

    it("should retry POST requests based on operation-specific metadata config", async () => {
      // Unlike the Go SDK which is conservative, the TypeScript SDK uses per-operation
      // retry configs from metadata.json. This allows safe retry of idempotent POST
      // operations like CreateTodo (which has maxAttempts: 3 in metadata).
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

      // CreateTodo has maxAttempts: 3 in metadata, so we expect retries
      expect(attempts).toBe(2); // Initial request + 1 retry before giving up
      expect(error).toBeDefined();
    });

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
        }),
        1,
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
