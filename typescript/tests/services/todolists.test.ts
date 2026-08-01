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

  // PUT /{accountId}/todolists/{id} is a full replace: BC3's
  // TodolistsController#update rebuilds the recordable from only the permitted
  // params, so an omitted description is erased and an omitted name is a 422.
  // `update` and `edit` are the merge-safe composites over that endpoint;
  // `replace` is the raw verbatim PUT.
  const describedTodolist = (id = 42, overrides: Record<string, unknown> = {}) => ({
    ...sampleTodolist(id),
    name: "Hardware",
    description: "<p>Ship the hardware</p>",
    ...overrides,
  });

  // A malformed writable field must abort before the PUT, never be forwarded.
  //
  // `?? ""` coalesces only null and undefined, so it rules out *erasure* while
  // leaving *corruption* wide open: a number, boolean, array or object rides
  // through verbatim into the full-replace PUT and overwrites the real value.
  // TypeScript has no runtime decoder to catch this — `schema.d.ts` is erased
  // at build time, so the GET's type is a compile-time claim nothing checks.
  // That places this composite with Python and Ruby, not with Go and Swift.
  // Shipped Todos/Cards analogues: #576.
  describe("malformed writable fields", () => {
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

    it.each(malformed)("update refuses a %s description before writing", async (_label, value) => {
      const id = 42;
      const requests: string[] = [];

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(describedTodolist(id, { description: value }));
        }),
        http.put(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("PUT");
          return HttpResponse.json(describedTodolist(id));
        })
      );

      await expect(client.todolists.update(id, { name: "Renamed list" })).rejects.toThrow(
        /todolist description is not a string/
      );
      expect(requests).toEqual(["GET"]);
    });

    it.each(malformed)("edit refuses a %s name before writing", async (_label, value) => {
      const id = 42;
      const requests: string[] = [];

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(describedTodolist(id, { name: value }));
        }),
        http.put(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("PUT");
          return HttpResponse.json(describedTodolist(id));
        })
      );

      await expect(
        client.todolists.edit(id, (t) => {
          t.description = "<p>New</p>";
        })
      ).rejects.toThrow(/todolist name is not a string/);
      expect(requests).toEqual(["GET"]);
    });

    it.each([
      ["absent", undefined],
      ["null", null],
    ])("treats an %s description as genuinely empty", async (_label, value) => {
      const id = 42;
      let putBody: Record<string, unknown> = {};
      const body = describedTodolist(id);
      if (value === undefined) delete (body as Record<string, unknown>).description;
      else (body as Record<string, unknown>).description = value;

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => HttpResponse.json(body)),
        http.put(`${BASE_URL}/todolists/${id}`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(describedTodolist(id));
        })
      );

      await client.todolists.update(id, { name: "Renamed list" });

      expect(putBody).toEqual({ name: "Renamed list", description: "" });
    });
  });

  describe("update", () => {
    it("preserves the description when only the name is set", async () => {
      const id = 42;
      const requests: string[] = [];
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(describedTodolist(id));
        }),
        http.put(`${BASE_URL}/todolists/${id}`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(describedTodolist(id, { name: "Renamed list" }));
        })
      );

      const todolist = await client.todolists.update(id, { name: "Renamed list" });

      expect(requests).toEqual(["GET", "PUT"]);
      expect(putBody).toEqual({
        name: "Renamed list",
        description: "<p>Ship the hardware</p>",
      });
      expect(todolist.id).toBe(id);
      expect(todolist.name).toBe("Renamed list");
    });

    it("preserves the name when only the description is set", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/todolists/42`, () => HttpResponse.json(describedTodolist())),
        http.put(`${BASE_URL}/todolists/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(describedTodolist());
        })
      );

      await client.todolists.update(42, { description: "<p>Rewritten</p>" });

      expect(putBody).toEqual({ name: "Hardware", description: "<p>Rewritten</p>" });
    });

    it("clears the description with an explicit empty string — present, never null", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/todolists/42`, () => HttpResponse.json(describedTodolist())),
        http.put(`${BASE_URL}/todolists/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(describedTodolist(42, { description: "" }));
        })
      );

      await client.todolists.update(42, { description: "" });

      expect(putBody).toHaveProperty("description");
      expect(putBody.description).toBe("");
      expect(putBody.description).not.toBeNull();
      expect(putBody.name).toBe("Hardware");
    });

    it("carries the writable fields over for a group without sniffing the variant", async () => {
      // The route serves todolist groups too, and BC3 answers with the group's
      // flat JSON (no groups_url, a group_position_url instead). The composite
      // reads {name, description} and must not branch on the variant.
      const { groups_url: _groupsUrl, ...groupShaped } = describedTodolist(42, {
        name: "Peripherals",
      });
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/todolists/42`, () =>
          HttpResponse.json({
            ...groupShaped,
            group_position_url: "https://3.basecampapi.com/12345/buckets/1/todolists/42/position.json",
          })
        ),
        http.put(`${BASE_URL}/todolists/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(groupShaped);
        })
      );

      await client.todolists.update(42, { name: "Renamed group" });

      expect(putBody).toEqual({
        name: "Renamed group",
        description: "<p>Ship the hardware</p>",
      });
    });

    it("hooks observe the wire operations GetTodolistOrGroup then UpdateTodolistOrGroup", async () => {
      const operations: OperationInfo[] = [];
      const hookedClient = createBasecampClient({
        accountId: "12345",
        accessToken: "test-token",
        enableRetry: false,
        hooks: {
          onOperationStart: (info) => {
            operations.push(info);
          },
        },
      });

      server.use(
        http.get(`${BASE_URL}/todolists/42`, () => HttpResponse.json(describedTodolist())),
        http.put(`${BASE_URL}/todolists/42`, () => HttpResponse.json(describedTodolist()))
      );

      await hookedClient.todolists.update(42, { name: "observed" });

      expect(operations.map((o) => o.operation)).toEqual([
        "GetTodolistOrGroup",
        "UpdateTodolistOrGroup",
      ]);
      // Scoped to the todolist id (unsuffixed {id} path param) on both legs.
      expect(operations.map((o) => o.resourceId)).toEqual([42, 42]);
    });
  });

  describe("edit", () => {
    it("hands the callback current state and PUTs everything back", async () => {
      let putBody: Record<string, unknown> = {};
      const requests: string[] = [];

      server.use(
        http.get(`${BASE_URL}/todolists/42`, () => {
          requests.push("GET");
          return HttpResponse.json(describedTodolist());
        }),
        http.put(`${BASE_URL}/todolists/42`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(describedTodolist(42, { name: "🚨 Hardware" }));
        })
      );

      const todolist = await client.todolists.edit(42, (t) => {
        expect(t.name).toBe("Hardware");
        expect(t.description).toBe("<p>Ship the hardware</p>");
        t.name = `🚨 ${t.name}`;
      });

      expect(requests).toEqual(["GET", "PUT"]);
      expect(putBody).toEqual({
        name: "🚨 Hardware",
        description: "<p>Ship the hardware</p>",
      });
      expect(todolist.name).toBe("🚨 Hardware");
    });

    it("clears the description by setting it empty — present-and-empty in the PUT body", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/todolists/42`, () => HttpResponse.json(describedTodolist())),
        http.put(`${BASE_URL}/todolists/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(describedTodolist(42, { description: "" }));
        })
      );

      await client.todolists.edit(42, (t) => {
        t.description = "";
      });

      expect(putBody).toEqual({ name: "Hardware", description: "" });
      expect(putBody.description).not.toBeNull();
    });

    it("supports async callbacks", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/todolists/42`, () => HttpResponse.json(describedTodolist())),
        http.put(`${BASE_URL}/todolists/42`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(describedTodolist());
        })
      );

      await client.todolists.edit(42, async (t) => {
        await Promise.resolve();
        t.name = "Async rename";
      });

      expect(putBody.name).toBe("Async rename");
    });

    it("aborts without a PUT when the callback throws", async () => {
      let putCount = 0;
      server.use(
        http.get(`${BASE_URL}/todolists/42`, () => HttpResponse.json(describedTodolist())),
        http.put(`${BASE_URL}/todolists/42`, () => {
          putCount++;
          return HttpResponse.json(describedTodolist());
        })
      );

      await expect(
        client.todolists.edit(42, () => {
          throw new Error("abort before the write");
        })
      ).rejects.toThrow("abort before the write");
      expect(putCount).toBe(0);
    });

    it("aborts without a PUT when the callback rejects", async () => {
      let putCount = 0;
      server.use(
        http.get(`${BASE_URL}/todolists/42`, () => HttpResponse.json(describedTodolist())),
        http.put(`${BASE_URL}/todolists/42`, () => {
          putCount++;
          return HttpResponse.json(describedTodolist());
        })
      );

      await expect(
        client.todolists.edit(42, async () => {
          await Promise.resolve();
          throw new Error("async abort");
        })
      ).rejects.toThrow("async abort");
      expect(putCount).toBe(0);
    });

    it("rejects an emptied name before the PUT — the server presence-validates it", async () => {
      let putCount = 0;
      server.use(
        http.get(`${BASE_URL}/todolists/42`, () => HttpResponse.json(describedTodolist())),
        http.put(`${BASE_URL}/todolists/42`, () => {
          putCount++;
          return HttpResponse.json(describedTodolist());
        })
      );

      const error = await client.todolists
        .edit(42, (t) => {
          t.name = "";
        })
        .catch((e: unknown) => e);

      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("validation");
      expect((error as BasecampError).message).toBe("Name is required");
      expect(putCount).toBe(0);
    });
  });

  describe("replace", () => {
    it("sends the request verbatim in a single PUT — no GET, omissions kept", async () => {
      const requests: string[] = [];
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/todolists/42`, () => {
          requests.push("GET");
          return HttpResponse.json(describedTodolist());
        }),
        http.put(`${BASE_URL}/todolists/42`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(describedTodolist(42, { name: "The whole new list", description: "" }));
        })
      );

      await client.todolists.replace(42, { name: "The whole new list" });

      expect(requests).toEqual(["PUT"]);
      expect(putBody).toEqual({ name: "The whole new list" });
      expect(putBody).not.toHaveProperty("description");
    });

    it("rejects a missing name without a request", async () => {
      let putCount = 0;
      server.use(
        http.put(`${BASE_URL}/todolists/42`, () => {
          putCount++;
          return HttpResponse.json(describedTodolist());
        })
      );

      const error = await client.todolists.replace(42, { name: "" }).catch((e: unknown) => e);

      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("validation");
      expect((error as BasecampError).message).toBe("Name is required");
      expect(putCount).toBe(0);
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

      await hookedClient.todolists.replace(id, { name: "Updated list" });

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
