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
        {
          id: 1001,
          type: "Todo",
          title: "Task 1",
          status: "active",
          // The generic recording projection carries the matching type's
          // rich-text companion array; a Todo recording surfaces
          // description_attachments (empty here — the Todo has no inline files).
          description_attachments: [],
        },
        { id: 1002, type: "Todo", title: "Task 2", status: "active" },
      ];

      server.use(
        http.get(`${BASE_URL}/projects/recordings.json`, ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("type")).toBe("Todo");
          return HttpResponse.json(recordings, {
            headers: { "X-Total-Count": "2" },
          });
        }),
      );

      const result = await service.list("Todo");

      expect(result).toBeInstanceOf(ListResult);
      expect(result).toHaveLength(2);
      expect(result[0].type).toBe("Todo");
      // The optional projection array surfaces on the matching-type recording.
      expect(result[0].description_attachments).toEqual([]);
      expect(result.meta.totalCount).toBe(2);
    });

    it("should include optional filters in query", async () => {
      // Held in an object, not a `let`: control-flow analysis cannot see the
      // assignment inside the handler closure, so a `let ... = null` binding
      // narrows to `null`, so reading `.searchParams` off it is a `never`. The
      // optional chaining still makes an unrun handler fail the assertions.
      const captured: { url?: URL } = {};

      server.use(
        http.get(`${BASE_URL}/projects/recordings.json`, ({ request }) => {
          captured.url = new URL(request.url);
          return HttpResponse.json([]);
        }),
      );

      // bucket is number[] → joined as CSV string in the query
      await service.list("Document", {
        bucket: [123],
        status: "archived",
        sort: "updated_at",
        direction: "asc",
      });

      expect(captured.url?.searchParams.get("type")).toBe("Document");
      expect(captured.url?.searchParams.get("bucket")).toBe("123");
      expect(captured.url?.searchParams.get("status")).toBe("archived");
      expect(captured.url?.searchParams.get("sort")).toBe("updated_at");
      expect(captured.url?.searchParams.get("direction")).toBe("asc");
    });

    it("should join multiple bucket IDs as CSV", async () => {
      // Object-held for the same reason as above.
      const captured: { url?: URL } = {};

      server.use(
        http.get(`${BASE_URL}/projects/recordings.json`, ({ request }) => {
          captured.url = new URL(request.url);
          return HttpResponse.json([]);
        }),
      );

      await service.list("Todo", { bucket: [1, 2, 3] });

      expect(captured.url?.searchParams.get("bucket")).toBe("1,2,3");
    });

    it("should return empty ListResult when no recordings", async () => {
      server.use(
        http.get(`${BASE_URL}/projects/recordings.json`, () => {
          return HttpResponse.json([], {
            headers: { "X-Total-Count": "0" },
          });
        }),
      );

      const result = await service.list("Todo");

      expect(result).toHaveLength(0);
      expect(result.meta.totalCount).toBe(0);
    });
  });

  describe("spotlight", () => {
    it("should spotlight a recording on the canonical flat route", async () => {
      const recording = { id: 3001, type: "Message", title: "Launch", status: "active" };
      server.use(
        http.post(`${BASE_URL}/recordings/3001/spotlight.json`, () => {
          return HttpResponse.json(recording, { status: 201 });
        }),
      );

      await expect(service.spotlight(3001)).resolves.toEqual(recording);
    });

    it("should surface ineligible-recording errors", async () => {
      server.use(
        http.post(`${BASE_URL}/recordings/3001/spotlight.json`, () => {
          return HttpResponse.json({ errors: ["Recording cannot be spotlighted"] }, { status: 422 });
        }),
      );

      await expect(service.spotlight(3001)).rejects.toThrow(BasecampError);
    });
  });

  describe("unspotlight", () => {
    it("should remove a spotlight on the canonical flat route", async () => {
      server.use(
        http.delete(`${BASE_URL}/recordings/3001/spotlight.json`, () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(service.unspotlight(3001)).resolves.toBeUndefined();
    });

    it("should surface permission errors", async () => {
      server.use(
        http.delete(`${BASE_URL}/recordings/3001/spotlight.json`, () => {
          return new HttpResponse(null, { status: 403 });
        }),
      );

      await expect(service.unspotlight(3001)).rejects.toThrow(BasecampError);
    });
  });

  describe("trash", () => {
    it("should move a recording to trash", async () => {
      server.use(
        http.put(`${BASE_URL}/recordings/3001/status/trashed.json`, () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(service.trash(3001)).resolves.toBeUndefined();
    });

    it("should throw error for non-existent recording", async () => {
      server.use(
        http.put(`${BASE_URL}/recordings/9999/status/trashed.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        }),
      );

      await expect(service.trash(9999)).rejects.toThrow(BasecampError);
    });
  });

  describe("archive", () => {
    it("should archive a recording", async () => {
      server.use(
        http.put(`${BASE_URL}/recordings/3001/status/archived.json`, () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(service.archive(3001)).resolves.toBeUndefined();
    });

    it("should throw error for non-existent recording", async () => {
      server.use(
        http.put(`${BASE_URL}/recordings/9999/status/archived.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        }),
      );

      await expect(service.archive(9999)).rejects.toThrow(BasecampError);
    });
  });

  describe("unarchive", () => {
    it("should unarchive a recording", async () => {
      server.use(
        http.put(`${BASE_URL}/recordings/3001/status/active.json`, () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(service.unarchive(3001)).resolves.toBeUndefined();
    });

    it("should throw error for non-existent recording", async () => {
      server.use(
        http.put(`${BASE_URL}/recordings/9999/status/active.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        }),
      );

      await expect(service.unarchive(9999)).rejects.toThrow(BasecampError);
    });
  });

  // Note: setClientVisibility() is on ClientVisibilityService in generated services
  // Use client.clientVisibility.setVisibility(recordingId, { visibleToClients: true })
});
