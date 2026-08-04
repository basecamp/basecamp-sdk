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
          // An unset due_on stays off the wire. Since bc3#12521 the JSON
          // representation reads that as "leave the due date unchanged".
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

  // bc3#12521 made the JSON card representation presence-aware: an omitted key
  // is left unchanged, and the due date is cleared by an explicit `""` or null.
  // These handlers MODEL that server, so the assertions are about the card that
  // ends up stored rather than only about the bytes that went out.
  describe("update (presence-aware)", () => {
    const presenceAwareServer = (initial: { due_on: string | null }) => {
      const stored: { title: string; due_on: string | null } = {
        title: "Original title",
        due_on: initial.due_on,
      };
      const methods: string[] = [];

      server.use(
        http.get(`${BASE_URL}/card_tables/cards/42`, () => {
          methods.push("GET");
          return HttpResponse.json({ ...sampleCard(42), ...stored });
        }),
        http.put(`${BASE_URL}/card_tables/cards/42`, async ({ request }) => {
          methods.push("PUT");
          const body = (await request.json()) as Record<string, unknown>;
          // Presence, not truthiness — `"due_on" in body` is the whole point.
          if ("title" in body) stored.title = String(body.title);
          if ("due_on" in body) {
            const value = body.due_on;
            stored.due_on = value === null || value === "" ? null : String(value);
          }
          return HttpResponse.json({ ...sampleCard(42), ...stored });
        })
      );

      return { stored, methods };
    };

    it("leaves the stored due date alone when the caller does not address it", async () => {
      const { stored, methods } = presenceAwareServer({ due_on: "2024-02-01" });

      const card = await client.cards.update(42, { title: "Updated card" });

      expect(stored.title).toBe("Updated card");
      // The date survives an update that never mentioned it — and survives
      // without a read-modify-write, which is what the single PUT pins.
      expect(stored.due_on).toBe("2024-02-01");
      expect(card.due_on).toBe("2024-02-01");
      expect(methods).toEqual(["PUT"]);
    });

    it("actually clears the stored due date when asked explicitly", async () => {
      const { stored, methods } = presenceAwareServer({ due_on: "2024-02-01" });

      const card = await client.cards.update(42, { dueOn: null });

      // The regression this pins: encoding the clear as an OMISSION leaves the
      // date standing on a presence-aware server, so the call silently no-ops.
      // Only a body that carries due_on clears it.
      expect(stored.due_on).toBeNull();
      expect(card.due_on).toBeNull();
      expect(methods).toEqual(["PUT"]);
    });

    it("sets the stored due date when given one", async () => {
      const { stored, methods } = presenceAwareServer({ due_on: null });

      await client.cards.update(42, { dueOn: "2024-03-15" });

      expect(stored.due_on).toBe("2024-03-15");
      expect(methods).toEqual(["PUT"]);
    });
  });

  // The wire-level half of the same contract. A GET handler is registered but
  // must never fire: `methods` is the assertion that `update` is a single PUT.
  describe("update (wire encoding)", () => {
    const capture = () => {
      const captured = { body: {} as Record<string, unknown>, methods: [] as string[] };

      server.use(
        http.get(`${BASE_URL}/card_tables/cards/42`, () => {
          captured.methods.push("GET");
          return HttpResponse.json(sampleCard(42));
        }),
        http.put(`${BASE_URL}/card_tables/cards/42`, async ({ request }) => {
          captured.methods.push("PUT");
          captured.body = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleCard(42));
        })
      );

      return captured;
    };

    it("sends an explicit clear as an empty string, present on the wire", async () => {
      const captured = capture();

      await client.cards.update(42, { dueOn: null });

      // Present AND empty. `""` is the spelling BC3 casts to nil; `null` would
      // violate body compaction (SPEC §18), and an omission would now be read
      // as "leave unchanged". JSON.stringify keeps `""` where it drops
      // `undefined`, so this is a genuine wire assertion.
      expect("due_on" in captured.body).toBe(true);
      expect(captured.body.due_on).toBe("");
      expect(captured.methods).toEqual(["PUT"]);
    });

    it("keeps due_on off the wire when the caller does not address it", async () => {
      const captured = capture();

      await client.cards.update(42, { title: "Renamed" });

      expect(captured.body).not.toHaveProperty("due_on");
      expect(captured.body.title).toBe("Renamed");
      expect(captured.methods).toEqual(["PUT"]);
    });

    it("never resends assignees on the caller's behalf", async () => {
      const captured = capture();

      await client.cards.update(42, { title: "Renamed" });

      // BC3 filters incoming ids through reachable_people, so echoing them
      // back could unassign someone who has since lost board access.
      expect(captured.body).not.toHaveProperty("assignee_ids");
    });

    it("sends an explicitly empty content, which is a clear", async () => {
      const captured = capture();

      await client.cards.update(42, { content: "", dueOn: null });

      expect(captured.body.content).toBe("");
      expect(captured.body.due_on).toBe("");
      expect(captured.methods).toEqual(["PUT"]);
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
