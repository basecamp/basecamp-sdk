/**
 * Tests for the LineupService
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

  describe("createMarker", () => {
    it("should create a new lineup marker", async () => {
      const mockMarker = {
        id: 1,
        title: "Product Launch",
        starts_on: "2024-03-01",
        ends_on: "2024-03-15",
        color: "green",
        status: "active",
      };

      server.use(
        http.post(`${BASE_URL}/lineup/markers.json`, async ({ request }) => {
          const body = await request.json() as {
            title: string;
            starts_on: string;
            ends_on: string;
            color?: string;
          };
          expect(body.title).toBe("Product Launch");
          expect(body.starts_on).toBe("2024-03-01");
          expect(body.ends_on).toBe("2024-03-15");
          expect(body.color).toBe("green");
          return HttpResponse.json({ marker: mockMarker });
        })
      );

      const marker = await client.lineup.createMarker({
        title: "Product Launch",
        startsOn: "2024-03-01",
        endsOn: "2024-03-15",
        color: "green",
      });

      expect(marker.id).toBe(1);
      expect(marker.title).toBe("Product Launch");
    });

    it("should throw validation error for missing title", async () => {
      await expect(
        client.lineup.createMarker({
          title: "",
          startsOn: "2024-03-01",
          endsOn: "2024-03-15",
        })
      ).rejects.toThrow("Marker title is required");
    });

    it("should throw validation error for missing starts_on", async () => {
      await expect(
        client.lineup.createMarker({
          title: "Test",
          startsOn: "",
          endsOn: "2024-03-15",
        })
      ).rejects.toThrow("Marker starts_on date is required");
    });

    it("should throw validation error for missing ends_on", async () => {
      await expect(
        client.lineup.createMarker({
          title: "Test",
          startsOn: "2024-03-01",
          endsOn: "",
        })
      ).rejects.toThrow("Marker ends_on date is required");
    });

    it("should throw validation error for invalid starts_on format", async () => {
      await expect(
        client.lineup.createMarker({
          title: "Test",
          startsOn: "March 1, 2024",
          endsOn: "2024-03-15",
        })
      ).rejects.toThrow("Marker starts_on must be in YYYY-MM-DD format");
    });

    it("should throw validation error for invalid ends_on format", async () => {
      await expect(
        client.lineup.createMarker({
          title: "Test",
          startsOn: "2024-03-01",
          endsOn: "03/15/2024",
        })
      ).rejects.toThrow("Marker ends_on must be in YYYY-MM-DD format");
    });
  });

  describe("updateMarker", () => {
    it("should update an existing marker", async () => {
      const markerId = 123;
      const mockMarker = {
        id: markerId,
        title: "Updated Launch",
        starts_on: "2024-03-01",
        ends_on: "2024-03-20",
        color: "blue",
      };

      server.use(
        http.put(`${BASE_URL}/lineup/markers/${markerId}`, async ({ request }) => {
          const body = await request.json() as { title?: string; ends_on?: string; color?: string };
          expect(body.title).toBe("Updated Launch");
          expect(body.ends_on).toBe("2024-03-20");
          expect(body.color).toBe("blue");
          return HttpResponse.json({ marker: mockMarker });
        })
      );

      const marker = await client.lineup.updateMarker(markerId, {
        title: "Updated Launch",
        endsOn: "2024-03-20",
        color: "blue",
      });

      expect(marker.title).toBe("Updated Launch");
    });

    it("should allow partial updates", async () => {
      const markerId = 123;
      const mockMarker = {
        id: markerId,
        title: "Existing Title",
        color: "red",
      };

      server.use(
        http.put(`${BASE_URL}/lineup/markers/${markerId}`, async ({ request }) => {
          const body = await request.json() as { color?: string };
          expect(body.color).toBe("red");
          return HttpResponse.json({ marker: mockMarker });
        })
      );

      const marker = await client.lineup.updateMarker(markerId, {
        color: "red",
      });

      expect(marker.color).toBe("red");
    });

    it("should throw validation error for invalid date format in update", async () => {
      await expect(
        client.lineup.updateMarker(123, { startsOn: "invalid-date" })
      ).rejects.toThrow("Marker starts_on must be in YYYY-MM-DD format");
    });
  });

  describe("deleteMarker", () => {
    it("should delete a marker", async () => {
      const markerId = 123;

      server.use(
        http.delete(`${BASE_URL}/lineup/markers/${markerId}`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.lineup.deleteMarker(markerId)).resolves.toBeUndefined();
    });

    it("should throw not_found error for non-existent marker", async () => {
      server.use(
        http.delete(`${BASE_URL}/lineup/markers/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.lineup.deleteMarker(999)).rejects.toThrow(BasecampError);
    });
  });
});
