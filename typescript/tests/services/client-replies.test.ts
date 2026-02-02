/**
 * Tests for the ClientRepliesService class
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient, type BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("ClientRepliesService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
  });

  describe("list", () => {
    it("should list all replies for a client recording", async () => {
      const mockReplies = [
        {
          id: 10,
          status: "active",
          visible_to_clients: true,
          created_at: "2024-01-02T00:00:00Z",
          updated_at: "2024-01-02T00:00:00Z",
          title: "",
          inherits_status: true,
          type: "Client::Reply",
          url: "https://3.basecampapi.com/12345/client/replies/10.json",
          app_url: "https://3.basecamp.com/12345/client/replies/10",
          bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
          content: "<p>Thanks for the update!</p>",
          bucket: { id: 1, name: "Test Project", type: "Project" },
          creator: { id: 888, name: "Client User" },
        },
        {
          id: 11,
          status: "active",
          visible_to_clients: true,
          created_at: "2024-01-03T00:00:00Z",
          updated_at: "2024-01-03T00:00:00Z",
          title: "",
          inherits_status: true,
          type: "Client::Reply",
          url: "https://3.basecampapi.com/12345/client/replies/11.json",
          app_url: "https://3.basecamp.com/12345/client/replies/11",
          bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
          content: "<p>Looking forward to the next milestone.</p>",
          bucket: { id: 1, name: "Test Project", type: "Project" },
          creator: { id: 888, name: "Client User" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/client/recordings/100/replies.json`, () => {
          return HttpResponse.json(mockReplies);
        })
      );

      const replies = await client.clientReplies.list(100);

      expect(replies).toHaveLength(2);
      expect(replies[0].content).toBe("<p>Thanks for the update!</p>");
      expect(replies[0].creator?.name).toBe("Client User");
      expect(replies[1].content).toBe("<p>Looking forward to the next milestone.</p>");
    });

    it("should return empty array when no replies exist", async () => {
      server.use(
        http.get(`${BASE_URL}/client/recordings/100/replies.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const replies = await client.clientReplies.list(100);
      expect(replies).toHaveLength(0);
    });
  });

  describe("get", () => {
    it("should get a client reply by ID", async () => {
      const mockReply = {
        id: 10,
        status: "active",
        visible_to_clients: true,
        created_at: "2024-01-02T00:00:00Z",
        updated_at: "2024-01-02T00:00:00Z",
        title: "",
        inherits_status: true,
        type: "Client::Reply",
        url: "https://3.basecampapi.com/12345/client/replies/10.json",
        app_url: "https://3.basecamp.com/12345/client/replies/10",
        bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
        content: "<p>Thanks for the update! This looks great.</p>",
        parent: {
          id: 100,
          title: "Project Kickoff",
          type: "Client::Correspondence",
          url: "https://3.basecampapi.com/12345/client/correspondences/100.json",
          app_url: "https://3.basecamp.com/12345/client/correspondences/100",
        },
        bucket: { id: 1, name: "Test Project", type: "Project" },
        creator: { id: 888, name: "Client User", email_address: "client@example.com" },
      };

      server.use(
        http.get(`${BASE_URL}/client/recordings/100/replies/10`, () => {
          return HttpResponse.json(mockReply);
        })
      );

      const reply = await client.clientReplies.get(100, 10);

      expect(reply.id).toBe(10);
      expect(reply.content).toBe("<p>Thanks for the update! This looks great.</p>");
      expect(reply.parent?.title).toBe("Project Kickoff");
      expect(reply.creator?.name).toBe("Client User");
    });

    it("should throw not_found for non-existent reply", async () => {
      server.use(
        http.get(`${BASE_URL}/client/recordings/100/replies/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      try {
        await client.clientReplies.get(100, 999);
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });
});
