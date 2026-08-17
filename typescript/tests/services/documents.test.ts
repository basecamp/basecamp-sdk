/**
 * Tests for the Documents service.
 *
 * Notes:
 * - Client-side check: create() rejects a missing title; the API validates the rest
 * - No domain-specific trash() (use recordings.trash())
 * - The write surface is a triad: `replace` is the generated full-replace PUT,
 *   `update` and `edit` are the merge-safe composites layered over it in
 *   `services/documents-extensions.ts`.
 *
 * `PUT /documents/{id}` is a full replace: BC3 rebuilds the Document from the
 * permitted params and swaps the recordable wholesale. The writable set is
 * exactly `{title, content}` and **both are optional** — omitting `title` is a
 * 200 that leaves the document reading back as "Untitled", omitting `content`
 * is a 200 that clears it. Neither omission is a 422, so nothing on the wire
 * tells you the sparse PUT went wrong; only the next GET does. That is what
 * `update` and `edit` exist to prevent, and what these tests pin: every PUT
 * they issue names both fields, empties included, never JSON null.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import type { JsonBodyType } from "msw";
import { server } from "../setup.js";
import type { DocumentsService } from "../../src/services/documents-extensions.js";
import { BasecampError } from "../../src/errors.js";
import { createBasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

/** A GET-shaped Document, with the two writable fields the triad round-trips. */
const sampleDocument = (id = 5001, overrides: Record<string, unknown> = {}) => ({
  id,
  title: "Project Overview",
  content: "<div>The plan so far.</div>",
  content_attachments: [],
  status: "active",
  created_at: "2022-11-22T08:30:00.000Z",
  updated_at: "2022-11-22T08:30:00.000Z",
  ...overrides,
});

