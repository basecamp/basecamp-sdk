import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

function sampleDraft(id: number, overrides: Record<string, unknown> = {}) {
  return {
    id,
    app_url: `https://3.basecamp.com/12345/buckets/2/documents/${id}`,
    title: "Quarterly plan",
    type: "document",
    bucket: { id: 2, name: "The Leto Laptop", app_url: "https://3.basecamp.com/12345/projects/2" },
    parent: { id: 500, title: "Docs & Files", app_url: "https://3.basecamp.com/12345/buckets/2/vaults/500" },
    excerpt: "First 300 chars of the body",
    created_at: "2026-07-01T00:00:00Z",
    updated_at: "2026-07-02T00:00:00Z",
    scheduled_posting_at: null,
    ...overrides,
  };
}

describe("DraftsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("listMyDrafts", () => {
    it("lists draft envelopes, including null parent and scheduled_posting_at", async () => {
      server.use(
        http.get(`${BASE_URL}/my/drafts.json`, () => {
          return HttpResponse.json([
            sampleDraft(1),
            sampleDraft(2, { parent: null, scheduled_posting_at: "2026-08-01T09:00:00Z", type: "message" }),
          ]);
        })
      );

      const result = await client.drafts.listMyDrafts();
      expect(result).toHaveLength(2);
      expect(result[0].parent?.title).toBe("Docs & Files");
      expect(result[0].scheduled_posting_at).toBeNull();
      // Bucket-rooted draft: parent is present-but-null, not absent.
      expect(result[1].parent).toBeNull();
      expect(result[1].scheduled_posting_at).toBe("2026-08-01T09:00:00Z");
    });

    it("surfaces 401 as BasecampError", async () => {
      server.use(
        http.get(`${BASE_URL}/my/drafts.json`, () => {
          return HttpResponse.json({ error: "Unauthorized" }, { status: 401 });
        })
      );

      const error = await client.drafts.listMyDrafts().catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(401);
    });
  });
});
