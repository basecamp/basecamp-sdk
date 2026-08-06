/**
 * Tests for the Uploads service (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - No domain-specific trash() method (use recordings.trash() instead)
 * - Client-side check: create() rejects a missing attachableSgid; the API validates the rest
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";
// Sourced from the shared, coverage-guarded fixture (spec/fixtures/manifest.yaml)
import versionsFixture from "../../../spec/fixtures/uploads/versions.json";

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
        download_url:
          "https://3.basecampapi.com/12345/blobs/abcd/download/report.pdf",
        status: "active",
        description_attachments: [],
      };

      server.use(
        http.get(`${BASE_URL}/uploads/7001`, () => {
          return HttpResponse.json(upload);
        }),
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
        }),
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
        {
          id: 7001,
          filename: "file1.pdf",
          status: "active",
          description_attachments: [],
        },
        {
          id: 7002,
          filename: "file2.xlsx",
          status: "active",
          description_attachments: [],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/vaults/1001/uploads.json`, () => {
          return HttpResponse.json(uploads);
        }),
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
        }),
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
        description_attachments: [],
        status: "active",
      };

      server.use(
        http.post(`${BASE_URL}/vaults/1001/uploads.json`, () => {
          return HttpResponse.json(newUpload);
        }),
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
        http.post(
          `${BASE_URL}/vaults/1001/uploads.json`,
          async ({ request }) => {
            const body = (await request.json()) as Record<string, unknown>;
            expect(body.subscriptions).toEqual([111, 222]);
            return HttpResponse.json({ id: 8002, title: "Test" });
          },
        ),
      );

      const result = await service.create(1001, {
        attachableSgid: "BAh7CEkiCGdpZAY6BkVUSSI...",
        subscriptions: [111, 222],
      });
      expect(result.id).toBe(8002);
    });

    it("should send all fields in request body", async () => {
      let capturedBody: {
        attachable_sgid?: string;
        description?: string;
        base_name?: string;
      } | null = null;

      server.use(
        http.post(
          `${BASE_URL}/vaults/1001/uploads.json`,
          async ({ request }) => {
            capturedBody = (await request.json()) as {
              attachable_sgid?: string;
              description?: string;
              base_name?: string;
            };
            return HttpResponse.json({ id: 1, title: "Test" });
          },
        ),
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

    // Client-side validation short-circuits before any HTTP call. No MSW handler
    // is registered here, so a leaked request fails via onUnhandledRequest: "error".
    it("rejects a missing attachableSgid", async () => {
      await expect(
        service.create(1001, { attachableSgid: "" })
      ).rejects.toMatchObject({ code: "validation", message: "Attachable sgid is required" });
    });
  });

  describe("update", () => {
    it("should update an existing upload", async () => {
      const updatedUpload = {
        id: 7001,
        title: "new-name.pdf",
        description: "Updated description",
        description_attachments: [],
        status: "active",
      };

      server.use(
        http.put(`${BASE_URL}/uploads/7001`, () => {
          return HttpResponse.json(updatedUpload);
        }),
      );

      const result = await service.update(7001, {
        description: "Updated description",
        baseName: "new-name",
      });

      expect(result.description).toBe("Updated description");
    });

    it("should send updated fields in request body", async () => {
      let capturedBody: { description?: string; base_name?: string } | null =
        null;

      server.use(
        http.put(`${BASE_URL}/uploads/7001`, async ({ request }) => {
          capturedBody = (await request.json()) as {
            description?: string;
            base_name?: string;
          };
          return HttpResponse.json({
            id: 7001,
            title: "Test",
            description_attachments: [],
          });
        }),
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
    // The endpoint returns EVENTS, not Uploads — the retype that closes #649.
    it("should return upload versions carrying the file each one recorded", async () => {
      server.use(
        http.get(`${BASE_URL}/uploads/7001/versions.json`, () => {
          return HttpResponse.json(versionsFixture);
        }),
      );

      const result = await service.listVersions(7001);

      expect(result).toHaveLength(3);
      expect(result[0].action).toBe("blob_changed");
      expect(result[0].upload?.filename).toBe("company-logo.png");
      expect(result[0].upload?.content_type).toBe("image/png");
      expect(result[0].upload?.byte_size).toBe(184829);
    });

    it("marks exactly one version current", async () => {
      server.use(
        http.get(`${BASE_URL}/uploads/7001/versions.json`, () => {
          return HttpResponse.json(versionsFixture);
        }),
      );

      const result = await service.listVersions(7001);

      expect(result.filter((v) => v.upload?.current)).toHaveLength(1);
      expect(result[0].upload?.current).toBe(true);
    });

    // The per-version URL serves THAT version's bytes; the upload's own always
    // serves the latest, which is the whole point of the feature.
    it("gives each version its own download URL", async () => {
      server.use(
        http.get(`${BASE_URL}/uploads/7001/versions.json`, () => {
          return HttpResponse.json(versionsFixture);
        }),
      );

      const result = await service.listVersions(7001);

      expect(result[0].upload?.download_url).not.toBe(result[1].upload?.download_url);
      expect(result[0].upload?.download_url).toContain("/versions/1069479501/");
    });

    it("tolerates a version whose recordable no longer resolves", async () => {
      server.use(
        http.get(`${BASE_URL}/uploads/7001/versions.json`, () => {
          return HttpResponse.json(versionsFixture);
        }),
      );

      const result = await service.listVersions(7001);

      expect(result[2].action).toBe("created");
      expect(result[2].upload).toBeUndefined();
    });

    it("should return empty array when no versions", async () => {
      server.use(
        http.get(`${BASE_URL}/uploads/7001/versions.json`, () => {
          return HttpResponse.json([]);
        }),
      );

      const result = await service.listVersions(7001);

      expect(result).toHaveLength(0);
    });
  });

  describe("createVersion", () => {
    it("posts the attachable sgid and returns the updated upload", async () => {
      let capturedBody: Record<string, unknown> | undefined;

      server.use(
        http.post(`${BASE_URL}/uploads/7001/versions.json`, async ({ request }) => {
          capturedBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(
            { id: 7001, filename: "company-logo.png", description_attachments: [] },
            { status: 201 },
          );
        }),
      );

      const result = await service.createVersion(7001, { attachableSgid: "sgid-abc" });

      expect(result.id).toBe(7001);
      expect(capturedBody?.attachable_sgid).toBe("sgid-abc");
    });

    // Presence-aware: omitted carries the previous description forward, ""
    // clears. Both spellings have to be distinguishable on the wire.
    it("omits description when it is not addressed", async () => {
      let capturedBody: Record<string, unknown> | undefined;

      server.use(
        http.post(`${BASE_URL}/uploads/7001/versions.json`, async ({ request }) => {
          capturedBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ id: 7001, description_attachments: [] }, { status: 201 });
        }),
      );

      await service.createVersion(7001, { attachableSgid: "sgid-abc" });

      expect(capturedBody).not.toHaveProperty("description");
      expect(capturedBody).not.toHaveProperty("base_name");
    });

    it("sends an explicit empty description to clear it", async () => {
      let capturedBody: Record<string, unknown> | undefined;

      server.use(
        http.post(`${BASE_URL}/uploads/7001/versions.json`, async ({ request }) => {
          capturedBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ id: 7001, description_attachments: [] }, { status: 201 });
        }),
      );

      await service.createVersion(7001, { attachableSgid: "sgid-abc", description: "" });

      expect(capturedBody).toHaveProperty("description");
      expect(capturedBody?.description).toBe("");
    });

    it("passes notify and subscriptions through", async () => {
      let capturedBody: Record<string, unknown> | undefined;

      server.use(
        http.post(`${BASE_URL}/uploads/7001/versions.json`, async ({ request }) => {
          capturedBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ id: 7001, description_attachments: [] }, { status: 201 });
        }),
      );

      await service.createVersion(7001, {
        attachableSgid: "sgid-abc",
        notify: "custom",
        subscriptions: [1049715915, 1049715916],
      });

      expect(capturedBody?.notify).toBe("custom");
      expect(capturedBody?.subscriptions).toEqual([1049715915, 1049715916]);
    });

    it("rejects a missing attachable sgid before hitting the wire", async () => {
      await expect(service.createVersion(7001, { attachableSgid: "" })).rejects.toThrow(
        BasecampError,
      );
    });

    // A replacement copies bytes into a new blob and keeps the old one, so it
    // always grows recorded storage. 507 is a limit, never a transient failure.
    it("reports a storage limit as limit_exceeded and does not retry", async () => {
      let requestCount = 0;

      server.use(
        http.post(`${BASE_URL}/uploads/7001/versions.json`, () => {
          requestCount += 1;
          return HttpResponse.json(
            { error: "The storage limit for this account has been reached." },
            { status: 507 },
          );
        }),
      );

      const error = await service
        .createVersion(7001, { attachableSgid: "sgid-abc" })
        .catch((e) => e as BasecampError);

      expect(error).toBeInstanceOf(BasecampError);
      expect(error.code).toBe("limit_exceeded");
      expect(error.retryable).toBe(false);
      expect(error.message).toContain("storage limit");
      expect(requestCount).toBe(1);
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
            download_url:
              "https://storage.3.basecamp.com/12345/blobs/abc/download/logo.png",
            description_attachments: [],
          });
        }),
        // Hop 1: origin-rewritten to API_ORIGIN
        http.get(
          `${API_ORIGIN}/12345/blobs/abc/download/logo.png`,
          ({ request }) => {
            authorizationHeaders.push(request.headers.get("authorization"));
            return new HttpResponse(null, {
              status: 302,
              headers: { Location: SIGNED_URL },
            });
          },
        ),
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
            description_attachments: [],
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
