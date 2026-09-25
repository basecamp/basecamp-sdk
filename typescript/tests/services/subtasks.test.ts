/**
 * Tests for the SubtasksService (generated from OpenAPI spec)
 *
 * A subtask is a CardStep on the wire — `type` stays "Kanban::Step" — reached
 * through the canonical flat /subtasks routes bc3 documents (bc3#12659).
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleSubtask = (id = 1, position = 1) => ({
  id,
  status: "active",
  visible_to_clients: false,
  created_at: "2026-07-02T00:23:00.000Z",
  updated_at: "2026-07-02T00:23:00.000Z",
  title: "Hero shot on the desk",
  inherits_status: true,
  type: "Kanban::Step",
  url: `${BASE_URL}/buckets/1/subtasks/${id}.json`,
  app_url: `https://3.basecamp.com/12345/buckets/1/todos/200#__recording_${id}`,
  position,
  completed: false,
  due_on: null,
  parent: { id: 200, title: "Shot list", type: "Todo", url: `${BASE_URL}/buckets/1/todos/200.json`, app_url: "https://3.basecamp.com/12345/buckets/1/todos/200" },
  bucket: { id: 1, name: "The Leto Laptop", type: "Project" },
  creator: { id: 100, name: "Matt Donahue" },
  assignees: [],
  completion_url: `${BASE_URL}/subtasks/${id}/completion.json`,
});

describe("SubtasksService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("list", () => {
    it("should list a recording's subtasks in position order", async () => {
      server.use(
        http.get(`${BASE_URL}/recordings/200/subtasks.json`, () => {
          return HttpResponse.json([sampleSubtask(1, 1), sampleSubtask(2, 2)], {
            headers: { "X-Total-Count": "2" },
          });
        })
      );

      const subtasks = await client.subtasks.list(200);
      expect(subtasks).toHaveLength(2);
      expect(subtasks[0]!.id).toBe(1);
      expect(subtasks[1]!.position).toBe(2);
      expect(subtasks[0]!.type).toBe("Kanban::Step");
    });

    it("should throw not_found for a recording the caller cannot see", async () => {
      server.use(
        http.get(`${BASE_URL}/recordings/999/subtasks.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.subtasks.list(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("get", () => {
    it("should return a single subtask", async () => {
      server.use(
        http.get(`${BASE_URL}/subtasks/42`, () => {
          return HttpResponse.json(sampleSubtask(42));
        })
      );

      const subtask = await client.subtasks.get(42);
      expect(subtask.id).toBe(42);
      expect(subtask.title).toBe("Hero shot on the desk");
      expect(subtask.completion_url).toBe(`${BASE_URL}/subtasks/42/completion.json`);
    });

    it("should throw not_found for a missing subtask", async () => {
      server.use(
        http.get(`${BASE_URL}/subtasks/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.subtasks.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("create", () => {
    it("should create a subtask under a recording", async () => {
      server.use(
        http.post(`${BASE_URL}/recordings/200/subtasks.json`, async ({ request }) => {
          const body = (await request.json()) as { title: string; due_on?: string; assignee_ids?: number[] };
          expect(body.title).toBe("Book the room");
          expect(body.due_on).toBe("2026-09-20");
          expect(body.assignee_ids).toEqual([30068628, 270913789]);
          return HttpResponse.json(sampleSubtask(99), { status: 201 });
        })
      );

      const subtask = await client.subtasks.create(200, {
        title: "Book the room",
        dueOn: "2026-09-20",
        assigneeIds: [30068628, 270913789],
      });
      expect(subtask.id).toBe(99);
    });

    it("should throw forbidden when the recording cannot hold subtasks", async () => {
      server.use(
        http.post(`${BASE_URL}/recordings/300/subtasks.json`, () => {
          return HttpResponse.json({ error: "Forbidden" }, { status: 403 });
        })
      );

      await expect(client.subtasks.create(300, { title: "Nope" })).rejects.toThrow(BasecampError);
    });
  });

  describe("update", () => {
    it("should send only the fields given", async () => {
      server.use(
        http.put(`${BASE_URL}/subtasks/42`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body).toEqual({ title: "Book the big room" });
          return HttpResponse.json({ ...sampleSubtask(42), title: "Book the big room" });
        })
      );

      const subtask = await client.subtasks.update(42, { title: "Book the big room" });
      expect(subtask.title).toBe("Book the big room");
    });

    it("should send an explicit empty assignee list to remove everyone", async () => {
      server.use(
        http.put(`${BASE_URL}/subtasks/42`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.assignee_ids).toEqual([]);
          return HttpResponse.json(sampleSubtask(42));
        })
      );

      await client.subtasks.update(42, { assigneeIds: [] });
    });

    it("should throw validation error on a 422", async () => {
      let reached = false;
      server.use(
        http.put(`${BASE_URL}/subtasks/42`, () => {
          reached = true;
          return HttpResponse.json({ error: "Validation failed" }, { status: 422 });
        })
      );

      await expect(client.subtasks.update(42, { title: "Book the big room" })).rejects.toThrow(BasecampError);
      expect(reached).toBe(true);
    });
  });

  describe("complete / uncomplete", () => {
    it("should POST and DELETE the completion resource", async () => {
      const methods: string[] = [];
      server.use(
        http.post(`${BASE_URL}/subtasks/42/completion.json`, ({ request }) => {
          methods.push(request.method);
          return new HttpResponse(null, { status: 204 });
        }),
        http.delete(`${BASE_URL}/subtasks/42/completion.json`, ({ request }) => {
          methods.push(request.method);
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.subtasks.complete(42)).resolves.toBeUndefined();
      await expect(client.subtasks.uncomplete(42)).resolves.toBeUndefined();
      expect(methods).toEqual(["POST", "DELETE"]);
    });

    it("should throw not_found when completing a missing subtask", async () => {
      server.use(
        http.post(`${BASE_URL}/subtasks/999/completion.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.subtasks.complete(999)).rejects.toThrow(BasecampError);
    });

    it("should throw not_found when uncompleting a missing subtask", async () => {
      server.use(
        http.delete(`${BASE_URL}/subtasks/999/completion.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.subtasks.uncomplete(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("reposition", () => {
    it("should PUT the 1-based position", async () => {
      server.use(
        http.put(`${BASE_URL}/subtasks/42/position.json`, async ({ request }) => {
          const body = (await request.json()) as { position: number };
          expect(body.position).toBe(4);
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.subtasks.reposition(42, { position: 4 })).resolves.toBeUndefined();
    });

    it("should throw validation error on a 422", async () => {
      server.use(
        http.put(`${BASE_URL}/subtasks/42/position.json`, () => {
          return HttpResponse.json({ errors: { position: ["must be greater than 0"] } }, { status: 422 });
        })
      );

      await expect(client.subtasks.reposition(42, { position: 0 })).rejects.toThrow(BasecampError);
    });
  });

  describe("delete", () => {
    it("should delete a subtask", async () => {
      server.use(
        http.delete(`${BASE_URL}/subtasks/42`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.subtasks.delete(42)).resolves.toBeUndefined();
    });

    it("should throw forbidden when deletion is limited to admins and the creator", async () => {
      server.use(
        http.delete(`${BASE_URL}/subtasks/42`, () => {
          return HttpResponse.json({ error: "Forbidden" }, { status: 403 });
        })
      );

      await expect(client.subtasks.delete(42)).rejects.toThrow(BasecampError);
    });
  });
});
