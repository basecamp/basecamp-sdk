/**
 * Tests for the Recordings service (generated from OpenAPI spec)
 *
 * Tests pagination (ListResult return type), bucket array ergonomics,
 * and all CRUD operations.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import type { RecordingsService } from "../../src/generated/services/recordings.js";
import { BasecampError } from "../../src/errors.js";
import { ListResult } from "../../src/pagination.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("RecordingsService", () => {
  let service: RecordingsService;

  beforeEach(() => {
    vi.clearAllMocks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
    service = client.recordings;
  });

  describe("list", () => {
    it("should list recordings by type and return ListResult", async () => {
      const recordings = [
        { id: 1001, type: "Todo", title: "Task 1", status: "active" },
        { id: 1002, type: "Todo", title: "Task 2", status: "active" },
      ];

      server.use(
        http.get(`${BASE_URL}/projects/recordings.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("type")).toBe("Todo");
          return HttpResponse.json(recordings, {
            headers: { "X-Total-Count": "2" },
          });
        })
      );

      const result = await service.list("Todo");

      expect(result).toBeInstanceOf(ListResult);
      expect(result).toHaveLength(2);
      expect(result[0].type).toBe("Todo");
      expect(result.meta.totalCount).toBe(2);
    });

    it("should include optional filters in query", async () => {
      let capturedUrl: URL | null = null;

      server.use(
        http.get(`${BASE_URL}/projects/recordings.json`, ({ request }) => {
          capturedUrl = new URL(request.url);
          return HttpResponse.json([]);
        })
      );

      // bucket is number[] → joined as CSV string in the query
      await service.list("Document", {
        bucket: [123],
        status: "archived",
        sort: "updated_at",
        direction: "asc",
      });

      expect(capturedUrl?.searchParams.get("type")).toBe("Document");
      expect(capturedUrl?.searchParams.get("bucket")).toBe("123");
      expect(capturedUrl?.searchParams.get("status")).toBe("archived");
      expect(capturedUrl?.searchParams.get("sort")).toBe("updated_at");
      expect(capturedUrl?.searchParams.get("direction")).toBe("asc");
    });

    it("should join multiple bucket IDs as CSV", async () => {
      let capturedUrl: URL | null = null;

      server.use(
        http.get(`${BASE_URL}/projects/recordings.json`, ({ request }) => {
          capturedUrl = new URL(request.url);
          return HttpResponse.json([]);
        })
      );

      await service.list("Todo", { bucket: [1, 2, 3] });

      expect(capturedUrl?.searchParams.get("bucket")).toBe("1,2,3");
    });

    it("should return empty ListResult when no recordings", async () => {
      server.use(
        http.get(`${BASE_URL}/projects/recordings.json`, () => {
          return HttpResponse.json([], {
            headers: { "X-Total-Count": "0" },
          });
        })
      );

      const result = await service.list("Todo");

      expect(result).toHaveLength(0);
      expect(result.meta.totalCount).toBe(0);
    });
  });

  describe("get", () => {
    it("should return a recording by ID", async () => {
      const recording = {
        id: 3001,
        type: "Message",
        title: "Welcome Message",
        status: "active",
        visible_to_clients: false,
        creator: { id: 1001, name: "Alice" },
      };

      server.use(
        http.get(`${BASE_URL}/buckets/123/recordings/3001`, () => {
          return HttpResponse.json(recording);
        })
      );

      const result = await service.get(123, 3001);

      expect(result.id).toBe(3001);
      expect(result.type).toBe("Message");
      expect(result.title).toBe("Welcome Message");
    });

    it("should throw not_found error for 404 response", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/123/recordings/9999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(service.get(123, 9999)).rejects.toThrow(BasecampError);

      try {
        await service.get(123, 9999);
      } catch (err) {
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });

  describe("trash", () => {
    it("should move a recording to trash", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/3001/status/trashed.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(service.trash(123, 3001)).resolves.toBeUndefined();
    });

    it("should throw error for non-existent recording", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/9999/status/trashed.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(service.trash(123, 9999)).rejects.toThrow(BasecampError);
    });
  });

  describe("archive", () => {
    it("should archive a recording", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/3001/status/archived.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(service.archive(123, 3001)).resolves.toBeUndefined();
    });

    it("should throw error for non-existent recording", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/9999/status/archived.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(service.archive(123, 9999)).rejects.toThrow(BasecampError);
    });
  });

  describe("unarchive", () => {
    it("should unarchive a recording", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/3001/status/active.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(service.unarchive(123, 3001)).resolves.toBeUndefined();
    });

    it("should throw error for non-existent recording", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/9999/status/active.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(service.unarchive(123, 9999)).rejects.toThrow(BasecampError);
    });
  });
});
