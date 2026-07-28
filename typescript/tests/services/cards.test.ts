/**
 * Tests for the CardsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";
import cardFixture from "../../../spec/fixtures/cards/get.json";

const BASE_URL = "https://3.basecampapi.com/12345";

// Sourced from the shared, coverage-guarded fixture (spec/fixtures/manifest.yaml)
// so this helper cannot drift from the validated Card shape; `id` is overridable
// per call.
const sampleCard = (id = cardFixture.id) => ({ ...cardFixture, id });

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
      expect(card.title).toBe(cardFixture.title);
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

  describe("updateVerbatim", () => {
    it("sends a single PUT with no read-before-write", async () => {
      const cardId = 42;
      const methods: string[] = [];

      server.use(
        http.put(`${BASE_URL}/card_tables/cards/${cardId}`, async ({ request }) => {
          methods.push("PUT");
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.title).toBe("Updated card");
          // The raw path is sharp: an unset due_on stays off the wire, and BC3
          // reads that as a clear.
          expect(body.due_on).toBeUndefined();
          return HttpResponse.json(sampleCard(cardId));
        })
      );

      const card = await client.cards.updateVerbatim(cardId, {
        title: "Updated card",
      });
      expect(card.id).toBe(cardId);
      expect(methods).toEqual(["PUT"]);
    });
  });

  describe("update (merge-safe)", () => {
    it("refetches and resends due_on when the caller does not address it", async () => {
      const cardId = 42;
      const methods: string[] = [];

      server.use(
        http.get(`${BASE_URL}/card_tables/cards/${cardId}`, () => {
          methods.push("GET");
          return HttpResponse.json({ ...sampleCard(cardId), due_on: "2024-02-01" });
        }),
        http.put(`${BASE_URL}/card_tables/cards/${cardId}`, async ({ request }) => {
          methods.push("PUT");
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.title).toBe("Updated card");
          // BC3 merges the body over `{ due_on: nil }`, so omitting due_on
          // would erase the date.
          expect(body.due_on).toBe("2024-02-01");
          expect(body.assignee_ids).toBeUndefined();
          return HttpResponse.json(sampleCard(cardId));
        })
      );

      const card = await client.cards.update(cardId, { title: "Updated card" });
      expect(card.id).toBe(cardId);
      expect(methods).toEqual(["GET", "PUT"]);
    });

    it("clears the due date by omitting due_on, with no GET", async () => {
      const cardId = 42;
      const methods: string[] = [];

      server.use(
        http.put(`${BASE_URL}/card_tables/cards/${cardId}`, async ({ request }) => {
          methods.push("PUT");
          const body = (await request.json()) as Record<string, unknown>;
          // Never `{"due_on": null}` — that would violate body compaction.
          expect(body.due_on).toBeUndefined();
          expect("due_on" in body).toBe(false);
          return HttpResponse.json(sampleCard(cardId));
        })
      );

      await client.cards.update(cardId, { dueOn: null });
      expect(methods).toEqual(["PUT"]);
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
