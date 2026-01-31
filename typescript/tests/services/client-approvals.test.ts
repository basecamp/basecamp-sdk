/**
 * Tests for the ClientApprovalsService class
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient, type BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("ClientApprovalsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
  });

  describe("list", () => {
    it("should list all client approvals in a project", async () => {
      const mockApprovals = [
        {
          id: 1,
          status: "active",
          visible_to_clients: true,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
          title: "Design Review",
          inherits_status: true,
          type: "Client::Approval",
          url: "https://3.basecampapi.com/12345/buckets/1/client/approvals/1.json",
          app_url: "https://3.basecamp.com/12345/buckets/1/client/approvals/1",
          bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
          subscription_url: "https://3.basecampapi.com/12345/buckets/1/recordings/1/subscription.json",
          content: "<p>Please review the attached designs</p>",
          subject: "Design Review",
          due_on: "2024-01-15",
          replies_count: 2,
          replies_url: "https://3.basecampapi.com/12345/buckets/1/client/recordings/1/replies.json",
          approval_status: "pending",
          bucket: { id: 1, name: "Test Project", type: "Project" },
          creator: { id: 999, name: "Test User" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/1/client/approvals.json`, () => {
          return HttpResponse.json(mockApprovals);
        })
      );

      const approvals = await client.clientApprovals.list(1);

      expect(approvals).toHaveLength(1);
      expect(approvals[0].subject).toBe("Design Review");
      expect(approvals[0].approval_status).toBe("pending");
      expect(approvals[0].due_on).toBe("2024-01-15");
    });

    it("should return empty array when no approvals exist", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/1/client/approvals.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const approvals = await client.clientApprovals.list(1);
      expect(approvals).toHaveLength(0);
    });
  });

  describe("get", () => {
    it("should get a client approval by ID", async () => {
      const mockApproval = {
        id: 1,
        status: "active",
        visible_to_clients: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
        title: "Design Review",
        inherits_status: true,
        type: "Client::Approval",
        url: "https://3.basecampapi.com/12345/buckets/1/client/approvals/1.json",
        app_url: "https://3.basecamp.com/12345/buckets/1/client/approvals/1",
        bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
        subscription_url: "https://3.basecampapi.com/12345/buckets/1/recordings/1/subscription.json",
        content: "<p>Please review the attached designs</p>",
        subject: "Design Review",
        due_on: "2024-01-15",
        replies_count: 2,
        replies_url: "https://3.basecampapi.com/12345/buckets/1/client/recordings/1/replies.json",
        approval_status: "approved",
        bucket: { id: 1, name: "Test Project", type: "Project" },
        creator: { id: 999, name: "Test User" },
        approver: { id: 888, name: "Client User" },
        responses: [
          {
            id: 10,
            status: "active",
            visible_to_clients: true,
            created_at: "2024-01-02T00:00:00Z",
            updated_at: "2024-01-02T00:00:00Z",
            title: "",
            inherits_status: true,
            type: "Client::Approval::Response",
            app_url: "https://3.basecamp.com/12345/buckets/1/client/approvals/1/responses/10",
            bookmark_url: "https://3.basecampapi.com/12345/my/bookmarks/BAh7.json",
            content: "<p>Looks great!</p>",
            approved: true,
            creator: { id: 888, name: "Client User" },
          },
        ],
      };

      server.use(
        http.get(`${BASE_URL}/buckets/1/client/approvals/1`, () => {
          return HttpResponse.json(mockApproval);
        })
      );

      const approval = await client.clientApprovals.get(1, 1);

      expect(approval.id).toBe(1);
      expect(approval.subject).toBe("Design Review");
      expect(approval.approval_status).toBe("approved");
      expect(approval.approver?.name).toBe("Client User");
      expect(approval.responses).toHaveLength(1);
      expect(approval.responses![0].approved).toBe(true);
    });

    it("should throw not_found for non-existent approval", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/1/client/approvals/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      try {
        await client.clientApprovals.get(1, 999);
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });
});
