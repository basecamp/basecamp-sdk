/**
 * Tests for the CardTablesService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleCardTable = (id = 1) => ({
  id,
  title: "Card Table",
  lists: [],
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
});

// Full recording shape for a wormhole, matching the generated Wormhole schema's
// required fields (a bare {id,title,linked,...} would decode but not reflect the
// contract). color/destination_url are required-but-nullable — null when unlinked.
const sampleWormhole = (id: number, linked: boolean) => ({
  id,
  status: "active",
  visible_to_clients: false,
  created_at: "2024-01-16T10:00:00Z",
  updated_at: "2024-01-21T11:00:00Z",
  title: linked ? "Design → Marketing backlog" : "Broken teleport",
  inherits_status: true,
  type: "Kanban::Wormhole",
  url: `${BASE_URL}/buckets/2085958499/card_tables/wormholes/${id}.json`,
  app_url: `https://3.basecamp.com/12345/buckets/2085958499/card_tables/wormholes/${id}`,
  parent: { id: 10, title: "Development Board", type: "Kanban::Board", url: "u", app_url: "a" },
  bucket: { id: 2085958499, name: "The Leto Laptop", type: "Project" },
  creator: { id: 1, name: "Victor Cooper" },
  color: linked ? "#f5d76e" : null,
  linked,
  destination_url: linked
    ? `${BASE_URL}/buckets/2085958500/card_tables/columns/1069479500.json`
    : null,
});

describe("CardTablesService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("get", () => {
    it("should return a single card table", async () => {
      const cardTableId = 42;

      server.use(
        http.get(`${BASE_URL}/card_tables/${cardTableId}`, () => {
          return HttpResponse.json(sampleCardTable(cardTableId));
        })
      );

      const cardTable = await client.cardTables.get(cardTableId);
      expect(cardTable.id).toBe(cardTableId);
      expect(cardTable.title).toBe("Card Table");
    });

    it("should throw not_found for missing card table", async () => {
      server.use(
        http.get(`${BASE_URL}/card_tables/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.cardTables.get(999)).rejects.toThrow(BasecampError);
    });

    it("should decode linked and unlinked wormholes", async () => {
      const cardTableId = 42;

      server.use(
        http.get(`${BASE_URL}/card_tables/${cardTableId}`, () => {
          return HttpResponse.json({
            ...sampleCardTable(cardTableId),
            // Card-table columns live under `lists`; `wormholes` is its sibling.
            lists: [{ id: 100, title: "To Do", cards_count: 0 }],
            wormholes: [sampleWormhole(1069479400, true), sampleWormhole(1069479401, false)],
          });
        })
      );

      const cardTable = await client.cardTables.get(cardTableId);
      expect(cardTable.wormholes).toHaveLength(2);
      expect(cardTable.wormholes?.[0].linked).toBe(true);
      expect(cardTable.wormholes?.[0].destination_url).not.toBeNull();
      expect(cardTable.wormholes?.[1].linked).toBe(false);
      expect(cardTable.wormholes?.[1].destination_url).toBeNull();
    });
  });
});
