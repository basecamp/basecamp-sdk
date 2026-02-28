/**
 * Tests for the TodosetsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleTodoset = (id = 1) => ({
  id,
  title: "To-dos",
  name: "To-dos",
  todolists_count: 3,
  todolists_url: `${BASE_URL}/buckets/100/todosets/${id}/todolists.json`,
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
});

describe("TodosetsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("get", () => {
    it("should return a single todoset", async () => {
      const todosetId = 42;

      server.use(
        http.get(`${BASE_URL}/todosets/${todosetId}`, () => {
          return HttpResponse.json(sampleTodoset(todosetId));
        })
      );

      const todoset = await client.todosets.get(todosetId);
      expect(todoset.id).toBe(todosetId);
      expect(todoset.name).toBe("To-dos");
    });

    it("should throw not_found for missing todoset", async () => {
      server.use(
        http.get(`${BASE_URL}/todosets/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.todosets.get(999)).rejects.toThrow(BasecampError);
    });
  });
});
