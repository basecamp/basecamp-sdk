import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

function feedEvent(id: number, overrides: Record<string, unknown> = {}) {
  return {
    id,
    kind: "message_created",
    action: "created",
    created_at: "2026-07-14T06:10:00.159Z",
    event_type: "message.created",
    bucket_id: 2085958499,
    creator_id: 1049715945,
    performed_by_id: null,
    recording_id: 1069479766,
    ...overrides,
  };
}

describe("EventFeedService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("pollEvents", () => {
    it("sends the entry point and comma-joined filters and decodes the envelope", async () => {
      let query: URLSearchParams | undefined;
      server.use(
        http.get(`${BASE_URL}/events.json`, ({ request }) => {
          query = new URL(request.url).searchParams;
          return HttpResponse.json({
            events: [
              feedEvent(1071915468),
              feedEvent(1071915470, {
                kind: "boost_created",
                event_type: "boost.created",
                performed_by_id: 1049715999,
                details: { boost_id: 501, boosted_event_id: 1071915468, boosted_event_type: "message.created" },
              }),
            ],
            position: "posAAA",
            next: "https://3.basecampapi.com/12345/events.json?position=posAAA&types=message.created%2Cboost.created",
          });
        })
      );

      const page = await client.eventFeed.pollEvents({
        since: "0",
        types: "message.created,boost.created",
        buckets: "2085958499",
        excludePerformers: "self",
        actorTypes: "agent,person",
      });

      expect(query?.get("since")).toBe("0");
      expect(query?.get("types")).toBe("message.created,boost.created");
      expect(query?.get("buckets")).toBe("2085958499");
      expect(query?.get("exclude_performers")).toBe("self");
      expect(query?.get("actor_types")).toBe("agent,person");
      expect(query?.has("position")).toBe(false);

      expect(page.position).toBe("posAAA");
      expect(page.next).toContain("position=posAAA");
      expect(page.events).toHaveLength(2);
      expect(page.events[0].performed_by_id).toBeNull();
      expect(page.events[0].details).toBeUndefined();
      expect(page.events[1].performed_by_id).toBe(1049715999);
      expect(page.events[1].details?.boost_id).toBe(501);
      expect(page.events[1].details?.boosted_event_type).toBe("message.created");
    });

    it("enters at the present with no query when called bare", async () => {
      let rawQuery: string | undefined;
      server.use(
        http.get(`${BASE_URL}/events.json`, ({ request }) => {
          rawQuery = new URL(request.url).search;
          return HttpResponse.json({ events: [], position: "posNOW" });
        })
      );

      const page = await client.eventFeed.pollEvents();
      expect(rawQuery).toBe("");
      expect(page.events).toEqual([]);
      expect(page.next).toBeUndefined();
    });

    it("surfaces a 409 filter mismatch as a non-retryable BasecampError", async () => {
      server.use(
        http.get(`${BASE_URL}/events.json`, () => {
          return HttpResponse.json(
            {
              error: "Positions are bound to the filter set they were minted for.",
              position_digest: "38b223c13c89dc89",
              filters_digest: "44136fa355b3678a",
            },
            { status: 409 }
          );
        })
      );

      const err = await client.eventFeed.pollEvents({ position: "posAAA" }).catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).httpStatus).toBe(409);
      expect((err as BasecampError).retryable).toBe(false);
      expect((err as BasecampError).message).toContain("Positions are bound");
    });

    it("surfaces a 410 stale position as a non-retryable BasecampError", async () => {
      server.use(
        http.get(`${BASE_URL}/events.json`, () => {
          return HttpResponse.json(
            {
              error: "That position predates this feed's epoch, so the history behind it can't be served.",
              epoch_after_id: 1071915000,
              resume: "https://3.basecampapi.com/12345/events.json?since=1071915000",
            },
            { status: 410 }
          );
        })
      );

      const err = await client.eventFeed.pollEvents({ position: "posOLD" }).catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).httpStatus).toBe(410);
      expect((err as BasecampError).retryable).toBe(false);
    });
  });

  describe("pollInbox", () => {
    it("decodes the inbox envelope and sends the reasons filter", async () => {
      let query: URLSearchParams | undefined;
      server.use(
        http.get(`${BASE_URL}/inbox.json`, ({ request }) => {
          query = new URL(request.url).searchParams;
          return HttpResponse.json({
            items: [
              {
                addressing_id: 991,
                reason: "mentioned",
                addressed_at: "2026-07-14T06:10:00.159Z",
                event: feedEvent(1071915468, { kind: "comment_created", event_type: "comment.created" }),
              },
            ],
            position: "inboxPos",
          });
        })
      );

      const page = await client.eventFeed.pollInbox({ since: "0", reasons: "mentioned,assigned" });
      expect(query?.get("since")).toBe("0");
      expect(query?.get("reasons")).toBe("mentioned,assigned");
      expect(page.position).toBe("inboxPos");
      expect(page.items).toHaveLength(1);
      expect(page.items[0].addressing_id).toBe(991);
      expect(page.items[0].event.event_type).toBe("comment.created");
    });

    it("surfaces the agents-only 403 as forbidden", async () => {
      server.use(
        http.get(`${BASE_URL}/inbox.json`, () => {
          return new HttpResponse(null, { status: 403 });
        })
      );

      const err = await client.eventFeed.pollInbox().catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("forbidden");
      expect((err as BasecampError).httpStatus).toBe(403);
    });
  });

  describe("createStreamTicket", () => {
    it("POSTs with no body and decodes the mint", async () => {
      let contentLength: string | null = null;
      server.use(
        http.post(`${BASE_URL}/events/stream_ticket.json`, ({ request }) => {
          contentLength = request.headers.get("content-length");
          return HttpResponse.json({
            ticket: "fixture-ticket-not-a-credential",
            expires_in: 120,
            url: "wss://cable.example.invalid/12345?ticket=fixture-ticket-not-a-credential",
          });
        })
      );

      const ticket = await client.eventFeed.createStreamTicket();
      expect(contentLength === null || contentLength === "0").toBe(true);
      expect(ticket.ticket).toBe("fixture-ticket-not-a-credential");
      expect(ticket.expires_in).toBe(120);
      expect(ticket.url).toContain("?ticket=");
    });

    it("surfaces 401 as auth_required", async () => {
      server.use(
        http.post(`${BASE_URL}/events/stream_ticket.json`, () => {
          return HttpResponse.json({ error: "Unauthorized" }, { status: 401 });
        })
      );

      const err = await client.eventFeed.createStreamTicket().catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("auth_required");
    });
  });
});
