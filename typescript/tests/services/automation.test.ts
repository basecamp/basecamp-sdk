/**
 * Tests for the AutomationService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("AutomationService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
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

      const markers = await client.automation.listLineupMarkers();
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

      const markers = await client.automation.listLineupMarkers();
      expect(markers).toHaveLength(0);
    });
  });
});
