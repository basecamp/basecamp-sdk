/**
 * Tests for the MessagesService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleMessage = (id = 1) => ({
  id,
  subject: "Weekly Update",
  content: "<p>Here is the update</p>",
  status: "active",
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
  creator: { id: 100, name: "Jane Doe" },
});

describe("MessagesService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("list", () => {
    it("should list messages on a message board", async () => {
      const boardId = 200;

      server.use(
        http.get(`${BASE_URL}/message_boards/${boardId}/messages.json`, () => {
          return HttpResponse.json([sampleMessage(1), sampleMessage(2)]);
        })
      );

      const messages = await client.messages.list(boardId);
      expect(messages).toHaveLength(2);
      expect(messages[0]!.id).toBe(1);
      expect(messages[1]!.id).toBe(2);
    });

    it("should return empty array when no messages exist", async () => {
      server.use(
        http.get(`${BASE_URL}/message_boards/200/messages.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const messages = await client.messages.list(200);
      expect(messages).toHaveLength(0);
    });
  });

  describe("get", () => {
    it("should return a single message", async () => {
      const messageId = 42;

      server.use(
        http.get(`${BASE_URL}/messages/${messageId}`, () => {
          return HttpResponse.json(sampleMessage(messageId));
        })
      );

      const message = await client.messages.get(messageId);
      expect(message.id).toBe(messageId);
      expect(message.subject).toBe("Weekly Update");
    });

    it("should throw not_found for missing message", async () => {
      server.use(
        http.get(`${BASE_URL}/messages/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.messages.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("create", () => {
    it("should create a message with subject and content", async () => {
      const boardId = 200;

      server.use(
        http.post(`${BASE_URL}/message_boards/${boardId}/messages.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.subject).toBe("New Post");
          expect(body.content).toBe("<p>Body text</p>");
          return HttpResponse.json(sampleMessage(99), { status: 201 });
        })
      );

      const message = await client.messages.create(boardId, {
        subject: "New Post",
        content: "<p>Body text</p>",
      });
      expect(message.id).toBe(99);
    });
  });

  describe("update", () => {
    it("should update a message", async () => {
      const messageId = 42;

      server.use(
        http.put(`${BASE_URL}/messages/${messageId}`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.subject).toBe("Updated Subject");
          return HttpResponse.json(sampleMessage(messageId));
        })
      );

      const message = await client.messages.update(messageId, {
        subject: "Updated Subject",
      });
      expect(message.id).toBe(messageId);
    });
  });

  describe("pin", () => {
    it("should pin a message", async () => {
      server.use(
        http.post(`${BASE_URL}/recordings/42/pin.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.messages.pin(42)).resolves.toBeUndefined();
    });
  });

  describe("unpin", () => {
    it("should unpin a message", async () => {
      server.use(
        http.delete(`${BASE_URL}/recordings/42/pin.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.messages.unpin(42)).resolves.toBeUndefined();
    });
  });
});
