/**
 * Tests for the ForwardsService class (generated from OpenAPI spec)
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

describe("ForwardsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
  });

  describe("getInbox", () => {
    it("should get an inbox by ID", async () => {
      const mockInbox = {
        id: 100,
        status: "active",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
        title: "Inbox",
        type: "Inbox",
        url: "https://3.basecampapi.com/12345/buckets/1/inboxes/100.json",
        app_url: "https://3.basecamp.com/12345/buckets/1/inboxes/100",
        bucket: { id: 1, name: "Test Project", type: "Project" },
      };

      server.use(
        http.get(`${BASE_URL}/buckets/1/inboxes/100`, () => {
          return HttpResponse.json(mockInbox);
        })
      );

      const inbox = await client.forwards.getInbox(1, 100);

      expect(inbox.id).toBe(100);
      expect(inbox.title).toBe("Inbox");
      expect(inbox.bucket?.name).toBe("Test Project");
    });
  });

  describe("list", () => {
    it("should list all forwards in an inbox", async () => {
      const mockForwards = [
        {
          id: 1,
          status: "active",
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
          subject: "Re: Project Update",
          content: "<p>Email content here</p>",
          from: "sender@example.com",
          type: "Inbox::Forward",
          url: "https://3.basecampapi.com/12345/buckets/1/inbox_forwards/1.json",
          app_url: "https://3.basecamp.com/12345/buckets/1/inbox_forwards/1",
        },
        {
          id: 2,
          status: "active",
          created_at: "2024-01-02T00:00:00Z",
          updated_at: "2024-01-02T00:00:00Z",
          subject: "Meeting Notes",
          content: "<p>Another email</p>",
          from: "other@example.com",
          type: "Inbox::Forward",
          url: "https://3.basecampapi.com/12345/buckets/1/inbox_forwards/2.json",
          app_url: "https://3.basecamp.com/12345/buckets/1/inbox_forwards/2",
        },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/1/inboxes/100/forwards.json`, () => {
          return HttpResponse.json(mockForwards);
        })
      );

      const forwards = await client.forwards.list(1, 100);

      expect(forwards).toHaveLength(2);
      expect(forwards[0].subject).toBe("Re: Project Update");
      expect(forwards[0].from).toBe("sender@example.com");
      expect(forwards[1].subject).toBe("Meeting Notes");
    });
  });

  describe("get", () => {
    it("should get a forward by ID", async () => {
      const mockForward = {
        id: 1,
        status: "active",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
        subject: "Re: Project Update",
        content: "<p>Email content here</p>",
        from: "sender@example.com",
        type: "Inbox::Forward",
        url: "https://3.basecampapi.com/12345/buckets/1/inbox_forwards/1.json",
        app_url: "https://3.basecamp.com/12345/buckets/1/inbox_forwards/1",
      };

      server.use(
        http.get(`${BASE_URL}/buckets/1/inbox_forwards/1`, () => {
          return HttpResponse.json(mockForward);
        })
      );

      const forward = await client.forwards.get(1, 1);

      expect(forward.id).toBe(1);
      expect(forward.subject).toBe("Re: Project Update");
      expect(forward.from).toBe("sender@example.com");
    });
  });

  describe("listReplies", () => {
    it("should list all replies to a forward", async () => {
      const mockReplies = [
        {
          id: 10,
          status: "active",
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
          content: "<p>Thanks for the update!</p>",
          type: "Inbox::Reply",
          url: "https://3.basecampapi.com/12345/buckets/1/inbox_replies/10.json",
          app_url: "https://3.basecamp.com/12345/buckets/1/inbox_replies/10",
        },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/1/inbox_forwards/1/replies.json`, () => {
          return HttpResponse.json(mockReplies);
        })
      );

      const replies = await client.forwards.listReplies(1, 1);

      expect(replies).toHaveLength(1);
      expect(replies[0].content).toBe("<p>Thanks for the update!</p>");
    });
  });

  describe("getReply", () => {
    it("should get a forward reply by ID", async () => {
      const mockReply = {
        id: 10,
        status: "active",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
        content: "<p>Thanks for the update!</p>",
        type: "Inbox::Reply",
        url: "https://3.basecampapi.com/12345/buckets/1/inbox_replies/10.json",
        app_url: "https://3.basecamp.com/12345/buckets/1/inbox_replies/10",
      };

      server.use(
        http.get(`${BASE_URL}/buckets/1/inbox_forwards/1/replies/10`, () => {
          return HttpResponse.json(mockReply);
        })
      );

      const reply = await client.forwards.getReply(1, 1, 10);

      expect(reply.id).toBe(10);
      expect(reply.content).toBe("<p>Thanks for the update!</p>");
    });
  });

  describe("createReply", () => {
    it("should create a reply to a forward", async () => {
      const mockReply = {
        id: 11,
        status: "active",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
        content: "<p>New reply content</p>",
        type: "Inbox::Reply",
        url: "https://3.basecampapi.com/12345/buckets/1/inbox_replies/11.json",
        app_url: "https://3.basecamp.com/12345/buckets/1/inbox_replies/11",
      };

      server.use(
        http.post(`${BASE_URL}/buckets/1/inbox_forwards/1/replies.json`, async ({ request }) => {
          const body = await request.json() as { content: string };
          expect(body.content).toBe("<p>New reply content</p>");
          return HttpResponse.json(mockReply);
        })
      );

      const reply = await client.forwards.createReply(1, 1, {
        content: "<p>New reply content</p>",
      });

      expect(reply.id).toBe(11);
      expect(reply.content).toBe("<p>New reply content</p>");
    });

    // Note: Client-side validation removed - generated services let API validate
  });
});
