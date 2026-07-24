/**
 * Tests for the CommentsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleComment = (id = 1) => ({
  id,
  content: "<p>Great work!</p>",
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
  creator: { id: 100, name: "Jane Doe" },
});

describe("CommentsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("get", () => {
    it("should return a single comment", async () => {
      const commentId = 42;

      server.use(
        http.get(`${BASE_URL}/comments/${commentId}`, () => {
          return HttpResponse.json(sampleComment(commentId));
        })
      );

      const comment = await client.comments.get(commentId);
      expect(comment.id).toBe(commentId);
      expect(comment.content).toBe("<p>Great work!</p>");
    });

    it("preserves float-spelled and null attachment dimensions at runtime", async () => {
      // A Comment's rich-text content is paired with a content_attachments
      // array. Pixel dimensions arrive float-spelled (1024.0) for images and
      // null for non-image blobs. The schema is nullable, so the generated
      // static type is `width?: number | null` — the present null is captured.
      // (In JS there is no int/float distinction, so 1024.0 is simply the
      // number 1024.) openapi-fetch performs no runtime validation; the values
      // below survive verbatim on the parsed object.
      const commentId = 77;
      server.use(
        http.get(`${BASE_URL}/comments/${commentId}`, () => {
          return HttpResponse.json({
            ...sampleComment(commentId),
            content_attachments: [
              {
                id: 1069480010,
                sgid: "BAh-img",
                filename: "celebration.png",
                content_type: "image/png",
                byte_size: 284111,
                download_url: `${BASE_URL}/buckets/1/blobs/img/download/celebration.png`,
                width: 1024.0,
                height: 768,
                previewable: true,
                preview_url: `${BASE_URL}/buckets/1/blobs/img/previews/celebration.png`,
                thumbnail_url: `${BASE_URL}/buckets/1/blobs/img/thumbnails/celebration.png`,
              },
              {
                id: 1069480011,
                sgid: "BAh-pdf",
                filename: "notes.pdf",
                content_type: "application/pdf",
                byte_size: 1048576,
                download_url: `${BASE_URL}/buckets/1/blobs/pdf/download/notes.pdf`,
                width: null,
                height: null,
                previewable: false,
                preview_url: `${BASE_URL}/buckets/1/blobs/pdf/previews/notes.pdf`,
                thumbnail_url: `${BASE_URL}/buckets/1/blobs/pdf/thumbnails/notes.pdf`,
              },
            ],
          });
        })
      );

      const comment = await client.comments.get(commentId);
      const attachments = comment.content_attachments;
      expect(attachments).toHaveLength(2);

      // Float-spelled 1024.0 is preserved as the number 1024.
      expect(attachments[0]!.width).toBe(1024);
      expect(attachments[0]!.height).toBe(768);
      // null is preserved verbatim, matching the static `width?: number | null`.
      expect(attachments[1]!.width).toBeNull();
      expect(attachments[1]!.height).toBeNull();
    });

    it("should throw not_found for missing comment", async () => {
      server.use(
        http.get(`${BASE_URL}/comments/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.comments.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("list", () => {
    it("should list comments on a recording", async () => {
      const recordingId = 200;

      server.use(
        http.get(`${BASE_URL}/recordings/${recordingId}/comments.json`, () => {
          return HttpResponse.json([sampleComment(1), sampleComment(2)]);
        })
      );

      const comments = await client.comments.list(recordingId);
      expect(comments).toHaveLength(2);
      expect(comments[0]!.id).toBe(1);
      expect(comments[1]!.id).toBe(2);
    });

    it("should return empty array when no comments exist", async () => {
      server.use(
        http.get(`${BASE_URL}/recordings/200/comments.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const comments = await client.comments.list(200);
      expect(comments).toHaveLength(0);
    });
  });

  describe("create", () => {
    it("should create a comment with content", async () => {
      const recordingId = 200;

      server.use(
        http.post(`${BASE_URL}/recordings/${recordingId}/comments.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.content).toBe("<p>New comment</p>");
          return HttpResponse.json(sampleComment(99), { status: 201 });
        })
      );

      const comment = await client.comments.create(recordingId, {
        content: "<p>New comment</p>",
      });
      expect(comment.id).toBe(99);
    });
  });

  describe("update", () => {
    it("should update a comment", async () => {
      const commentId = 42;

      server.use(
        http.put(`${BASE_URL}/comments/${commentId}`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.content).toBe("<p>Updated comment</p>");
          return HttpResponse.json(sampleComment(commentId));
        })
      );

      const comment = await client.comments.update(commentId, {
        content: "<p>Updated comment</p>",
      });
      expect(comment.id).toBe(commentId);
    });
  });
});
