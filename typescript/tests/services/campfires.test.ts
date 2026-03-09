/**
 * Tests for the CampfiresService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleCampfire = (id = 1) => ({
  id,
  title: "Campfire",
  topic: "General chat",
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
});

const sampleLine = (id = 1) => ({
  id,
  content: "<p>Hello everyone!</p>",
  created_at: "2024-01-15T10:00:00Z",
  creator: { id: 100, name: "Jane Doe" },
});

describe("CampfiresService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("get", () => {
    it("should return a single campfire", async () => {
      const campfireId = 42;

      server.use(
        http.get(`${BASE_URL}/chats/${campfireId}`, () => {
          return HttpResponse.json(sampleCampfire(campfireId));
        })
      );

      const campfire = await client.campfires.get(campfireId);
      expect(campfire.id).toBe(campfireId);
      expect(campfire.title).toBe("Campfire");
    });
  });

  describe("list", () => {
    it("should list all campfires", async () => {
      server.use(
        http.get(`${BASE_URL}/chats.json`, () => {
          return HttpResponse.json([sampleCampfire(1), sampleCampfire(2)]);
        })
      );

      const campfires = await client.campfires.list();
      expect(campfires).toHaveLength(2);
      expect(campfires[0]!.id).toBe(1);
      expect(campfires[1]!.id).toBe(2);
    });

    it("should propagate files_url on list responses", async () => {
      server.use(
        http.get(`${BASE_URL}/chats.json`, () => {
          return HttpResponse.json([
            { ...sampleCampfire(1), files_url: "https://3.basecampapi.com/12345/chats/1/uploads.json" },
          ]);
        })
      );

      const campfires = await client.campfires.list();
      expect(campfires[0]!.files_url).toBe("https://3.basecampapi.com/12345/chats/1/uploads.json");
    });
  });

  describe("listLines", () => {
    it("should list lines in a campfire", async () => {
      const campfireId = 42;

      server.use(
        http.get(`${BASE_URL}/chats/${campfireId}/lines.json`, () => {
          return HttpResponse.json([sampleLine(1), sampleLine(2)]);
        })
      );

      const lines = await client.campfires.listLines(campfireId);
      expect(lines).toHaveLength(2);
      expect(lines[0]!.id).toBe(1);
      expect(lines[1]!.id).toBe(2);
    });

    it("should handle mixed text and upload lines", async () => {
      const campfireId = 42;
      const uploadLine = {
        id: 3,
        title: "report.pdf",
        type: "Chat::Lines::Upload",
        created_at: "2024-01-15T10:00:00Z",
        attachments: [
          {
            filename: "report.pdf",
            content_type: "application/pdf",
            byte_size: 1048576,
            download_url: "https://3.basecampapi.com/12345/uploads/200/download/report.pdf",
          },
        ],
      };

      server.use(
        http.get(`${BASE_URL}/chats/${campfireId}/lines.json`, () => {
          return HttpResponse.json([sampleLine(1), uploadLine]);
        })
      );

      const lines = await client.campfires.listLines(campfireId);
      expect(lines).toHaveLength(2);
      // Text line
      expect(lines[0]!.content).toBe("<p>Hello everyone!</p>");
      // Upload line — no content, has attachments
      expect(lines[1]!.content).toBeUndefined();
      expect(lines[1]!.attachments).toHaveLength(1);
      expect(lines[1]!.attachments![0]!.filename).toBe("report.pdf");
      expect(lines[1]!.attachments![0]!.byte_size).toBe(1048576);
    });
  });

  describe("createLine", () => {
    it("should create a line with content", async () => {
      const campfireId = 42;

      server.use(
        http.post(`${BASE_URL}/chats/${campfireId}/lines.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.content).toBe("Hello world!");
          return HttpResponse.json(sampleLine(99), { status: 201 });
        })
      );

      const line = await client.campfires.createLine(campfireId, {
        content: "Hello world!",
      });
      expect(line.id).toBe(99);
    });
  });

  describe("getLine", () => {
    it("should return a single line", async () => {
      const campfireId = 42;
      const lineId = 10;

      server.use(
        http.get(`${BASE_URL}/chats/${campfireId}/lines/${lineId}`, () => {
          return HttpResponse.json(sampleLine(lineId));
        })
      );

      const line = await client.campfires.getLine(campfireId, lineId);
      expect(line.id).toBe(lineId);
      expect(line.content).toBe("<p>Hello everyone!</p>");
    });
  });

  describe("deleteLine", () => {
    it("should delete a line", async () => {
      server.use(
        http.delete(`${BASE_URL}/chats/42/lines/10`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.campfires.deleteLine(42, 10)).resolves.toBeUndefined();
    });
  });

  describe("listUploads", () => {
    it("should list uploaded files in a campfire", async () => {
      const campfireId = 42;
      const uploadLine = {
        id: 100,
        title: "report.pdf",
        type: "Chat::Lines::Upload",
        created_at: "2024-01-15T10:00:00Z",
        attachments: [
          {
            title: "report.pdf",
            url: "https://3.basecampapi.com/12345/uploads/200.json",
            filename: "report.pdf",
            content_type: "application/pdf",
            byte_size: 1048576,
            download_url: "https://3.basecampapi.com/12345/uploads/200/download/report.pdf",
          },
        ],
      };

      server.use(
        http.get(`${BASE_URL}/chats/${campfireId}/uploads.json`, () => {
          return HttpResponse.json([uploadLine]);
        })
      );

      const uploads = await client.campfires.listUploads(campfireId);
      expect(uploads).toHaveLength(1);
      expect(uploads[0]!.id).toBe(100);
      expect(uploads[0]!.title).toBe("report.pdf");
      expect(uploads[0]!.attachments).toHaveLength(1);
      expect(uploads[0]!.attachments![0]!.filename).toBe("report.pdf");
      expect(uploads[0]!.attachments![0]!.byte_size).toBe(1048576);
      expect(uploads[0]!.attachments![0]!.download_url).toBe(
        "https://3.basecampapi.com/12345/uploads/200/download/report.pdf"
      );
    });
  });

  describe("createUpload", () => {
    it("should upload a file with correct path and query", async () => {
      const campfireId = 42;
      const uploadLine = {
        id: 101,
        title: "photo.png",
        type: "Chat::Lines::Upload",
        created_at: "2024-01-15T10:00:00Z",
        attachments: [
          {
            filename: "photo.png",
            content_type: "image/png",
            byte_size: 2048,
          },
        ],
      };

      server.use(
        http.post(`${BASE_URL}/chats/${campfireId}/uploads.json`, async ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get("name")).toBe("photo.png");
          expect(request.headers.get("content-type")).toBe("image/png");
          return HttpResponse.json(uploadLine, { status: 201 });
        })
      );

      const result = await client.campfires.createUpload(
        campfireId,
        new Uint8Array([1, 2, 3]),
        "image/png",
        "photo.png"
      );
      expect(result.id).toBe(101);
      expect(result.attachments).toHaveLength(1);
      expect(result.attachments![0]!.filename).toBe("photo.png");
    });
  });

  describe("get with files_url", () => {
    it("should return files_url on campfire", async () => {
      const campfireId = 42;

      server.use(
        http.get(`${BASE_URL}/chats/${campfireId}`, () => {
          return HttpResponse.json({
            ...sampleCampfire(campfireId),
            files_url: "https://3.basecampapi.com/12345/chats/42/uploads.json",
          });
        })
      );

      const campfire = await client.campfires.get(campfireId);
      expect(campfire.files_url).toBe("https://3.basecampapi.com/12345/chats/42/uploads.json");
    });
  });

  describe("listUploads error handling", () => {
    it("should surface 403 as BasecampError", async () => {
      server.use(
        http.get(`${BASE_URL}/chats/42/uploads.json`, () => {
          return HttpResponse.json({ error: "Forbidden" }, { status: 403 });
        })
      );

      const error = await client.campfires.listUploads(42).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(403);
    });
  });

  describe("createUpload error handling", () => {
    it("should surface 422 as BasecampError", async () => {
      server.use(
        http.post(`${BASE_URL}/chats/42/uploads.json`, () => {
          return HttpResponse.json({ error: "Unprocessable" }, { status: 422 });
        })
      );

      const error = await client.campfires
        .createUpload(42, new Uint8Array([1]), "image/png", "test.png")
        .catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(422);
    });
  });
});
