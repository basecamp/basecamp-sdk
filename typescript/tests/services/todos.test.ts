/**
 * Tests for the TodosService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleTodo = (id = 1) => ({
  id,
  content: "Buy milk",
  description: "<p>From the store</p>",
  completed: false,
  due_on: "2024-03-01",
  assignees: [{ id: 100, name: "Jane Doe" }],
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
});

describe("TodosService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("list", () => {
    it("should list todos in a todolist", async () => {
      const todolistId = 200;

      server.use(
        http.get(`${BASE_URL}/todolists/${todolistId}/todos.json`, () => {
          return HttpResponse.json([sampleTodo(1), sampleTodo(2)]);
        })
      );

      const todos = await client.todos.list(todolistId);
      expect(todos).toHaveLength(2);
      expect(todos[0]!.id).toBe(1);
      expect(todos[1]!.id).toBe(2);
    });

    it("should return empty array when no todos exist", async () => {
      server.use(
        http.get(`${BASE_URL}/todolists/200/todos.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const todos = await client.todos.list(200);
      expect(todos).toHaveLength(0);
    });
  });

  describe("get", () => {
    it("should return a single todo", async () => {
      const todoId = 42;

      server.use(
        http.get(`${BASE_URL}/todos/${todoId}`, () => {
          return HttpResponse.json(sampleTodo(todoId));
        })
      );

      const todo = await client.todos.get(todoId);
      expect(todo.id).toBe(todoId);
      expect(todo.content).toBe("Buy milk");
    });

    it("should throw not_found for missing todo", async () => {
      server.use(
        http.get(`${BASE_URL}/todos/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.todos.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("create", () => {
    it("should create a todo with content and assignee_ids", async () => {
      const todolistId = 200;

      server.use(
        http.post(`${BASE_URL}/todolists/${todolistId}/todos.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.content).toBe("New task");
          expect(body.assignee_ids).toEqual([1, 2]);
          return HttpResponse.json(sampleTodo(99), { status: 201 });
        })
      );

      const todo = await client.todos.create(todolistId, {
        content: "New task",
        assigneeIds: [1, 2],
      });
      expect(todo.id).toBe(99);
    });
  });

  describe("update", () => {
    it("should update a todo", async () => {
      const todoId = 42;

      server.use(
        http.put(`${BASE_URL}/todos/${todoId}`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.content).toBe("Updated task");
          return HttpResponse.json(sampleTodo(todoId));
        })
      );

      const todo = await client.todos.update(todoId, {
        content: "Updated task",
      });
      expect(todo.id).toBe(todoId);
    });
  });

  describe("complete", () => {
    it("should mark a todo as complete", async () => {
      server.use(
        http.post(`${BASE_URL}/todos/42/completion.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.todos.complete(42)).resolves.toBeUndefined();
    });
  });

  describe("uncomplete", () => {
    it("should mark a todo as incomplete", async () => {
      server.use(
        http.delete(`${BASE_URL}/todos/42/completion.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.todos.uncomplete(42)).resolves.toBeUndefined();
    });
  });

  describe("reposition", () => {
    it("should reposition a todo with position and parent_id", async () => {
      server.use(
        http.put(`${BASE_URL}/todos/42/position.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.position).toBe(3);
          expect(body.parent_id).toBe(999);
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(
        client.todos.reposition(42, { position: 3, parentId: 999 })
      ).resolves.toBeUndefined();
    });
  });
});
