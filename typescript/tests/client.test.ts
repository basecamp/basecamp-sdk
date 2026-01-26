/**
 * Client tests using MSW for mocking
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import { createBasecampClient } from "../src/client.js";

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
        http.get(`${BASE_URL}/buckets/123/todolists/999.json`, () => {
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
        "/buckets/{projectId}/todolists/{todolistId}.json",
        {
          params: { path: { projectId: 123, todolistId: 999 } },
        }
      );

      expect(data).toBeUndefined();
      expect(error).toBeDefined();
    });
  });
});
