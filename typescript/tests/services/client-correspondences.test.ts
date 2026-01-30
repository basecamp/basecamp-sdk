/**
 * Tests for the ClientCorrespondencesService class
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient, type BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("ClientCorrespondencesService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
  });

  describe("list", () => {
    it("should list all client correspondences in a project", async () => {
      const mockCorrespondences = [
        {
          id: 1,
          status: "active",
          visible_to_clients: true,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
          title: "Project Kickoff",
          inherits_status: true,
          type: "Client::Correspondence",
          url: "https://3.basecampapi.com/12345/buckets/1/client/correspondences/1.json",
          app_url: "https://3.basecamp.com/12345/buckets/1/client/correspondences/1",
          bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
          subscription_url: "https://3.basecampapi.com/12345/buckets/1/recordings/1/subscription.json",
          content: "<p>Welcome to the project!</p>",
          subject: "Project Kickoff",
          replies_count: 3,
          replies_url: "https://3.basecampapi.com/12345/buckets/1/client/recordings/1/replies.json",
          bucket: { id: 1, name: "Test Project", type: "Project" },
          creator: { id: 999, name: "Test User" },
        },
        {
          id: 2,
          status: "active",
          visible_to_clients: true,
          created_at: "2024-01-05T00:00:00Z",
          updated_at: "2024-01-05T00:00:00Z",
          title: "Weekly Update",
          inherits_status: true,
          type: "Client::Correspondence",
          url: "https://3.basecampapi.com/12345/buckets/1/client/correspondences/2.json",
          app_url: "https://3.basecamp.com/12345/buckets/1/client/correspondences/2",
          bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
          subscription_url: "https://3.basecampapi.com/12345/buckets/1/recordings/2/subscription.json",
          content: "<p>Here's the weekly progress update</p>",
          subject: "Weekly Update",
          replies_count: 1,
          replies_url: "https://3.basecampapi.com/12345/buckets/1/client/recordings/2/replies.json",
          bucket: { id: 1, name: "Test Project", type: "Project" },
          creator: { id: 999, name: "Test User" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/1/client/correspondences.json`, () => {
          return HttpResponse.json(mockCorrespondences);
        })
      );

      const correspondences = await client.clientCorrespondences.list(1);

      expect(correspondences).toHaveLength(2);
      expect(correspondences[0].subject).toBe("Project Kickoff");
      expect(correspondences[0].replies_count).toBe(3);
      expect(correspondences[1].subject).toBe("Weekly Update");
    });

    it("should return empty array when no correspondences exist", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/1/client/correspondences.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const correspondences = await client.clientCorrespondences.list(1);
      expect(correspondences).toHaveLength(0);
    });
  });

  describe("get", () => {
    it("should get a client correspondence by ID", async () => {
      const mockCorrespondence = {
        id: 1,
        status: "active",
        visible_to_clients: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
        title: "Project Kickoff",
        inherits_status: true,
        type: "Client::Correspondence",
        url: "https://3.basecampapi.com/12345/buckets/1/client/correspondences/1.json",
        app_url: "https://3.basecamp.com/12345/buckets/1/client/correspondences/1",
        bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
        subscription_url: "https://3.basecampapi.com/12345/buckets/1/recordings/1/subscription.json",
        content: "<p>Welcome to the project! We're excited to get started.</p>",
        subject: "Project Kickoff",
        replies_count: 3,
        replies_url: "https://3.basecampapi.com/12345/buckets/1/client/recordings/1/replies.json",
        bucket: { id: 1, name: "Test Project", type: "Project" },
        creator: { id: 999, name: "Test User", email_address: "test@example.com" },
      };

      server.use(
        http.get(`${BASE_URL}/buckets/1/client/correspondences/1`, () => {
          return HttpResponse.json(mockCorrespondence);
        })
      );

      const correspondence = await client.clientCorrespondences.get(1, 1);

      expect(correspondence.id).toBe(1);
      expect(correspondence.subject).toBe("Project Kickoff");
      expect(correspondence.content).toContain("Welcome to the project!");
      expect(correspondence.creator?.name).toBe("Test User");
    });

    it("should throw not_found for non-existent correspondence", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/1/client/correspondences/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      try {
        await client.clientCorrespondences.get(1, 999);
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });
});
