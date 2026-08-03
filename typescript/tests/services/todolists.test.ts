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
import groupFixture from "../../../spec/fixtures/todolist_groups/get.json";
import groupListFixture from "../../../spec/fixtures/todolist_groups/list.json";
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

  // #544: `Todolist`, `TodolistGroup` and the `TodolistOrGroup` union are one
  // flat structure. A group IS a to-do list — BC3's
  // todolists/groups/{index,show}.json.jbuilder render
  // todolists/_todolist.json.jbuilder — so both routes decode into the same
  // shape and a group carries its own description, which the old group
  // projection modelled away. Discrimination is structural: groups_url (parent
  // is a Todoset) XOR group_position_url (parent is a Todolist), never the
  // `type` string, which reads "Todolist" for both.
  //
  // Both stubs are the shared, coverage-guarded group fixtures, so they cannot
  // drift from the validated shape. The list case lives here rather than in
  // todolistGroups.test.ts because what it pins is this one shape serving both
  // routes.
  describe("flat todolist/group shape", () => {
    it("get returns a group with its description and group_position_url intact", async () => {
      const id = groupFixture.id;

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => HttpResponse.json(groupFixture))
      );

      const group = await client.todolists.get(id);

      expect(group.description).toBe(groupFixture.description);
      expect(group.description_attachments).toEqual([]);
      expect(group.group_position_url).toBe(groupFixture.group_position_url);
      // The group half of the structural discriminator is exclusive: a group's
      // parent is a Todolist, so it has no groups_url.
      expect(group).not.toHaveProperty("groups_url");
      expect(group.type).toBe("Todolist");
    });

    it("lists groups as an array of that same flat shape", async () => {
      const todolistId = todolistFixture.id;

      server.use(
        http.get(`${BASE_URL}/todolists/${todolistId}/groups.json`, () =>
          HttpResponse.json(groupListFixture)
        )
      );

      const groups = await client.todolistGroups.list(todolistId);

      expect(groups).toHaveLength(groupListFixture.length);
      expect(groups[0]!.description).toBe(groupListFixture[0]!.description);
      expect(groups[0]!.group_position_url).toBe(groupListFixture[0]!.group_position_url);
      expect(groups[0]!.type).toBe("Todolist");
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
  //
  // #544 flattened the wire shape, which removed the *envelope-arm* rung of the
  // read path (`{ todolist } | { group }`) and nothing else: a flat Smithy shape
  // changes what the API returns, it does not hand TypeScript a decoder it never
  // had. The path is object → scalar now — the body, then each writable field —
  // and both rungs are exercised below. Structural safety for this SDK is #578.
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

    // Asserting only the message and the request sequence is vacuous about the
    // taxonomy: a wrong `code` satisfies both. The value arrived in a
    // successful API response, so this is `api_error` — the caller passed
    // nothing wrong. (The empty-name path is the opposite and stays a caller
    // error, since BC3 presence-validates `name`.) Pin all four properties.
    const expectResponseError = (error: unknown, field: string, requests: string[]) => {
      expect(error).toBeInstanceOf(BasecampError);
      const basecampError = error as BasecampError;
      expect(basecampError.code).toBe("api_error");
      expect(basecampError.message).toMatch(new RegExp(`todolist ${field} is not a string`));
      expect(requests).toEqual(["GET"]);
    };

    const rejection = async (promise: Promise<unknown>): Promise<unknown> =>
      promise.then(
        () => {
          throw new Error("expected the call to reject, but it resolved");
        },
        (error: unknown) => error
      );

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

      const error = await rejection(client.todolists.update(id, { name: "Renamed list" }));
      expectResponseError(error, "description", requests);
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

      const error = await rejection(
        client.todolists.edit(id, (t) => {
          t.description = "<p>New</p>";
        })
      );
      expectResponseError(error, "name", requests);
    });

    // `name` is required and presence-validated, so absent, null and "" from
    // the wire are all malformed. Classification is by ORIGIN: this name came
    // off the wire, so it is api_error. Before the fix, absent/null collapsed
    // to "" and that empty name was PUT over the real one, successfully and
    // silently — the only instance in this family that corrupted data on the
    // wire rather than misclassifying or swallowing.
    it.each([
      ["absent", undefined],
      ["null", null],
      ["empty", ""],
    ])("update refuses an %s name from the response", async (_label, value) => {
      const id = 42;
      const requests: string[] = [];
      const body = describedTodolist(id) as Record<string, unknown>;
      if (value === undefined) delete body.name;
      else body.name = value;

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(body);
        }),
        http.put(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("PUT");
          return HttpResponse.json(describedTodolist(id));
        })
      );

      const error = await rejection(
        client.todolists.update(id, { description: "<p>New</p>" })
      );
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("api_error");
      expect((error as BasecampError).message).toMatch(/todolist name is (missing|null|empty)/);
      expect(requests).toEqual(["GET"]);
    });

    // The same via the edit closure, which is the path that actually wrote the
    // empty name: a callback deriving from `t.name` PUT it over the real one.
    it("edit refuses an absent name from the response", async () => {
      const id = 42;
      const requests: string[] = [];
      const body = describedTodolist(id) as Record<string, unknown>;
      delete body.name;

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(body);
        }),
        http.put(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("PUT");
          return HttpResponse.json(describedTodolist(id));
        })
      );

      const error = await rejection(
        client.todolists.edit(id, (t) => {
          t.name = `${t.name} (revised)`;
        })
      );
      expect((error as BasecampError).code).toBe("api_error");
      expect(requests).toEqual(["GET"]);
    });

    // The mirror of the read step: caller provenance, so `usage`. The
    // TodolistFields annotation is erased at build time, so a closure assigning
    // a non-string — trivially reachable from plain JS or via `as any` — would
    // otherwise walk straight into the full-replace PUT.
    it.each([
      ["number", 42],
      ["boolean", true],
      ["array", []],
      ["object", { a: 1 }],
      ["zero", 0],
      ["false", false],
    ])("edit refuses a caller-supplied %s", async (_label, bad) => {
      const id = 42;
      const requests: string[] = [];

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(describedTodolist(id));
        }),
        http.put(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("PUT");
          return HttpResponse.json(describedTodolist(id));
        })
      );

      const error = await rejection(
        client.todolists.edit(id, (t) => {
          (t as unknown as Record<string, unknown>).description = bad;
        })
      );
      expect((error as BasecampError).code).toBe("usage");
      expect((error as BasecampError).message).toMatch(/must be a string/);
      expect(requests).toEqual(["GET"]);
    });

    // The mirror case: same value, caller origin, so `usage` not `api_error`.
    it("a caller-supplied empty name is a usage error", async () => {
      const id = 42;
      const requests: string[] = [];

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(describedTodolist(id));
        }),
        http.put(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("PUT");
          return HttpResponse.json(describedTodolist(id));
        })
      );

      const error = await rejection(client.todolists.update(id, { name: "" }));
      expect((error as BasecampError).code).toBe("usage");
      expect(requests).toEqual(["GET"]);
    });

    // Rung 1: the malformed *body*, one level up from malformed fields. `null`
    // makes the very next field read throw a raw TypeError, while a scalar or an
    // array answers `undefined` for every key — which without this guard is
    // misdiagnosed one rung down as "name is missing from the response", a
    // body-shape fault reported as a field fault. The array is the case that
    // used to fall through silently (`"todolist" in response` returns FALSE on an
    // array), and it is still the sharp one now that the arms are gone: an array
    // IS an object to `typeof`, so only the explicit `Array.isArray` check keeps
    // it out. Hence the negative assertion — landing on the field message would
    // mean the body guard had stopped covering it.
    it.each([
      ["number", 42],
      ["string", "nope"],
      ["null", null],
      ["array", ["a"]],
      // A list body served on the single-record route. Kept small: the message
      // embeds the value and SPEC section 9 caps it at 500 units, so a full
      // fixture would truncate the assertion's own suffix away.
      ["array holding a todolist", [{ id: 42, name: "Hardware", description: "<p>Ship it</p>" }]],
      ["boolean", true],
    ])("refuses a %s response body", async (_label, body) => {
      const id = 42;
      const requests: string[] = [];
      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(body);
        }),
        http.put(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("PUT");
          return HttpResponse.json(describedTodolist(id));
        })
      );

      const error = await rejection(client.todolists.update(id, { name: "Renamed" }));
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("api_error");
      expect((error as BasecampError).message).toMatch(/where a todolist object was expected/);
      expect((error as BasecampError).message).not.toMatch(/is missing from the response/);
      expect(requests).toEqual(["GET"]);
    });

    // The arms are gone (#544) and nothing unwraps any more, so a body still
    // shaped like the old envelope is just an object with no writable fields at
    // the top level — and the required-field rung refuses it. Two of these were
    // live defects under the old three-rung path: `{ todolist: {...} }` was
    // unwrapped and its interior written back, and `{ todolist: null }` was
    // dereferenced into a native TypeError. Both must now fail as an ordinary
    // malformed todolist, before any PUT.
    it.each([
      [
        "a todolist envelope",
        { todolist: { name: "Hardware", description: "<p>Ship the hardware</p>" } },
      ],
      ["a group envelope", { group: { name: "Peripherals", description: "<p>Cables</p>" } }],
      ["a null todolist arm", { todolist: null }],
      ["a null group arm", { group: null }],
    ])("refuses %s rather than unwrapping it", async (_label, body) => {
      const id = 42;
      const requests: string[] = [];
      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(body);
        }),
        http.put(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("PUT");
          return HttpResponse.json(describedTodolist(id));
        })
      );

      const error = await rejection(client.todolists.update(id, { name: "Renamed" }));
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("api_error");
      expect((error as BasecampError).message).toMatch(/todolist name is missing from the response/);
      expect(requests).toEqual(["GET"]);
    });

    // Row 10: the guard's own error path must not throw. JSON.stringify raises
    // TypeError on a circular structure, and a value can carry a toJSON that
    // throws — either would replace the clean error with an incidental one.
    it("reports a circular caller value without throwing a TypeError", async () => {
      const id = 42;
      const circular: Record<string, unknown> = {};
      circular.self = circular;

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => HttpResponse.json(describedTodolist(id)))
      );

      const error = await rejection(
        client.todolists.edit(id, (t) => {
          (t as unknown as Record<string, unknown>).description = circular;
        })
      );
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("usage");
      expect((error as BasecampError).message).toMatch(/must be a string, got object/);
    });

    it("reports a value whose toJSON throws without losing the diagnosis", async () => {
      const id = 42;
      const hostile = {
        toJSON() {
          throw new Error("nope");
        },
      };

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => HttpResponse.json(describedTodolist(id)))
      );

      const error = await rejection(
        client.todolists.edit(id, (t) => {
          (t as unknown as Record<string, unknown>).description = hostile;
        })
      );
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("usage");
    });

    // The inverse of the refusal this replaces. The old `group` arm modelled no
    // description, so the composite refused it outright rather than invent an
    // empty one and erase the real value on the full-replace PUT. A flat group
    // carries its description like any list, so there is nothing to refuse and
    // nothing to invent: the real description is read and resent verbatim.
    it("preserves a flat group's description rather than inventing one", async () => {
      const id = groupFixture.id;
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => HttpResponse.json(groupFixture)),
        http.put(`${BASE_URL}/todolists/${id}`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ ...groupFixture, name: "Renamed group" });
        })
      );

      await client.todolists.update(id, { name: "Renamed group" });

      expect(putBody).toEqual({
        name: "Renamed group",
        description: groupFixture.description,
      });
      expect(putBody.description).not.toBe("");
    });

    // SPEC section 9 caps error messages at 500 units; the malformed value is
    // embedded in the message, so a huge body must not blow past it.
    it("caps the message at the SPEC section 9 limit", async () => {
      const id = 42;
      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () =>
          HttpResponse.json(describedTodolist(id, { description: Array(50_000).fill("x") }))
        )
      );

      const error = await rejection(client.todolists.update(id, { name: "Renamed" }));
      expect((error as BasecampError).message.length).toBeLessThanOrEqual(500);
      expect((error as BasecampError).hint).toBeTruthy();
      expect((error as BasecampError).retryable).toBe(false);
    });

    // `description` is @required and never null since #544 — BC3's
    // `format_api_content` funnels a blank rich text through `call_pipeline`,
    // which returns `""` — so an absent key and an explicit `null` are both
    // malformed, exactly as for `name`. This is the data-loss case these
    // composites exist to remove: the old read collapsed either to `""`, and
    // the full-replace PUT then wrote that `""` over the record's real
    // description on a call that only renamed it. Refuse before the PUT.
    it.each([
      ["absent", undefined],
      ["null", null],
    ])("update refuses an %s description before writing", async (_label, value) => {
      const id = 42;
      const requests: string[] = [];
      const body = describedTodolist(id) as Record<string, unknown>;
      if (value === undefined) delete body.description;
      else body.description = value;

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(body);
        }),
        http.put(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("PUT");
          return HttpResponse.json(describedTodolist(id));
        })
      );

      const error = await rejection(client.todolists.update(id, { name: "Renamed list" }));
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).code).toBe("api_error");
      expect((error as BasecampError).message).toMatch(
        /todolist description is (missing from|null in) the response/
      );
      expect((error as BasecampError).retryable).toBe(false);
      expect(requests).toEqual(["GET"]);
    });

    // The same through the edit closure, which is the path that hands the value
    // to caller code: the read must fail before the closure ever sees a
    // description the server never sent.
    it.each([
      ["absent", undefined],
      ["null", null],
    ])("edit refuses an %s description before the callback runs", async (_label, value) => {
      const id = 42;
      const requests: string[] = [];
      let called = false;
      const body = describedTodolist(id) as Record<string, unknown>;
      if (value === undefined) delete body.description;
      else body.description = value;

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(body);
        }),
        http.put(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("PUT");
          return HttpResponse.json(describedTodolist(id));
        })
      );

      const error = await rejection(
        client.todolists.edit(id, (t) => {
          called = true;
          t.name = `${t.name} (revised)`;
        })
      );
      expect((error as BasecampError).code).toBe("api_error");
      expect(called).toBe(false);
      expect(requests).toEqual(["GET"]);
    });

    // The case those refusals must not swallow, and by far the common one: a
    // description-less list carries a present-and-empty description. `""` is a
    // real value, so it round-trips and reaches the PUT — refusing it would
    // break every list without a description.
    it("preserves a present-and-empty description through the round trip", async () => {
      const id = 42;
      const requests: string[] = [];
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/todolists/${id}`, () => {
          requests.push("GET");
          return HttpResponse.json(describedTodolist(id, { description: "" }));
        }),
        http.put(`${BASE_URL}/todolists/${id}`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(describedTodolist(id, { description: "" }));
        })
      );

      await client.todolists.update(id, { name: "Renamed list" });

      expect(requests).toEqual(["GET", "PUT"]);
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
      // The route serves todolist groups too, and since #544 a group is
      // literally the same structure — no groups_url, a group_position_url
      // instead, and a description of its own. The composite reads
      // {name, description} and must not branch on the variant.
      const { groups_url: _groupsUrl, ...groupShaped } = describedTodolist(42, {
        name: "Peripherals",
      });
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/todolists/42`, () =>
          HttpResponse.json({
            ...groupShaped,
            group_position_url: "https://3.basecampapi.com/12345/buckets/1/todolists/groups/42/position.json",
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
      // `usage`, not `validation`. The composite now owns the caller-emptied
      // name so it can classify by origin, which also aligns TypeScript with the
      // other four SDKs (Python/Ruby UsageError, Kotlin Usage, Swift usage).
      // Previously TS was the odd one out, inheriting `validation` from the
      // generated `replace()` — a third code for the same condition. That
      // generated error still governs direct `replace()` callers, where the name
      // really is the caller's (asserted separately below).
      expect((error as BasecampError).code).toBe("usage");
      expect((error as BasecampError).message).toMatch(/name must not be empty/);
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