describe("DocumentsService", () => {
  let service: DocumentsService;

  beforeEach(() => {
    vi.clearAllMocks();
    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
    service = client.documents;
  });

  describe("get", () => {
    it("should return a document by ID", async () => {
      const document = {
        id: 5001,
        title: "Meeting Notes",
        content: "<p>Notes from the meeting...</p>",
        content_attachments: [],
        status: "active",
        comments_count: 3,
      };

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => {
          return HttpResponse.json(document);
        }),
      );

      const result = await service.get(5001);

      expect(result.id).toBe(5001);
      expect(result.title).toBe("Meeting Notes");
      expect(result.content).toContain("Notes from the meeting");
    });

    it("should throw not_found error for 404 response", async () => {
      server.use(
        http.get(`${BASE_URL}/documents/9999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        }),
      );

      await expect(service.get(9999)).rejects.toThrow(BasecampError);

      try {
        await service.get(9999);
      } catch (err) {
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });

  describe("list", () => {
    it("should return documents in a vault", async () => {
      const documents = [
        { id: 5001, title: "Document 1", status: "active" },
        { id: 5002, title: "Document 2", status: "active" },
      ];

      server.use(
        http.get(`${BASE_URL}/vaults/1001/documents.json`, () => {
          return HttpResponse.json(documents);
        }),
      );

      const result = await service.list(1001);

      expect(result).toHaveLength(2);
      expect(result[0].title).toBe("Document 1");
      expect(result[1].title).toBe("Document 2");
    });

    it("should return empty array when no documents", async () => {
      server.use(
        http.get(`${BASE_URL}/vaults/1001/documents.json`, () => {
          return HttpResponse.json([]);
        }),
      );

      const result = await service.list(1001);

      expect(result).toHaveLength(0);
    });
  });

  describe("create", () => {
    it("should create a new document", async () => {
      const newDocument = {
        id: 6001,
        title: "New Document",
        content: "<p>Content here</p>",
        content_attachments: [],
        status: "active",
      };

      server.use(
        http.post(`${BASE_URL}/vaults/1001/documents.json`, () => {
          return HttpResponse.json(newDocument);
        }),
      );

      const result = await service.create(1001, {
        title: "New Document",
        content: "<p>Content here</p>",
      });

      expect(result.id).toBe(6001);
      expect(result.title).toBe("New Document");
    });

    it("should pass subscriptions in request body", async () => {
      server.use(
        http.post(
          `${BASE_URL}/vaults/1001/documents.json`,
          async ({ request }) => {
            const body = (await request.json()) as Record<string, unknown>;
            expect(body.subscriptions).toEqual([111, 222]);
            return HttpResponse.json({ id: 6002, title: "Test" });
          },
        ),
      );

      const result = await service.create(1001, {
        title: "Quiet Doc",
        subscriptions: [111, 222],
      });
      expect(result.id).toBe(6002);
    });

    it("should send all fields in request body", async () => {
      // Held in an object rather than a bare `let`: TS's control-flow analysis
      // cannot see the assignment inside the MSW handler closure, so a
      // `let x: T | null = null` narrows to `null` at the assertions below and
      // every property read becomes an error on `never`. A property of a const
      // object is not narrowed that way.
      const captured: {
        body?: { title?: string; content?: string; status?: string };
      } = {};

      server.use(
        http.post(
          `${BASE_URL}/vaults/1001/documents.json`,
          async ({ request }) => {
            captured.body = (await request.json()) as {
              title?: string;
              content?: string;
              status?: string;
            };
            return HttpResponse.json({ id: 1, title: "Test" });
          },
        ),
      );

      await service.create(1001, {
        title: "Test Doc",
        content: "<h1>Hello</h1>",
        status: "drafted",
      });

      expect(captured.body?.title).toBe("Test Doc");
      expect(captured.body?.content).toBe("<h1>Hello</h1>");
      expect(captured.body?.status).toBe("drafted");
    });

    // Client-side validation short-circuits before any HTTP call. No MSW handler
    // is registered here, so a leaked request fails via onUnhandledRequest: "error".
    it("rejects a missing title", async () => {
      await expect(
        service.create(1001, { title: "" })
      ).rejects.toMatchObject({ code: "validation", message: "Title is required" });
    });
  });

  describe("replace", () => {
    it("sends the sparse request verbatim with no GET", async () => {
      const requests: string[] = [];
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => {
          requests.push("GET");
          return HttpResponse.json(sampleDocument());
        }),
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleDocument(5001, { title: "the whole new document" }));
        }),
      );

      const result = await service.replace(5001, { title: "the whole new document" });

      expect(result.id).toBe(5001);
      // replace is the deliberate overwrite — it reads nothing first.
      expect(requests).toEqual(["PUT"]);
      expect(putBody.title).toBe("the whole new document");
      // The unset field is omitted and the server clears it. That is the whole
      // reason `update`/`edit` exist.
      expect(putBody).not.toHaveProperty("content");
    });

    it("sends both fields when both are given", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(
            sampleDocument(5001, { title: "Updated Title", content: "<p>Updated content</p>" }),
          );
        }),
      );

      const result = await service.replace(5001, {
        title: "Updated Title",
        content: "<p>Updated content</p>",
      });

      expect(result.title).toBe("Updated Title");
      expect(result.content).toContain("Updated content");
      expect(putBody.title).toBe("Updated Title");
      expect(putBody.content).toBe("<p>Updated content</p>");
    });

    it("surfaces a 404 as BasecampError", async () => {
      server.use(
        http.put(`${BASE_URL}/documents/9999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        }),
      );

      await expect(service.replace(9999, { title: "gone" })).rejects.toThrow(BasecampError);
    });
  });

  describe("update", () => {
    it("merges: an omitted content is preserved from the GET", async () => {
      const requests: string[] = [];
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => {
          requests.push("GET");
          return HttpResponse.json(sampleDocument());
        }),
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleDocument(5001, { title: "Q3 Plan" }));
        }),
      );

      const document = await service.update(5001, { title: "Q3 Plan" });

      expect(document.id).toBe(5001);
      expect(requests).toEqual(["GET", "PUT"]);
      expect(putBody.title).toBe("Q3 Plan");
      // The field the caller never mentioned rides back verbatim. A sparse PUT
      // here would have been a silent 200 that erased it.
      expect(putBody.content).toBe("<div>The plan so far.</div>");
    });

    it("merges: an omitted title is preserved from the GET", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleDocument());
        }),
      );

      await service.update(5001, { content: "<div>Rewritten.</div>" });

      expect(putBody.content).toBe("<div>Rewritten.</div>");
      // Omitting title on the wire is a 200 that leaves the document titled
      // "Untitled" — never a 422 — so preservation is the only defence.
      expect(putBody.title).toBe("Project Overview");
    });

    it("clears content with an explicitly-passed empty string", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleDocument());
        }),
      );

      await service.update(5001, { content: "" });

      // A clear is an empty string, never an omission and never JSON null.
      expect(putBody).toHaveProperty("content");
      expect(putBody.content).toBe("");
      expect(putBody.title).toBe("Project Overview");
    });

    it("clears title with an explicitly-passed empty string", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleDocument());
        }),
      );

      await service.update(5001, { title: "" });

      expect(putBody.title).toBe("");
      expect(putBody.content).toBe("<div>The plan so far.</div>");
    });

    it("names exactly title and content, never JSON null", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleDocument());
        }),
      );

      await service.update(5001, { title: "Q3 Plan", content: "" });

      expect(Object.keys(putBody).sort()).toEqual(["content", "title"]);
      expect(Object.values(putBody).every((v) => v !== null)).toBe(true);
    });

    it("hooks observe the wire operations GetDocument then ReplaceDocument", async () => {
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
        http.get(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
        http.put(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
      );

      await hookedClient.documents.update(5001, { title: "observed" });

      // The composite is not a synthetic operation: hooks see the two real ones.
      expect(operations).toEqual(["GetDocument", "ReplaceDocument"]);
    });
  });

  describe("edit", () => {
    it("hands the callback current state and PUTs everything back", async () => {
      const requests: string[] = [];
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => {
          requests.push("GET");
          return HttpResponse.json(sampleDocument());
        }),
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          requests.push("PUT");
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleDocument());
        }),
      );

      const document = await service.edit(5001, (d) => {
        expect(d.title).toBe("Project Overview");
        expect(d.content).toBe("<div>The plan so far.</div>");
        d.title = `🚨 ${d.title}`;
      });

      expect(document.id).toBe(5001);
      expect(requests).toEqual(["GET", "PUT"]);
      expect(putBody.title).toBe("🚨 Project Overview");
      expect(putBody.content).toBe("<div>The plan so far.</div>");
    });

    it("clears content by setting it empty — present-and-empty in the PUT body", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleDocument());
        }),
      );

      await service.edit(5001, (d) => {
        d.content = "";
      });

      // Present and empty, not omitted: on a full-replace endpoint an omission
      // is the server's own clear-by-default and reads as an accident.
      expect(putBody).toHaveProperty("content");
      expect(putBody.content).toBe("");
      expect(putBody.title).toBe("Project Overview");
    });

    it("clears title by setting it empty — present-and-empty in the PUT body", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleDocument());
        }),
      );

      await service.edit(5001, (d) => {
        d.title = "";
      });

      expect(putBody).toHaveProperty("title");
      expect(putBody.title).toBe("");
      expect(putBody.content).toBe("<div>The plan so far.</div>");
    });

    it("aborts without a PUT when the callback throws", async () => {
      let putCount = 0;

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
        http.put(`${BASE_URL}/documents/5001`, () => {
          putCount++;
          return HttpResponse.json(sampleDocument());
        }),
      );

      await expect(
        service.edit(5001, () => {
          throw new Error("abort");
        }),
      ).rejects.toThrow("abort");
      expect(putCount).toBe(0);
    });

    it("supports async callbacks", async () => {
      let putBody: Record<string, unknown> = {};

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleDocument());
        }),
      );

      await service.edit(5001, async (d) => {
        d.content = await Promise.resolve("<div>async content</div>");
      });

      expect(putBody.content).toBe("<div>async content</div>");
      expect(putBody.title).toBe("Project Overview");
    });

    it("hooks observe the wire operations GetDocument then ReplaceDocument", async () => {
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
        http.get(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
        http.put(`${BASE_URL}/documents/5001`, () => HttpResponse.json(sampleDocument())),
      );

      await hookedClient.documents.edit(5001, (d) => {
        d.title = "observed";
      });

      expect(operations).toEqual(["GetDocument", "ReplaceDocument"]);
    });
  });

  // --- #576: a malformed GET field must never reach the full-replace PUT ----
  //
  // `update`/`edit` GET the document, read each writable field, and PUT the
  // FULL representation back, so every value read is written -- including one
  // the caller never mentioned. `?? ""` coalesces only null and undefined, so
  // it rules out *erasure* while leaving *corruption* wide open: all eight
  // malformed shapes would ride through VERBATIM into the PUT.
  //
  // TypeScript has no runtime decoder to catch this -- `schema.d.ts` is erased
  // at build time, so `Document` is a compile-time claim nothing validates.
  // That places this composite with Python and Ruby, not with Go and Swift.
  //
  // The assertion that matters is the ORDERING: `requests` must be ["GET"] --
  // exactly one request. A guard that fires after the PUT has already lost the
  // field.
  describe("malformed writable fields (#576)", () => {
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

    const writableStrings = ["title", "content"] as const;

    // Serve a GET carrying `body` and a PUT that records that it happened.
    // `body` is typed as MSW's own response-body type rather than `unknown`:
    // the malformed-*envelope* cases below deliberately serve arrays, scalars
    // and null, so the parameter has to stay as wide as JSON itself -- but no
    // wider, or the `HttpResponse.json` call cannot accept it.
    const serve = (body: JsonBodyType, requests: string[]) => {
      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => {
          requests.push("GET");
          return HttpResponse.json(body);
        }),
        http.put(`${BASE_URL}/documents/5001`, () => {
          requests.push("PUT");
          return HttpResponse.json(sampleDocument());
        }),
      );
    };

    const rejection = async (promise: Promise<unknown>): Promise<unknown> =>
      promise.then(
        () => {
          throw new Error("expected the call to reject, but it resolved");
        },
        (error: unknown) => error,
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
        serve(sampleDocument(5001, { [field]: value }), requests);

        const error = await rejection(service.update(5001, { title: "New title" }));
        expectResponseError(
          error,
          new RegExp(`Document field "${field}" is not a string`),
          requests,
        );
      });

      it(`edit refuses a malformed ${field} before writing`, async () => {
        const requests: string[] = [];
        serve(sampleDocument(5001, { [field]: 42 }), requests);

        const error = await rejection(
          service.edit(5001, (d) => {
            d.title = "New title";
          }),
        );
        expectResponseError(
          error,
          new RegExp(`Document field "${field}" is not a string`),
          requests,
        );
      });

    }

    // The other half of the rule: for an OPTIONAL field, absent and null are
    // not malformed, they are empty. Guarding types must not turn a
    // legitimately blank field into an error. The call sets the other writable
    // string so that `content` is never overwritten by the caller.
    //
    // `content` only. `title` is `@required` in the spec and gets the opposite
    // treatment below.
    it.each([
      ["absent", undefined],
      ["null", null],
    ])("treats a %s content as genuinely empty", async (_label, value) => {
      let putBody: Record<string, unknown> = {};
      const body: Record<string, unknown> = sampleDocument(5001, { content: value });
      if (value === undefined) delete body["content"];

      server.use(
        http.get(`${BASE_URL}/documents/5001`, () => HttpResponse.json(body)),
        http.put(`${BASE_URL}/documents/5001`, async ({ request }) => {
          putBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleDocument());
        }),
      );

      await service.update(5001, { title: "set by the caller" });

      expect(putBody["content"]).toBe("");
      expect(putBody["title"]).toBe("set by the caller");
    });

    // `Document.title` is `@required` in the spec, and BC3 can never render it
    // blank (`Document#title` is `super.presence || "Untitled"`). So an absent
    // or null title in a 2xx body is a MALFORMED RESPONSE, not an empty title
    // — and coalescing it to "" would blank the real title on a call that only
    // touched `content`. Same defect class as a forwarded non-string, in the
    // one shape `?? ""` looks correct.
    it.each([
      ["absent", undefined],
      ["null", null],
      // BC3 can never render a blank title, so "" is malformed too — and it is
      // the shape a missing/null check alone would let through.
      ["blank", ""],
      // BC3 blanks via `presence`, whose blank case includes whitespace-only.
      ["whitespace", "   "],
    ])("update refuses a %s title before writing", async (_label, value) => {
      const requests: string[] = [];
      const body: Record<string, unknown> = sampleDocument(5001, { title: value });
      if (value === undefined) delete body["title"];
      serve(body, requests);

      const error = await rejection(service.update(5001, { content: "<div>New body.</div>" }));
      expectResponseError(error, /Document field "title" is required/, requests);
    });

    it.each([
      ["absent", undefined],
      ["null", null],
      ["blank", ""],
      // BC3 blanks via `presence`, whose blank case includes whitespace-only.
      ["whitespace", "   "],
    ])("edit refuses a %s title before writing", async (_label, value) => {
      const requests: string[] = [];
      const body: Record<string, unknown> = sampleDocument(5001, { title: value });
      if (value === undefined) delete body["title"];
      serve(body, requests);

      const error = await rejection(
        service.edit(5001, (d) => {
          d.content = "<div>New body.</div>";
        }),
      );
      expectResponseError(error, /Document field "title" is required/, requests);
    });

    // One level up from the field guards: a successful GET can return a
    // scalar, an array or null, and reading a property off null throws a raw
    // TypeError instead of the documented statusless api_error.
    it.each([
      ["array", []],
      ["string", "document"],
      ["number", 42],
      ["null", null],
      ["boolean", true],
    ])("update refuses a %s response body before writing", async (_label, body) => {
      const requests: string[] = [];
      serve(body, requests);

      const error = await rejection(service.update(5001, { title: "New title" }));
      expectResponseError(error, /GetDocument returned/, requests);
    });

    it.each([
      ["array", []],
      ["null", null],
    ])("edit refuses a %s response body before writing", async (_label, body) => {
      const requests: string[] = [];
      serve(body, requests);

      const error = await rejection(
        service.edit(5001, (d) => {
          d.title = "New title";
        }),
      );
      expectResponseError(error, /GetDocument returned/, requests);
    });
  });

  // Note: trash() is on RecordingsService, not DocumentsService (spec-conformant)
  // Use client.recordings.trash(documentId) instead
});
