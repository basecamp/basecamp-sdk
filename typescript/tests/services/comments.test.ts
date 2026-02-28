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
