/**
 * Tests for the WormholesService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleWormhole = (id = 1, linked = true) => ({
  id,
  status: "active",
  visible_to_clients: false,
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
  title: "Design → Marketing backlog",
  inherits_status: true,
  type: "Kanban::Wormhole",
  url: `${BASE_URL}/buckets/2085958499/card_tables/wormholes/${id}.json`,
  app_url: `https://3.basecamp.com/12345/buckets/2085958499/card_tables/wormholes/${id}`,
  parent: { id: 10, title: "Development Board", type: "Kanban::Board", url: "u", app_url: "a" },
  bucket: { id: 2085958499, name: "The Leto Laptop", type: "Project" },
  creator: { id: 1, name: "Victor Cooper" },
  color: "#f5d76e",
  linked,
  destination_url: linked
    ? `${BASE_URL}/buckets/2085958500/card_tables/columns/1069479500.json`
    : null,
});

describe("WormholesService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("create", () => {
    it("should create a wormhole to a destination column", async () => {
      const bucketId = 2085958499;
      const cardTableId = 1069479345;

      server.use(
        http.post(
          `${BASE_URL}/buckets/${bucketId}/card_tables/${cardTableId}/wormholes.json`,
          async ({ request }) => {
            const body = (await request.json()) as Record<string, unknown>;
            expect(body.destination_recording_id).toBe(1069479500);
            return HttpResponse.json(sampleWormhole(99), { status: 201 });
          }
        )
      );

      const wormhole = await client.wormholes.create(bucketId, cardTableId, {
        destinationRecordingId: 1069479500,
      });
      expect(wormhole.id).toBe(99);
      expect(wormhole.linked).toBe(true);
      expect(wormhole.destination_url).not.toBeNull();
    });

    it("should throw validation at the 4-wormhole limit (422)", async () => {
      const bucketId = 2085958499;
      const cardTableId = 1069479345;

      server.use(
        http.post(
          `${BASE_URL}/buckets/${bucketId}/card_tables/${cardTableId}/wormholes.json`,
          () => HttpResponse.json({ error: "Limit reached" }, { status: 422 })
        )
      );

      await expect(
        client.wormholes.create(bucketId, cardTableId, { destinationRecordingId: 1069479500 })
      ).rejects.toThrow(BasecampError);
    });

    it("should throw not_found for a bad destination (404)", async () => {
      const bucketId = 2085958499;
      const cardTableId = 1069479345;

      server.use(
        http.post(
          `${BASE_URL}/buckets/${bucketId}/card_tables/${cardTableId}/wormholes.json`,
          () => HttpResponse.json({ error: "Not found" }, { status: 404 })
        )
      );

      await expect(
        client.wormholes.create(bucketId, cardTableId, { destinationRecordingId: 999 })
      ).rejects.toThrow(BasecampError);
    });
  });

  describe("update", () => {
    it("should update a wormhole's destination", async () => {
      const bucketId = 2085958499;
      const wormholeId = 1069479400;

      server.use(
        http.put(
          `${BASE_URL}/buckets/${bucketId}/card_tables/wormholes/${wormholeId}`,
          async ({ request }) => {
            const body = (await request.json()) as Record<string, unknown>;
            expect(body.destination_recording_id).toBe(1069479501);
            return HttpResponse.json(sampleWormhole(wormholeId));
          }
        )
      );

      const wormhole = await client.wormholes.update(bucketId, wormholeId, {
        destinationRecordingId: 1069479501,
      });
      expect(wormhole.id).toBe(wormholeId);
    });

    it("should throw not_found for a missing wormhole (404)", async () => {
      const bucketId = 2085958499;
      const wormholeId = 999;

      server.use(
        http.put(
          `${BASE_URL}/buckets/${bucketId}/card_tables/wormholes/${wormholeId}`,
          () => HttpResponse.json({ error: "Not found" }, { status: 404 })
        )
      );

      await expect(
        client.wormholes.update(bucketId, wormholeId, { destinationRecordingId: 1 })
      ).rejects.toThrow(BasecampError);
    });
  });

  describe("delete", () => {
    it("should delete a wormhole", async () => {
      const bucketId = 2085958499;
      const wormholeId = 1069479400;

      server.use(
        http.delete(
          `${BASE_URL}/buckets/${bucketId}/card_tables/wormholes/${wormholeId}`,
          () => new HttpResponse(null, { status: 204 })
        )
      );

      await expect(client.wormholes.delete(bucketId, wormholeId)).resolves.toBeUndefined();
    });

    it("should throw forbidden when deletion is not allowed (403)", async () => {
      const bucketId = 2085958499;
      const wormholeId = 1069479400;

      server.use(
        http.delete(
          `${BASE_URL}/buckets/${bucketId}/card_tables/wormholes/${wormholeId}`,
          () => HttpResponse.json({ error: "Forbidden" }, { status: 403 })
        )
      );

      await expect(client.wormholes.delete(bucketId, wormholeId)).rejects.toThrow(BasecampError);
    });

    it("should throw not_found for a missing wormhole (404)", async () => {
      const bucketId = 2085958499;
      const wormholeId = 999;

      server.use(
        http.delete(
          `${BASE_URL}/buckets/${bucketId}/card_tables/wormholes/${wormholeId}`,
          () => HttpResponse.json({ error: "Not found" }, { status: 404 })
        )
      );

      await expect(client.wormholes.delete(bucketId, wormholeId)).rejects.toThrow(BasecampError);
    });
  });
});
