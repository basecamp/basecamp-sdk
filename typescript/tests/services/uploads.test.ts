/**
 * Tests for the Uploads service (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - No domain-specific trash() method (use recordings.trash() instead)
 * - No client-side validation (API validates)
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

// Infer the service type from client.uploads so download() is visible on the
// type (the subclass lives in src/services/uploads-extensions.ts).
type UploadsServiceT = ReturnType<typeof createBasecampClient>["uploads"];

describe("UploadsService", () => {
  let service: UploadsServiceT;

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
        http.get(`${BASE_URL}/uploads/7001`, () => {
          return HttpResponse.json(upload);
        })
      );

      const result = await service.get(7001);

      expect(result.id).toBe(7001);
      expect(result.filename).toBe("report.pdf");
      expect(result.byte_size).toBe(1024000);
    });

    it("should throw not_found error for 404 response", async () => {
      server.use(
        http.get(`${BASE_URL}/uploads/9999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(service.get(9999)).rejects.toThrow(BasecampError);

      try {
        await service.get(9999);
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
        http.get(`${BASE_URL}/vaults/1001/uploads.json`, () => {
          return HttpResponse.json(uploads);
        })
      );

      const result = await service.list(1001);

      expect(result).toHaveLength(2);
      expect(result[0].filename).toBe("file1.pdf");
      expect(result[1].filename).toBe("file2.xlsx");
    });

    it("should return empty array when no uploads", async () => {
      server.use(
        http.get(`${BASE_URL}/vaults/1001/uploads.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const result = await service.list(1001);

      expect(result).toHaveLength(0);
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
        http.post(`${BASE_URL}/vaults/1001/uploads.json`, () => {
          return HttpResponse.json(newUpload);
        })
      );

      const result = await service.create(1001, {
        attachableSgid: "BAh7CEkiCGdpZAY6BkVUSSI...",
        description: "Q4 Presentation",
      });

      expect(result.id).toBe(8001);
      expect(result.description).toBe("Q4 Presentation");
    });

    it("should pass subscriptions in request body", async () => {
      server.use(
        http.post(`${BASE_URL}/vaults/1001/uploads.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.subscriptions).toEqual([111, 222]);
          return HttpResponse.json({ id: 8002, title: "Test" });
        })
      );

      const result = await service.create(1001, {
        attachableSgid: "BAh7CEkiCGdpZAY6BkVUSSI...",
        subscriptions: [111, 222],
      });
      expect(result.id).toBe(8002);
    });

    it("should send all fields in request body", async () => {
      let capturedBody: { attachable_sgid?: string; description?: string; base_name?: string } | null = null;

      server.use(
        http.post(`${BASE_URL}/vaults/1001/uploads.json`, async ({ request }) => {
          capturedBody = (await request.json()) as { attachable_sgid?: string; description?: string; base_name?: string };
          return HttpResponse.json({ id: 1, title: "Test" });
        })
      );

      await service.create(1001, {
        attachableSgid: "test-sgid",
        description: "<p>Description</p>",
        baseName: "custom-name",
      });

      expect(capturedBody?.attachable_sgid).toBe("test-sgid");
      expect(capturedBody?.description).toBe("<p>Description</p>");
      expect(capturedBody?.base_name).toBe("custom-name");
    });

    // Note: Client-side validation removed - generated services let API validate
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
        http.put(`${BASE_URL}/uploads/7001`, () => {
          return HttpResponse.json(updatedUpload);
        })
      );

      const result = await service.update(7001, {
        description: "Updated description",
        baseName: "new-name",
      });

      expect(result.description).toBe("Updated description");
    });

    it("should send updated fields in request body", async () => {
      let capturedBody: { description?: string; base_name?: string } | null = null;

      server.use(
        http.put(`${BASE_URL}/uploads/7001`, async ({ request }) => {
          capturedBody = (await request.json()) as { description?: string; base_name?: string };
          return HttpResponse.json({ id: 7001, title: "Test" });
        })
      );

      await service.update(7001, {
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
        http.get(`${BASE_URL}/uploads/7001/versions.json`, () => {
          return HttpResponse.json(uploads);
        })
      );

      const result = await service.listVersions(7001);

      expect(result).toHaveLength(3);
    });

    it("should return empty array when no versions", async () => {
      server.use(
        http.get(`${BASE_URL}/uploads/7001/versions.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const result = await service.listVersions(7001);

      expect(result).toHaveLength(0);
    });
  });

  // Note: trash() is on RecordingsService, not UploadsService (spec-conformant)
  // Use client.recordings.trash(uploadId) instead

  describe("download", () => {
    const API_ORIGIN = "https://3.basecampapi.com";
    const SIGNED_URL = "https://signed.example/bucket/xyz?sig=abc";

    it("delegates through the downloadURL primitive", async () => {
      const authorizationHeaders: Array<string | null> = [];

      server.use(
        // Metadata fetch
        http.get(`${BASE_URL}/uploads/1069479400`, ({ request }) => {
          authorizationHeaders.push(request.headers.get("authorization"));
          return HttpResponse.json({
            id: 1069479400,
            filename: "logo.png",
            download_url: "https://storage.3.basecamp.com/12345/blobs/abc/download/logo.png",
          });
        }),
        // Hop 1: origin-rewritten to API_ORIGIN
        http.get(`${API_ORIGIN}/12345/blobs/abc/download/logo.png`, ({ request }) => {
          authorizationHeaders.push(request.headers.get("authorization"));
          return new HttpResponse(null, {
            status: 302,
            headers: { Location: SIGNED_URL },
          });
        }),
        // Hop 2: signed URL (no auth)
        http.get(SIGNED_URL, ({ request }) => {
          authorizationHeaders.push(request.headers.get("authorization"));
          return new HttpResponse("pixels", {
            status: 200,
            headers: { "Content-Type": "image/png", "Content-Length": "6" },
          });
        }),
      );

      const result = await service.download(1069479400);

      expect(result.contentType).toBe("image/png");
      expect(result.contentLength).toBe(6);
      // filename from upload metadata wins over URL-derived
      expect(result.filename).toBe("logo.png");

      const bodyText = await new Response(result.body).text();
      expect(bodyText).toBe("pixels");

      // Metadata request + auth'd download hop must carry bearer; signed hop must not
      expect(authorizationHeaders).toHaveLength(3);
      expect(authorizationHeaders[0]).toBe("Bearer test-token");
      expect(authorizationHeaders[1]).toBe("Bearer test-token");
      expect(authorizationHeaders[2]).toBeNull();
    });

    it("throws usage error when upload has no download_url", async () => {
      let downloadHopCalled = false;

      server.use(
        http.get(`${BASE_URL}/uploads/1069479400`, () => {
          return HttpResponse.json({
            id: 1069479400,
            filename: "logo.png",
            download_url: null,
          });
        }),
        // No download hop should fire — this handler would record it if so
        http.get(`${API_ORIGIN}/12345/blobs/*`, () => {
          downloadHopCalled = true;
          return new HttpResponse(null, { status: 500 });
        }),
      );

      const error = await service.download(1069479400).catch((err) => err);

      expect(error).toBeInstanceOf(BasecampError);
      const e = error as BasecampError;
      expect(e.code).toBe("usage");
      expect(e.message).toContain("1069479400");
      expect(e.message).toContain("download_url");
      expect(downloadHopCalled).toBe(false);
    });
  });
});

