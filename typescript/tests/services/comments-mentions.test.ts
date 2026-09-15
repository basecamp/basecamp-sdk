/**
 * Tests for the mention composites on CommentsService
 * (src/services/comments-extensions.ts).
 *
 * The conformance fixture pins the happy path — one people read, then the
 * write, with the tag placed inside the content's first block. These carry the
 * rest: the read is never skipped on the strength of an unsigned sgid already
 * in the content, a failed lookup posts nothing, and a repeated id costs one
 * read.
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import { CommentsService } from "../../src/index.js";
import { personSGID } from "../helpers/sgid.js";

const BASE_URL = "https://3.basecampapi.com/12345";
const VICTOR = 1049715914;
const ANNIE = 1049715915;

const VICTOR_SGID = personSGID(VICTOR);
const ANNIE_SGID = personSGID(ANNIE);

const attachment = (sgid: string): string => `<bc-attachment sgid="${sgid}"></bc-attachment>`;

const personBody = (id: number, sgid: string) => ({
  id,
  attachable_sgid: sgid,
  name: `Person ${id}`,
  email_address: `p${id}@honchodesign.com`,
  personable_type: "User",
});

const commentBody = (content: string) => ({
  id: 1069479370,
  status: "active",
  visible_to_clients: false,
  created_at: "2022-10-31T14:22:33.169Z",
  updated_at: "2022-10-31T14:22:33.169Z",
  title: "Re: We won Leto!",
  inherits_status: true,
  type: "Comment",
  url: `${BASE_URL}/comments/1069479370.json`,
  app_url: "https://3.basecamp.com/12345/buckets/1/comments/1069479370",
  parent: { id: 1, title: "We won Leto!", type: "Message", url: "", app_url: "" },
  bucket: { id: 1, name: "The Leto Laptop", type: "Project" },
  creator: personBody(VICTOR, VICTOR_SGID),
  content,
  content_attachments: [],
});

/**
 * Records every (method, path) MSW served, in order.
 *
 * Synchronous on purpose: a listener that awaited the request body would push
 * its entry after the assertion ran. Written bodies are captured in the POST
 * handler itself, where the await is part of serving the request.
 */
function trackRequests(): { calls: string[] } {
  const calls: string[] = [];
  server.events.removeAllListeners("request:start");
  server.events.on("request:start", ({ request }) => {
    calls.push(`${request.method} ${new URL(request.url).pathname}`);
  });
  return { calls };
}

