/**
 * Tests for the CardsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleCard = (id = 1) => ({
  id,
  title: "Design mockups",
  content: "<p>Create initial designs</p>",
  due_on: "2024-03-01",
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
});

describe("CardsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("list", () => {
    it("should list cards in a column", async () => {
      const columnId = 200;

      server.use(
        http.get(`${BASE_URL}/card_tables/lists/${columnId}/cards.json`, () => {
          return HttpResponse.json([sampleCard(1), sampleCard(2)]);
        })
      );

      const cards = await client.cards.list(columnId);
      expect(cards).toHaveLength(2);
      expect(cards[0]!.id).toBe(1);
      expect(cards[1]!.id).toBe(2);
    });

    it("should return empty array when no cards exist", async () => {
      server.use(
        http.get(`${BASE_URL}/card_tables/lists/200/cards.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const cards = await client.cards.list(200);
      expect(cards).toHaveLength(0);
    });
  });

  describe("get", () => {
    it("should return a single card", async () => {
      const cardId = 42;

      server.use(
        http.get(`${BASE_URL}/card_tables/cards/${cardId}`, () => {
          return HttpResponse.json(sampleCard(cardId));
        })
      );

      const card = await client.cards.get(cardId);
      expect(card.id).toBe(cardId);
      expect(card.title).toBe("Design mockups");
    });

    it("should throw not_found for missing card", async () => {
      server.use(
        http.get(`${BASE_URL}/card_tables/cards/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.cards.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("create", () => {
    it("should create a card with title", async () => {
      const columnId = 200;

      server.use(
        http.post(`${BASE_URL}/card_tables/lists/${columnId}/cards.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.title).toBe("New card");
          return HttpResponse.json(sampleCard(99), { status: 201 });
        })
      );

      const card = await client.cards.create(columnId, {
        title: "New card",
      });
      expect(card.id).toBe(99);
    });
  });

  describe("update", () => {
    it("should update a card", async () => {
      const cardId = 42;

      server.use(
        http.put(`${BASE_URL}/card_tables/cards/${cardId}`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.title).toBe("Updated card");
          return HttpResponse.json(sampleCard(cardId));
        })
      );

      const card = await client.cards.update(cardId, {
        title: "Updated card",
      });
      expect(card.id).toBe(cardId);
    });
  });

  describe("move", () => {
    it("should move a card to a different column", async () => {
      const cardId = 42;

      server.use(
        http.post(`${BASE_URL}/card_tables/cards/${cardId}/moves.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.column_id).toBe(300);
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(
        client.cards.move(cardId, { columnId: 300 })
      ).resolves.toBeUndefined();
    });
  });
});
