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
  columns: [],
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
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
  });
});
