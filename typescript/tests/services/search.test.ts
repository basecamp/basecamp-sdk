/**
 * Tests for the SearchService (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - No client-side validation (API validates)
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
          expect(url.searchParams.get("q")).toBe("project");
          return HttpResponse.json(mockResults);
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
          expect(url.searchParams.get("q")).toBe("test");
          expect(url.searchParams.get("sort")).toBe("best_match");
          return HttpResponse.json([]);
        })
      );

      const results = await client.search.search("test", { sort: "best_match" });
      expect(results).toHaveLength(0);
    });

    it("should encode array filters as bracketed repeated keys", async () => {
      server.use(
        http.get(`${BASE_URL}/search.json`, ({ request }) => {
          const url = new URL(request.url);
          // Rails' permit(bucket_ids: []) only accepts the bracketed repeated
          // form. Assert on the decoded query, not the raw literal brackets.
          expect(url.searchParams.getAll("bucket_ids[]")).toEqual(["1", "2"]);
          expect(url.searchParams.getAll("type_names[]")).toEqual([
            "Message",
            "Todo",
          ]);
          expect(url.searchParams.getAll("creator_ids[]")).toEqual(["7"]);
          // The bare and double-bracketed forms must be absent.
          expect(url.searchParams.has("bucket_ids")).toBe(false);
          expect(url.searchParams.has("bucket_ids[][]")).toBe(false);
          return HttpResponse.json([]);
        })
      );

      const results = await client.search.search("hello", {
        bucketIds: [1, 2],
        typeNames: ["Message", "Todo"],
        creatorIds: [7],
      });
      expect(results).toHaveLength(0);
    });

    it("should encode the full filter surface (arrays, scalars, deprecated singulars)", async () => {
      server.use(
        http.get(`${BASE_URL}/search.json`, ({ request }) => {
          const p = new URL(request.url).searchParams;
          expect(p.get("q")).toBe("hello");
          expect(p.getAll("bucket_ids[]")).toEqual(["1", "2"]);
          expect(p.getAll("type_names[]")).toEqual(["Message"]);
          expect(p.getAll("creator_ids[]")).toEqual(["7"]);
          expect(p.get("file_type")).toBe("Image");
          expect(p.get("exclude_chat")).toBe("true");
          expect(p.get("since")).toBe("last_30_days");
          expect(p.get("sort")).toBe("recency");
          // Deprecated singulars.
          expect(p.get("type")).toBe("Message");
          expect(p.get("bucket_id")).toBe("9");
          expect(p.get("creator_id")).toBe("3");
          return HttpResponse.json([]);
        })
      );

      await client.search.search("hello", {
        bucketIds: [1, 2],
        typeNames: ["Message"],
        creatorIds: [7],
        fileType: "Image",
        excludeChat: true,
        since: "last_30_days",
        sort: "recency",
        type: "Message",
        bucketId: 9,
        creatorId: 3,
      });
    });

    // Note: Client-side validation removed - generated services let API validate

    it("should return empty array when no results", async () => {
      server.use(
        http.get(`${BASE_URL}/search.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const results = await client.search.search("nonexistent");
      expect(results).toHaveLength(0);
    });
  });

  describe("metadata", () => {
    it("should return the available search filter options", async () => {
      const mockMetadata = {
        recording_search_types: [
          { key: null, value: "Everything" },
          { key: "Message", value: "Messages" },
        ],
        file_search_types: [
          { key: null, value: "All files" },
          { key: "Image", value: "Images" },
        ],
        default_creator_label: "Anyone",
        default_bucket_label: "All projects",
        default_circle_label: "All pings",
        default_file_type_label: "All files",
        default_type_label: "Everything",
      };

      server.use(
        http.get(`${BASE_URL}/searches/metadata.json`, () => {
          return HttpResponse.json(mockMetadata);
        })
      );

      const metadata = await client.search.metadata();
      expect(metadata.recording_search_types).toHaveLength(2);
      // The default "everything" option carries a null key.
      expect(metadata.recording_search_types![0]!.key).toBeNull();
      expect(metadata.recording_search_types![1]!.value).toBe("Messages");
      expect(metadata.file_search_types![1]!.key).toBe("Image");
      expect(metadata.default_creator_label).toBe("Anyone");
      expect(metadata.default_type_label).toBe("Everything");
    });
  });
});
