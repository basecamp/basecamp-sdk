/**
 * Tests for the WebhooksService class (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - No client-side validation (API validates)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient, type BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("WebhooksService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
  });

  describe("list", () => {
    it("should list all webhooks for a project", async () => {
      const mockWebhooks = [
        {
          id: 1,
          active: true,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
          payload_url: "https://example.com/webhook1",
          types: ["Todo", "Comment"],
          url: "https://3.basecampapi.com/12345/webhooks/1.json",
          app_url: "https://3.basecamp.com/12345/webhooks/1",
        },
        {
          id: 2,
          active: false,
          created_at: "2024-01-02T00:00:00Z",
          updated_at: "2024-01-02T00:00:00Z",
          payload_url: "https://example.com/webhook2",
          types: ["Message"],
          url: "https://3.basecampapi.com/12345/webhooks/2.json",
          app_url: "https://3.basecamp.com/12345/webhooks/2",
        },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/1/webhooks.json`, () => {
          return HttpResponse.json(mockWebhooks);
        })
      );

      const webhooks = await client.webhooks.list(1);

      expect(webhooks).toHaveLength(2);
      expect(webhooks[0].payload_url).toBe("https://example.com/webhook1");
      expect(webhooks[0].types).toEqual(["Todo", "Comment"]);
      expect(webhooks[0].active).toBe(true);
      expect(webhooks[1].active).toBe(false);
    });

    it("should return empty array when no webhooks exist", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/1/webhooks.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const webhooks = await client.webhooks.list(1);
      expect(webhooks).toHaveLength(0);
    });
  });

  describe("get", () => {
    it("should get a webhook by ID", async () => {
      const mockWebhook = {
        id: 1,
        active: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
        payload_url: "https://example.com/webhook1",
        types: ["Todo", "Comment", "Todolist"],
        url: "https://3.basecampapi.com/12345/webhooks/1.json",
        app_url: "https://3.basecamp.com/12345/webhooks/1",
      };

      server.use(
        http.get(`${BASE_URL}/webhooks/1`, () => {
          return HttpResponse.json(mockWebhook);
        })
      );

      const webhook = await client.webhooks.get(1);

      expect(webhook.id).toBe(1);
      expect(webhook.payload_url).toBe("https://example.com/webhook1");
      expect(webhook.types).toContain("Todo");
      expect(webhook.types).toContain("Comment");
    });

    it("should parse recent_deliveries with nested event", async () => {
      const fixture = await import("../../../spec/fixtures/webhooks/get.json");
      const mockWebhook = fixture.default ?? fixture;

      server.use(
        http.get(`${BASE_URL}/webhooks/1`, () => {
          return HttpResponse.json(mockWebhook);
        })
      );

      const webhook = await client.webhooks.get(1);

      expect(webhook.recent_deliveries).toBeDefined();
      expect(webhook.recent_deliveries).toHaveLength(1);

      const delivery = webhook.recent_deliveries![0];
      expect(delivery.id).toBe(1230);
      expect(delivery.request?.body?.kind).toBe("todo_created");
      expect(delivery.request?.body?.recording?.type).toBe("Todo");
      expect(delivery.request?.body?.recording?.content).toBe("<div>Ship the feature by Friday</div>");
      expect(delivery.request?.body?.creator?.can_ping).toBe(true);
      expect(delivery.response?.code).toBe(200);
    });

    it("should throw not_found for non-existent webhook", async () => {
      server.use(
        http.get(`${BASE_URL}/webhooks/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      try {
        await client.webhooks.get(999);
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });

  describe("create", () => {
    it("should create a new webhook", async () => {
      const mockWebhook = {
        id: 3,
        active: true,
        created_at: "2024-01-03T00:00:00Z",
        updated_at: "2024-01-03T00:00:00Z",
        payload_url: "https://example.com/new-webhook",
        types: ["Todo", "Message"],
        url: "https://3.basecampapi.com/12345/webhooks/3.json",
        app_url: "https://3.basecamp.com/12345/webhooks/3",
      };

      server.use(
        http.post(`${BASE_URL}/buckets/1/webhooks.json`, async ({ request }) => {
          const body = (await request.json()) as {
            payload_url: string;
            types: string[];
            active: boolean;
          };
          expect(body.payload_url).toBe("https://example.com/new-webhook");
          expect(body.types).toEqual(["Todo", "Message"]);
          expect(body.active).toBe(true);
          return HttpResponse.json(mockWebhook);
        })
      );

      const webhook = await client.webhooks.create(1, {
        payloadUrl: "https://example.com/new-webhook",
        types: ["Todo", "Message"],
        active: true,
      });

      expect(webhook.id).toBe(3);
      expect(webhook.payload_url).toBe("https://example.com/new-webhook");
    });

    // Note: Client-side validation removed - generated services let API validate
  });

  describe("update", () => {
    it("should update an existing webhook", async () => {
      const mockWebhook = {
        id: 1,
        active: false,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-04T00:00:00Z",
        payload_url: "https://example.com/webhook1",
        types: ["Todo", "Comment"],
        url: "https://3.basecampapi.com/12345/webhooks/1.json",
        app_url: "https://3.basecamp.com/12345/webhooks/1",
      };

      server.use(
        http.put(`${BASE_URL}/webhooks/1`, () => {
          return HttpResponse.json(mockWebhook);
        })
      );

      const webhook = await client.webhooks.update(1, {
        active: false,
      });

      expect(webhook.active).toBe(false);
    });

    it("should update webhook URL and types", async () => {
      const mockWebhook = {
        id: 1,
        active: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-04T00:00:00Z",
        payload_url: "https://new-example.com/webhook",
        types: ["Message", "Document"],
        url: "https://3.basecampapi.com/12345/webhooks/1.json",
        app_url: "https://3.basecamp.com/12345/webhooks/1",
      };

      server.use(
        http.put(`${BASE_URL}/webhooks/1`, async ({ request }) => {
          const body = (await request.json()) as {
            payload_url: string;
            types: string[];
          };
          expect(body.payload_url).toBe("https://new-example.com/webhook");
          expect(body.types).toEqual(["Message", "Document"]);
          return HttpResponse.json(mockWebhook);
        })
      );

      const webhook = await client.webhooks.update(1, {
        payloadUrl: "https://new-example.com/webhook",
        types: ["Message", "Document"],
      });

      expect(webhook.payload_url).toBe("https://new-example.com/webhook");
      expect(webhook.types).toEqual(["Message", "Document"]);
    });
  });

  describe("delete", () => {
    it("should delete a webhook", async () => {
      server.use(
        http.delete(`${BASE_URL}/webhooks/1`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      // Should not throw
      await client.webhooks.delete(1);
    });
  });
});
