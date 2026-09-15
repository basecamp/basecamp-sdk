/**
 * Tests for the `recordings.summarize` composite
 * (src/services/recordings-extensions.ts).
 *
 * The conformance fixture (conformance/tests/recording_summary.json) pins the
 * routing matrix, the projection's shape and the request sequence across all
 * runners. These tests carry what a static fixture cannot: the discovery
 * caches' lifetime and refresh floor, the candidate budget, the listing cap,
 * and the error identities those produce.
 */
import { describe, it, expect, beforeEach, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import {
  RecordingsService,
  CampfireDiscoveryIncompleteError,
  UnresolvedRecordingError,
  BucketMismatchError,
  RecordingRoutingError,
  CAMPFIRE_INDEX_TTL_MS,
  MAX_CAMPFIRE_CANDIDATES,
  MAX_CAMPFIRE_LISTING,
  summarizableEventTypes,
  summarizableRecordingTypes,
} from "../../src/index.js";
import type { RecordingReadSources } from "../../src/index.js";
import { personSGID } from "../helpers/sgid.js";

const BASE_URL = "https://3.basecampapi.com/12345";
const BUCKET = 2085958499;
const OTHER_BUCKET = 2085958500;
const VICTOR = 1049715914;

/** The recording columns every routed read shares. */
const recording = (id: number, type: string, extra: Record<string, unknown> = {}) => ({
  id,
  status: "active",
  visible_to_clients: false,
  created_at: "2022-10-28T15:25:00.000Z",
  updated_at: "2022-10-28T15:25:00.000Z",
  title: `Recording ${id}`,
  inherits_status: true,
  type,
  url: `${BASE_URL}/recordings/${id}.json`,
  app_url: `https://3.basecamp.com/12345/buckets/${BUCKET}/recordings/${id}`,
  bucket: { id: BUCKET, name: "The Leto Laptop", type: "Project" },
  creator: { id: VICTOR, name: "Victor Cooper" },
  ...extra,
});

const campfire = (id: number, bucketId: number) => ({
  ...recording(id, "Chat::Transcript"),
  id,
  bucket: { id: bucketId, name: `Project ${bucketId}`, type: "Project" },
});

/** Records every path MSW served, in order. */
function trackRequests(): string[] {
  const paths: string[] = [];
  server.events.removeAllListeners("request:start");
  server.events.on("request:start", ({ request }) => {
    paths.push(new URL(request.url).pathname);
  });
  return paths;
}

describe("recordings.summarize", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("routing", () => {
    // One row per routed key: the pointer the caller holds, and the single
    // generated read it must reach. The fixture pins the same matrix across
    // every runner; this is the native mirror, and it is the thing a new
    // routing entry has to be added to.
    const ROUTES: [string, { eventType?: string; recordingType?: string }, string, Record<string, unknown>][] = [
      ["comment.created", { eventType: "comment.created" }, "/comments/1", recording(1, "Comment", { content: "hi" })],
      ["message.created", { eventType: "message.created" }, "/messages/1", recording(1, "Message", { subject: "S", content: "hi" })],
      ["todo.completed", { eventType: "todo.completed" }, "/todos/1", recording(1, "Todo", { content: "T", description: "d" })],
      ["card.moved", { eventType: "card.moved" }, "/card_tables/cards/1", recording(1, "Kanban::Card", { content: "c" })],
      ["Comment", { recordingType: "Comment" }, "/comments/1", recording(1, "Comment", { content: "hi" })],
      ["Document", { recordingType: "Document" }, "/documents/1", recording(1, "Document", { content: "d" })],
      ["Upload", { recordingType: "Upload" }, "/uploads/1", recording(1, "Upload", { filename: "f.png", description: "d" })],
      ["Schedule::Entry", { recordingType: "Schedule::Entry" }, "/schedule_entries/1", recording(1, "Schedule::Entry", { summary: "s", description: "d" })],
      ["Question", { recordingType: "Question" }, "/questions/1", recording(1, "Question")],
      ["Question::Answer", { recordingType: "Question::Answer" }, "/question_answers/1", recording(1, "Question::Answer", { content: "a" })],
      ["Todolist", { recordingType: "Todolist" }, "/todolists/1", recording(1, "Todolist", { name: "n", description: "d" })],
      ["Vault", { recordingType: "Vault" }, "/vaults/1", recording(1, "Vault")],
      ["Inbox::Forward", { recordingType: "Inbox::Forward" }, "/inbox_forwards/1", recording(1, "Inbox::Forward", { subject: "s", content: "c" })],
      ["Client::Approval", { recordingType: "Client::Approval" }, "/client/approvals/1", recording(1, "Client::Approval", { subject: "s", content: "c" })],
      ["Client::Correspondence", { recordingType: "Client::Correspondence" }, "/client/correspondences/1", recording(1, "Client::Correspondence", { subject: "s", content: "c" })],
      ["GoogleDocument", { recordingType: "GoogleDocument" }, "/google_documents/1", recording(1, "GoogleDocument", { description: "d" })],
      ["CloudFile", { recordingType: "CloudFile" }, "/cloud_files/1", recording(1, "CloudFile", { description: "d" })],
      ["Kanban::Step", { recordingType: "Kanban::Step" }, "/card_tables/steps/1", recording(1, "Kanban::Step")],
      ["Questionnaire", { recordingType: "Questionnaire" }, "/questionnaires/1", recording(1, "Questionnaire", { name: "n" })],
      ["Schedule", { recordingType: "Schedule" }, "/schedules/1", recording(1, "Schedule")],
      ["Todoset", { recordingType: "Todoset" }, "/todosets/1", recording(1, "Todoset", { name: "n" })],
      ["Message::Board", { recordingType: "Message::Board" }, "/message_boards/1", recording(1, "Message::Board")],
      ["Kanban::Board", { recordingType: "Kanban::Board" }, "/card_tables/1", recording(1, "Kanban::Board")],
      ["Kanban::Column", { recordingType: "Kanban::Column" }, "/card_tables/columns/1", recording(1, "Kanban::Column", { description: "d" })],
      ["Inbox", { recordingType: "Inbox" }, "/inboxes/1", recording(1, "Inbox")],
      ["Chat::Transcript", { recordingType: "Chat::Transcript" }, "/chats/1", recording(1, "Chat::Transcript")],
    ];

    it.each(ROUTES)("routes %s to one typed read", async (_label, ref, path, body) => {
      const paths = trackRequests();
      server.use(http.get(`${BASE_URL}${path}`, () => HttpResponse.json(body)));

      const summary = await client.recordings.summarize({ bucketId: BUCKET, recordingId: 1, ...ref });

      expect(paths).toEqual([`/12345${path}`]);
      expect(summary.id).toBe(1);
      expect(summary.bucket?.id).toBe(BUCKET);
      expect(summary.creator?.id).toBe(VICTOR);
      expect(summary.mentioned_person_ids).toEqual([]);
    });

    it("prefers the recording type over the event type", async () => {
      const paths = trackRequests();
      server.use(http.get(`${BASE_URL}/documents/1`, () => HttpResponse.json(recording(1, "Document"))));

      await client.recordings.summarize({
        bucketId: BUCKET,
        recordingId: 1,
        eventType: "comment.created",
        recordingType: "Document",
      });

      expect(paths).toEqual(["/12345/documents/1"]);
    });

    it("reads a chat line subtype through discovery, whichever subtype it is", async () => {
      for (const subtype of ["Chat::Lines::Text", "Chat::Lines::RichText", "Chat::Lines::Code"]) {
        const fresh = createBasecampClient({ accountId: "12345", accessToken: "t", enableRetry: false });
        server.use(
          http.get(`${BASE_URL}/projects/${BUCKET}`, () =>
            HttpResponse.json({ id: BUCKET, dock: [{ id: 77, name: "chat", title: "Campfire", enabled: true, url: "", app_url: "" }] }),
          ),
          http.get(`${BASE_URL}/chats/77/lines/9`, () => HttpResponse.json(recording(9, subtype, { content: "hello" }))),
        );
        const summary = await fresh.recordings.summarize({ bucketId: BUCKET, recordingId: 9, recordingType: subtype });
        expect(summary.campfire_id).toBe(77);
      }
    });

    it("lists what it routes, and does not claim boost", () => {
      expect(summarizableRecordingTypes()).toContain("Chat::Lines::*");
      expect(summarizableRecordingTypes()).toContain("Kanban::Card");
      expect(summarizableEventTypes()).toEqual(["card.*", "chat.line.*", "comment.*", "message.*", "todo.*"]);
    });
  });

  describe("projection", () => {
    it("reads the mentions out of the recording's rich text", async () => {
      const sgid = personSGID(VICTOR);
      server.use(
        http.get(`${BASE_URL}/comments/1`, () =>
          HttpResponse.json(
            recording(1, "Comment", { content: `<div><bc-attachment sgid="${sgid}"></bc-attachment> hi</div>` }),
          ),
        ),
      );

      const summary = await client.recordings.summarize({
        bucketId: BUCKET,
        recordingId: 1,
        recordingType: "Comment",
      });

      expect(summary.mentioned_person_ids).toEqual([VICTOR]);
      expect(summary.content).toContain("bc-attachment");
    });

    it("reports no mentions for a chat line BC3 never read as markup", async () => {
      // A Text line's content is HTML-escaped on the way out, so a literal
      // bc-attachment in it mentions nobody — only RichText and Integration
      // lines carry rich text.
      const sgid = personSGID(VICTOR);
      const line = `<bc-attachment sgid="${sgid}"></bc-attachment>`;
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () =>
          HttpResponse.json({ id: BUCKET, dock: [{ id: 77, name: "chat", title: "Campfire", enabled: true, url: "", app_url: "" }] }),
        ),
        http.get(`${BASE_URL}/chats/77/lines/9`, () =>
          HttpResponse.json(recording(9, "Chat::Lines::Text", { content: line })),
        ),
      );

      const text = await client.recordings.summarize({ bucketId: BUCKET, recordingId: 9, recordingType: "Chat::Lines::Text" });
      expect(text.mentioned_person_ids).toEqual([]);

      const fresh = createBasecampClient({ accountId: "12345", accessToken: "t", enableRetry: false });
      server.use(
        http.get(`${BASE_URL}/chats/77/lines/9`, () =>
          HttpResponse.json(recording(9, "Chat::Lines::RichText", { content: line })),
        ),
      );
      const rich = await fresh.recordings.summarize({ bucketId: BUCKET, recordingId: 9, recordingType: "Chat::Lines::RichText" });
      expect(rich.mentioned_person_ids).toEqual([VICTOR]);
    });

    it("defaults a required scalar the read omitted to Go's zero value", async () => {
      // Three mutants of these defaults survived the whole suite — including
      // reverting updated_at to "", which is what it was before it was found to
      // be wrong. A default nothing asserts is a silent-regression candidate,
      // and this file has already lost one test that way.
      server.use(
        http.get(`${BASE_URL}/comments/1`, () =>
          HttpResponse.json({
            id: 1,
            content: "hi",
            bucket: { id: BUCKET, name: "B", type: "Project" },
            creator: { id: VICTOR, name: "Victor Cooper" },
          }),
        ),
      );

      const summary = await client.recordings.summarize({
        bucketId: BUCKET,
        recordingId: 1,
        recordingType: "Comment",
      });

      expect(summary.status).toBe("");
      expect(summary.type).toBe("");
      expect(summary.app_url).toBe("");
      expect(summary.title).toBe("");
      // Not "": the Go field is a time.Time, whose zero marshals as the zero
      // instant.
      expect(summary.updated_at).toBe("0001-01-01T00:00:00Z");
      // And the key is present rather than dropped, which is the point.
      expect(Object.keys(summary)).toEqual(expect.arrayContaining(["updated_at", "status", "type", "app_url"]));
    });

    it("carries the assignees of an assignable type and omits them elsewhere", async () => {
      server.use(
        http.get(`${BASE_URL}/todos/1`, () =>
          HttpResponse.json(recording(1, "Todo", { content: "T", assignees: [{ id: VICTOR, name: "Victor Cooper" }] })),
        ),
        http.get(`${BASE_URL}/comments/2`, () => HttpResponse.json(recording(2, "Comment", { content: "" }))),
      );

      const todo = await client.recordings.summarize({ bucketId: BUCKET, recordingId: 1, recordingType: "Todo" });
      expect(todo.assignees?.[0]?.id).toBe(VICTOR);

      const comment = await client.recordings.summarize({ bucketId: BUCKET, recordingId: 2, recordingType: "Comment" });
      expect(comment.assignees).toBeUndefined();
    });

    it("prefers a to-do's title over its content, and projects the description as the rich text", async () => {
      server.use(
        http.get(`${BASE_URL}/todos/1`, () =>
          HttpResponse.json(recording(1, "Todo", { title: "", content: "Ship it", description: "<div>with care</div>" })),
        ),
      );
      const summary = await client.recordings.summarize({ bucketId: BUCKET, recordingId: 1, recordingType: "Todo" });
      expect(summary.title).toBe("Ship it");
      expect(summary.content).toBe("<div>with care</div>");
    });
  });

  describe("refusals", () => {
    it("refuses boost.created before any request", async () => {
      const paths = trackRequests();
      await expect(
        client.recordings.summarize({ bucketId: BUCKET, recordingId: 1, eventType: "boost.created" }),
      ).rejects.toThrow(/names no recording type/);
      expect(paths).toEqual([]);

      const err = await client.recordings
        .summarize({ bucketId: BUCKET, recordingId: 1, eventType: "boost.created" })
        .catch((e: unknown) => e);
      expect(err).toBeInstanceOf(RecordingRoutingError);
      expect((err as RecordingRoutingError).kind).toBe("no_recording_type");
      expect((err as RecordingRoutingError).code).toBe("usage");
    });

    it("refuses a type outside the routed set, and a string that is not a feed type", async () => {
      const paths = trackRequests();
      for (const ref of [
        { recordingType: "Timesheet::Entry" },
        { recordingType: "Client::Reply" },
        { eventType: "widget.created" },
        { eventType: "comment" },
        { eventType: "comment." },
        {},
      ]) {
        const err = await client.recordings
          .summarize({ bucketId: BUCKET, recordingId: 1, ...ref })
          .catch((e: unknown) => e);
        expect((err as RecordingRoutingError).kind).toBe("unknown_recording_type");
      }
      expect(paths).toEqual([]);
    });

    it("refuses a type that names an Object.prototype member, before any request", async () => {
      // The routing tables take an untrusted string off a feed row, and a plain
      // object literal answers for every Object.prototype member — which would
      // route "constructor" to something that is not a read at all.
      const paths = trackRequests();
      for (const ref of [
        { recordingType: "constructor" },
        { recordingType: "toString" },
        { recordingType: "__proto__" },
        { eventType: "constructor.created" },
        { eventType: "valueOf.created" },
      ]) {
        const err = await client.recordings
          .summarize({ bucketId: BUCKET, recordingId: 1, ...ref })
          .catch((e: unknown) => e);
        expect(err).toBeInstanceOf(RecordingRoutingError);
        expect((err as RecordingRoutingError).kind).toBe("unknown_recording_type");
      }
      expect(paths).toEqual([]);
    });

    it("refuses a pointer with no usable ids", async () => {
      const paths = trackRequests();
      for (const ref of [
        { bucketId: 0, recordingId: 1 },
        { bucketId: BUCKET, recordingId: 0 },
        { bucketId: -1, recordingId: 1 },
      ]) {
        const err = await client.recordings
          .summarize({ ...ref, recordingType: "Comment" })
          .catch((e: unknown) => e);
        expect((err as BasecampError).code).toBe("usage");
      }
      expect(paths).toEqual([]);
    });

    it("refuses a read that returned a recording from another bucket", async () => {
      server.use(
        http.get(`${BASE_URL}/comments/1`, () =>
          HttpResponse.json({
            ...recording(1, "Comment", { content: "" }),
            bucket: { id: OTHER_BUCKET, name: "Elsewhere", type: "Project" },
          }),
        ),
      );

      const err = await client.recordings
        .summarize({ bucketId: BUCKET, recordingId: 1, recordingType: "Comment" })
        .catch((e: unknown) => e);

      expect(err).toBeInstanceOf(BucketMismatchError);
      expect((err as BucketMismatchError).kind).toBe("bucket_mismatch");
      expect((err as BucketMismatchError).bucketId).toBe(OTHER_BUCKET);
      // Statusless: the transport succeeded, so no status describes the verdict.
      expect((err as BucketMismatchError).httpStatus).toBeUndefined();
    });

    it("accepts a read whose bucket carries no id, as Go's zero value does", async () => {
      server.use(
        http.get(`${BASE_URL}/comments/1`, () =>
          HttpResponse.json({
            ...recording(1, "Comment", { content: "" }),
            bucket: { name: "Nameless", type: "Project" },
          }),
        ),
      );

      const summary = await client.recordings.summarize({
        bucketId: BUCKET,
        recordingId: 1,
        recordingType: "Comment",
      });
      expect(summary.id).toBe(1);
    });

    it("explains itself when the service was built without the client's reads", async () => {
      const bare = new RecordingsService(client.raw);
      await expect(
        bare.summarize({ bucketId: BUCKET, recordingId: 1, recordingType: "Comment" }),
      ).rejects.toThrow(/createBasecampClient/);
    });
  });

  describe("chat line discovery", () => {
    const dock = (...campfireIds: number[]) =>
      HttpResponse.json({
        id: BUCKET,
        dock: campfireIds.map((id) => ({ id, name: "chat", title: "Campfire", enabled: true, url: "", app_url: "" })),
      });

    const notFound = () => HttpResponse.json({ error: "Record not found" }, { status: 404 });

    const line = (id: number) => recording(id, "Chat::Lines::Text", { content: "hello" });

    it("reads the project dock first and stops at the Campfire that answers", async () => {
      const paths = trackRequests();
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => dock(77)),
        http.get(`${BASE_URL}/chats/77/lines/9`, () => HttpResponse.json(line(9))),
      );

      const summary = await client.recordings.summarize({
        bucketId: BUCKET,
        recordingId: 9,
        eventType: "chat.line.created",
      });

      expect(paths).toEqual([`/12345/projects/${BUCKET}`, "/12345/chats/77/lines/9"]);
      expect(summary.campfire_id).toBe(77);
    });

    it("falls back to the account listing, filtered to the bucket, in listing order", async () => {
      const paths = trackRequests();
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => notFound()),
        http.get(`${BASE_URL}/chats.json`, () =>
          HttpResponse.json([campfire(50, OTHER_BUCKET), campfire(60, BUCKET), campfire(70, BUCKET)]),
        ),
        http.get(`${BASE_URL}/chats/60/lines/9`, () => notFound()),
        http.get(`${BASE_URL}/chats/70/lines/9`, () => HttpResponse.json(line(9))),
      );

      const summary = await client.recordings.summarize({
        bucketId: BUCKET,
        recordingId: 9,
        eventType: "chat.line.created",
      });

      // The Campfire in the other bucket is never tried.
      expect(paths).toEqual([
        `/12345/projects/${BUCKET}`,
        "/12345/chats.json",
        "/12345/chats/60/lines/9",
        "/12345/chats/70/lines/9",
      ]);
      expect(summary.campfire_id).toBe(70);
    });

    it("returns a candidate's non-404 answer as that read's error, and stops there", async () => {
      const paths = trackRequests();
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => notFound()),
        http.get(`${BASE_URL}/chats.json`, () => HttpResponse.json([campfire(60, BUCKET), campfire(70, BUCKET)])),
        http.get(`${BASE_URL}/chats/60/lines/9`, () => HttpResponse.json({ error: "Access denied" }, { status: 403 })),
        http.get(`${BASE_URL}/chats/70/lines/9`, () => HttpResponse.json(line(9))),
      );

      const err = await client.recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e);

      expect((err as BasecampError).code).toBe("forbidden");
      expect(paths).not.toContain("/12345/chats/70/lines/9");
    });

    it("reports a line under no visible Campfire as unresolved, not as a failed read", async () => {
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => notFound()),
        http.get(`${BASE_URL}/chats.json`, () => HttpResponse.json([campfire(60, BUCKET), campfire(70, BUCKET)])),
        http.get(`${BASE_URL}/chats/:campfireId/lines/9`, () => notFound()),
      );

      const err = await client.recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e);

      expect(err).toBeInstanceOf(UnresolvedRecordingError);
      const unresolved = err as UnresolvedRecordingError;
      expect(unresolved.kind).toBe("recording_unresolved");
      expect(unresolved.campfireIds).toEqual([60, 70]);
      expect(unresolved.refreshed).toBe(false);
      // Statelessly distinguishable from a read's own 404: same code, no status.
      expect(unresolved.code).toBe("not_found");
      expect(unresolved.httpStatus).toBeUndefined();
    });

    it("reports a budget spent before the listing was consulted as incomplete", async () => {
      // The dock alone exhausts the budget, so the account listing is never
      // reached: candidates may exist there unsearched, and nothing unsearched
      // is ever reported absent.
      const ids = Array.from({ length: MAX_CAMPFIRE_CANDIDATES + 5 }, (_, i) => 100 + i);
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => dock(...ids)),
        http.get(`${BASE_URL}/chats/:campfireId/lines/9`, () => notFound()),
      );

      const err = await client.recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e);

      expect(err).toBeInstanceOf(CampfireDiscoveryIncompleteError);
      expect((err as CampfireDiscoveryIncompleteError).kind).toBe("campfire_discovery_incomplete");
      expect((err as CampfireDiscoveryIncompleteError).reason).toBe(
        `the candidate budget of ${MAX_CAMPFIRE_CANDIDATES} was spent before the account listing was consulted`,
      );
    });

    it("does not fetch a listing that cannot help, so its failure cannot mask the verdict", async () => {
      // The regression this boundary exists for. With the budget already spent,
      // fetching the listing costs a request that can return no candidate this
      // call may try — and a failure on it would replace a deterministic
      // "incomplete" with a transient error a consumer retries forever.
      const paths = trackRequests();
      const ids = Array.from({ length: MAX_CAMPFIRE_CANDIDATES + 5 }, (_, i) => 100 + i);
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => dock(...ids)),
        http.get(`${BASE_URL}/chats.json`, () => HttpResponse.json({ error: "boom" }, { status: 500 })),
        http.get(`${BASE_URL}/chats/:campfireId/lines/9`, () => notFound()),
      );

      const err = await client.recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e);

      expect(err).toBeInstanceOf(CampfireDiscoveryIncompleteError);
      expect(paths).not.toContain("/12345/chats.json");
    });

    it("reports a bucket with more visible Campfires than the budget as incomplete, never absent", async () => {
      // The other side of the boundary: the listing WAS consulted, and ran out
      // of budget partway through its candidates. The verdict is still
      // incomplete, but for the other reason, and the messages say which.
      const campfires = Array.from({ length: MAX_CAMPFIRE_CANDIDATES + 5 }, (_, i) =>
        campfire(100 + i, BUCKET),
      );
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => notFound()),
        http.get(`${BASE_URL}/chats.json`, () => HttpResponse.json(campfires)),
        http.get(`${BASE_URL}/chats/:campfireId/lines/9`, () => notFound()),
      );

      const err = await client.recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e);

      expect(err).toBeInstanceOf(CampfireDiscoveryIncompleteError);
      expect((err as CampfireDiscoveryIncompleteError).reason).toBe(
        `more than ${MAX_CAMPFIRE_CANDIDATES} visible campfires in the bucket`,
      );
    });

    it("passes a listing failure that is not an overflow through as that read's error", async () => {
      // Only a listing OVER ITS CAP is incomplete. A 403 is not a settled
      // verdict about where the line is, and turning it into one would tell a
      // consumer that discovery finished when the source never answered. One
      // widened catch is all it takes.
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => notFound()),
        http.get(`${BASE_URL}/chats.json`, () =>
          HttpResponse.json({ error: "Access denied" }, { status: 403 }),
        ),
      );

      const err = await client.recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e);

      expect((err as BasecampError).code).toBe("forbidden");
      expect(err).not.toBeInstanceOf(CampfireDiscoveryIncompleteError);
      expect(err).not.toBeInstanceOf(UnresolvedRecordingError);
    });

    it("reports a listing past its cap as incomplete, and does not cache it", async () => {
      const paths = trackRequests();
      const overflowing = Array.from({ length: MAX_CAMPFIRE_LISTING + 1 }, (_, i) => campfire(1000 + i, OTHER_BUCKET));
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => notFound()),
        http.get(`${BASE_URL}/chats.json`, () => HttpResponse.json(overflowing)),
      );

      const err = await client.recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e);
      expect(err).toBeInstanceOf(CampfireDiscoveryIncompleteError);
      expect((err as CampfireDiscoveryIncompleteError).reason).toBe(
        "campfire listing exceeds MaxCampfireListing",
      );

      // A failed load leaves nothing behind: the next call reads the listing
      // again rather than serving a half-built snapshot.
      paths.length = 0;
      await client.recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch(() => undefined);
      expect(paths).toContain("/12345/chats.json");
    });

    it("skips a dock item with no id, as Go's zero value does", async () => {
      // An absent key and a JSON null both decode to Go's zero value and are
      // skipped at its own filter. An earlier fix here turned both into a hard
      // error, which failed the whole summarize on a dock entry Go simply
      // ignores — and deleted the test that covered it. This is that test back.
      const paths = trackRequests();
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () =>
          HttpResponse.json({
            id: BUCKET,
            dock: [
              { name: "chat", title: "Campfire", enabled: true, url: "", app_url: "" },
              { id: null, name: "chat", title: "Campfire", enabled: true, url: "", app_url: "" },
              { id: 77, name: "chat", title: "Campfire", enabled: true, url: "", app_url: "" },
            ],
          }),
        ),
        http.get(`${BASE_URL}/chats/77/lines/9`, () =>
          HttpResponse.json(recording(9, "Chat::Lines::Text", { content: "hi" })),
        ),
      );

      const summary = await client.recordings.summarize({
        bucketId: BUCKET,
        recordingId: 9,
        eventType: "chat.line.created",
      });

      expect(summary.campfire_id).toBe(77);
      expect(paths).toEqual([`/12345/projects/${BUCKET}`, "/12345/chats/77/lines/9"]);
    });

    it("refuses a dock item whose id is not a whole number, as Go's decoder does", async () => {
      // Go never reaches its `item.ID != 0` filter for a malformed id:
      // json.Unmarshal fails and the project read's own error is what surfaces.
      // Dropping the item instead would let discovery go on and report the line
      // unresolved — a composite verdict standing in for a failed read, which is
      // the distinction this whole path exists to protect.
      // A string, a fraction and a number past int64 all fail Go's decode and
      // take the whole project read with them. A fraction reaching the search
      // would have issued GET /chats/1.5/lines/{id}.
      for (const badId of ["77", 1.5, 1e20]) {
        const fresh = createBasecampClient({ accountId: "12345", accessToken: "t", enableRetry: false });
        server.use(
          http.get(`${BASE_URL}/projects/${BUCKET}`, () =>
            HttpResponse.json({
              id: BUCKET,
              dock: [{ id: badId, name: "chat", title: "C", enabled: true, url: "", app_url: "" }],
            }),
          ),
        );

        const err = await fresh.recordings
          .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
          .catch((e: unknown) => e);

        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("api_error");
        expect((err as BasecampError).httpStatus).toBeUndefined();
        expect((err as BasecampError).retryable).toBe(false);
        expect((err as BasecampError).message).toContain("project dock item");
        expect(err).not.toBeInstanceOf(UnresolvedRecordingError);
      }
    });

    it("skips a zero dock id without failing, as Go's filter does", async () => {
      const paths = trackRequests();
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () =>
          HttpResponse.json({
            id: BUCKET,
            dock: [
              { id: 0, name: "chat", title: "Campfire", enabled: true, url: "", app_url: "" },
              { id: 77, name: "chat", title: "Campfire", enabled: true, url: "", app_url: "" },
            ],
          }),
        ),
        http.get(`${BASE_URL}/chats/77/lines/9`, () =>
          HttpResponse.json(recording(9, "Chat::Lines::Text", { content: "hi" })),
        ),
      );

      const summary = await client.recordings.summarize({
        bucketId: BUCKET,
        recordingId: 9,
        eventType: "chat.line.created",
      });

      expect(summary.campfire_id).toBe(77);
      expect(paths).toEqual([`/12345/projects/${BUCKET}`, "/12345/chats/77/lines/9"]);
    });

    it("does not read a statusless not_found as \"the line is not in this Campfire\"", async () => {
      // `not_found` is also the code an UnresolvedRecordingError carries, and
      // that one is deliberately statusless. A verdict reaching discovery from
      // a hook, a middleware or a caller-supplied read source must surface, not
      // be counted as a candidate saying "not here" and end as unresolved.
      const verdict = new BasecampError("not_found", "a composite's own conclusion");
      const recordings = new RecordingsService(
        client.raw,
        undefined,
        undefined,
        undefined,
        undefined,
        undefined,
        () =>
          ({
            ...client,
            campfires: {
              ...client.campfires,
              getLine: () => Promise.reject(verdict),
            },
          }) as unknown as RecordingReadSources,
      );
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () =>
          HttpResponse.json({ id: BUCKET, dock: [{ id: 77, name: "chat", title: "C", enabled: true, url: "", app_url: "" }] }),
        ),
      );

      await expect(
        recordings.summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" }),
      ).rejects.toBe(verdict);
    });

    it("treats a bucket that is not a project as having no dock, and raises any other dock failure", async () => {
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => HttpResponse.json({ error: "nope" }, { status: 500 })),
      );
      const err = await client.recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).not.toBe("recording_unresolved");
      expect((err as BasecampError).httpStatus).toBe(500);
    });
  });

  describe("discovery caching", () => {
    /** A service with a clock the test drives, wired to the real client's reads. */
    function serviceWithClock(clock: { ms: number }): RecordingsService {
      return new RecordingsService(
        client.raw,
        undefined,
        undefined,
        undefined,
        undefined,
        undefined,
        () => client,
        () => clock.ms,
      );
    }

    const notFound = () => HttpResponse.json({ error: "Record not found" }, { status: 404 });

    it("reuses the dock snapshot across calls within the TTL, then re-reads it", async () => {
      const clock = { ms: 0 };
      const recordings = serviceWithClock(clock);
      const paths = trackRequests();
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () =>
          HttpResponse.json({ id: BUCKET, dock: [{ id: 77, name: "chat", title: "C", enabled: true, url: "", app_url: "" }] }),
        ),
        http.get(`${BASE_URL}/chats/77/lines/:lineId`, () =>
          HttpResponse.json(recording(9, "Chat::Lines::Text", { content: "hi" })),
        ),
      );

      await recordings.summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" });
      clock.ms += 60_000;
      await recordings.summarize({ bucketId: BUCKET, recordingId: 10, eventType: "chat.line.created" });
      expect(paths.filter((p) => p === `/12345/projects/${BUCKET}`)).toHaveLength(1);

      clock.ms += CAMPFIRE_INDEX_TTL_MS;
      await recordings.summarize({ bucketId: BUCKET, recordingId: 11, eventType: "chat.line.created" });
      expect(paths.filter((p) => p === `/12345/projects/${BUCKET}`)).toHaveLength(2);
    });

    it("re-reads the cached sources before concluding unresolved, but not more often than the floor", async () => {
      const clock = { ms: 0 };
      const recordings = serviceWithClock(clock);
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => HttpResponse.json({ id: BUCKET, dock: [] })),
        http.get(`${BASE_URL}/chats.json`, () => HttpResponse.json([campfire(60, BUCKET)])),
        http.get(`${BASE_URL}/chats/:campfireId/lines/:lineId`, () => notFound()),
      );

      // First call: both sources are loaded during the call, so there is
      // nothing older to refresh.
      const first = (await recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e)) as UnresolvedRecordingError;
      expect(first.refreshed).toBe(false);

      // Still inside the floor: the sources are not re-read, and the verdict
      // says so rather than claiming to have looked again.
      clock.ms += 1_000;
      const paths = trackRequests();
      const throttled = (await recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e)) as UnresolvedRecordingError;
      expect(throttled.refreshed).toBe(false);
      expect(paths).not.toContain(`/12345/projects/${BUCKET}`);

      // Past the floor: both sources are re-read before the same conclusion.
      clock.ms += 60_000;
      paths.length = 0;
      const refreshed = (await recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e)) as UnresolvedRecordingError;
      expect(refreshed.refreshed).toBe(true);
      expect(paths).toContain(`/12345/projects/${BUCKET}`);
      expect(paths).toContain("/12345/chats.json");
    });

    it("gives the dock's refresh its say before the listing is fetched", async () => {
      // The ordering the reference argues for at length: a listing that is down
      // must never stand between a project's line and the one project read that
      // finds it. Here the refreshed dock holds the Campfire and the listing
      // would fail — if the fetch came first, the call would surface a 500.
      const clock = { ms: 0 };
      const recordings = serviceWithClock(clock);
      let dockIds: number[] = [];
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () =>
          HttpResponse.json({
            id: BUCKET,
            dock: dockIds.map((id) => ({
              id,
              name: "chat",
              title: "C",
              enabled: true,
              url: "",
              app_url: "",
            })),
          }),
        ),
        http.get(`${BASE_URL}/chats.json`, () => HttpResponse.json({ error: "boom" }, { status: 500 })),
        http.get(`${BASE_URL}/chats/77/lines/9`, () =>
          HttpResponse.json(recording(9, "Chat::Lines::Text", { content: "hi" })),
        ),
      );

      // First call: empty dock, listing down, so the line is not found and the
      // listing's failure surfaces.
      const first = await recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e);
      expect((first as BasecampError).httpStatus).toBe(500);

      // The Campfire appears, and the clock passes the refresh floor.
      dockIds = [77];
      clock.ms += 60_000;
      const paths = trackRequests();
      const summary = await recordings.summarize({
        bucketId: BUCKET,
        recordingId: 9,
        eventType: "chat.line.created",
      });

      expect(summary.campfire_id).toBe(77);
      expect(paths).not.toContain("/12345/chats.json");
    });

    it("names the candidates the refreshed sources no longer list", async () => {
      const clock = { ms: 0 };
      const recordings = serviceWithClock(clock);
      let visible = [campfire(60, BUCKET), campfire(70, BUCKET)];
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => HttpResponse.json({ id: BUCKET, dock: [] })),
        http.get(`${BASE_URL}/chats.json`, () => HttpResponse.json(visible)),
        http.get(`${BASE_URL}/chats/:campfireId/lines/:lineId`, () => notFound()),
      );

      await recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch(() => undefined);

      // 70 goes out of view between the two calls: the second conclusion says
      // visibility changed, rather than silently reporting the same absence.
      visible = [campfire(60, BUCKET)];
      clock.ms += 60_000;
      const err = (await recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e)) as UnresolvedRecordingError;

      expect(err.refreshed).toBe(true);
      expect(err.staleCampfireIds).toEqual([70]);
    });

    /**
     * A service whose dock read is the test's own function, so a loader can
     * fail in ways MSW cannot produce — synchronously, or with an AbortError.
     */
    function serviceWithDockRead(get: () => Promise<{ id: number; dock?: unknown[] }>): RecordingsService {
      return new RecordingsService(
        client.raw,
        undefined,
        undefined,
        undefined,
        undefined,
        undefined,
        () =>
          ({ ...client, projects: { ...client.projects, get } }) as unknown as RecordingReadSources,
      );
    }

    const chatDock = { id: BUCKET, dock: [{ id: 77, name: "chat", title: "C", enabled: true, url: "", app_url: "" }] };

    it("releases the single-flight slot when a loader throws synchronously", async () => {
      // The slot is taken after the wrapper promise exists and its release
      // handler is attached, so no exit can leave a key marked in flight — and
      // a leaked key would park every later caller forever, since a waiter
      // waits with no timeout. This test would hang rather than fail if that
      // ordering were ever inverted.
      let calls = 0;
      const recordings = serviceWithDockRead(() => {
        calls++;
        if (calls === 1) throw new Error("the loader died before it returned a promise");
        return Promise.resolve(chatDock);
      });
      server.use(
        http.get(`${BASE_URL}/chats/77/lines/9`, () =>
          HttpResponse.json(recording(9, "Chat::Lines::Text", { content: "hi" })),
        ),
      );

      await expect(
        recordings.summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" }),
      ).rejects.toThrow("the loader died");

      const summary = await recordings.summarize({
        bucketId: BUCKET,
        recordingId: 9,
        eventType: "chat.line.created",
      });
      expect(summary.campfire_id).toBe(77);
      expect(calls).toBe(2);
    });

    it("shares one failed load with every waiter rather than re-running it", async () => {
      // An aborted fetch rejects with an AbortError whether a request timeout
      // or a caller's cancellation fired the signal, so a waiter that decided
      // anything from the rejection could not tell a dead caller from a slow
      // server — and would re-run a load Go shares, once per waiter. Nothing
      // reads the rejection: every waiter gets the same object, and the load
      // ran once.
      const aborted = Object.assign(new Error("This operation was aborted"), { name: "AbortError" });
      let calls = 0;
      const recordings = serviceWithDockRead(() => {
        calls++;
        return calls === 1 ? Promise.reject(aborted) : Promise.resolve(chatDock);
      });
      server.use(
        http.get(`${BASE_URL}/chats/77/lines/9`, () =>
          HttpResponse.json(recording(9, "Chat::Lines::Text", { content: "hi" })),
        ),
      );

      const outcomes = await Promise.allSettled([
        recordings.summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" }),
        recordings.summarize({ bucketId: BUCKET, recordingId: 10, eventType: "chat.line.created" }),
        recordings.summarize({ bucketId: BUCKET, recordingId: 11, eventType: "chat.line.created" }),
      ]);

      expect(calls).toBe(1);
      for (const outcome of outcomes) {
        expect(outcome.status).toBe("rejected");
        expect((outcome as PromiseRejectedResult).reason).toBe(aborted);
      }

      // And the slot is free afterwards: the next call loads for itself.
      const summary = await recordings.summarize({
        bucketId: BUCKET,
        recordingId: 9,
        eventType: "chat.line.created",
      });
      expect(summary.campfire_id).toBe(77);
      expect(calls).toBe(2);
    });

    it("reads a clock a wall-clock jump cannot move", async () => {
      // The discovery caches time themselves on performance.now(), which is
      // monotonic. On Date.now() a backwards step would keep an entry past its
      // TTL, decline a refresh that was due, and quietly cost the refreshed /
      // stale-candidate signal in the unresolved verdict.
      const paths = trackRequests();
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () =>
          HttpResponse.json({ id: BUCKET, dock: [{ id: 77, name: "chat", title: "C", enabled: true, url: "", app_url: "" }] }),
        ),
        http.get(`${BASE_URL}/chats/77/lines/:lineId`, () =>
          HttpResponse.json(recording(9, "Chat::Lines::Text", { content: "hi" })),
        ),
      );

      await client.recordings.summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" });

      const wallClock = vi.spyOn(Date, "now").mockReturnValue(Date.now() + 24 * 60 * 60 * 1000);
      try {
        await client.recordings.summarize({ bucketId: BUCKET, recordingId: 10, eventType: "chat.line.created" });
      } finally {
        wallClock.mockRestore();
      }

      // A day of wall clock is far past the TTL; the snapshot is still served.
      expect(paths.filter((path) => path === `/12345/projects/${BUCKET}`)).toHaveLength(1);
    });

    it("is unresolved, not incomplete, when a spent budget consulted both sources", async () => {
      // The third property of the budget boundary, and the one that produces a
      // WRONG VERDICT rather than a wasted request. Everything was searched and
      // every candidate answered 404, so nothing is unsearched: the answer is
      // recording_unresolved, which a consumer treats as settled, not
      // campfire_discovery_incomplete, which tells it to look again.
      //
      // The same case makes property 1 observable. The clock is past the
      // refresh floor, so a dock refresh guarded only on `dock.cached` WOULD
      // fire a request here; guarded on the spent budget as well, it does not.
      const clock = { ms: 0 };
      const recordings = serviceWithClock(clock);
      const dockIds = Array.from({ length: MAX_CAMPFIRE_CANDIDATES - 1 }, (_, i) => 200 + i);
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () =>
          HttpResponse.json({
            id: BUCKET,
            dock: dockIds.map((id) => ({
              id,
              name: "chat",
              title: "C",
              enabled: true,
              url: "",
              app_url: "",
            })),
          }),
        ),
        http.get(`${BASE_URL}/chats.json`, () =>
          HttpResponse.json([campfire(300, BUCKET), campfire(400, OTHER_BUCKET)]),
        ),
        http.get(`${BASE_URL}/chats/:campfireId/lines/9`, () => notFound()),
      );

      // First call fills both caches and spends the budget exactly: 49 dock
      // candidates and the one the listing adds, with none left over.
      const first = (await recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e)) as UnresolvedRecordingError;
      expect(first).toBeInstanceOf(UnresolvedRecordingError);
      expect(first.campfireIds).toHaveLength(MAX_CAMPFIRE_CANDIDATES);

      // Second call, past the refresh floor, with both sources cached.
      clock.ms += 60_000;
      const paths = trackRequests();
      const err = (await recordings
        .summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" })
        .catch((e: unknown) => e)) as UnresolvedRecordingError;

      expect(err).toBeInstanceOf(UnresolvedRecordingError);
      expect(err).not.toBeInstanceOf(CampfireDiscoveryIncompleteError);
      expect(err.kind).toBe("recording_unresolved");
      expect(err.campfireIds).toHaveLength(MAX_CAMPFIRE_CANDIDATES);
      expect(err.refreshed).toBe(false);
      // Neither source is re-read once the budget is spent.
      expect(paths).not.toContain(`/12345/projects/${BUCKET}`);
      expect(paths).not.toContain("/12345/chats.json");
    });

    it("loads a source once for concurrent callers", async () => {
      const paths = trackRequests();
      let listings = 0;
      server.use(
        http.get(`${BASE_URL}/projects/${BUCKET}`, () => HttpResponse.json({ id: BUCKET, dock: [] })),
        http.get(`${BASE_URL}/chats.json`, () => {
          listings++;
          return HttpResponse.json([campfire(60, BUCKET)]);
        }),
        http.get(`${BASE_URL}/chats/60/lines/:lineId`, () =>
          HttpResponse.json(recording(9, "Chat::Lines::Text", { content: "hi" })),
        ),
      );

      await Promise.all([
        client.recordings.summarize({ bucketId: BUCKET, recordingId: 9, eventType: "chat.line.created" }),
        client.recordings.summarize({ bucketId: BUCKET, recordingId: 10, eventType: "chat.line.created" }),
        client.recordings.summarize({ bucketId: BUCKET, recordingId: 11, eventType: "chat.line.created" }),
      ]);

      expect(listings).toBe(1);
      expect(paths.filter((p) => p === `/12345/projects/${BUCKET}`)).toHaveLength(1);
    });
  });
});
