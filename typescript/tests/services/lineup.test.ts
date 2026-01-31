/**
 * Tests for the LineupService (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - Method names: create(), update(), delete() (not createMarker, updateMarker, deleteMarker)
 * - No client-side validation (API validates)
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
            title: string;
            starts_on: string;
            ends_on: string;
            color?: string;
          };
          expect(body.title).toBe("Product Launch");
          expect(body.starts_on).toBe("2024-03-01");
          expect(body.ends_on).toBe("2024-03-15");
          expect(body.color).toBe("green");
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(
        client.lineup.create({
          title: "Product Launch",
          startsOn: "2024-03-01",
          endsOn: "2024-03-15",
          color: "green",
        })
      ).resolves.toBeUndefined();
    });

    // Note: Client-side validation removed - generated services let API validate
  });

  describe("update", () => {
    it("should update an existing marker", async () => {
      const markerId = 123;

      server.use(
        http.put(`${BASE_URL}/lineup/markers/${markerId}`, async ({ request }) => {
          const body = (await request.json()) as { title?: string; ends_on?: string; color?: string };
          expect(body.title).toBe("Updated Launch");
          expect(body.ends_on).toBe("2024-03-20");
          expect(body.color).toBe("blue");
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(
        client.lineup.update(markerId, {
          title: "Updated Launch",
          endsOn: "2024-03-20",
          color: "blue",
        })
      ).resolves.toBeUndefined();
    });

    it("should allow partial updates", async () => {
      const markerId = 123;

      server.use(
        http.put(`${BASE_URL}/lineup/markers/${markerId}`, async ({ request }) => {
          const body = (await request.json()) as { color?: string };
          expect(body.color).toBe("red");
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(
        client.lineup.update(markerId, {
          color: "red",
        })
      ).resolves.toBeUndefined();
    });

    // Note: Client-side validation removed - generated services let API validate
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
});
