/**
 * Tests for the BoostsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleBoost = (id = 1) => ({
  id,
  content: "🎉",
  created_at: "2024-01-15T10:00:00Z",
  booster: { id: 100, name: "Jane Doe" },
  recording: { id: 200, title: "Some recording", type: "Todo", url: `${BASE_URL}/buckets/1/todos/200` },
});

describe("BoostsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("boost (get)", () => {
    it("should return a single boost", async () => {
      const boostId = 42;

      server.use(
        http.get(`${BASE_URL}/boosts/${boostId}`, () => {
          return HttpResponse.json(sampleBoost(boostId));
        })
      );

      const boost = await client.boosts.get(boostId);
      expect(boost.id).toBe(boostId);
      expect(boost.content).toBe("🎉");
      // `booster` is optional in the generated schema, so assert it arrived
      // before reading through it: a boost served without one then fails on
      // the presence check rather than on an undefined property read.
      expect(boost.booster).toBeDefined();
      expect(boost.booster?.name).toBe("Jane Doe");
    });

    it("should throw not_found for missing boost", async () => {
      server.use(
        http.get(`${BASE_URL}/boosts/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.boosts.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("update", () => {
    it("should replace the content of a boost", async () => {
      let received: Record<string, unknown> | undefined;
      server.use(
        http.put(`${BASE_URL}/boosts/42`, async ({ request }) => {
          received = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ ...sampleBoost(42), content: "Done!" });
        })
      );

      const boost = await client.boosts.update(42, { content: "Done!" });
      expect(boost.id).toBe(42);
      expect(boost.content).toBe("Done!");
      expect(received).toEqual({ content: "Done!" });
    });

    it("should reject missing content before any request", async () => {
      let reached = false;
      server.use(
        http.put(`${BASE_URL}/boosts/42`, () => {
          reached = true;
          return HttpResponse.json(sampleBoost(42));
        })
      );

      const err = await client.boosts
        .update(42, {} as { content: string })
        .catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("validation");
      expect(reached).toBe(false);
    });

    it.each([
      [403, "forbidden"],
      [404, "not_found"],
      [422, "validation"],
    ])("should throw %i as %s", async (status, code) => {
      server.use(
        http.put(`${BASE_URL}/boosts/42`, () => new HttpResponse(null, { status }))
      );

      const err = await client.boosts.update(42, { content: "Done!" }).catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe(code);
    });
  });

  describe("deleteBoost", () => {
    it("should delete a boost", async () => {
      server.use(
        http.delete(`${BASE_URL}/boosts/42`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.boosts.delete(42)).resolves.toBeUndefined();
    });
  });

  describe("listForRecording", () => {
    it("should list boosts on a recording", async () => {
      const recordingId = 200;

      server.use(
        http.get(`${BASE_URL}/recordings/${recordingId}/boosts.json`, () => {
          return HttpResponse.json([sampleBoost(1), sampleBoost(2)]);
        })
      );

      const boosts = await client.boosts.listForRecording(recordingId);
      expect(boosts).toHaveLength(2);
      expect(boosts[0]!.id).toBe(1);
      expect(boosts[1]!.id).toBe(2);
    });
  });

  describe("createForRecording", () => {
    it("should create a boost on a recording", async () => {
      const recordingId = 200;

      server.use(
        http.post(`${BASE_URL}/recordings/${recordingId}/boosts.json`, async ({ request }) => {
          const body = (await request.json()) as { content: string };
          expect(body.content).toBe("🔥");
          return HttpResponse.json(sampleBoost(99), { status: 201 });
        })
      );

      const boost = await client.boosts.createForRecording(recordingId, {
        content: "🔥",
      });
      expect(boost.id).toBe(99);
    });
  });

  describe("listForEvent", () => {
    it("should list boosts on an event", async () => {
      const recordingId = 200;
      const eventId = 300;

      server.use(
        http.get(
          `${BASE_URL}/recordings/${recordingId}/events/${eventId}/boosts.json`,
          () => {
            return HttpResponse.json([sampleBoost(5)]);
          }
        )
      );

      const boosts = await client.boosts.listForEvent(recordingId, eventId);
      expect(boosts).toHaveLength(1);
      expect(boosts[0]!.id).toBe(5);
    });
  });

  describe("createForEvent", () => {
    it("should create a boost on an event", async () => {
      const recordingId = 200;
      const eventId = 300;

      server.use(
        http.post(
          `${BASE_URL}/recordings/${recordingId}/events/${eventId}/boosts.json`,
          async ({ request }) => {
            const body = (await request.json()) as { content: string };
            expect(body.content).toBe("👍");
            return HttpResponse.json(sampleBoost(77), { status: 201 });
          }
        )
      );

      const boost = await client.boosts.createForEvent(recordingId, eventId, {
        content: "👍",
      });
      expect(boost.id).toBe(77);
    });
  });
});
