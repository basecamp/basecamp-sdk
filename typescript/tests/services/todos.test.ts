/**
 * Tests for the TodosService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import type { JsonBodyType } from "msw";
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

    it("preserves float-spelled and null attachment dimensions at runtime", async () => {
      // A Todo's rich-text description is paired with a description_attachments
      // array. Pixel dimensions arrive float-spelled (1024.0) for images and
      // null for non-image blobs. The schema is nullable, so the generated
      // static type is `width?: number | null` — the present null is captured.
      // (In JS there is no int/float distinction, so 1024.0 is simply the
      // number 1024.) openapi-fetch performs no runtime validation; the values
      // below survive verbatim on the parsed object.
      const todoId = 77;
      server.use(
        http.get(`${BASE_URL}/todos/${todoId}`, () => {
          return HttpResponse.json({
            ...sampleTodo(todoId),
            description_attachments: [
              {
                id: 1069480000,
                sgid: "BAh-img",
                filename: "leto-schematic.png",
                content_type: "image/png",
                byte_size: 284111,
                download_url: `${BASE_URL}/buckets/1/blobs/img/download/leto-schematic.png`,
                width: 1024.0,
                height: 768,
                previewable: true,
                preview_url: `${BASE_URL}/buckets/1/blobs/img/previews/leto-schematic.png`,
                thumbnail_url: `${BASE_URL}/buckets/1/blobs/img/thumbnails/leto-schematic.png`,
              },
              {
                id: 1069480001,
                sgid: "BAh-pdf",
                filename: "leto-spec.pdf",
                content_type: "application/pdf",
                byte_size: 1048576,
                download_url: `${BASE_URL}/buckets/1/blobs/pdf/download/leto-spec.pdf`,
                width: null,
                height: null,
                previewable: false,
                preview_url: `${BASE_URL}/buckets/1/blobs/pdf/previews/leto-spec.pdf`,
                thumbnail_url: `${BASE_URL}/buckets/1/blobs/pdf/thumbnails/leto-spec.pdf`,
              },
            ],
          });
        })
      );

      const todo = await client.todos.get(todoId);
      const attachments = todo.description_attachments;
      expect(attachments).toHaveLength(2);

      // Float-spelled 1024.0 is preserved as the number 1024.
      expect(attachments[0]!.width).toBe(1024);
      expect(attachments[0]!.height).toBe(768);
      // null is preserved verbatim despite the static `width?: number` type.
      expect(attachments[1]!.width).toBeNull();
      expect(attachments[1]!.height).toBeNull();
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

  describe("createTodosetTodo", () => {
    it("creates a loose to-do directly under the project's to-do set", async () => {
      server.use(
        http.post(`${BASE_URL}/buckets/2/todosets/9/todos.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body.content).toBe("Loose task");
          expect(body.assignee_ids).toEqual([1, 2]);
          return HttpResponse.json(sampleTodo(1000), { status: 201 });
        })
      );

      const todo = await client.todos.createTodosetTodo(2, 9, {
        content: "Loose task",
        assigneeIds: [1, 2],
      });
      expect(todo.id).toBe(1000);
    });

    it("surfaces 422 as BasecampError", async () => {
      server.use(
        http.post(`${BASE_URL}/buckets/2/todosets/9/todos.json`, () => {
          return HttpResponse.json({ error: "Content can't be blank" }, { status: 422 });
        })
      );

      const error = await client.todos
        .createTodosetTodo(2, 9, { content: "x" })
        .catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(422);
    });
  });

  describe("update", () => {
    const fullTodo = (id = 42) => ({
      ...sampleTodo(id),
      starts_on: "2024-02-01",
      completion_subscribers: [{ id: 555, name: "Sub Scriber" }],
    });

    it("merges: an omitted field is preserved from the GET", async () => {
      const todoId = 42;
      const requests: string[] = [];
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/todos/${todoId}`, () => {
          requests.push("GET");
          return HttpResponse.json(fullTodo(todoId));
        }),
        http.put(`${BASE_URL}/todos/${todoId}`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(fullTodo(todoId));
        })
      );

      const todo = await client.todos.update(todoId, { content: "Updated task" });
      expect(todo.id).toBe(todoId);
      expect(requests).toEqual(["GET", "PUT"]);
      expect(putBody.content).toBe("Updated task");
      expect(putBody.description).toBe("<p>From the store</p>");
      expect(putBody.due_on).toBe("2024-03-01");
      expect(putBody.starts_on).toBe("2024-02-01");
      expect(putBody.assignee_ids).toEqual([100]);
      expect(putBody.completion_subscriber_ids).toEqual([555]);
      expect(putBody).not.toHaveProperty("notify");
    });

    it("clears with an explicitly-passed empty array", async () => {
      let putBody: Record<string, unknown> = {};
      server.use(
        http.get(`${BASE_URL}/todos/42`, () => HttpResponse.json(fullTodo())),
        http.put(`${BASE_URL}/todos/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(fullTodo());
        })
      );

      await client.todos.update(42, { assigneeIds: [] });
      expect(putBody.assignee_ids).toEqual([]);
      expect(putBody.completion_subscriber_ids).toEqual([555]);
      expect(putBody.content).toBe("Buy milk");
    });

    it("sends notify only when true", async () => {
      let putBody: Record<string, unknown> = {};
      server.use(
        http.get(`${BASE_URL}/todos/42`, () => HttpResponse.json(fullTodo())),
        http.put(`${BASE_URL}/todos/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(fullTodo());
        })
      );

      await client.todos.update(42, { content: "ping", notify: true });
      expect(putBody.notify).toBe(true);
    });

    it("hooks observe the wire operations GetTodo then ReplaceTodo", async () => {
      const operations: string[] = [];
      const hookedClient = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
        hooks: {
          onOperationStart: (info) => {
            operations.push(info.operation);
          },
        },
      });

      server.use(
        http.get(`${BASE_URL}/todos/42`, () => HttpResponse.json(fullTodo())),
        http.put(`${BASE_URL}/todos/42`, () => HttpResponse.json(fullTodo()))
      );

      await hookedClient.todos.update(42, { content: "observed" });
      expect(operations).toEqual(["GetTodo", "ReplaceTodo"]);
    });
  });

  describe("edit", () => {
    const fullTodo = (id = 42) => ({
      ...sampleTodo(id),
      completion_subscribers: [{ id: 555, name: "Sub Scriber" }],
    });

    it("hands the callback current state and PUTs everything back", async () => {
      let putBody: Record<string, unknown> = {};
      server.use(
        http.get(`${BASE_URL}/todos/42`, () => HttpResponse.json(fullTodo())),
        http.put(`${BASE_URL}/todos/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(fullTodo());
        })
      );

      const todo = await client.todos.edit(42, (t) => {
        expect(t.content).toBe("Buy milk");
        t.content = `🚨 ${t.content}`;
      });
      expect(todo.id).toBe(42);
      expect(putBody.content).toBe("🚨 Buy milk");
      expect(putBody.description).toBe("<p>From the store</p>");
      expect(putBody.assignee_ids).toEqual([100]);
    });

    it("clears a date by setting it empty — omitted from the PUT body", async () => {
      let putBody: Record<string, unknown> = {};
      server.use(
        http.get(`${BASE_URL}/todos/42`, () => HttpResponse.json(fullTodo())),
        http.put(`${BASE_URL}/todos/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(fullTodo());
        })
      );

      await client.todos.edit(42, (t) => {
        expect(t.dueOn).toBe("2024-03-01");
        t.dueOn = "";
      });
      expect(putBody).not.toHaveProperty("due_on");
      expect(putBody.content).toBe("Buy milk");
    });

    it("clears description and ID lists explicitly — present-and-empty in the PUT body", async () => {
      let putBody: Record<string, unknown> = {};
      server.use(
        http.get(`${BASE_URL}/todos/42`, () => HttpResponse.json(fullTodo())),
        http.put(`${BASE_URL}/todos/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(fullTodo());
        })
      );

      await client.todos.edit(42, (t) => {
        t.description = "";
        t.assigneeIds = [];
        t.completionSubscriberIds = [];
      });
      expect(putBody.description).toBe("");
      expect(putBody.assignee_ids).toEqual([]);
      expect(putBody.completion_subscriber_ids).toEqual([]);
    });

    it("aborts without a PUT when the callback throws", async () => {
      let putCount = 0;
      server.use(
        http.get(`${BASE_URL}/todos/42`, () => HttpResponse.json(fullTodo())),
        http.put(`${BASE_URL}/todos/42`, () => {
          putCount++;
          return HttpResponse.json(fullTodo());
        })
      );

      await expect(
        client.todos.edit(42, () => {
          throw new Error("abort");
        })
      ).rejects.toThrow("abort");
      expect(putCount).toBe(0);
    });

    it("supports async callbacks", async () => {
      let putBody: Record<string, unknown> = {};
      server.use(
        http.get(`${BASE_URL}/todos/42`, () => HttpResponse.json(fullTodo())),
        http.put(`${BASE_URL}/todos/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(fullTodo());
        })
      );

      await client.todos.edit(42, async (t) => {
        t.content = await Promise.resolve("async content");
      });
      expect(putBody.content).toBe("async content");
    });

    it("hooks observe the wire operations GetTodo then ReplaceTodo", async () => {
      const operations: string[] = [];
      const hookedClient = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
        hooks: {
          onOperationStart: (info) => {
            operations.push(info.operation);
          },
        },
      });

      server.use(
        http.get(`${BASE_URL}/todos/42`, () => HttpResponse.json(fullTodo())),
        http.put(`${BASE_URL}/todos/42`, () => HttpResponse.json(fullTodo()))
      );

      await hookedClient.todos.edit(42, (t) => {
        t.content = "observed";
      });
      expect(operations).toEqual(["GetTodo", "ReplaceTodo"]);
    });
  });

  describe("replace", () => {
    it("sends the sparse request verbatim with no GET", async () => {
      const requests: string[] = [];
      let putBody: Record<string, unknown> = {};
      server.use(
        http.get(`${BASE_URL}/todos/42`, () => {
          requests.push("GET");
          return HttpResponse.json(sampleTodo(42));
        }),
        http.put(`${BASE_URL}/todos/42`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleTodo(42));
        })
      );

      const todo = await client.todos.replace(42, { content: "the whole new todo" });
      expect(todo.id).toBe(42);
      expect(requests).toEqual(["PUT"]);
      expect(putBody.content).toBe("the whole new todo");
      // Unset fields are omitted — the server clears them.
      expect(putBody).not.toHaveProperty("description");
      expect(putBody).not.toHaveProperty("assignee_ids");
      expect(putBody).not.toHaveProperty("completion_subscriber_ids");
      expect(putBody).not.toHaveProperty("due_on");
      expect(putBody).not.toHaveProperty("starts_on");
      expect(putBody).not.toHaveProperty("notify");
    });

    it("requires content", async () => {
      await expect(
        client.todos.replace(42, { content: "" })
      ).rejects.toThrow(BasecampError);
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

    it("should retry an idempotent POST (CompleteTodo) on 503 then succeed", async () => {
      // CompleteTodo is a POST but is flagged idempotent in metadata
      // (idempotent.natural === true), so the retry middleware MUST retry it on
      // 503. Drives the generated service path (client.todos.complete), not the
      // raw client, so this fails if CompleteTodo's metadata idempotent flag is
      // ever flipped off — the analog of Swift's testGeneratedIdempotentPostRetries.
      let attempts = 0;

      server.use(
        http.post(`${BASE_URL}/todos/42/completion.json`, () => {
          attempts++;
          if (attempts === 1) {
            return new HttpResponse(null, { status: 503 });
          }
          return new HttpResponse(null, { status: 204 });
        })
      );

      // The suite default is enableRetry:false; this operation needs a
      // retry-enabled client to exercise the POST idempotency gate. Set it
      // explicitly rather than relying on createBasecampClient's default so the
      // test keeps testing retry even if that default ever changes.
      const retryClient = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: true,
      });

      await expect(retryClient.todos.complete(42)).resolves.toBeUndefined();
      expect(attempts).toBe(2); // initial 503 + 1 retry that succeeds
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

  // --- #576: a malformed GET field must never reach the full-replace PUT ----
  //
  // `update`/`edit` GET the todo, read each writable field, and PUT the FULL
  // representation back, so every value read is written -- including one the
  // caller never mentioned. `?? ""` coalesces only null and undefined, so it
  // ruled out *erasure* while leaving *corruption* wide open: all eight
  // malformed shapes rode through VERBATIM into the PUT, a broader surface
  // than Python's, which at least collapsed the falsey four to "".
  //
  // TypeScript has no runtime decoder to catch this -- `schema.d.ts` is erased
  // at build time, so `Todo` is a compile-time claim nothing validates. That
  // places this composite with Python and Ruby, not with Go and Swift.
  //
  // The assertion that matters is the ORDERING: `requests` must be ["GET"]. A
  // guard that fires after the PUT has already lost the field.
  describe("malformed writable fields (#576)", () => {
    const fullTodo = (id = 42, overrides: Record<string, unknown> = {}) => ({
      ...sampleTodo(id),
      starts_on: "2024-02-01",
      completion_subscribers: [{ id: 555, name: "Sub Scriber" }],
      ...overrides,
    });

    const malformed: [string, unknown][] = [
      ["false", false],
      ["zero", 0],
      ["empty array", []],
      ["empty object", {}],
      ["number", 42],
      ["true", true],
      ["array", ["x"]],
      ["object", { a: 1 }],
    ];

    const writableStrings = ["content", "description", "due_on", "starts_on"] as const;
    const idLists = ["assignees", "completion_subscribers"] as const;

    // Serve a GET carrying `body` and a PUT that records that it happened.
    // `body` is typed as MSW's own response-body type rather than `unknown`:
    // the malformed-*envelope* cases below deliberately serve arrays, scalars
    // and null, so the parameter has to stay as wide as JSON itself -- but no
    // wider, or the `HttpResponse.json` call cannot accept it.
    const serve = (body: JsonBodyType, requests: string[]) => {
      server.use(
        http.get(`${BASE_URL}/todos/42`, () => {
          requests.push("GET");
          return HttpResponse.json(body);
        }),
        http.put(`${BASE_URL}/todos/42`, () => {
          requests.push("PUT");
          return HttpResponse.json(fullTodo());
        })
      );
    };

    const rejection = async (promise: Promise<unknown>): Promise<unknown> =>
      promise.then(
        () => {
          throw new Error("expected the call to reject, but it resolved");
        },
        (error: unknown) => error
      );

    // Asserting only the message is vacuous about the taxonomy: a wrong `code`
    // satisfies it. The value arrived in a successful API response, so this is
    // `api_error` -- the caller passed nothing wrong.
    const expectResponseError = (error: unknown, pattern: RegExp, requests: string[]) => {
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("api_error");
      expect((error as BasecampError).message).toMatch(pattern);
      expect(requests).toEqual(["GET"]);
    };

    for (const field of writableStrings) {
      it.each(malformed)(`update refuses a %s ${field} before writing`, async (_label, value) => {
        const requests: string[] = [];
        serve(fullTodo(42, { [field]: value }), requests);

        const error = await rejection(client.todos.update(42, { content: "New title" }));
        expectResponseError(error, new RegExp(`Todo field "${field}" is not a string`), requests);
      });

      it(`edit refuses a malformed ${field} before writing`, async () => {
        const requests: string[] = [];
        serve(fullTodo(42, { [field]: 42 }), requests);

        const error = await rejection(
          client.todos.edit(42, (t) => {
            t.content = "New title";
          })
        );
        expectResponseError(error, new RegExp(`Todo field "${field}" is not a string`), requests);
      });

      // The other half of the rule: absent and null are not malformed, they
      // are empty. The call overlays an ID list rather than a string so that
      // no writable string under test is overwritten by the caller.
      it.each([
        ["absent", undefined],
        ["null", null],
      ])(`treats a %s ${field} as genuinely empty`, async (_label, value) => {
        let putBody: Record<string, unknown> = {};
        const body = fullTodo(42, { [field]: value });
        if (value === undefined) delete (body as Record<string, unknown>)[field];
        server.use(
          http.get(`${BASE_URL}/todos/42`, () => HttpResponse.json(body)),
          http.put(`${BASE_URL}/todos/42`, async ({ request }) => {
            putBody = (await request.json()) as Record<string, unknown>;
            return HttpResponse.json(fullTodo());
          })
        );

        if (field === "content") {
          // Pre-existing, unchanged by these guards: the generated `replace`
          // presence-validates content, so an empty one is refused client-side
          // as a caller `validation` error before any PUT. Recorded here so the
          // divergence from Python and Ruby (which send `content: ""`) is
          // visible rather than folded into a skipped case.
          const error = await rejection(client.todos.update(42, { assigneeIds: [100] }));
          expect((error as BasecampError).code).toBe("validation");
          return;
        }

        await client.todos.update(42, { assigneeIds: [100] });

        if (field === "due_on" || field === "starts_on") {
          // Dates ride only when non-empty: "" is a format error and an
          // omitted date is how the server clears one.
          expect(putBody).not.toHaveProperty(field);
        } else {
          expect(putBody[field]).toBe("");
        }
      });
    }

    for (const field of idLists) {
      it.each(malformed.filter(([, v]) => !Array.isArray(v)))(
        `update refuses a %s ${field} before writing`,
        async (_label, value) => {
          const requests: string[] = [];
          serve(fullTodo(42, { [field]: value }), requests);

          const error = await rejection(client.todos.update(42, { content: "New title" }));
          expectResponseError(error, new RegExp(`Todo field "${field}" is not an array`), requests);
        }
      );

      it(`update refuses a non-object ${field} element before writing`, async () => {
        const requests: string[] = [];
        serve(fullTodo(42, { [field]: ["nope"] }), requests);

        const error = await rejection(client.todos.update(42, { content: "New title" }));
        expectResponseError(
          error,
          new RegExp(`Todo field "${field}"\\[0\\] is not an object`),
          requests
        );
      });

      // The ID lists are resent in full, so a string, float, boolean or null
      // id would be written as the complete assignee set.
      it.each([
        ["string", "100"],
        ["float", 10.5],
        ["NaN", Number.NaN],
        ["null", null],
        ["boolean", true],
      ])(`update refuses a %s ${field} id before writing`, async (_label, badId) => {
        const requests: string[] = [];
        serve(fullTodo(42, { [field]: [{ id: badId, name: "Jane" }] }), requests);

        const error = await rejection(client.todos.update(42, { content: "New title" }));
        expectResponseError(error, new RegExp(`Todo field "${field}"\\[0\\]`), requests);
      });
    }

    // One level up from the field guards: a successful GET can return a
    // scalar, an array or null, and reading a property off null throws a raw
    // TypeError instead of the documented statusless api_error.
    it.each([
      ["array", []],
      ["string", "todo"],
      ["number", 42],
      ["null", null],
      ["boolean", true],
    ])("update refuses a %s response body before writing", async (_label, body) => {
      const requests: string[] = [];
      serve(body, requests);

      const error = await rejection(client.todos.update(42, { content: "New title" }));
      expectResponseError(error, /GetTodo returned/, requests);
    });
  });
});
