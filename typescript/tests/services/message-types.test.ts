/**
 * Tests for the MessageTypesService class (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - No client-side validation (API validates)
 *
 * Message types (categories) are bucket-scoped: every operation requires a
 * project id and hits /buckets/{projectId}/categories(.json). See #368.
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient, type BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";
const PROJECT_ID = 89;

describe("MessageTypesService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
  });

  describe("list", () => {
    it("should list all message types in a project (bucket-scoped path)", async () => {
      const mockTypes = [
        { id: 1, name: "Announcement", icon: "📢", created_at: "2024-01-01T00:00:00Z", updated_at: "2024-01-01T00:00:00Z" },
        { id: 2, name: "Question", icon: "❓", created_at: "2024-01-01T00:00:00Z", updated_at: "2024-01-01T00:00:00Z" },
      ];

      let requestedPath: string | undefined;
      server.use(
        http.get(`${BASE_URL}/buckets/${PROJECT_ID}/categories.json`, ({ request }) => {
          requestedPath = new URL(request.url).pathname;
          return HttpResponse.json(mockTypes);
        })
      );

      const types = await client.messageTypes.list(PROJECT_ID);

      expect(requestedPath).toBe(`/12345/buckets/${PROJECT_ID}/categories.json`);
      expect(types).toHaveLength(2);
      expect(types[0].name).toBe("Announcement");
      expect(types[0].icon).toBe("📢");
      expect(types[1].name).toBe("Question");
    });

    it("should return empty array when no types exist", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/${PROJECT_ID}/categories.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const types = await client.messageTypes.list(PROJECT_ID);
      expect(types).toHaveLength(0);
    });

    it("should surface API errors", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/${PROJECT_ID}/categories.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.messageTypes.list(PROJECT_ID)).rejects.toThrow(BasecampError);
    });
  });

  describe("get", () => {
    it("should get a message type by ID", async () => {
      const mockType = {
        id: 1,
        name: "Announcement",
        icon: "📢",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      };

      let requestedPath: string | undefined;
      server.use(
        http.get(`${BASE_URL}/buckets/${PROJECT_ID}/categories/1`, ({ request }) => {
          requestedPath = new URL(request.url).pathname;
          return HttpResponse.json(mockType);
        })
      );

      const type = await client.messageTypes.get(PROJECT_ID, 1);

      expect(requestedPath).toBe(`/12345/buckets/${PROJECT_ID}/categories/1`);
      expect(type.id).toBe(1);
      expect(type.name).toBe("Announcement");
      expect(type.icon).toBe("📢");
    });
  });

  describe("create", () => {
    it("should create a new message type", async () => {
      const mockType = {
        id: 3,
        name: "Update",
        icon: "🔄",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      };

      let requestedPath: string | undefined;
      server.use(
        http.post(`${BASE_URL}/buckets/${PROJECT_ID}/categories.json`, async ({ request }) => {
          requestedPath = new URL(request.url).pathname;
          const body = await request.json() as { name: string; icon: string };
          expect(body.name).toBe("Update");
          expect(body.icon).toBe("🔄");
          return HttpResponse.json(mockType);
        })
      );

      const type = await client.messageTypes.create(PROJECT_ID, {
        name: "Update",
        icon: "🔄",
      });

      expect(requestedPath).toBe(`/12345/buckets/${PROJECT_ID}/categories.json`);
      expect(type.id).toBe(3);
      expect(type.name).toBe("Update");
    });

    // Note: Client-side validation removed - generated services let API validate
  });

  describe("update", () => {
    it("should update an existing message type", async () => {
      const mockType = {
        id: 1,
        name: "Updated Name",
        icon: "🎉",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-02T00:00:00Z",
      };

      let requestedPath: string | undefined;
      server.use(
        http.put(`${BASE_URL}/buckets/${PROJECT_ID}/categories/1`, ({ request }) => {
          requestedPath = new URL(request.url).pathname;
          return HttpResponse.json(mockType);
        })
      );

      const type = await client.messageTypes.update(PROJECT_ID, 1, {
        name: "Updated Name",
        icon: "🎉",
      });

      expect(requestedPath).toBe(`/12345/buckets/${PROJECT_ID}/categories/1`);
      expect(type.name).toBe("Updated Name");
      expect(type.icon).toBe("🎉");
    });
  });

  describe("delete", () => {
    it("should delete a message type", async () => {
      let requestedPath: string | undefined;
      server.use(
        http.delete(`${BASE_URL}/buckets/${PROJECT_ID}/categories/1`, ({ request }) => {
          requestedPath = new URL(request.url).pathname;
          return new HttpResponse(null, { status: 204 });
        })
      );

      // Should not throw
      await client.messageTypes.delete(PROJECT_ID, 1);
      expect(requestedPath).toBe(`/12345/buckets/${PROJECT_ID}/categories/1`);
    });
  });
});
