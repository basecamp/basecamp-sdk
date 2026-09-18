/**
 * Tests for the LineupService (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - Method names: create(), update(), delete() (not createMarker, updateMarker, deleteMarker)
 * - Client-side checks: create() rejects a missing name or date; the API validates the rest
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("LineupService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("create", () => {
    it("should create a new lineup marker", async () => {
      server.use(
        http.post(`${BASE_URL}/lineup/markers.json`, async ({ request }) => {
          const body = (await request.json()) as {
            name: string;
            date: string;
          };
          expect(body.name).toBe("Product Launch");
          expect(body.date).toBe("2024-03-01");
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(
        client.lineup.create({
          name: "Product Launch",
          date: "2024-03-01",
        })
      ).resolves.toBeUndefined();
    });
  });

  describe("update", () => {
    it("should update an existing marker", async () => {
      const markerId = 123;

      server.use(
        http.put(`${BASE_URL}/lineup/markers/${markerId}`, async ({ request }) => {
          const body = (await request.json()) as { name?: string; date?: string };
          expect(body.name).toBe("Updated Launch");
          expect(body.date).toBe("2024-03-20");
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(
        client.lineup.update(markerId, {
          name: "Updated Launch",
          date: "2024-03-20",
        })
      ).resolves.toBeUndefined();
    });

    it("should allow partial updates", async () => {
      const markerId = 123;

      server.use(
        http.put(`${BASE_URL}/lineup/markers/${markerId}`, async ({ request }) => {
          const body = (await request.json()) as { name?: string };
          expect(body.name).toBe("New Name");
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(
        client.lineup.update(markerId, {
          name: "New Name",
        })
      ).resolves.toBeUndefined();
    });
  });

  describe("delete", () => {
    it("should delete a marker", async () => {
      const markerId = 123;

      server.use(
        http.delete(`${BASE_URL}/lineup/markers/${markerId}`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.lineup.delete(markerId)).resolves.toBeUndefined();
    });

    it("should throw not_found error for non-existent marker", async () => {
      server.use(
        http.delete(`${BASE_URL}/lineup/markers/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.lineup.delete(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("listLineupMarkers", () => {
    it("should return lineup markers", async () => {
      server.use(
        http.get(`${BASE_URL}/lineup/markers.json`, () => {
          return HttpResponse.json([
            {
              id: 1069479400,
              name: "Product Launch",
              date: "2024-03-01",
              created_at: "2024-02-15T10:30:00.000Z",
              updated_at: "2024-02-15T10:30:00.000Z",
            },
            {
              id: 1069479401,
              name: "Quarterly Review",
              date: "2024-06-15",
              created_at: "2024-03-01T09:00:00.000Z",
              updated_at: "2024-03-01T09:00:00.000Z",
            },
          ]);
        })
      );

      const markers = await client.lineup.listLineupMarkers();
      expect(markers).toHaveLength(2);
      expect(markers[0]!.id).toBe(1069479400);
      expect(markers[0]!.name).toBe("Product Launch");
      expect(markers[0]!.date).toBe("2024-03-01");
    });

    it("should return empty array when no markers", async () => {
      server.use(
        http.get(`${BASE_URL}/lineup/markers.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const markers = await client.lineup.listLineupMarkers();
      expect(markers).toHaveLength(0);
    });
  });
});
