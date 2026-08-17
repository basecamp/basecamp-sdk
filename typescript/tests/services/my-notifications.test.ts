/**
 * Tests for MyNotificationsService — verifies system actor normalization.
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("MyNotificationsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("system actor normalization", () => {
    it("should normalize LocalPerson creator.id to number and preserve system_label", async () => {
      server.use(
        http.get(`${BASE_URL}/my/readings.json`, () => {
          return HttpResponse.json({
            unreads: [
              {
                id: 42,
                title: "System notification",
                created_at: "2024-01-01T00:00:00Z",
                updated_at: "2024-01-01T00:00:00Z",
                creator: {
                  id: "basecamp",
                  name: "Basecamp",
                  personable_type: "LocalPerson",
                },
              },
            ],
            reads: [],
            memories: [],
            bubble_ups_count: 0,
            scheduled_bubble_ups_count: 0,
          });
        })
      );

      const result = await client.myNotifications.myNotifications();
      // Assert on the response's real members rather than casting it to
      // `Record<string, unknown[]>`. The response is a fixed-shape struct and
      // the test never iterates arbitrary keys -- the cast existed only to
      // index `unreads` by string, and it threw away the typing of exactly the
      // fields under test. `unreads` and `creator` are both optional in the
      // schema, so assert presence first: a response missing either now fails
      // on the presence check instead of on an undefined property read.
      expect(result.unreads).toBeDefined();
      const creator = result.unreads![0].creator;
      expect(creator).toBeDefined();

      expect(creator!.id).toBe(0);
      expect(typeof creator!.id).toBe("number");
      expect(creator!.system_label).toBe("basecamp");
      expect(creator!.personable_type).toBe("LocalPerson");
    });

    it("should leave numeric string creator.id as number", async () => {
      server.use(
        http.get(`${BASE_URL}/my/readings.json`, () => {
          return HttpResponse.json({
            unreads: [
              {
                id: 42,
                title: "Normal notification",
                created_at: "2024-01-01T00:00:00Z",
                updated_at: "2024-01-01T00:00:00Z",
                creator: {
                  id: "99999",
                  name: "Real Person",
                  personable_type: "User",
                },
              },
            ],
            reads: [],
            memories: [],
            bubble_ups_count: 0,
            scheduled_bubble_ups_count: 0,
          });
        })
      );

      const result = await client.myNotifications.myNotifications();
      // See the note in the first case for why this reads the typed members.
      expect(result.unreads).toBeDefined();
      const creator = result.unreads![0].creator;
      expect(creator).toBeDefined();

      expect(creator!.id).toBe(99999);
      expect(typeof creator!.id).toBe("number");
      expect(creator!.system_label).toBeUndefined();
    });

    it("should treat junk string as sentinel", async () => {
      server.use(
        http.get(`${BASE_URL}/my/readings.json`, () => {
          return HttpResponse.json({
            unreads: [
              {
                id: 42,
                title: "Junk notification",
                created_at: "2024-01-01T00:00:00Z",
                updated_at: "2024-01-01T00:00:00Z",
                creator: {
                  id: "123abc",
                  name: "Unknown",
                  personable_type: "LocalPerson",
                },
              },
            ],
            reads: [],
            memories: [],
            bubble_ups_count: 0,
            scheduled_bubble_ups_count: 0,
          });
        })
      );

      const result = await client.myNotifications.myNotifications();
      // See the note in the first case for why this reads the typed members.
      expect(result.unreads).toBeDefined();
      const creator = result.unreads![0].creator;
      expect(creator).toBeDefined();

      // "123abc" is not a valid ID — treated as sentinel
      expect(creator!.id).toBe(0);
      expect(creator!.system_label).toBe("123abc");
    });

    it("should treat overflow numeric string as sentinel (JS cannot represent losslessly)", async () => {
      server.use(
        http.get(`${BASE_URL}/my/readings.json`, () => {
          return HttpResponse.json({
            unreads: [
              {
                id: 42,
                title: "Overflow notification",
                created_at: "2024-01-01T00:00:00Z",
                updated_at: "2024-01-01T00:00:00Z",
                creator: {
                  id: "9223372036854775808",
                  name: "Overflow",
                  personable_type: "LocalPerson",
                },
              },
            ],
            reads: [],
            memories: [],
            bubble_ups_count: 0,
            scheduled_bubble_ups_count: 0,
          });
        })
      );

      const result = await client.myNotifications.myNotifications();
      // See the note in the first case for why this reads the typed members.
      expect(result.unreads).toBeDefined();
      const creator = result.unreads![0].creator;
      expect(creator).toBeDefined();

      // Overflow can't be represented as a safe integer — preserved as label
      expect(creator!.id).toBe(0);
      expect(creator!.system_label).toBe("9223372036854775808");
    });
  });

  describe("bubbleUps", () => {
    it("should return current and scheduled bubble-ups", async () => {
      server.use(
        http.get(`${BASE_URL}/my/readings/bubble_ups.json`, () => {
          return HttpResponse.json(
            [
              {
                id: 2,
                created_at: "2026-07-21T00:01:43.009Z",
                updated_at: "2026-07-21T00:01:43.031Z",
                section: "bubbles",
                unread_count: 0,
                read_at: "2026-07-21T00:01:43.031Z",
                title: "We won Leto!",
                type: "Message",
                bucket_name: "The Leto Laptop",
              },
              {
                id: 3,
                created_at: "2026-07-21T00:02:00.000Z",
                updated_at: "2026-07-21T00:02:00.000Z",
                section: "bubbles",
                unread_count: 1,
                title: "Scheduled follow-up",
                type: "Todo",
                bubble_up_at: "2026-08-01T00:00:00Z",
              },
            ],
            { headers: { "X-Total-Count": "2" } }
          );
        })
      );

      const result = await client.myNotifications.bubbleUps();

      expect(result).toHaveLength(2);
      expect(result[0]!.id).toBe(2);
      expect(result[0]!.title).toBe("We won Leto!");
      expect((result[0] as Record<string, unknown>).type).toBe("Message");
      expect((result[1] as Record<string, unknown>).bubble_up_at).toBe("2026-08-01T00:00:00Z");
      // Paginated: the ListResult carries the X-Total-Count metadata.
      expect(result.meta.totalCount).toBe(2);
    });

    it("should propagate a 4xx as a BasecampError", async () => {
      server.use(
        http.get(`${BASE_URL}/my/readings/bubble_ups.json`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.myNotifications.bubbleUps()).rejects.toThrow(BasecampError);
    });
  });
});
