/**
 * Tests for the EverythingService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("EverythingService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("everythingFiles", () => {
    it("encodes the kind filter and repeatable people_ids[] in the query string", async () => {
      let captured = "";
      server.use(
        http.get(`${BASE_URL}/files.json`, ({ request }) => {
          captured = new URL(request.url).search;
          return HttpResponse.json([]);
        })
      );

      await client.everything.everythingFiles({ kind: "images", peopleIds: [11, 22] });

      const params = new URLSearchParams(captured);
      expect(params.get("kind")).toBe("images");
      // people_ids[] is a repeatable array param: both ids must appear under the
      // bracketed key, not a single comma-joined value.
      expect(params.getAll("people_ids[]")).toEqual(["11", "22"]);
    });

    it("should decode the heterogeneous /files.json feed: Upload, Document, and Attachment variants in one array", async () => {
      const fixture = [
        {
          id: 900,
          type: "Upload",
          status: "active",
          visible_to_clients: false,
          title: "logo.png",
          inherits_status: true,
          filename: "logo.png",
          content_type: "image/png",
          byte_size: 1281,
          width: 1024.0,
          height: 768.0,
          url: "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
          app_url: "https://3.basecamp.com/1/buckets/2/uploads/900",
          download_url: "https://3.basecampapi.com/1/buckets/2/uploads/900/download/logo.png",
          app_download_url: "https://storage.3.basecamp.com/1/buckets/2/uploads/900/download/logo.png",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 901,
          type: "Document",
          status: "active",
          visible_to_clients: false,
          title: "Spec",
          inherits_status: true,
          content_type: "text/html",
          url: "https://3.basecampapi.com/1/buckets/2/documents/901.json",
          app_url: "https://3.basecamp.com/1/buckets/2/documents/901",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 902,
          type: "Attachment",
          attachable_sgid: "sgid-902",
          filename: "chart.avif",
          content_type: "image/avif",
          byte_size: 4096,
          width: null,
          height: null,
          download_url: "https://storage.3.basecamp.com/1/blobs/902/download/chart.avif",
          parent: { id: 800, title: "A message", type: "Message" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/files.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingFiles()) as any[];
      expect(result).toHaveLength(3);

      // variant 1: full Upload recording
      expect(result[0].type).toBe("Upload");
      expect(result[0].filename).toBe("logo.png");
      expect(result[0].app_download_url).toBe(
        "https://storage.3.basecamp.com/1/buckets/2/uploads/900/download/logo.png"
      );
      expect(result[0].width).toBe(1024);

      // variant 2: Basecamp Document recording
      expect(result[1].type).toBe("Document");
      expect(result[1].title).toBe("Spec");

      // variant 3: rich-text Attachment envelope
      expect(result[2].type).toBe("Attachment");
      expect(result[2].attachable_sgid).toBe("sgid-902");
      expect(result[2].parent).toBeDefined();
      expect(result[2].width).toBeNull();
    });
  });

  describe("everythingMessages", () => {
    it("should decode the paginated /messages.json Recording feed with embedded buckets", async () => {
      const fixture = [
        {
          id: 1001,
          type: "Message",
          title: "Kickoff",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 1002,
          type: "Message",
          title: "Status update",
          bucket: { id: 3, name: "Honcho Rollout", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/messages.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingMessages()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(1001);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[1].id).toBe(1002);
      expect(result[1].bucket.id).toBe(3);
    });
  });

  describe("everythingComments", () => {
    it("should decode the /comments.json Recording feed with embedded buckets", async () => {
      const fixture = [
        {
          id: 2001,
          type: "Comment",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 2002,
          type: "Comment",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 5, name: "Annie Bryan" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/comments.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingComments()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(2001);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[1].id).toBe(2002);
    });
  });

  describe("everythingCheckins", () => {
    it("should decode the /checkins.json Recording feed with embedded buckets", async () => {
      const fixture = [
        {
          id: 3001,
          type: "Question::Answer",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 3002,
          type: "Question::Answer",
          bucket: { id: 4, name: "Marketing Site", type: "Project" },
          creator: { id: 5, name: "Annie Bryan" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/checkins.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingCheckins()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(3001);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[1].bucket.id).toBe(4);
    });
  });

  describe("everythingForwards", () => {
    it("should decode the /forwards.json Recording feed with embedded buckets", async () => {
      const fixture = [
        {
          id: 4001,
          type: "Inbox::Forward",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 4002,
          type: "Inbox::Forward",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 5, name: "Annie Bryan" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/forwards.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingForwards()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(4001);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[1].id).toBe(4002);
    });
  });

  describe("everythingBoosts", () => {
    it("should decode the /boosts.json feed with booster and nested recording", async () => {
      const fixture = [
        {
          id: 5001,
          content: "👏",
          created_at: "2024-01-15T10:00:00Z",
          booster: { id: 1, name: "Victor Cooper" },
          recording: {
            id: 800,
            title: "A message",
            type: "Message",
            bucket: { id: 9, name: "The Leto Laptop", type: "Project" },
          },
        },
        {
          id: 5002,
          content: "🔥",
          created_at: "2024-01-15T11:00:00Z",
          booster: { id: 5, name: "Annie Bryan" },
          recording: { id: 801, title: "A comment", type: "Comment" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/boosts.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingBoosts()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(5001);
      expect(result[0].booster).toEqual({ id: 1, name: "Victor Cooper" });
      expect(result[0].recording.id).toBe(800);
      expect(result[0].recording.title).toBe("A message");
      expect(result[0].recording.type).toBe("Message");
      // The boosted recording carries its bucket for project context (the
      // everything feed renders the recording with its bucket).
      expect(result[0].recording.bucket).toEqual({ id: 9, name: "The Leto Laptop", type: "Project" });
      expect(result[1].id).toBe(5002);
      expect(result[1].recording.id).toBe(801);
    });
  });

  describe("everythingOverdueTodos", () => {
    it("should decode the unpaginated /todos/overdue.json array, oldest-first", async () => {
      const fixture = [
        {
          id: 6001,
          type: "Todo",
          title: "Ship the thing",
          due_on: "2024-01-10",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
        },
        {
          id: 6002,
          type: "Todo",
          title: "Review the docs",
          due_on: "2024-01-20",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/todos/overdue.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingOverdueTodos()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(6001);
      expect(result[1].id).toBe(6002);
      // oldest-first ordering by due_on
      expect(result[0].due_on).toBe("2024-01-10");
      expect(result[1].due_on).toBe("2024-01-20");
      expect(result[0].due_on! < result[1].due_on!).toBe(true);
    });
  });

  describe("everythingOverdueCards", () => {
    it("should decode the unpaginated /cards/overdue.json array, oldest-first", async () => {
      const fixture = [
        {
          id: 7001,
          type: "Kanban::Card",
          title: "Draft proposal",
          due_on: "2024-02-01",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
        },
        {
          id: 7002,
          type: "Kanban::Card",
          title: "Send invoice",
          due_on: "2024-02-14",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/cards/overdue.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingOverdueCards()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(7001);
      expect(result[1].id).toBe(7002);
      // oldest-first ordering by due_on
      expect(result[0].due_on).toBe("2024-02-01");
      expect(result[1].due_on).toBe("2024-02-14");
      expect(result[0].due_on! < result[1].due_on!).toBe(true);
    });
  });

  describe("error propagation", () => {
    const cases: Array<[string, string, () => Promise<unknown>]> = [
      ["everythingMessages", "/messages.json", () => client.everything.everythingMessages()],
      ["everythingComments", "/comments.json", () => client.everything.everythingComments()],
      ["everythingCheckins", "/checkins.json", () => client.everything.everythingCheckins()],
      ["everythingForwards", "/forwards.json", () => client.everything.everythingForwards()],
      ["everythingBoosts", "/boosts.json", () => client.everything.everythingBoosts()],
      ["everythingFiles", "/files.json", () => client.everything.everythingFiles()],
      ["everythingOverdueTodos", "/todos/overdue.json", () => client.everything.everythingOverdueTodos()],
      ["everythingOverdueCards", "/cards/overdue.json", () => client.everything.everythingOverdueCards()],
    ];

    it.each(cases)("%s propagates a 4xx as a BasecampError", async (_name, path, call) => {
      server.use(
        http.get(`${BASE_URL}${path}`, () => HttpResponse.json({ error: "Not found" }, { status: 404 }))
      );

      await expect(call()).rejects.toThrow(BasecampError);
    });
  });
});