describe("comments mention composites", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("expandMentions", () => {
    it("resolves each id and places the tag inside the first block", async () => {
      const tracked = trackRequests();
      server.use(http.get(`${BASE_URL}/people/${VICTOR}`, () => HttpResponse.json(personBody(VICTOR, VICTOR_SGID))));

      const expanded = await client.comments.expandMentions("<div>On it.</div>", [VICTOR]);

      expect(expanded).toBe(`<div>${attachment(VICTOR_SGID)} On it.</div>`);
      expect(tracked.calls).toEqual([`GET /12345/people/${VICTOR}`]);
    });

    it("reads each distinct id once, in order", async () => {
      const tracked = trackRequests();
      server.use(
        http.get(`${BASE_URL}/people/${VICTOR}`, () => HttpResponse.json(personBody(VICTOR, VICTOR_SGID))),
        http.get(`${BASE_URL}/people/${ANNIE}`, () => HttpResponse.json(personBody(ANNIE, ANNIE_SGID))),
      );

      const expanded = await client.comments.expandMentions("<div>Hi</div>", [VICTOR, ANNIE, VICTOR]);

      expect(tracked.calls).toEqual([`GET /12345/people/${VICTOR}`, `GET /12345/people/${ANNIE}`]);
      expect(expanded).toBe(`<div>${attachment(VICTOR_SGID)} ${attachment(ANNIE_SGID)} Hi</div>`);
    });

    it("reads the person even when the content already names them, and keeps the read's sgid", async () => {
      // The trust boundary: an sgid found in caller-supplied content is
      // unsigned, so it can neither prove the mention exists nor stand in for
      // the people read. A stale tag naming the right person is left where it
      // is, and the authoritative mention is added alongside it.
      const stale = personSGID(VICTOR, { digest: "f".repeat(40) });
      expect(stale).not.toBe(VICTOR_SGID);
      const tracked = trackRequests();
      server.use(http.get(`${BASE_URL}/people/${VICTOR}`, () => HttpResponse.json(personBody(VICTOR, VICTOR_SGID))));

      const expanded = await client.comments.expandMentions(`<div>${attachment(stale)} On it.</div>`, [VICTOR]);

      expect(tracked.calls).toEqual([`GET /12345/people/${VICTOR}`]);
      expect(expanded).toBe(`<div>${attachment(VICTOR_SGID)} ${attachment(stale)} On it.</div>`);
    });

    it("adds nothing when the content already carries the exact sgid the read returned", async () => {
      server.use(http.get(`${BASE_URL}/people/${VICTOR}`, () => HttpResponse.json(personBody(VICTOR, VICTOR_SGID))));
      const content = `<div>${attachment(VICTOR_SGID)} On it.</div>`;
      expect(await client.comments.expandMentions(content, [VICTOR])).toBe(content);
    });

    it("makes no request when no one is mentioned", async () => {
      const tracked = trackRequests();
      expect(await client.comments.expandMentions("<div>On it.</div>", [])).toBe("<div>On it.</div>");
      expect(tracked.calls).toEqual([]);
    });

    it("refuses a non-positive id before any request", async () => {
      const tracked = trackRequests();
      await expect(client.comments.expandMentions("<div>Hi</div>", [0])).rejects.toThrow(/invalid mention person id/);
      await expect(client.comments.expandMentions("<div>Hi</div>", [-3])).rejects.toThrow(BasecampError);
      expect(tracked.calls).toEqual([]);
    });

    it("fails the expansion when a person read fails, naming the mention it was resolving", async () => {
      server.use(
        http.get(`${BASE_URL}/people/${VICTOR}`, () => HttpResponse.json({ error: "Not found" }, { status: 404 })),
      );

      const err = await client.comments
        .expandMentions("<div>Hi</div>", [VICTOR])
        .catch((e: unknown) => e);

      expect(err).toBeInstanceOf(BasecampError);
      // The read's classification survives the added context.
      expect((err as BasecampError).code).toBe("not_found");
      expect((err as BasecampError).message).toContain(`resolving mention for person ${VICTOR}`);
    });

    it("fails when a person carries no attachable_sgid to mention", async () => {
      server.use(http.get(`${BASE_URL}/people/${VICTOR}`, () => HttpResponse.json({ id: VICTOR, name: "Victor" })));
      await expect(client.comments.expandMentions("<div>Hi</div>", [VICTOR])).rejects.toThrow(/no attachable_sgid/);
    });

    it("explains itself when the service was built without the client's people read", async () => {
      const bare = new CommentsService(client.raw);
      await expect(bare.expandMentions("<div>Hi</div>", [VICTOR])).rejects.toThrow(/createBasecampClient/);
    });
  });

  describe("createWithMentions", () => {
    it("resolves every mention before posting, and writes the expanded content", async () => {
      const tracked = trackRequests();
      const posted: unknown[] = [];
      server.use(
        http.get(`${BASE_URL}/people/${VICTOR}`, () => HttpResponse.json(personBody(VICTOR, VICTOR_SGID))),
        http.post(`${BASE_URL}/recordings/1069479351/comments.json`, async ({ request }) => {
          posted.push(await request.json());
          return HttpResponse.json(commentBody(`<div>${attachment(VICTOR_SGID)} On it.</div>`), { status: 201 });
        }),
      );

      const comment = await client.comments.createWithMentions(1069479351, "<div>On it.</div>", [VICTOR]);

      expect(tracked.calls).toEqual([
        `GET /12345/people/${VICTOR}`,
        "POST /12345/recordings/1069479351/comments.json",
      ]);
      expect(posted).toEqual([{ content: `<div>${attachment(VICTOR_SGID)} On it.</div>` }]);
      expect(comment.id).toBe(1069479370);
    });

    it("refuses a content that is not a string, and posts nothing", async () => {
      // The guard reads the value that goes out, not a coerced copy of it. `{}`
      // and `[]` are truthy, so a plain-JavaScript caller could sail past a
      // truthiness check into the mention walk — a raw TypeError with mentions
      // requested, and with none, a POST body the reference cannot produce,
      // since Go's signature takes a string. The second assertion is the one
      // that matters: an exception-only test passes in the world where the
      // write went out anyway.
      const tracked = trackRequests();
      server.use(
        http.post(`${BASE_URL}/recordings/1069479351/comments.json`, () =>
          HttpResponse.json(commentBody("<div>posted</div>"), { status: 201 }),
        ),
      );

      for (const content of [{}, [], 42, true, null, undefined] as unknown[]) {
        const err = await client.comments
          .createWithMentions(1069479351, content as string, [])
          .catch((e: unknown) => e);
        expect(err, JSON.stringify(content ?? null)).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("usage");
      }
      expect(tracked.calls.some((call) => call.startsWith("POST"))).toBe(false);
    });

    it("posts nothing when a mention lookup fails", async () => {
      const tracked = trackRequests();
      server.use(
        http.get(`${BASE_URL}/people/${VICTOR}`, () => HttpResponse.json(personBody(VICTOR, VICTOR_SGID))),
        http.get(`${BASE_URL}/people/${ANNIE}`, () => HttpResponse.json({ error: "Forbidden" }, { status: 403 })),
      );

      await expect(
        client.comments.createWithMentions(1069479351, "<div>On it.</div>", [VICTOR, ANNIE]),
      ).rejects.toThrow(BasecampError);

      expect(tracked.calls.some((call) => call.startsWith("POST"))).toBe(false);
    });

    it("posts the content unchanged when no one is mentioned", async () => {
      const tracked = trackRequests();
      const posted: unknown[] = [];
      server.use(
        http.post(`${BASE_URL}/recordings/1069479351/comments.json`, async ({ request }) => {
          posted.push(await request.json());
          return HttpResponse.json(commentBody("<div>On it.</div>"), { status: 201 });
        }),
      );

      await client.comments.createWithMentions(1069479351, "<div>On it.</div>");

      expect(tracked.calls).toEqual(["POST /12345/recordings/1069479351/comments.json"]);
      expect(posted).toEqual([{ content: "<div>On it.</div>" }]);
    });

    it("refuses empty content before any request", async () => {
      const tracked = trackRequests();
      await expect(client.comments.createWithMentions(1069479351, "", [VICTOR])).rejects.toThrow(
        /comment content is required/,
      );
      expect(tracked.calls).toEqual([]);
    });
  });
});
