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
      const projectId = 100;
      const boostId = 42;

      server.use(
        http.get(`${BASE_URL}/buckets/${projectId}/boosts/${boostId}`, () => {
          return HttpResponse.json(sampleBoost(boostId));
        })
      );

      const boost = await client.boosts.get(projectId, boostId);
      expect(boost.id).toBe(boostId);
      expect(boost.content).toBe("🎉");
      expect(boost.booster.name).toBe("Jane Doe");
    });

    it("should throw not_found for missing boost", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/100/boosts/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.boosts.get(100, 999)).rejects.toThrow(BasecampError);
    });
  });

  describe("deleteBoost", () => {
    it("should delete a boost", async () => {
      server.use(
        http.delete(`${BASE_URL}/buckets/100/boosts/42`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.boosts.delete(100, 42)).resolves.toBeUndefined();
    });
  });

  describe("listForRecording", () => {
    it("should list boosts on a recording", async () => {
      const projectId = 100;
      const recordingId = 200;

      server.use(
        http.get(`${BASE_URL}/buckets/${projectId}/recordings/${recordingId}/boosts.json`, () => {
          return HttpResponse.json([sampleBoost(1), sampleBoost(2)]);
        })
      );

      const boosts = await client.boosts.listForRecording(projectId, recordingId);
      expect(boosts).toHaveLength(2);
      expect(boosts[0]!.id).toBe(1);
      expect(boosts[1]!.id).toBe(2);
    });
  });

  describe("createForRecording", () => {
    it("should create a boost on a recording", async () => {
      const projectId = 100;
      const recordingId = 200;

      server.use(
        http.post(`${BASE_URL}/buckets/${projectId}/recordings/${recordingId}/boosts.json`, async ({ request }) => {
          const body = (await request.json()) as { content: string };
          expect(body.content).toBe("🔥");
          return HttpResponse.json(sampleBoost(99), { status: 201 });
        })
      );

      const boost = await client.boosts.createForRecording(projectId, recordingId, {
        content: "🔥",
      });
      expect(boost.id).toBe(99);
    });
  });

  describe("listForEvent", () => {
    it("should list boosts on an event", async () => {
      const projectId = 100;
      const recordingId = 200;
      const eventId = 300;

      server.use(
        http.get(
          `${BASE_URL}/buckets/${projectId}/recordings/${recordingId}/events/${eventId}/boosts.json`,
          () => {
            return HttpResponse.json([sampleBoost(5)]);
          }
        )
      );

      const boosts = await client.boosts.listForEvent(projectId, recordingId, eventId);
      expect(boosts).toHaveLength(1);
      expect(boosts[0]!.id).toBe(5);
    });
  });

  describe("createForEvent", () => {
    it("should create a boost on an event", async () => {
      const projectId = 100;
      const recordingId = 200;
      const eventId = 300;

      server.use(
        http.post(
          `${BASE_URL}/buckets/${projectId}/recordings/${recordingId}/events/${eventId}/boosts.json`,
          async ({ request }) => {
            const body = (await request.json()) as { content: string };
            expect(body.content).toBe("👍");
            return HttpResponse.json(sampleBoost(77), { status: 201 });
          }
        )
      );

      const boost = await client.boosts.createForEvent(projectId, recordingId, eventId, {
        content: "👍",
      });
      expect(boost.id).toBe(77);
    });
  });
});
