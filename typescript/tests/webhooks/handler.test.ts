import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import {
  WebhookReceiver,
  WebhookVerificationError,
  type WebhookEventHandler,
} from "../../src/webhooks/handler.js";
import { signWebhookPayload } from "../../src/webhooks/verify.js";
import type { WebhookEvent } from "../../src/webhooks/events.js";

const fixturesDir = join(__dirname, "..", "..", "..", "spec", "fixtures", "webhooks");
const todoCreatedBody = readFileSync(join(fixturesDir, "event-todo-created.json"), "utf8");
const messageCopiedBody = readFileSync(join(fixturesDir, "event-message-copied.json"), "utf8");
const unknownFutureBody = readFileSync(join(fixturesDir, "event-unknown-future.json"), "utf8");

const emptyHeaders = {};

describe("WebhookReceiver", () => {
  describe("routing", () => {
    it("routes to exact kind handler", async () => {
      const receiver = new WebhookReceiver();
      const events: WebhookEvent[] = [];
      receiver.on("todo_created", (e) => { events.push(e); });

      await receiver.handleRequest(todoCreatedBody, emptyHeaders);

      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe("todo_created");
    });

    it("routes to glob prefix handler (todo_*)", async () => {
      const receiver = new WebhookReceiver();
      const events: WebhookEvent[] = [];
      receiver.on("todo_*", (e) => { events.push(e); });

      await receiver.handleRequest(todoCreatedBody, emptyHeaders);

      expect(events).toHaveLength(1);
    });

    it("routes to glob suffix handler (*_created)", async () => {
      const receiver = new WebhookReceiver();
      const events: WebhookEvent[] = [];
      receiver.on("*_created", (e) => { events.push(e); });

      await receiver.handleRequest(todoCreatedBody, emptyHeaders);

      expect(events).toHaveLength(1);
    });

    it("glob does not match unrelated kinds", async () => {
      const receiver = new WebhookReceiver();
      const events: WebhookEvent[] = [];
      receiver.on("message_*", (e) => { events.push(e); });

      await receiver.handleRequest(todoCreatedBody, emptyHeaders);

      expect(events).toHaveLength(0);
    });

    it("fires onAny handlers for all events", async () => {
      const receiver = new WebhookReceiver();
      const events: WebhookEvent[] = [];
      receiver.onAny((e) => { events.push(e); });

      await receiver.handleRequest(todoCreatedBody, emptyHeaders);
      await receiver.handleRequest(messageCopiedBody, emptyHeaders);

      expect(events).toHaveLength(2);
    });

    it("fires multiple handlers per kind", async () => {
      const receiver = new WebhookReceiver();
      const results: string[] = [];
      receiver.on("todo_created", () => { results.push("a"); });
      receiver.on("todo_created", () => { results.push("b"); });

      await receiver.handleRequest(todoCreatedBody, emptyHeaders);

      expect(results).toEqual(["a", "b"]);
    });
  });

  describe("unknown events", () => {
    it("does not error on unknown event kind", async () => {
      const receiver = new WebhookReceiver();
      receiver.on("todo_created", () => {});

      const event = await receiver.handleRequest(unknownFutureBody, emptyHeaders);
      expect(event.kind).toBe("new_thing_activated");
    });

    it("routes unknown kind to catch-all handler", async () => {
      const receiver = new WebhookReceiver();
      const events: WebhookEvent[] = [];
      receiver.on("todo_created", () => {});
      receiver.onAny((e) => { events.push(e); });

      await receiver.handleRequest(unknownFutureBody, emptyHeaders);

      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe("new_thing_activated");
    });
  });

  describe("dedup", () => {
    it("does not trigger handlers for duplicate event IDs", async () => {
      const receiver = new WebhookReceiver();
      let count = 0;
      receiver.on("todo_created", () => { count++; });

      await receiver.handleRequest(todoCreatedBody, emptyHeaders);
      await receiver.handleRequest(todoCreatedBody, emptyHeaders);

      expect(count).toBe(1);
    });

    it("returns the parsed event even for duplicates", async () => {
      const receiver = new WebhookReceiver();
      receiver.on("todo_created", () => {});

      await receiver.handleRequest(todoCreatedBody, emptyHeaders);
      const event = await receiver.handleRequest(todoCreatedBody, emptyHeaders);

      expect(event.kind).toBe("todo_created");
    });

    it("retries succeed after handler error (dedup only on success)", async () => {
      const receiver = new WebhookReceiver();
      let calls = 0;
      receiver.on("todo_created", () => {
        calls++;
        if (calls === 1) throw new Error("transient failure");
      });

      // First attempt fails
      await expect(receiver.handleRequest(todoCreatedBody, emptyHeaders)).rejects.toThrow("transient failure");
      expect(calls).toBe(1);

      // Retry of same event should run handlers again (not suppressed by dedup)
      await receiver.handleRequest(todoCreatedBody, emptyHeaders);
      expect(calls).toBe(2);

      // Third delivery is now a true duplicate (second succeeded)
      await receiver.handleRequest(todoCreatedBody, emptyHeaders);
      expect(calls).toBe(2);
    });

    it("deduplicates exact same raw string ID", async () => {
      const receiver = new WebhookReceiver();
      let calls = 0;
      receiver.onAny(() => { calls++; });

      const event = '{"id":9007199254741000,"kind":"a","created_at":"2022-01-01T00:00:00Z","recording":{"id":1},"creator":{"id":1}}';

      await receiver.handleRequest(event, emptyHeaders);
      await receiver.handleRequest(event, emptyHeaders);

      expect(calls).toBe(1);
    });

    it("deduplicates int64 IDs without precision loss", async () => {
      const receiver = new WebhookReceiver();
      let calls = 0;
      receiver.onAny(() => { calls++; });

      // Two events with IDs that differ only in the last digits —
      // as JS numbers they collide (both become 9007199254741000),
      // but as strings they're distinct.
      const event1 = '{"id":9007199254741000,"kind":"a","created_at":"2022-01-01T00:00:00Z","recording":{"id":1},"creator":{"id":1}}';
      const event2 = '{"id":9007199254741001,"kind":"b","created_at":"2022-01-01T00:00:00Z","recording":{"id":2},"creator":{"id":1}}';

      await receiver.handleRequest(event1, emptyHeaders);
      await receiver.handleRequest(event2, emptyHeaders);

      // Both should fire — they are distinct events
      expect(calls).toBe(2);
    });

    it("allows duplicates when dedup is disabled", async () => {
      const receiver = new WebhookReceiver({ dedupWindowSize: 0 });
      let count = 0;
      receiver.on("todo_created", () => { count++; });

      await receiver.handleRequest(todoCreatedBody, emptyHeaders);
      await receiver.handleRequest(todoCreatedBody, emptyHeaders);

      expect(count).toBe(2);
    });
  });

  describe("middleware", () => {
    it("runs middleware in order before handlers", async () => {
      const receiver = new WebhookReceiver();
      const order: number[] = [];

      receiver.use(async (_event, next) => { order.push(1); await next(); });
      receiver.use(async (_event, next) => { order.push(2); await next(); });
      receiver.onAny(() => { order.push(3); });

      await receiver.handleRequest(todoCreatedBody, emptyHeaders);

      expect(order).toEqual([1, 2, 3]);
    });

    it("aborts chain when middleware throws", async () => {
      const receiver = new WebhookReceiver();
      let handlerCalled = false;

      receiver.use(async () => { throw new Error("abort"); });
      receiver.onAny(() => { handlerCalled = true; });

      await expect(receiver.handleRequest(todoCreatedBody, emptyHeaders)).rejects.toThrow("abort");
      expect(handlerCalled).toBe(false);
    });
  });

  describe("signature verification", () => {
    it("accepts valid signature", async () => {
      const secret = "test-secret";
      const sig = signWebhookPayload(todoCreatedBody, secret);
      const receiver = new WebhookReceiver({ secret });
      const events: WebhookEvent[] = [];
      receiver.onAny((e) => { events.push(e); });

      await receiver.handleRequest(todoCreatedBody, { "x-basecamp-signature": sig });

      expect(events).toHaveLength(1);
    });

    it("rejects bad signature with WebhookVerificationError", async () => {
      const receiver = new WebhookReceiver({ secret: "test-secret" });

      await expect(
        receiver.handleRequest(todoCreatedBody, { "x-basecamp-signature": "bad-sig" }),
      ).rejects.toThrow(WebhookVerificationError);
    });

    it("rejects missing signature when secret is set", async () => {
      const receiver = new WebhookReceiver({ secret: "test-secret" });

      await expect(
        receiver.handleRequest(todoCreatedBody, {}),
      ).rejects.toThrow(WebhookVerificationError);
    });

    it("skips verification when no secret configured", async () => {
      const receiver = new WebhookReceiver();
      const events: WebhookEvent[] = [];
      receiver.onAny((e) => { events.push(e); });

      await receiver.handleRequest(todoCreatedBody, {});

      expect(events).toHaveLength(1);
    });

    it("reads signature from function-based headers", async () => {
      const secret = "test-secret";
      const sig = signWebhookPayload(todoCreatedBody, secret);
      const receiver = new WebhookReceiver({ secret });
      receiver.onAny(() => {});

      const headers = (name: string) => {
        if (name === "x-basecamp-signature") return sig;
        return undefined;
      };

      await expect(receiver.handleRequest(todoCreatedBody, headers)).resolves.toBeDefined();
    });
  });

  describe("event parsing", () => {
    it("parses todo_created event with recording and creator", async () => {
      const receiver = new WebhookReceiver();
      let captured: WebhookEvent | undefined;
      receiver.onAny((e) => { captured = e; });

      await receiver.handleRequest(todoCreatedBody, emptyHeaders);

      expect(captured).toBeDefined();
      expect(captured!.id).toBe(9007199254741001);
      expect(captured!.recording?.type).toBe("Todo");
      expect(captured!.recording?.title).toBe("Ship the feature");
      expect(captured!.recording?.content).toBe("<div>Ship the feature by Friday</div>");
      expect(captured!.creator?.name).toBe("Annie Bryan");
      expect(captured!.creator?.can_ping).toBe(true);
      expect(captured!.copy).toBeUndefined();
    });

    it("parses message_copied event with copy field", async () => {
      const receiver = new WebhookReceiver({ dedupWindowSize: 0 });
      let captured: WebhookEvent | undefined;
      receiver.onAny((e) => { captured = e; });

      await receiver.handleRequest(messageCopiedBody, emptyHeaders);

      expect(captured!.kind).toBe("message_copied");
      expect(captured!.copy).toBeDefined();
      expect(captured!.copy!.id).toBe(9007199254741350);
      expect(captured!.copy!.bucket?.id).toBe(2085958500);
    });

    it("parses unknown future event without error", async () => {
      const receiver = new WebhookReceiver({ dedupWindowSize: 0 });
      let captured: WebhookEvent | undefined;
      receiver.onAny((e) => { captured = e; });

      await receiver.handleRequest(unknownFutureBody, emptyHeaders);

      expect(captured!.kind).toBe("new_thing_activated");
      expect(captured!.recording?.type).toBe("NewRecordingType");
    });
  });
});
