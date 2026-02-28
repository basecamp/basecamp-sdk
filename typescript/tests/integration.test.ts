/**
 * Integration smoke tests — verify the full middleware stack works in concert:
 * auth → hooks → cache → retry all interacting together.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import { createBasecampClient } from "../src/client.js";
import { BasecampError } from "../src/errors.js";
import type { BasecampHooks } from "../src/hooks.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("Integration", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("cache + retry interaction", () => {
    it("cache works after retry", async () => {
      let callCount = 0;
      server.use(
        http.get(`${BASE_URL}/projects.json`, ({ request }) => {
          callCount++;
          if (callCount === 1) {
            return new HttpResponse(null, {
              status: 429,
              headers: { "Retry-After": "0" },
            });
          }
          const ifNoneMatch = request.headers.get("If-None-Match");
          if (ifNoneMatch === '"v1"') {
            return new HttpResponse(null, { status: 304 });
          }
          return HttpResponse.json([{ id: 1 }], {
            headers: { ETag: '"v1"' },
          });
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableCache: true,
        enableRetry: true,
      });

      // Call 1: middleware pipeline sees 429 → retry middleware does raw fetch → 200 with ETag.
      // The retry's raw-fetch response flows back through the middleware stack in reverse,
      // so the cache middleware's onResponse sees the 200 + ETag and stores it.
      const result1 = await client.GET("/projects.json");
      expect(result1.data).toHaveLength(1);

      // Call 2: cache middleware attaches If-None-Match → server returns 304 → cache serves stored body.
      const result2 = await client.GET("/projects.json");
      expect(result2.data).toHaveLength(1);

      // Breakdown:
      //   callCount 1 = initial request, 429
      //   callCount 2 = retry (raw fetch, bypasses cache onRequest — no If-None-Match), 200
      //   callCount 3 = second client.GET, cache attaches If-None-Match → 304
      expect(callCount).toBe(3);
    });
  });

  describe("hooks + retry interaction", () => {
    it("hooks fire for the initial request and onRetry fires for retries", async () => {
      let attempts = 0;

      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          attempts++;
          if (attempts === 1) {
            return new HttpResponse(null, {
              status: 429,
              headers: { "Retry-After": "0" },
            });
          }
          return HttpResponse.json([{ id: 1 }]);
        })
      );

      const hooks: BasecampHooks = {
        onRequestStart: vi.fn(),
        onRequestEnd: vi.fn(),
        onRetry: vi.fn(),
      };

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        hooks,
        enableRetry: true,
      });

      await client.GET("/projects.json");

      // onRequestStart fires for the initial request going through the middleware stack.
      // The retry uses raw fetch, so it bypasses the hooks middleware.
      expect(hooks.onRequestStart).toHaveBeenCalledTimes(1);
      expect(hooks.onRequestStart).toHaveBeenCalledWith(
        expect.objectContaining({
          method: "GET",
          url: expect.stringContaining("/projects.json"),
          attempt: 1,
        })
      );

      // onRetry fires from the retry middleware when it decides to retry.
      expect(hooks.onRetry).toHaveBeenCalledTimes(1);
      expect(hooks.onRetry).toHaveBeenCalledWith(
        expect.objectContaining({
          method: "GET",
          url: expect.stringContaining("/projects.json"),
        }),
        1, // attempt number
        expect.any(Error),
        expect.any(Number) // delay
      );

      // onRequestEnd fires once — for the final response that propagates back
      // through the middleware stack (the 200 returned by the retry's raw fetch).
      expect(hooks.onRequestEnd).toHaveBeenCalledTimes(1);
      expect(hooks.onRequestEnd).toHaveBeenCalledWith(
        expect.objectContaining({
          method: "GET",
          url: expect.stringContaining("/projects.json"),
        }),
        expect.objectContaining({
          statusCode: 200,
          durationMs: expect.any(Number),
        })
      );
    });
  });

  describe("service lazy initialization", () => {
    it("service accessor returns same instance on repeated access", () => {
      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
      });

      const projects1 = client.projects;
      const projects2 = client.projects;
      expect(projects1).toBe(projects2);
    });
  });

  describe("error propagation through full stack", () => {
    it("404 propagates as BasecampError with code not_found, no retry", async () => {
      let callCount = 0;

      server.use(
        http.get(`${BASE_URL}/buckets/123/todolists/999.json`, () => {
          callCount++;
          return HttpResponse.json(
            { error: "Not found" },
            { status: 404 }
          );
        })
      );

      const hooks: BasecampHooks = {
        onRequestStart: vi.fn(),
        onRequestEnd: vi.fn(),
        onRetry: vi.fn(),
      };

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        hooks,
        enableRetry: true,
        enableCache: true,
      });

      const { error } = await client.GET(
        "/buckets/{projectId}/todolists/{todolistId}.json",
        { params: { path: { projectId: 123, todolistId: 999 } } }
      );

      expect(error).toBeDefined();

      // 404 is not in the default retryOn list [429, 503], so no retry happens.
      expect(callCount).toBe(1);
      expect(hooks.onRetry).not.toHaveBeenCalled();

      // Hooks still fire for the initial request.
      expect(hooks.onRequestStart).toHaveBeenCalledTimes(1);
      expect(hooks.onRequestEnd).toHaveBeenCalledTimes(1);
      expect(hooks.onRequestEnd).toHaveBeenCalledWith(
        expect.objectContaining({ method: "GET" }),
        expect.objectContaining({ statusCode: 404 })
      );
    });

    it("404 from service call throws BasecampError with code not_found", async () => {
      server.use(
        http.get(`${BASE_URL}/projects/999`, () => {
          return HttpResponse.json(
            { error: "Not found" },
            { status: 404 }
          );
        })
      );

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: true,
      });

      try {
        await client.projects.get(999);
        expect.unreachable("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("not_found");
        expect((err as BasecampError).httpStatus).toBe(404);
      }
    });
  });

  describe("auth + hooks + service integration", () => {
    it("service call triggers both operation and request hooks", async () => {
      server.use(
        http.get(`${BASE_URL}/projects.json`, () => {
          return HttpResponse.json([
            { id: 1, name: "Project 1" },
          ]);
        })
      );

      const hooks: BasecampHooks = {
        onOperationStart: vi.fn(),
        onOperationEnd: vi.fn(),
        onRequestStart: vi.fn(),
        onRequestEnd: vi.fn(),
      };

      const client = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        hooks,
        enableRetry: true,
      });

      const result = await client.projects.list();

      expect(result).toHaveLength(1);

      // Operation hooks fire from the service layer (BaseService.requestPaginated).
      expect(hooks.onOperationStart).toHaveBeenCalledTimes(1);
      expect(hooks.onOperationStart).toHaveBeenCalledWith(
        expect.objectContaining({
          service: "Projects",
          operation: "ListProjects",
          resourceType: "project",
          isMutation: false,
        })
      );

      expect(hooks.onOperationEnd).toHaveBeenCalledTimes(1);
      expect(hooks.onOperationEnd).toHaveBeenCalledWith(
        expect.objectContaining({
          service: "Projects",
          operation: "ListProjects",
        }),
        expect.objectContaining({
          durationMs: expect.any(Number),
        })
      );

      // Request hooks fire from the hooks middleware (HTTP layer).
      expect(hooks.onRequestStart).toHaveBeenCalledTimes(1);
      expect(hooks.onRequestStart).toHaveBeenCalledWith(
        expect.objectContaining({
          method: "GET",
          url: expect.stringContaining("/projects.json"),
          attempt: 1,
        })
      );

      expect(hooks.onRequestEnd).toHaveBeenCalledTimes(1);
      expect(hooks.onRequestEnd).toHaveBeenCalledWith(
        expect.objectContaining({
          method: "GET",
          url: expect.stringContaining("/projects.json"),
          attempt: 1,
        }),
        expect.objectContaining({
          statusCode: 200,
          fromCache: false,
        })
      );
    });
  });
});
