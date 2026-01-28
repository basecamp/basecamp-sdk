/**
 * Tests for the Documents service
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { DocumentsService } from "../../src/services/documents.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("DocumentsService", () => {
  let service: DocumentsService;

  beforeEach(() => {
    vi.clearAllMocks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
    service = client.documents;
  });

  describe("get", () => {
    it("should return a document by ID", async () => {
      const document = {
        id: 5001,
        title: "Meeting Notes",
        content: "<p>Notes from the meeting...</p>",
        status: "active",
        comments_count: 3,
      };

      server.use(
        http.get(`${BASE_URL}/buckets/123/documents/5001`, () => {
          return HttpResponse.json({ document });
        })
      );

      const result = await service.get(123, 5001);

      expect(result.id).toBe(5001);
      expect(result.title).toBe("Meeting Notes");
      expect(result.content).toContain("Notes from the meeting");
    });

    it("should throw not_found error for 404 response", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/123/documents/9999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(service.get(123, 9999)).rejects.toThrow(BasecampError);

      try {
        await service.get(123, 9999);
      } catch (err) {
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });

  describe("list", () => {
    it("should return documents in a vault", async () => {
      const documents = [
        { id: 5001, title: "Document 1", status: "active" },
        { id: 5002, title: "Document 2", status: "active" },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/123/vaults/1001/documents.json`, () => {
          return HttpResponse.json({ documents });
        })
      );

      const result = await service.list(123, 1001);

      expect(result).toHaveLength(2);
      expect(result[0].title).toBe("Document 1");
      expect(result[1].title).toBe("Document 2");
    });

    it("should return empty array when no documents", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/123/vaults/1001/documents.json`, () => {
          return HttpResponse.json({ documents: [] });
        })
      );

      const result = await service.list(123, 1001);

      expect(result).toEqual([]);
    });
  });

  describe("create", () => {
    it("should create a new document", async () => {
      const newDocument = {
        id: 6001,
        title: "New Document",
        content: "<p>Content here</p>",
        status: "active",
      };

      server.use(
        http.post(`${BASE_URL}/buckets/123/vaults/1001/documents.json`, () => {
          return HttpResponse.json({ document: newDocument });
        })
      );

      const result = await service.create(123, 1001, {
        title: "New Document",
        content: "<p>Content here</p>",
      });

      expect(result.id).toBe(6001);
      expect(result.title).toBe("New Document");
    });

    it("should send all fields in request body", async () => {
      let capturedBody: { title?: string; content?: string; status?: string } | null = null;

      server.use(
        http.post(`${BASE_URL}/buckets/123/vaults/1001/documents.json`, async ({ request }) => {
          capturedBody = (await request.json()) as { title?: string; content?: string; status?: string };
          return HttpResponse.json({ document: { id: 1, title: "Test" } });
        })
      );

      await service.create(123, 1001, {
        title: "Test Doc",
        content: "<h1>Hello</h1>",
        status: "drafted",
      });

      expect(capturedBody?.title).toBe("Test Doc");
      expect(capturedBody?.content).toBe("<h1>Hello</h1>");
      expect(capturedBody?.status).toBe("drafted");
    });

    it("should throw validation error when title is missing", async () => {
      await expect(
        service.create(123, 1001, { title: "" })
      ).rejects.toThrow(BasecampError);

      try {
        await service.create(123, 1001, { title: "" });
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("title");
      }
    });
  });

  describe("update", () => {
    it("should update an existing document", async () => {
      const updatedDocument = {
        id: 5001,
        title: "Updated Title",
        content: "<p>Updated content</p>",
        status: "active",
      };

      server.use(
        http.put(`${BASE_URL}/buckets/123/documents/5001`, () => {
          return HttpResponse.json({ document: updatedDocument });
        })
      );

      const result = await service.update(123, 5001, {
        title: "Updated Title",
        content: "<p>Updated content</p>",
      });

      expect(result.title).toBe("Updated Title");
      expect(result.content).toContain("Updated content");
    });

    it("should send updated fields in request body", async () => {
      let capturedBody: { title?: string; content?: string } | null = null;

      server.use(
        http.put(`${BASE_URL}/buckets/123/documents/5001`, async ({ request }) => {
          capturedBody = (await request.json()) as { title?: string; content?: string };
          return HttpResponse.json({ document: { id: 5001, title: "Updated" } });
        })
      );

      await service.update(123, 5001, { title: "New Title" });

      expect(capturedBody?.title).toBe("New Title");
    });
  });

  describe("trash", () => {
    it("should move a document to trash", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/5001/status/trashed.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(service.trash(123, 5001)).resolves.toBeUndefined();
    });

    it("should throw error for non-existent document", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/9999/status/trashed.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(service.trash(123, 9999)).rejects.toThrow(BasecampError);
    });
  });
});
