/**
 * Tests for the Attachments service (generated from OpenAPI spec)
 *
 * Note: Generated services are spec-conformant:
 * - create() signature: create(data, contentType, name) - not create({ filename, contentType, data })
 * - No client-side validation (API validates)
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import type { AttachmentsService } from "../../src/generated/services/attachments.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("AttachmentsService", () => {
  let service: AttachmentsService;

  beforeEach(() => {
    vi.clearAllMocks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
    service = client.attachments;
  });

  describe("create", () => {
    it("should upload a file and return attachable_sgid", async () => {
      const sgid = "BAh7CEkiCGdpZAY6BkVUSSIxZ2lkOi8vYmM...";

      server.use(
        http.post(`${BASE_URL}/attachments.json`, () => {
          return HttpResponse.json({ attachable_sgid: sgid });
        })
      );

      // Generated signature: create(data, contentType, name)
      const result = await service.create(
        new Uint8Array([1, 2, 3, 4]),
        "application/pdf",
        "test.pdf"
      );

      expect(result.attachable_sgid).toBe(sgid);
    });

    it("should include filename in query params", async () => {
      let capturedUrl: string | null = null;

      server.use(
        http.post(`${BASE_URL}/attachments.json`, ({ request }) => {
          capturedUrl = request.url;
          return HttpResponse.json({ attachable_sgid: "test-sgid" });
        })
      );

      await service.create(
        new Uint8Array([1, 2, 3]),
        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        "report.xlsx"
      );

      expect(capturedUrl).toContain("name=report.xlsx");
    });

    it("should set Content-Type header to the file's MIME type", async () => {
      let capturedContentType: string | null = null;

      server.use(
        http.post(`${BASE_URL}/attachments.json`, ({ request }) => {
          capturedContentType = request.headers.get("Content-Type");
          return HttpResponse.json({ attachable_sgid: "test-sgid" });
        })
      );

      await service.create(
        new Uint8Array([1, 2, 3, 4]),
        "image/png",
        "image.png"
      );

      expect(capturedContentType).toBe("image/png");
    });

    // Note: Client-side validation removed - generated services let API validate

    it("should work with ArrayBuffer data", async () => {
      server.use(
        http.post(`${BASE_URL}/attachments.json`, () => {
          return HttpResponse.json({ attachable_sgid: "buffer-sgid" });
        })
      );

      const buffer = new ArrayBuffer(4);
      new Uint8Array(buffer).set([1, 2, 3, 4]);

      const result = await service.create(
        buffer,
        "application/octet-stream",
        "test.bin"
      );

      expect(result.attachable_sgid).toBe("buffer-sgid");
    });
  });
});
