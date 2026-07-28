/**
 * Tests for the TodolistsService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";
import todolistFixture from "../../../spec/fixtures/todolists/get.json";
import type { OperationInfo } from "../../src/hooks.js";

const BASE_URL = "https://3.basecampapi.com/12345";

// Sourced from the shared, coverage-guarded fixture (spec/fixtures/manifest.yaml)
// so this helper cannot drift from the validated Todolist shape; `id` is
// overridable per call.
const sampleTodolist = (id = todolistFixture.id) => ({ ...todolistFixture, id });

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
      expect(todolist.name).toBe(todolistFixture.name);
      // bubble_up_url is @required on Todolist: todolists/_todolist.json.jbuilder
      // renders the shared recording partial with bubbleupable: true
      // unconditionally, so every projection of this shape carries it. The stub
      // is the canonical fixture, so this also proves the fixture carries it.
      expect(todolist.bubble_up_url).toBeDefined();
    });

    it("should throw not_found for missing todolist", async () => {
      server.use(
        http.get(`${BASE_URL}/todolists/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.todolists.get(999)).rejects.toThrow(BasecampError);
    });

    it("scopes the resource to the todolist id (unsuffixed {id} path param)", async () => {
      const id = 42;
      let captured: OperationInfo | undefined;

      const hookedClient = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
        hooks: {
          onOperationStart: (info) => {
            captured = info;
          },
        },
      });

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => HttpResponse.json(sampleTodolist(id)))
      );

      await hookedClient.todolists.get(id);

      expect(captured?.operation).toBe("GetTodolistOrGroup");
      expect(captured?.resourceId).toBe(id);
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

    it("scopes the resource to the todolist id (unsuffixed {id} path param)", async () => {
      const id = 42;
      let captured: OperationInfo | undefined;

      const hookedClient = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
        hooks: {
          onOperationStart: (info) => {
            captured = info;
          },
        },
      });

      server.use(
        http.put(`${BASE_URL}/todolists/${id}`, () => HttpResponse.json(sampleTodolist(id)))
      );

      await hookedClient.todolists.update(id, { name: "Updated list" });

      expect(captured?.operation).toBe("UpdateTodolistOrGroup");
      expect(captured?.resourceId).toBe(id);
    });
  });

  describe("reposition", () => {
    it("should reposition a todolist within its todoset", async () => {
      const id = 42;

      server.use(
        http.put(`${BASE_URL}/todosets/todolists/${id}/position.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.position).toBe(3);
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.todolists.reposition(id, { position: 3 })).resolves.toBeUndefined();
    });

    it("should throw not_found for missing todolist", async () => {
      server.use(
        http.put(`${BASE_URL}/todosets/todolists/999/position.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.todolists.reposition(999, { position: 1 })).rejects.toThrow(BasecampError);
    });
  });
});
