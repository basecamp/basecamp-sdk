import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

function sampleBookmark(id: number) {
  return {
    id,
    created_at: "2026-07-01T00:00:00Z",
    updated_at: "2026-07-02T00:00:00Z",
    recording: {
      id: 900,
      status: "active",
      visible_to_clients: false,
      created_at: "2026-06-01T00:00:00Z",
      updated_at: "2026-06-02T00:00:00Z",
      title: "Kickoff notes",
      inherits_status: true,
      type: "Document",
      url: "https://3.basecampapi.com/12345/buckets/2/documents/900.json",
      app_url: "https://3.basecamp.com/12345/buckets/2/documents/900",
      bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
      creator: { id: 1, name: "Victor Cooper" },
    },
  };
}

describe("BookmarksService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("listMyBookmarks", () => {
    it("lists the bookmark envelopes with their wrapped recordings", async () => {
      server.use(
        http.get(`${BASE_URL}/my/bookmarks.json`, () => {
          return HttpResponse.json([sampleBookmark(1), sampleBookmark(2)]);
        })
      );

      const result = await client.bookmarks.listMyBookmarks();
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(1);
      expect(result[0].recording.title).toBe("Kickoff notes");
    });

    it("surfaces 401 as BasecampError", async () => {
      server.use(
        http.get(`${BASE_URL}/my/bookmarks.json`, () => {
          return HttpResponse.json({ error: "Unauthorized" }, { status: 401 });
        })
      );

      const error = await client.bookmarks.listMyBookmarks().catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(401);
    });
  });

  describe("getBookmark", () => {
    it("reports the bookmarked state", async () => {
      server.use(
        http.get(`${BASE_URL}/recordings/900/bookmark.json`, () => {
          return HttpResponse.json({ bookmarked: true });
        })
      );

      const status = await client.bookmarks.getBookmark(900);
      expect(status.bookmarked).toBe(true);
    });

    it("surfaces 404 as BasecampError", async () => {
      server.use(
        http.get(`${BASE_URL}/recordings/999/bookmark.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      const error = await client.bookmarks.getBookmark(999).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(404);
    });
  });

  describe("createBookmark", () => {
    it("bookmarks the recording and returns the envelope", async () => {
      server.use(
        http.post(`${BASE_URL}/recordings/900/bookmark.json`, () => {
          return HttpResponse.json(sampleBookmark(7), { status: 201 });
        })
      );

      const bookmark = await client.bookmarks.createBookmark(900);
      expect(bookmark.id).toBe(7);
      expect(bookmark.recording.id).toBe(900);
    });

    it("surfaces 404 as BasecampError", async () => {
      server.use(
        http.post(`${BASE_URL}/recordings/999/bookmark.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      const error = await client.bookmarks.createBookmark(999).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(404);
    });
  });

  describe("deleteBookmark", () => {
    it("removes the bookmark (204)", async () => {
      let called = false;
      server.use(
        http.delete(`${BASE_URL}/recordings/900/bookmark.json`, () => {
          called = true;
          return new HttpResponse(null, { status: 204 });
        })
      );

      await client.bookmarks.deleteBookmark(900);
      expect(called).toBe(true);
    });

    it("surfaces 403 as BasecampError", async () => {
      server.use(
        http.delete(`${BASE_URL}/recordings/900/bookmark.json`, () => {
          return HttpResponse.json({ error: "Forbidden" }, { status: 403 });
        })
      );

      const error = await client.bookmarks.deleteBookmark(900).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(403);
    });
  });
});
