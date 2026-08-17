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
// Sourced from the shared, coverage-guarded fixture (spec/fixtures/manifest.yaml):
// eight hits covering the generic recording envelope and all four branches
// `api_search_result_template_path` special-cases. Imported rather than restated
// so this cannot drift from the copy the other five SDKs and the conformance
// runners assert against.
import searchResultsFixture from "../../../spec/fixtures/search/results.json" with { type: "json" };

const BASE_URL = "https://3.basecampapi.com/12345";

type SearchHit = Record<string, unknown>;
const SEARCH_RESULTS = searchResultsFixture as unknown as SearchHit[];
const attachmentHit = SEARCH_RESULTS.find((h) => h.type === undefined)!;
const uploadLineHit = SEARCH_RESULTS.find((h) => h.type === "Chat::Lines::Upload")!;
const kanbanHit = SEARCH_RESULTS.find((h) => h.type === "Kanban::Column")!;
const needleHit = SEARCH_RESULTS.find((h) => h.type === "Gauge::Needle")!;

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

    /**
     * The four branches BC3's `api_search_result_template_path` special-cases.
     * Each drives the real service against a hit taken verbatim from the shared
     * fixture, so a branch that changes shape upstream cannot pass here while
     * failing the conformance runners.
     */
    describe("special-cased result branches", () => {
      const respondWith = (hits: SearchHit[]) => {
        server.use(
          http.get(`${BASE_URL}/search.json`, () => HttpResponse.json(hits)),
        );
      };

      it("decodes a file-attachment hit, which omits the five envelope keys", async () => {
        respondWith([attachmentHit]);

        const [hit] = await client.search.search("leto hero");

        // searches/_attachment.json.jbuilder writes its own projection instead
        // of decorating the recording envelope, so the ABSENCE of these five is
        // the branch discriminator. (In consumer code they are optional in the
        // generated types, so an unguarded read is a strictNullChecks error —
        // this file is not type-checked, `tsconfig.json` excludes `tests`.)
        expect(hit.id).toBeUndefined();
        expect(hit.title).toBeUndefined();
        expect(hit.type).toBeUndefined();
        expect(hit.url).toBeUndefined();
        expect(hit.app_url).toBeUndefined();

        expect(hit.filename).toBe("leto-hero.jpg");
        expect(hit.content_type).toBe("image/jpeg");
        expect(hit.byte_size).toBe(512000);
        expect(hit.previewable).toBe(true);
        // Float-spelled on the wire (1920.0); JSON has one number type, so this
        // is 1920 in JavaScript. The narrowing is load-bearing in the
        // statically-typed tiers, and the conformance fixture pins it there.
        expect(hit.width).toBe(1920);
        expect(hit.height).toBe(1080);
        expect(hit.download_url).toContain("/download/");
        expect(hit.app_download_url).toContain("/download/");
        // Present-and-null on every branch, this one included: the show
        // template nil-overwrites both after rendering the recording partial.
        expect(hit.content).toBeNull();
        expect(hit.description).toBeNull();
        expect(hit.parent?.type).toBe("Message");
      });

      it("decodes a chat upload line's bespoke attachments aggregate", async () => {
        respondWith([uploadLineHit]);

        const [hit] = await client.search.search("benchmarks");

        expect(hit.type).toBe("Chat::Lines::Upload");
        // Chat lines pass `boostable`, so the envelope emits the boost pair.
        expect(hit.boosts_count).toBe(1);
        expect(hit.boosts_url).toContain("/boosts.json");

        // NOT a RichTextAttachment: the line builds a six-key aggregate inline,
        // with no id, no sgid and no preview keys. SearchResultAttachment is
        // the optional-field superset of this variant and the rich-text one.
        const attachment = hit.attachments![0]!;
        expect(Object.keys(attachment).sort()).toEqual([
          "byte_size",
          "content_type",
          "download_url",
          "filename",
          "title",
          "url",
        ]);
        expect(attachment.title).toBe("leto-benchmarks.pdf");
        expect(attachment.content_type).toBe("application/pdf");
        expect(attachment.id).toBeUndefined();
        expect(attachment.sgid).toBeUndefined();
      });

      it("decodes a kanban list, whose color is present-and-null", async () => {
        respondWith([kanbanHit]);

        const [hit] = await client.search.search("in progress");

        expect(hit.type).toBe("Kanban::Column");
        expect(hit.cards_count).toBe(4);
        expect(hit.comment_count).toBe(1);
        expect(hit.cards_url).toContain("/cards.json");
        // Emitted unconditionally with a null value when unset — present-and-
        // null is the normal case here, not a malformed body.
        expect(hit.color).toBeNull();
        // Envelope keys the list branch reaches: subscribable and positioned.
        expect(hit.subscription_url).toContain("/subscription.json");
        expect(hit.position).toBe(2);
        expect(hit.subscribers?.map((p) => p.name)).toEqual(["Victor Cooper"]);
        // on_hold is a whole nested list, not a flag.
        expect(hit.on_hold?.cards_count).toBe(0);
        expect(hit.on_hold?.cards_url).toContain("/cards.json");
      });

      it("decodes a gauge needle, which carries both count pairs", async () => {
        respondWith([needleHit]);

        const [hit] = await client.search.search("progress update");

        expect(hit.type).toBe("Gauge::Needle");
        // Commentable AND boostable, plus the branch partial's own singular
        // comment_count — a distinct key from the envelope's comments_count.
        expect(hit.comments_count).toBe(2);
        expect(hit.comment_count).toBe(2);
        expect(hit.boosts_count).toBe(3);
        expect(hit.color).toBe("green");
        expect(hit.position).toBe(72);
        // description is nil-overwritten; its companion array is not.
        expect(hit.description).toBeNull();
        expect(hit.description_attachments).toHaveLength(1);

        // The OTHER attachments variant: the rich-text one, id and sgid set.
        const attachment = hit.attachments![0]!;
        expect(attachment.id).toBe(1069479631);
        expect(attachment.sgid).toContain("--srchndl1");
        expect(attachment.width).toBe(1024);
        expect(attachment.previewable).toBe(true);
      });

      it("decodes every branch in one response", async () => {
        respondWith(SEARCH_RESULTS);

        const results = await client.search.search("Leto");

        expect(results).toHaveLength(8);
        // bubble_up_url rides the polymorphic projection: todolists/_todolist
        // is the only partial passing bubbleupable: true.
        expect(
          results.filter((r) => r.bubble_up_url !== undefined).map((r) => r.type),
        ).toEqual(["Todolist"]);
      });
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
