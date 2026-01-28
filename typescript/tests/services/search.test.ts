/**
 * Tests for the SearchService
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("SearchService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("search", () => {
    it("should search for content across the account", async () => {
      const mockResults = [
        {
          id: 1,
          title: "Project Plan",
          type: "Document",
          status: "active",
          url: "https://example.com/1",
          app_url: "https://basecamp.com/1",
        },
        {
          id: 2,
          title: "Meeting Notes",
          type: "Message",
          status: "active",
          url: "https://example.com/2",
          app_url: "https://basecamp.com/2",
        },
      ];

      server.use(
        http.get(`${BASE_URL}/search.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("query")).toBe("project");
          return HttpResponse.json({ results: mockResults });
        })
      );

      const results = await client.search.search("project");
      expect(results).toHaveLength(2);
      expect(results[0]!.title).toBe("Project Plan");
      expect(results[1]!.type).toBe("Message");
    });

    it("should support sort option", async () => {
      server.use(
        http.get(`${BASE_URL}/search.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("query")).toBe("test");
          expect(url.searchParams.get("sort")).toBe("updated_at");
          return HttpResponse.json({ results: [] });
        })
      );

      const results = await client.search.search("test", { sort: "updated_at" });
      expect(results).toHaveLength(0);
    });

    it("should throw validation error for empty query", async () => {
      await expect(client.search.search("")).rejects.toThrow(BasecampError);
      await expect(client.search.search("")).rejects.toThrow("Search query is required");
    });

    it("should throw validation error for whitespace-only query", async () => {
      await expect(client.search.search("   ")).rejects.toThrow(BasecampError);
    });

    it("should return empty array when no results", async () => {
      server.use(
        http.get(`${BASE_URL}/search.json`, () => {
          return HttpResponse.json({ results: [] });
        })
      );

      const results = await client.search.search("nonexistent");
      expect(results).toEqual([]);
    });
  });

  describe("metadata", () => {
    it("should return search metadata with available projects", async () => {
      const mockMetadata = {
        projects: [
          { id: 1, name: "Project A" },
          { id: 2, name: "Project B" },
        ],
      };

      server.use(
        http.get(`${BASE_URL}/searches/metadata.json`, () => {
          return HttpResponse.json({ metadata: mockMetadata });
        })
      );

      const metadata = await client.search.metadata();
      expect(metadata.projects).toHaveLength(2);
      expect(metadata.projects![0]!.name).toBe("Project A");
    });

    it("should return empty projects array when no projects available", async () => {
      server.use(
        http.get(`${BASE_URL}/searches/metadata.json`, () => {
          return HttpResponse.json({ metadata: { projects: [] } });
        })
      );

      const metadata = await client.search.metadata();
      expect(metadata.projects).toEqual([]);
    });
  });
});
