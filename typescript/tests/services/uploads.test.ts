/**
 * Tests for the Uploads service
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { UploadsService } from "../../src/services/uploads.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("UploadsService", () => {
  let service: UploadsService;

  beforeEach(() => {
    vi.clearAllMocks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
    service = client.uploads;
  });

  describe("get", () => {
    it("should return an upload by ID", async () => {
      const upload = {
        id: 7001,
        title: "report.pdf",
        filename: "report.pdf",
        content_type: "application/pdf",
        byte_size: 1024000,
        download_url: "https://example.com/download/report.pdf",
        status: "active",
      };

      server.use(
        http.get(`${BASE_URL}/buckets/123/uploads/7001`, () => {
          return HttpResponse.json(upload);
        })
      );

      const result = await service.get(123, 7001);

      expect(result.id).toBe(7001);
      expect(result.filename).toBe("report.pdf");
      expect(result.byte_size).toBe(1024000);
    });

    it("should throw not_found error for 404 response", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/123/uploads/9999`, () => {
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
    it("should return uploads in a vault", async () => {
      const uploads = [
        { id: 7001, filename: "file1.pdf", status: "active" },
        { id: 7002, filename: "file2.xlsx", status: "active" },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/123/vaults/1001/uploads.json`, () => {
          return HttpResponse.json(uploads);
        })
      );

      const result = await service.list(123, 1001);

      expect(result).toHaveLength(2);
      expect(result[0].filename).toBe("file1.pdf");
      expect(result[1].filename).toBe("file2.xlsx");
    });

    it("should return empty array when no uploads", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/123/vaults/1001/uploads.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const result = await service.list(123, 1001);

      expect(result).toEqual([]);
    });
  });

  describe("create", () => {
    it("should create a new upload", async () => {
      const newUpload = {
        id: 8001,
        title: "presentation.pptx",
        filename: "presentation.pptx",
        description: "Q4 Presentation",
        status: "active",
      };

      server.use(
        http.post(`${BASE_URL}/buckets/123/vaults/1001/uploads.json`, () => {
          return HttpResponse.json(newUpload);
        })
      );

      const result = await service.create(123, 1001, {
        attachableSgid: "BAh7CEkiCGdpZAY6BkVUSSI...",
        description: "Q4 Presentation",
      });

      expect(result.id).toBe(8001);
      expect(result.description).toBe("Q4 Presentation");
    });

    it("should send all fields in request body", async () => {
      let capturedBody: { attachable_sgid?: string; description?: string; base_name?: string } | null = null;

      server.use(
        http.post(`${BASE_URL}/buckets/123/vaults/1001/uploads.json`, async ({ request }) => {
          capturedBody = (await request.json()) as { attachable_sgid?: string; description?: string; base_name?: string };
          return HttpResponse.json({ id: 1, title: "Test" });
        })
      );

      await service.create(123, 1001, {
        attachableSgid: "test-sgid",
        description: "<p>Description</p>",
        baseName: "custom-name",
      });

      expect(capturedBody?.attachable_sgid).toBe("test-sgid");
      expect(capturedBody?.description).toBe("<p>Description</p>");
      expect(capturedBody?.base_name).toBe("custom-name");
    });

    it("should throw validation error when attachableSgid is missing", async () => {
      await expect(
        service.create(123, 1001, { attachableSgid: "" })
      ).rejects.toThrow(BasecampError);

      try {
        await service.create(123, 1001, { attachableSgid: "" });
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("attachable_sgid");
      }
    });
  });

  describe("update", () => {
    it("should update an existing upload", async () => {
      const updatedUpload = {
        id: 7001,
        title: "new-name.pdf",
        description: "Updated description",
        status: "active",
      };

      server.use(
        http.put(`${BASE_URL}/buckets/123/uploads/7001`, () => {
          return HttpResponse.json(updatedUpload);
        })
      );

      const result = await service.update(123, 7001, {
        description: "Updated description",
        baseName: "new-name",
      });

      expect(result.description).toBe("Updated description");
    });

    it("should send updated fields in request body", async () => {
      let capturedBody: { description?: string; base_name?: string } | null = null;

      server.use(
        http.put(`${BASE_URL}/buckets/123/uploads/7001`, async ({ request }) => {
          capturedBody = (await request.json()) as { description?: string; base_name?: string };
          return HttpResponse.json({ id: 7001, title: "Test" });
        })
      );

      await service.update(123, 7001, {
        description: "New description",
        baseName: "renamed-file",
      });

      expect(capturedBody?.description).toBe("New description");
      expect(capturedBody?.base_name).toBe("renamed-file");
    });
  });

  describe("listVersions", () => {
    it("should return upload versions", async () => {
      const uploads = [
        { id: 7001, filename: "file_v3.pdf", created_at: "2024-03-15T10:00:00Z" },
        { id: 7001, filename: "file_v2.pdf", created_at: "2024-02-10T10:00:00Z" },
        { id: 7001, filename: "file_v1.pdf", created_at: "2024-01-05T10:00:00Z" },
      ];

      server.use(
        http.get(`${BASE_URL}/buckets/123/uploads/7001/versions.json`, () => {
          return HttpResponse.json(uploads);
        })
      );

      const result = await service.listVersions(123, 7001);

      expect(result).toHaveLength(3);
    });

    it("should return empty array when no versions", async () => {
      server.use(
        http.get(`${BASE_URL}/buckets/123/uploads/7001/versions.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const result = await service.listVersions(123, 7001);

      expect(result).toEqual([]);
    });
  });

  describe("trash", () => {
    it("should move an upload to trash", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/7001/status/trashed.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(service.trash(123, 7001)).resolves.toBeUndefined();
    });

    it("should throw error for non-existent upload", async () => {
      server.use(
        http.put(`${BASE_URL}/buckets/123/recordings/9999/status/trashed.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(service.trash(123, 9999)).rejects.toThrow(BasecampError);
    });
  });
});
