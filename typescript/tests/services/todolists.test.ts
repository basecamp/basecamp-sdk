/**
 * Tests for the TodolistsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleTodolist = (id = 1) => ({
  id,
  name: "Launch list",
  description: "<p>Things to do before launch</p>",
  completed: false,
  completed_ratio: "0/5",
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
});

describe("TodolistsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("get", () => {
    it("should return a single todolist", async () => {
      const id = 42;

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          return HttpResponse.json(sampleTodolist(id));
        })
      );

      const todolist = await client.todolists.get(id);
      expect(todolist.id).toBe(id);
      expect(todolist.name).toBe("Launch list");
    });

    it("should throw not_found for missing todolist", async () => {
      server.use(
        http.get(`${BASE_URL}/todolists/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.todolists.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("list", () => {
    it("should list todolists in a todoset", async () => {
      const todosetId = 200;

      server.use(
        http.get(`${BASE_URL}/todosets/${todosetId}/todolists.json`, () => {
          return HttpResponse.json([sampleTodolist(1), sampleTodolist(2)]);
        })
      );

      const todolists = await client.todolists.list(todosetId);
      expect(todolists).toHaveLength(2);
      expect(todolists[0]!.id).toBe(1);
      expect(todolists[1]!.id).toBe(2);
    });

    it("should return empty array when no todolists exist", async () => {
      server.use(
        http.get(`${BASE_URL}/todosets/200/todolists.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const todolists = await client.todolists.list(200);
      expect(todolists).toHaveLength(0);
    });
  });

  describe("create", () => {
    it("should create a todolist with name and description", async () => {
      const todosetId = 200;

      server.use(
        http.post(`${BASE_URL}/todosets/${todosetId}/todolists.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.name).toBe("New list");
          expect(body.description).toBe("<p>Details</p>");
          return HttpResponse.json(sampleTodolist(99), { status: 201 });
        })
      );

      const todolist = await client.todolists.create(todosetId, {
        name: "New list",
        description: "<p>Details</p>",
      });
      expect(todolist.id).toBe(99);
    });
  });

  describe("update", () => {
    it("should update a todolist", async () => {
      const id = 42;

      server.use(
        http.put(`${BASE_URL}/todolists/${id}`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.name).toBe("Updated list");
          return HttpResponse.json(sampleTodolist(id));
        })
      );

      const todolist = await client.todolists.update(id, {
        name: "Updated list",
      });
      expect(todolist.id).toBe(id);
    });
  });
});
