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

  // --- #576: a malformed GET due_on must never reach the replacement PUT ----
  //
  // `update` reads the card only to resend its due date, so that one value is
  // the whole reason the composite exists -- and `if (current.due_on)` failed
  // in both directions at once. A falsey non-string was DROPPED, and an
  // omitted `due_on` is exactly how BC3 erases a card's due date
  // (`{ due_on: nil }.merge(card_params)`), the behaviour this composite
  // exists to prevent. A truthy non-string was forwarded VERBATIM and written
  // to the card.
  //
  // TypeScript has no runtime decoder to catch either -- `schema.d.ts` is
  // erased at build time, so `Card` is a compile-time claim nothing validates.
  //
  // The assertion that matters is the ORDERING: `requests` must be ["GET"].
  describe("malformed due_on (#576)", () => {
    const malformed: [string, unknown][] = [
      ["false", false],
      ["zero", 0],
      ["empty array", []],
      ["empty object", {}],
      ["number", 42],
      ["true", true],
      ["array", ["x"]],
      ["object", { a: 1 }],
    ];

    const serve = (body: unknown, requests: string[]) => {
      server.use(
        http.get(`${BASE_URL}/card_tables/cards/42`, () => {
          requests.push("GET");
          return HttpResponse.json(body);
        }),
        http.put(`${BASE_URL}/card_tables/cards/42`, () => {
          requests.push("PUT");
          return HttpResponse.json(sampleCard(42));
        })
      );
    };

    const rejection = async (promise: Promise<unknown>): Promise<unknown> =>
      promise.then(
        () => {
          throw new Error("expected the call to reject, but it resolved");
        },
        (error: unknown) => error
      );

    it.each(malformed)("update refuses a %s due_on before writing", async (_label, value) => {
      const requests: string[] = [];
      serve({ ...sampleCard(42), due_on: value }, requests);

      const error = await rejection(client.cards.update(42, { title: "Renamed" }));

      expect(error).toBeInstanceOf(BasecampError);
      // api_error, not usage: the value arrived in a successful response.
      expect((error as BasecampError).code).toBe("api_error");
      expect((error as BasecampError).message).toMatch(/Card field "due_on" is not a string/);
      expect(requests).toEqual(["GET"]);
    });

    // The other half of the rule: a card with no due date is not malformed.
    it.each([
      ["absent", undefined],
      ["null", null],
    ])("treats a %s due_on as genuinely empty", async (_label, value) => {
      let putBody: Record<string, unknown> = {};
      const body: Record<string, unknown> = { ...sampleCard(42), due_on: value };
      if (value === undefined) delete body.due_on;
      server.use(
        http.get(`${BASE_URL}/card_tables/cards/42`, () => HttpResponse.json(body)),
        http.put(`${BASE_URL}/card_tables/cards/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleCard(42));
        })
      );

      await client.cards.update(42, { title: "Renamed" });

      expect(putBody).not.toHaveProperty("due_on");
    });

    // One level up from the field guard: a successful GET can return a scalar,
    // an array or null, and reading `current.due_on` off null throws a raw
    // TypeError instead of the documented statusless api_error.
    it.each([
      ["array", []],
      ["string", "card"],
      ["number", 42],
      ["null", null],
      ["boolean", true],
    ])("update refuses a %s response body before writing", async (_label, body) => {
      const requests: string[] = [];
      serve(body, requests);

      const error = await rejection(client.cards.update(42, { title: "Renamed" }));

      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("api_error");
      expect((error as BasecampError).message).toMatch(/GetCard returned/);
      expect(requests).toEqual(["GET"]);
    });
  });
});
