/**
 * Tests for the Attachments service
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { AttachmentsService } from "../../src/services/attachments.js";
import { BasecampError } from "../../src/errors.js";
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

      const result = await service.create({
        filename: "test.pdf",
        contentType: "application/pdf",
        data: new Uint8Array([1, 2, 3, 4]),
      });

      expect(result.attachableSgid).toBe(sgid);
    });

    it("should include filename in query params", async () => {
      let capturedUrl: string | null = null;

      server.use(
        http.post(`${BASE_URL}/attachments.json`, ({ request }) => {
          capturedUrl = request.url;
          return HttpResponse.json({ attachable_sgid: "test-sgid" });
        })
      );

      await service.create({
        filename: "report.xlsx",
        contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        data: new Uint8Array([1, 2, 3]),
      });

      expect(capturedUrl).toContain("name=report.xlsx");
    });

    it("should throw validation error when filename is missing", async () => {
      await expect(
        service.create({
          filename: "",
          contentType: "image/png",
          data: new Uint8Array([1]),
        })
      ).rejects.toThrow(BasecampError);

      try {
        await service.create({
          filename: "",
          contentType: "image/png",
          data: new Uint8Array([1]),
        });
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("filename");
      }
    });

    it("should throw validation error when content type is missing", async () => {
      await expect(
        service.create({
          filename: "test.pdf",
          contentType: "",
          data: new Uint8Array([1]),
        })
      ).rejects.toThrow(BasecampError);

      try {
        await service.create({
          filename: "test.pdf",
          contentType: "",
          data: new Uint8Array([1]),
        });
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("content type");
      }
    });

    it("should throw validation error when data is empty", async () => {
      await expect(
        service.create({
          filename: "test.pdf",
          contentType: "application/pdf",
          data: new Uint8Array([]),
        })
      ).rejects.toThrow(BasecampError);

      try {
        await service.create({
          filename: "test.pdf",
          contentType: "application/pdf",
          data: new Uint8Array([]),
        });
      } catch (err) {
        expect((err as BasecampError).code).toBe("validation");
        expect((err as BasecampError).message).toContain("empty");
      }
    });

    it("should work with ArrayBuffer data", async () => {
      server.use(
        http.post(`${BASE_URL}/attachments.json`, () => {
          return HttpResponse.json({ attachable_sgid: "buffer-sgid" });
        })
      );

      const buffer = new ArrayBuffer(4);
      new Uint8Array(buffer).set([1, 2, 3, 4]);

      const result = await service.create({
        filename: "test.bin",
        contentType: "application/octet-stream",
        data: buffer,
      });

      expect(result.attachableSgid).toBe("buffer-sgid");
    });

    it("should work with Blob data", async () => {
      server.use(
        http.post(`${BASE_URL}/attachments.json`, () => {
          return HttpResponse.json({ attachable_sgid: "blob-sgid" });
        })
      );

      const blob = new Blob([new Uint8Array([1, 2, 3, 4])], { type: "image/png" });

      const result = await service.create({
        filename: "image.png",
        contentType: "image/png",
        data: blob,
      });

      expect(result.attachableSgid).toBe("blob-sgid");
    });
  });
});
