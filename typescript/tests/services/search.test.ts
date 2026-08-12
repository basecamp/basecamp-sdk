/**
 * Tests for the SearchService (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - This service performs no client-side validation; the API validates
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
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
          // BC3 emits `json.content nil` / `json.description nil`
          // unconditionally on every search result, so the keys are always
          // present and always null. A stub that omits them is a payload the
          // API cannot produce, and would let the required-and-nullable
          // contract regress unnoticed.
          content: null,
          description: null,
        },
        {
          id: 2,
          title: "Meeting Notes",
          type: "Message",
          status: "active",
          url: "https://example.com/2",
          app_url: "https://basecamp.com/2",
          content: null,
          description: null,
          // The searchable text lives here instead — a highlighted, truncated
          // excerpt, not plain text despite the name.
          plain_text_content:
            'Notes from the <mark class="circled-text"><span></span>Leto</mark> kickoff.',
          // The search projection carries the matching type's rich-text
          // companion array; a Message result surfaces content_attachments.
          content_attachments: [
            {
              id: 900,
              sgid: "BAh-img",
              filename: "diagram.png",
              content_type: "image/png",
              byte_size: 2048,
              download_url:
                "https://example.com/blobs/img/download/diagram.png",
              width: 1024.0,
              height: 768,
              previewable: true,
              preview_url: "https://example.com/blobs/img/previews/diagram.png",
              thumbnail_url:
                "https://example.com/blobs/img/thumbnails/diagram.png",
            },
          ],
        },
        {
          // A file-attachment hit (searches/_attachment.json.jbuilder): the
          // one branch that omits the id/title/type/url/app_url envelope keys
          // and carries the ten file keys instead. width/height ride only on
          // previewable files and may arrive float-spelled (1920.0).
          parent: {
            id: 10,
            title: "Message Board",
            type: "Message",
            url: "https://example.com/buckets/1/messages/11.json",
            app_url: "https://basecamp.com/buckets/1/messages/11",
          },
          bucket: { id: 1, name: "Leto", type: "Project" },
          created_at: "2022-10-28T15:25:00.000Z",
          filename: "leto-hero.jpg",
          content_type: "image/jpeg",
          byte_size: 512000,
          previewable: true,
          width: 1920.0,
          height: 1080,
          preview_url: "https://example.com/blobs/hero/previews/leto-hero.jpg",
          thumbnail_url:
            "https://example.com/blobs/hero/thumbnails/leto-hero.jpg",
          download_url: "https://example.com/blobs/hero/download/leto-hero.jpg",
          app_download_url:
            "https://basecamp.com/blobs/hero/download/leto-hero.jpg",
          content: null,
          description: null,
        },
      ];

      server.use(
        http.get(`${BASE_URL}/search.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("q")).toBe("project");
          return HttpResponse.json(mockResults);
        }),
      );

      const results = await client.search.search("project");
      expect(results).toHaveLength(3);
      // The optional projection array surfaces on the matching-type result.
      expect(results[1]!.content_attachments).toHaveLength(1);

      // The contract: present and null, never absent.
      for (const r of results) {
        expect(r.content).toBeNull();
        expect(r.description).toBeNull();
      }
      expect(results[1]!.plain_text_content).toContain(
        'mark class="circled-text"',
      );
      expect(results[0]!.plain_text_content).toBeUndefined();
      expect(results[1]!.content_attachments![0]!.width).toBe(1024);
      expect(results[0]!.title).toBe("Project Plan");
      expect(results[1]!.type).toBe("Message");

      // The file-attachment hit: no envelope keys, file keys instead.
      const hit = results[2]!;
      expect(hit.id).toBeUndefined();
      expect(hit.title).toBeUndefined();
      expect(hit.type).toBeUndefined();
      expect(hit.url).toBeUndefined();
      expect(hit.app_url).toBeUndefined();
      expect(hit.filename).toBe("leto-hero.jpg");
      expect(hit.content_type).toBe("image/jpeg");
      expect(hit.byte_size).toBe(512000);
      expect(hit.previewable).toBe(true);
      expect(hit.width).toBe(1920);
      expect(hit.height).toBe(1080);
      expect(hit.preview_url).toContain("/previews/");
      expect(hit.thumbnail_url).toContain("/thumbnails/");
      expect(hit.download_url).toContain("/download/");
      expect(hit.app_download_url).toContain("/download/");
    });

    it("should support sort option", async () => {
      server.use(
        http.get(`${BASE_URL}/search.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("q")).toBe("test");
          expect(url.searchParams.get("sort")).toBe("best_match");
          return HttpResponse.json([]);
        }),
      );

      const results = await client.search.search("test", {
        sort: "best_match",
      });
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
        }),
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
        }),
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


    it("should return empty array when no results", async () => {
      server.use(
        http.get(`${BASE_URL}/search.json`, () => {
          return HttpResponse.json([]);
        }),
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
        }),
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
