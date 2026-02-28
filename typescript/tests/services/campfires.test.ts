/**
 * Tests for the CampfiresService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleCampfire = (id = 1) => ({
  id,
  title: "Campfire",
  topic: "General chat",
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
});

const sampleLine = (id = 1) => ({
  id,
  content: "<p>Hello everyone!</p>",
  created_at: "2024-01-15T10:00:00Z",
  creator: { id: 100, name: "Jane Doe" },
});

describe("CampfiresService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("get", () => {
    it("should return a single campfire", async () => {
      const campfireId = 42;

      server.use(
        http.get(`${BASE_URL}/chats/${campfireId}`, () => {
          return HttpResponse.json(sampleCampfire(campfireId));
        })
      );

      const campfire = await client.campfires.get(campfireId);
      expect(campfire.id).toBe(campfireId);
      expect(campfire.title).toBe("Campfire");
    });
  });

  describe("list", () => {
    it("should list all campfires", async () => {
      server.use(
        http.get(`${BASE_URL}/chats.json`, () => {
          return HttpResponse.json([sampleCampfire(1), sampleCampfire(2)]);
        })
      );

      const campfires = await client.campfires.list();
      expect(campfires).toHaveLength(2);
      expect(campfires[0]!.id).toBe(1);
      expect(campfires[1]!.id).toBe(2);
    });
  });

  describe("listLines", () => {
    it("should list lines in a campfire", async () => {
      const campfireId = 42;

      server.use(
        http.get(`${BASE_URL}/chats/${campfireId}/lines.json`, () => {
          return HttpResponse.json([sampleLine(1), sampleLine(2)]);
        })
      );

      const lines = await client.campfires.listLines(campfireId);
      expect(lines).toHaveLength(2);
      expect(lines[0]!.id).toBe(1);
      expect(lines[1]!.id).toBe(2);
    });
  });

  describe("createLine", () => {
    it("should create a line with content", async () => {
      const campfireId = 42;

      server.use(
        http.post(`${BASE_URL}/chats/${campfireId}/lines.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.content).toBe("Hello world!");
          return HttpResponse.json(sampleLine(99), { status: 201 });
        })
      );

      const line = await client.campfires.createLine(campfireId, {
        content: "Hello world!",
      });
      expect(line.id).toBe(99);
    });
  });

  describe("getLine", () => {
    it("should return a single line", async () => {
      const campfireId = 42;
      const lineId = 10;

      server.use(
        http.get(`${BASE_URL}/chats/${campfireId}/lines/${lineId}`, () => {
          return HttpResponse.json(sampleLine(lineId));
        })
      );

      const line = await client.campfires.getLine(campfireId, lineId);
      expect(line.id).toBe(lineId);
      expect(line.content).toBe("<p>Hello everyone!</p>");
    });
  });

  describe("deleteLine", () => {
    it("should delete a line", async () => {
      server.use(
        http.delete(`${BASE_URL}/chats/42/lines/10`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.campfires.deleteLine(42, 10)).resolves.toBeUndefined();
    });
  });
});
