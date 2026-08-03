/**
 * Conformance test runner for the TypeScript Basecamp SDK.
 *
 * Reads JSON test definitions from conformance/tests/ and executes them
 * against the SDK using MSW (Mock Service Worker) for HTTP mocking.
 *
 * Mirrors the Go reference runner at conformance/runner/go/main.go.
 */

import { describe, it, expect, afterEach, afterAll, beforeAll } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { createBasecampClient, BasecampError } from "@37signals/basecamp";
import type { BasecampClient } from "@37signals/basecamp";
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";
import { checkDelayGaps } from "./delay-gaps.js";
import { errorRaisedFailure } from "./error-raised.js";
import { checkRequestCount, requestCountApplies } from "./request-count.js";

// =============================================================================
// Types mirroring conformance/schema.json
// =============================================================================

interface MockResponse {
  status?: number;
  networkError?: boolean;
  headers?: Record<string, string>;
  body?: unknown;
  delay?: number;
}

interface Assertion {
  type: string;
  expected?: unknown;
  min?: number;
  max?: number;
  path?: string;
  index?: number;
}

interface TestCase {
  name: string;
  description?: string;
  operation: string;
  method?: string;
  path?: string;
  pathParams?: Record<string, number | string>;
  queryParams?: Record<string, unknown>;
  requestBody?: Record<string, unknown>;
  mockResponses: MockResponse[];
  assertions: Assertion[];
  tags?: string[];
  configOverrides?: { baseUrl?: string; maxPages?: number; maxItems?: number; page?: number };
  /**
   * Live tests are loaded by live-runner.test.ts; this runner ignores them.
   * Defaults to "mock" when omitted.
   */
  mode?: "mock" | "live";
}

// =============================================================================
// Constants
// =============================================================================

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const TESTS_DIR = path.resolve(__dirname, "../../tests");

/**
 * Allowance for Node timers firing early. libuv rounds a timer's deadline down
 * internally, so `setTimeout(2000)` can elapse in ~1999.9ms of wall clock.
 * Small enough that no real backoff regression fits inside it.
 */
const TIMER_SLACK_MS = 2;
const TEST_ACCOUNT_ID = "999";

/**
 * Tests the TS SDK cannot pass, each with its reason. Kept per-line in sync
 * with SPEC §19's zero-skip roster.
 */
const TS_SDK_SKIPS: Record<string, string> = {
  "Large integer IDs preserved without precision loss":
    "JavaScript loses precision on integers > Number.MAX_SAFE_INTEGER (2^53)",
};

/**
 * Operations whose fixtures describe multi-hop download flows: the fixture
 * `path` refers to the raw download URL (or a hop other than request 0), so
 * the generic first-request path invariant does not apply. DownloadURL keeps
 * its own stricter hop-1 invariant below.
 */
const MULTI_HOP_OPERATIONS = new Set(["DownloadURL", "UploadsDownload"]);

/**
 * Recognize a genuine transport-level failure (fetch rejection). The TS SDK
 * does not wrap these into a BasecampError, so the raw error surfaces: MSW's
 * HttpResponse.error() rejects with a TypeError ("Failed to fetch"); undici
 * variants say "fetch failed". Match on the message only — an arbitrary
 * TypeError from an unrelated runner/SDK bug must NOT be misclassified as a
 * transport failure (that would let an `errorType: "network"` assertion pass
 * while hiding a real regression).
 */
function isNetworkRejection(err: unknown): boolean {
  if (!(err instanceof Error)) return false;
  return /failed to fetch|fetch failed|network(?: request)? failed|socket|econn/i.test(
    err.message,
  );
}

// =============================================================================
// Test infrastructure
// =============================================================================

const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

// =============================================================================
// Operation dispatcher
// =============================================================================

/** Fixture wire keys (snake_case) → SDK todo write params (camelCase). */
const TODO_WIRE_TO_SDK: Record<string, string> = {
  content: "content",
  description: "description",
  assignee_ids: "assigneeIds",
  completion_subscriber_ids: "completionSubscriberIds",
  due_on: "dueOn",
  starts_on: "startsOn",
  notify: "notify",
};

/** Map only the keys present in the fixture requestBody onto SDK param names. */
function mapTodoWireFields(body: Record<string, unknown>): Record<string, unknown> {
  const mapped: Record<string, unknown> = {};
  for (const [wire, sdk] of Object.entries(TODO_WIRE_TO_SDK)) {
    if (wire in body) mapped[sdk] = body[wire];
  }
  return mapped;
}

/**
 * Fixture wire keys → SDK todolist write params. The writable set on
 * `PUT /{accountId}/todolists/{id}` is exactly {name, description}, and both
 * spell the same in either direction.
 */
const TODOLIST_WIRE_TO_SDK: Record<string, string> = {
  name: "name",
  description: "description",
};

/** Map only the keys present in the fixture requestBody onto SDK param names. */
function mapTodolistWireFields(body: Record<string, unknown>): Record<string, unknown> {
  const mapped: Record<string, unknown> = {};
  for (const [wire, sdk] of Object.entries(TODOLIST_WIRE_TO_SDK)) {
    if (wire in body) mapped[sdk] = body[wire];
  }
  return mapped;
}

/**
 * Executes the appropriate SDK method for the given operation name.
 * Returns { error?, httpStatus? } so assertions can inspect outcomes.
 *
 * For request body fields: always provides non-empty values to bypass
 * client-side validation (e.g., name="", content=""). The mock server
 * returns whatever status code the test specifies regardless.
 */
async function executeOperation(
  client: BasecampClient,
  tc: TestCase,
): Promise<{ error?: BasecampError | Error; httpStatus?: number; meta?: Record<string, unknown>; result?: unknown }> {
  const params = tc.pathParams ?? {};
  const body = tc.requestBody ?? {};

  try {
    switch (tc.operation) {
      case "ListProjects": {
        const maxItems = tc.configOverrides?.maxItems;
        const page = tc.configOverrides?.page;
        const projects = await client.projects.list(
          maxItems || page ? { ...(maxItems ? { maxItems } : {}), ...(page ? { page } : {}) } : undefined,
        );
        return {
          meta: {
            totalCount: projects.meta?.totalCount ?? 0,
            truncated: projects.meta?.truncated ?? false,
          },
        };
      }

      case "GetProject": {
        const project = await client.projects.get(Number(params.projectId));
        return { result: project };
      }

      case "CreateProject":
        // Always send a non-empty name to bypass client-side validation.
        // The mock server controls what status/body is returned.
        await client.projects.create({
          name: String(body.name || "Conformance Test"),
        });
        break;

      case "UpdateProject":
        await client.projects.update(Number(params.projectId), {
          name: String(body.name || "Conformance Test"),
        });
        break;

      case "TrashProject":
        await client.projects.trash(Number(params.projectId));
        break;

      case "ListTodos": {
        const todos = await client.todos.list(Number(params.todolistId));
        return {
          meta: {
            totalCount: todos.meta?.totalCount ?? 0,
            truncated: todos.meta?.truncated ?? false,
          },
        };
      }

      case "GetTodo":
        await client.todos.get(Number(params.todoId));
        break;

      case "CreateTodo":
        // Always send non-empty content to bypass client-side validation.
        await client.todos.create(Number(params.todolistId), {
          content: String(body.content || "Conformance Test"),
          dueOn: body.due_on ? String(body.due_on) : undefined,
        });
        break;

      case "CreateTodosetTodo":
        await client.todos.createTodosetTodo(Number(params.bucketId), Number(params.todosetId), {
          content: String(body.content || "Conformance Test"),
        });
        break;

      case "CompleteTodo":
        await client.todos.complete(Number(params.todoId));
        break;

      case "Subscribe":
        await client.subscriptions.subscribe(Number(params.recordingId));
        break;

      case "ListMyBookmarks":
        await client.bookmarks.listMyBookmarks();
        break;

      case "ListMyDrafts":
        await client.drafts.listMyDrafts();
        break;

      case "GetMyNote":
        await client.myNotes.getMyNote();
        break;

      case "PrioritizeAssignment":
        await client.myAssignments.prioritizeAssignment({ id: Number(body.id) });
        break;

      case "DeprioritizeAssignment":
        await client.myAssignments.deprioritizeAssignment(Number(params.recordingId));
        break;

      case "ReorderUpNext":
        await client.myAssignments.reorderUpNext({
          sourceId: Number(body.source_id),
          position: Number(body.position),
        });
        break;

      case "GetCalendar":
        await client.calendars.getCalendar(Number(params.calendarId));
        break;

      case "UpdateCalendar":
        await client.calendars.updateCalendar(Number(params.calendarId), {
          calendar: body.calendar as { color: string },
        });
        break;

      case "UpdateMyNote":
        await client.myNotes.updateMyNote({ note: body.note as { content: string } });
        break;

      case "GetBookmark":
        await client.bookmarks.getBookmark(Number(params.recordingId));
        break;

      case "CreateBookmark":
        await client.bookmarks.createBookmark(Number(params.recordingId));
        break;

      case "DeleteBookmark":
        await client.bookmarks.deleteBookmark(Number(params.recordingId));
        break;

      case "ListFolders":
        await client.folders.listFolders();
        break;

      case "GetFolder":
        await client.folders.getFolder(Number(params.folderId));
        break;

      case "CreateFolder":
        await client.folders.createFolder({
          name: body.name as string | undefined,
          projectIds: body.project_ids as number[] | undefined,
        });
        break;

      case "UpdateFolder":
        await client.folders.updateFolder(Number(params.folderId), { name: body.name as string });
        break;

      case "DeleteFolder":
        await client.folders.deleteFolder(Number(params.folderId));
        break;

      case "UpdateTodo":
        // Merge-safe update: GET then full PUT; only fixture-present keys are passed.
        await client.todos.update(Number(params.todoId), mapTodoWireFields(body));
        break;

      case "UpdateTodolist":
        // Synthetic scenario key: the merge-safe composite, not a wire
        // operation. GET then full PUT; only fixture-present keys are passed.
        // Variant-agnostic — the same call covers a list and a group.
        await client.todolists.update(Number(params.id), mapTodolistWireFields(body));
        break;

      case "UpdateScheduleEntry":
        // Only spread participantIds when the fixture carries the key: an
        // absent key must not become [] or null on the wire.
        await client.schedules.updateEntry(Number(params.entryId), {
          ...(body.summary !== undefined ? { summary: String(body.summary) } : {}),
          ...(body.participant_ids !== undefined
            ? { participantIds: body.participant_ids as number[] }
            : {}),
        });
        break;

      case "UpdateCard":
        // Merge-safe composite: GET then PUT, resending the fetched due_on.
        await client.cards.update(Number(params.cardId), {
          ...(body.title !== undefined ? { title: String(body.title) } : {}),
          ...(body.content !== undefined ? { content: String(body.content) } : {}),
          ...(body.due_on !== undefined
            ? { dueOn: body.due_on === "" ? null : String(body.due_on) }
            : {}),
          ...(body.assignee_ids !== undefined
            ? { assigneeIds: body.assignee_ids as number[] }
            : {}),
        });
        break;

      case "UpdateCardVerbatim":
        // Raw single PUT, no read-before-write.
        await client.cards.updateVerbatim(Number(params.cardId), {
          ...(body.title !== undefined ? { title: String(body.title) } : {}),
          ...(body.content !== undefined ? { content: String(body.content) } : {}),
          ...(body.due_on !== undefined ? { dueOn: String(body.due_on) } : {}),
          ...(body.assignee_ids !== undefined
            ? { assigneeIds: body.assignee_ids as number[] }
            : {}),
        });
        break;

      case "EditTodo":
        // Synthetic scenario key: read-modify-write via the edit callback,
        // assigning each fixture-present key onto the mapped TodoFields member.
        await client.todos.edit(Number(params.todoId), (t) => {
          const mapped = mapTodoWireFields(body);
          for (const [key, value] of Object.entries(mapped)) {
            (t as unknown as Record<string, unknown>)[key] = value;
          }
        });
        break;

      case "EditTodolist":
        // Synthetic scenario key: read-modify-write via the edit callback,
        // assigning each fixture-present key onto the mapped TodolistFields
        // member.
        await client.todolists.edit(Number(params.id), (t) => {
          const mapped = mapTodolistWireFields(body);
          for (const [key, value] of Object.entries(mapped)) {
            (t as unknown as Record<string, unknown>)[key] = value;
          }
        });
        break;

      case "ReplaceTodo":
        // Verbatim sparse PUT — no GET. Fixtures always include content.
        await client.todos.replace(
          Number(params.todoId),
          mapTodoWireFields(body) as { content: string },
        );
        break;

      case "ReplaceTodolist":
        // Verbatim sparse PUT — no GET. name is required server-side, so
        // fixtures always include it.
        await client.todolists.replace(
          Number(params.id),
          mapTodolistWireFields(body) as { name: string },
        );
        break;

      case "GetTodolistOrGroup": {
        // Read-only, so mapTodolistWireFields (a requestBody mapper) does not
        // apply. One flat Todolist for a to-do list and a group alike since
        // #544; the decoded value is the case result so the responseBody
        // assertions read it rather than the raw wire body.
        const todolist = await client.todolists.get(Number(params.id));
        return { result: todolist };
      }

      case "UpdateDocument":
        // Synthetic scenario key: the merge-safe composite, not a wire
        // operation. GET then full PUT; only fixture-present keys are passed.
        await client.documents.update(Number(params.documentId), {
          ...(body.title !== undefined ? { title: String(body.title) } : {}),
          ...(body.content !== undefined ? { content: String(body.content) } : {}),
        });
        break;

      case "EditDocument":
        // Synthetic scenario key: read-modify-write via the edit callback,
        // assigning each fixture-present key onto the DocumentFields member.
        await client.documents.edit(Number(params.documentId), (d) => {
          if (body.title !== undefined) d.title = String(body.title);
          if (body.content !== undefined) d.content = String(body.content);
        });
        break;

      case "ReplaceDocument":
        // Verbatim sparse PUT — no GET. Neither field is required server-side,
        // so an omitted one stays omitted and the server clears it.
        await client.documents.replace(Number(params.documentId), {
          ...(body.title !== undefined ? { title: String(body.title) } : {}),
          ...(body.content !== undefined ? { content: String(body.content) } : {}),
        });
        break;

      case "GetTimesheetEntry":
        await client.timesheets.get(Number(params.entryId));
        break;

      case "DestroyTimesheetEntry":
        await client.timesheets.destroy(Number(params.entryId));
        break;

      case "UpdateTimesheetEntry":
        await client.timesheets.update(Number(params.entryId), {
          hours: body.hours ? String(body.hours) : undefined,
        });
        break;

      case "GetProjectTimesheet":
        await client.timesheets.forProject(Number(params.projectId));
        break;

      case "ListWebhooks":
        await client.webhooks.list(Number(params.bucketId));
        break;

      case "CreateWebhook":
        await client.webhooks.create(Number(params.bucketId), {
          payloadUrl: String(body.payload_url || "https://example.com/hook"),
          types: Array.isArray(body.types) ? body.types.map(String) : [],
        });
        break;

      case "GetProjectTimeline":
        await client.timeline.projectTimeline(Number(params.projectId));
        break;

      case "GetProgressReport":
        await client.reports.progress();
        break;

      case "GetPersonProgress":
        await client.reports.personProgress(Number(params.personId));
        break;

      case "GetEverythingMessages":
        await client.everything.everythingMessages();
        break;

      case "GetEverythingComments":
        await client.everything.everythingComments();
        break;

      case "GetEverythingCheckins":
        await client.everything.everythingCheckins();
        break;

      case "GetEverythingForwards":
        await client.everything.everythingForwards();
        break;

      case "GetEverythingFiles":
        await client.everything.everythingFiles();
        break;

      case "GetEverythingOverdueTodos":
        await client.everything.everythingOverdueTodos();
        break;

      case "GetEverythingOverdueCards":
        await client.everything.everythingOverdueCards();
        break;

      case "GetEverythingOpenTodos":
        await client.everything.everythingOpenTodos();
        break;

      case "GetEverythingCompletedTodos":
        await client.everything.everythingCompletedTodos();
        break;

      case "GetEverythingUnassignedTodos":
        await client.everything.everythingUnassignedTodos();
        break;

      case "GetEverythingNoDueDateTodos":
        await client.everything.everythingNoDueDateTodos();
        break;

      case "GetEverythingOpenCards":
        await client.everything.everythingOpenCards();
        break;

      case "GetEverythingCompletedCards":
        await client.everything.everythingCompletedCards();
        break;

      case "GetEverythingUnassignedCards":
        await client.everything.everythingUnassignedCards();
        break;

      case "GetEverythingNoDueDateCards":
        await client.everything.everythingNoDueDateCards();
        break;

      case "GetEverythingNotNowCards":
        await client.everything.everythingNotNowCards();
        break;

      case "GetTool":
        await client.tools.get(Number(params.toolId));
        break;

      case "CreateTool":
        await client.tools.create(Number(params.bucketId), {
          toolType: String(body.tool_type),
          title: body.title === undefined ? undefined : String(body.title),
        });
        break;

      case "EnableTool":
        await client.tools.enable(Number(params.toolId));
        break;

      case "UploadsDownload": {
        const result = await client.uploads.download(Number(params.uploadId));
        // Drain the stream so the socket can be reused and we don't leak.
        await new Response(result.body).arrayBuffer();
        break;
      }

      case "DownloadURL": {
        if (!tc.path) {
          throw new Error("DownloadURL test case requires a non-empty path");
        }
        const rawURL = "https://storage.3.basecamp.com" + tc.path;
        const result = await client.downloadURL(rawURL);
        // Fire-and-forget cancel — matches typescript/tests/download.test.ts.
        // Awaiting MSW's mocked ReadableStream.cancel() can hang past vitest's
        // default 5s test timeout, so don't await it here. void marks intent;
        // .catch() suppresses any unhandled-rejection from the discarded Promise.
        void result.body.cancel().catch(() => {});
        return {};
      }

      case "ListForwards":
        await client.forwards.list(Number(params.inboxId));
        break;

      // #588: nine flat spellings bc3 only draws bucket-scoped. Each pins the
      // bucketId segment on the wire — the segment whose absence made them 404.
      case "ListChatbots":
        await client.campfires.listChatbots(
          Number(params.bucketId),
          Number(params.campfireId),
        );
        break;

      case "GetChatbot":
        await client.campfires.getChatbot(
          Number(params.bucketId),
          Number(params.campfireId),
          Number(params.chatbotId),
        );
        break;

      case "CreateChatbot":
        await client.campfires.createChatbot(
          Number(params.bucketId),
          Number(params.campfireId),
          {
            serviceName: String(body.service_name),
            commandUrl: String(body.command_url),
          },
        );
        break;

      case "UpdateChatbot":
        await client.campfires.updateChatbot(
          Number(params.bucketId),
          Number(params.campfireId),
          Number(params.chatbotId),
          {
            serviceName: String(body.service_name),
            commandUrl: String(body.command_url),
          },
        );
        break;

      case "DeleteChatbot":
        await client.campfires.deleteChatbot(
          Number(params.bucketId),
          Number(params.campfireId),
          Number(params.chatbotId),
        );
        break;

      case "ListClientApprovals":
        await client.clientApprovals.list(Number(params.bucketId));
        break;

      case "ListClientCorrespondences":
        await client.clientCorrespondences.list(Number(params.bucketId));
        break;

      case "ListClientReplies":
        await client.clientReplies.list(
          Number(params.bucketId),
          Number(params.recordingId),
        );
        break;

      case "GetClientReply":
        await client.clientReplies.get(
          Number(params.bucketId),
          Number(params.recordingId),
          Number(params.replyId),
        );
        break;

      case "ListTodolistGroups": {
        // Groups decode into the same flat Todolist as their parent list.
        // Dispatch convention documented on the fixture: the FIRST element is
        // the case result, so the responseBody assertions read element 0. An
        // empty decode is a failure, not a vacuous pass — that is precisely the
        // regression these cases exist to catch.
        const groups = await client.todolistGroups.list(Number(params.todolistId));
        if (groups.length === 0) {
          throw new Error(
            "ListTodolistGroups decoded no groups; the fixture serves a non-empty list",
          );
        }
        return { result: groups[0] };
      }

      case "RepositionTodolistGroup":
        await client.todolistGroups.reposition(Number(params.groupId), {
          position: Number(body.position),
        });
        break;

      default:
      throw new Error(`Unknown operation: ${tc.operation}`);
    }

    // Success path: no error
    return {};
  } catch (err) {
    if (err instanceof BasecampError) {
      return { error: err, httpStatus: err.httpStatus };
    }
    if (err instanceof Error) {
      return { error: err };
    }
    return { error: new Error(String(err)) };
  }
}

// =============================================================================
// Mock server setup
// =============================================================================

/** True when any mock response advertises a Link rel="next" header (the TS SDK auto-paginates). */
function suiteHasLinkNext(tc: TestCase): boolean {
  return tc.mockResponses.some((r) => r.headers?.["Link"]?.includes('rel="next"'));
}

/**
 * Installs a method-agnostic catch-all MSW handler on the active API origin
 * that serves mockResponses sequentially (per hop, regardless of method or
 * path). Requests to other origins are NOT swallowed — MSW's
 * unhandled-request behavior still applies to them.
 * Returns a tracker object with request metadata.
 */
function installMockHandlers(tc: TestCase): {
  requestCount: () => number;
  requestTimes: () => number[];
  requestPaths: () => string[];
  requestMethods: () => string[];
  requestBodies: () => unknown[];
  requestHeaders: () => Record<string, string>[];
} {
  // Defense-in-depth backstop for the operationally-harmful mockResponses
  // shapes: neither mode set (would be served as `status ?? 200`, a false
  // positive) or both active. The AUTHORITATIVE oneOf enforcement is
  // `make conformance-fixtures-check` (check-jsonschema against
  // conformance/schema.json), which runs before the runners and rejects
  // {status, networkError:false} / non-true networkError that this truthiness
  // backstop intentionally lets through for cross-runner parity.
  tc.mockResponses.forEach((mock, i) => {
    const hasStatus = mock.status !== undefined;
    const hasNetworkError = mock.networkError === true;
    if (hasStatus === hasNetworkError) {
      throw new Error(
        `[${tc.name}] mockResponses[${i}] must set exactly one of status or networkError (got status=${String(mock.status)}, networkError=${String(mock.networkError)})`,
      );
    }
  });

  let responseIndex = 0;
  const times: number[] = [];
  const paths: string[] = [];
  const methods: string[] = [];
  const bodies: unknown[] = [];
  const requestHeadersList: Record<string, string>[] = [];
  let count = 0;

  // Active API origin: from configOverrides.baseUrl when present, else the
  // runner's default base URL.
  const baseUrl =
    tc.configOverrides?.baseUrl ?? `http://localhost:9876/${TEST_ACCOUNT_ID}`;
  const origin = new URL(baseUrl).origin;

  // Catch-all handler for all requests to the active API origin. Matched as
  // a RegExp on the request URL — a string pattern goes through MSW's
  // path-to-regexp parsing, which trips over bracketed IPv6 origins like
  // http://[::1]:3000.
  const originPattern = new RegExp(
    `^${origin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/`,
  );
  const handler = http.all(originPattern, async ({ request }) => {
    count++;
    // performance.now(), not Date.now(): Date.now() is floored to whole
    // milliseconds, so bracketing a 2000ms sleep can read 1999 purely from
    // rounding. See TIMER_SLACK_MS for the second, separate reason this
    // assertion needs care.
    times.push(performance.now());
    const url = new URL(request.url);
    paths.push(url.pathname);
    methods.push(request.method);
    const headerObj: Record<string, string> = {};
    request.headers.forEach((v, k) => { headerObj[k] = v; });
    requestHeadersList.push(headerObj);

    // Capture the JSON request body (parsed), or undefined when there is no
    // body or it isn't JSON. Clone first: the resolver must not consume the
    // request body MSW may still need.
    let parsedBody: unknown;
    try {
      const text = await request.clone().text();
      parsedBody = text ? JSON.parse(text) : undefined;
    } catch {
      parsedBody = undefined;
    }
    bodies.push(parsedBody);

    const idx = responseIndex++;

    if (idx >= tc.mockResponses.length) {
      // Queue exhausted. When a mock response advertises a Link rel="next"
      // header, the TS SDK auto-paginates past the scripted responses; an
      // empty 200 (with no Link header) terminates that loop cleanly.
      // Otherwise a beyond-queue request is unexpected — fail loudly with 500.
      if (suiteHasLinkNext(tc)) {
        return new HttpResponse(JSON.stringify([]), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }
      return new HttpResponse(
        JSON.stringify({ error: `mock response queue exhausted: request ${idx + 1} beyond ${tc.mockResponses.length} scripted responses` }),
        { status: 500, headers: { "Content-Type": "application/json" } },
      );
    }

    const mock = tc.mockResponses[idx]!;

    // Apply delay if specified
    if (mock.delay && mock.delay > 0) {
      await new Promise((resolve) => setTimeout(resolve, mock.delay));
    }

    // Genuine transport failure for this queued entry: MSW's
    // HttpResponse.error() rejects the fetch the way a real network error does
    // (the request counter is already incremented above).
    if (mock.networkError) {
      return HttpResponse.error();
    }

    // Build response headers
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (mock.headers) {
      for (const [k, v] of Object.entries(mock.headers)) {
        headers[k] = v;
      }
    }

    // Build response body.
    // For list operations, ensure the mock body is an array (the TS SDK's
    // openapi-fetch integration expects a raw JSON array for list endpoints,
    // not a wrapper object like {"projects": [...]}).
    let bodyToSerialize = mock.body;
    if (bodyToSerialize !== undefined && bodyToSerialize !== null) {
      // If body is an object with a single array property (e.g. {"projects": [...]}),
      // unwrap it for the TS SDK which expects raw arrays from list endpoints.
      //
      // Success bodies only: an error body with one array-valued key is the
      // unwrapped field map ({"payload_url": ["is invalid"]}), and unwrapping
      // it would rewrite the fixture on the wire.
      if (
        mock.status < 400 &&
        typeof bodyToSerialize === "object" &&
        !Array.isArray(bodyToSerialize)
      ) {
        const values = Object.values(bodyToSerialize as Record<string, unknown>);
        if (values.length === 1 && Array.isArray(values[0])) {
          bodyToSerialize = values[0];
        }
      }
    }

    const responseBody =
      bodyToSerialize !== undefined ? JSON.stringify(bodyToSerialize) : null;

    return new HttpResponse(responseBody, {
      // networkError entries return early above; the schema guarantees a
      // status is present on every non-networkError entry.
      status: mock.status ?? 200,
      headers,
    });
  });

  server.use(handler);

  return {
    requestCount: () => count,
    requestTimes: () => times,
    requestPaths: () => paths,
    requestMethods: () => methods,
    requestBodies: () => bodies,
    requestHeaders: () => requestHeadersList,
  };
}

// =============================================================================
// Assertion checker
// =============================================================================

/**
 * Resolve a per-request assertion index (0-based; negative counts from end)
 * against the number of captured requests. Returns the concrete index, or
 * undefined if out of range.
 */
function resolveRequestIndex(count: number, index: number): number | undefined {
  let i = index;
  if (i < 0) i += count;
  return i >= 0 && i < count ? i : undefined;
}

/** Resolve captured headers at index (0-based; negative counts from end), or undefined if out of range. */
function requestHeadersAt(
  all: Record<string, string>[],
  index: number,
): Record<string, string> | undefined {
  const i = resolveRequestIndex(all.length, index);
  return i === undefined ? undefined : all[i];
}

/**
 * Look up a dot-notation key inside a captured JSON request body,
 * distinguishing an absent key from a present-but-falsy value.
 */
function lookupBodyPath(
  body: unknown,
  dotPath: string,
): { present: boolean; value?: unknown } {
  let current: unknown = body;
  for (const key of dotPath.split(".")) {
    if (current === null || typeof current !== "object" || Array.isArray(current)) {
      return { present: false };
    }
    const obj = current as Record<string, unknown>;
    if (!(key in obj)) return { present: false };
    current = obj[key];
  }
  return { present: true, value: current };
}

/** Substitute {param} placeholders in a fixture path with pathParams values. */
function substitutePathParams(
  fixturePath: string,
  pathParams: Record<string, number | string> | undefined,
): string {
  return fixturePath.replace(/\{(\w+)\}/g, (match, name: string) => {
    const value = pathParams?.[name];
    return value === undefined ? match : String(value);
  });
}

function checkAssertions(
  tc: TestCase,
  tracker: ReturnType<typeof installMockHandlers>,
  result: { error?: BasecampError | Error; httpStatus?: number; meta?: Record<string, unknown> },
): void {
  // DownloadURL implicit invariant: hop 1 must hit the test case path.
  // The MSW handler is origin-wide so hop 2's relative-resolved URL is
  // served, but a regression that misroutes hop 1 to a different path on
  // the same origin would otherwise pass silently.
  if (tc.operation === "DownloadURL") {
    const recordedPaths = tracker.requestPaths();
    if (recordedPaths.length > 0 && recordedPaths[0] !== tc.path) {
      throw new Error(
        `[${tc.name}] DownloadURL hop 1 expected path ${tc.path}, got ${recordedPaths[0]}`,
      );
    }
  }

  // Generic implicit invariant for single-target operations: the handler is
  // an origin-wide catch-all, so a misrouted first request would otherwise be
  // served silently. When the fixture defines a path, the FIRST captured
  // request's URL path must contain the pathParams-substituted fixture path.
  // DownloadURL-style multi-hop operations are exempt (DownloadURL keeps its
  // stricter hop-1 check above).
  if (tc.path && !MULTI_HOP_OPERATIONS.has(tc.operation)) {
    const recordedPaths = tracker.requestPaths();
    if (recordedPaths.length > 0) {
      const expectedFragment = substitutePathParams(tc.path, tc.pathParams);
      if (!recordedPaths[0]!.includes(expectedFragment)) {
        throw new Error(
          `[${tc.name}] expected first request path to contain ${expectedFragment}, got ${recordedPaths[0]}`,
        );
      }
    }
  }

  // Implicit method invariant: the MSW handler is method-agnostic, so a
  // wrong-verb request (e.g. a PUT regressing to POST) would consume a
  // queued response silently. When the fixture declares a method and carries
  // no explicit requestMethod assertions, the first request must use it.
  if (tc.method && !tc.assertions.some((a) => a.type === "requestMethod")) {
    const methods = tracker.requestMethods();
    if (methods.length > 0 && methods[0] !== tc.method.toUpperCase()) {
      throw new Error(
        `[${tc.name}] expected first request method ${tc.method.toUpperCase()}, got ${methods[0]}`,
      );
    }
  }

  for (const assertion of tc.assertions) {
    switch (assertion.type) {
      case "requestCount": {
        // The TS SDK auto-paginates list operations, so a fixture that counts
        // first-page requests only is inapplicable — but ONLY its count is.
        // The rest of the case still runs. See requestCountApplies (#573).
        if (!requestCountApplies(tc.tags)) break;
        const failure = checkRequestCount(tracker.requestCount(), Number(assertion.expected));
        if (failure) throw new Error(`[${tc.name}] ${failure}`);
        break;
      }

      case "delayBetweenRequests": {
        // Not all gaps are retry gaps — the download flow's final gap is the
        // redirect hop to the signed URL, which is deliberately un-delayed —
        // so those fixtures name a gap with an index. See checkDelayGaps for
        // the contract and for why timer slack exists.
        const failure = checkDelayGaps(
          tracker.requestTimes(),
          assertion.min,
          assertion.index,
          TIMER_SLACK_MS,
        );
        expect(failure, `[${tc.name}] ${failure}`).toBeUndefined();
        break;
      }

      case "noError": {
        expect(
          result.error,
          `[${tc.name}] expected no error, got: ${result.error?.message}`,
        ).toBeUndefined();
        break;
      }

      // The inverse of noError, and deliberately code-agnostic. See
      // errorRaisedFailure for the contract and for why the branch lives there
      // rather than inline: no committed fixture can reach its failing side, so
      // it is unit-tested instead.
      case "errorRaised": {
        const failure = errorRaisedFailure(result.error !== undefined);
        expect(failure, `[${tc.name}] ${failure}`).toBeUndefined();
        break;
      }

      case "statusCode": {
        const expected = Number(assertion.expected);
        if (result.error instanceof BasecampError) {
          expect(
            result.error.httpStatus,
            `[${tc.name}] expected HTTP status ${expected}, got ${result.error.httpStatus}`,
          ).toBe(expected);
        } else if (result.error) {
          // Non-BasecampError: the operation threw but not with an HTTP status.
          throw new Error(
            `[${tc.name}] expected HTTP status ${expected}, but got non-HTTP error: ${result.error.message}`,
          );
        } else {
          // No error: check that the expected status is a success code
          // (2xx codes don't produce errors in the SDK)
          if (expected >= 400) {
            throw new Error(
              `[${tc.name}] expected error with HTTP status ${expected}, but operation succeeded`,
            );
          }
          // For 2xx, the assertion passes (the operation returned successfully)
        }
        break;
      }

      case "requestPath": {
        const expected = String(assertion.expected);
        const idx = assertion.index ?? 0;
        const recordedPaths = tracker.requestPaths();
        const i = resolveRequestIndex(recordedPaths.length, idx);
        if (i === undefined) {
          throw new Error(
            `[${tc.name}] expected request path ${expected} on request index ${idx}, but only ${recordedPaths.length} requests were recorded`,
          );
        }
        expect(
          recordedPaths[i],
          `[${tc.name}] expected request path ${expected} on request index ${idx}, got ${recordedPaths[i]}`,
        ).toBe(expected);
        break;
      }

      case "requestMethod": {
        const expected = String(assertion.expected);
        const idx = assertion.index ?? 0;
        const methods = tracker.requestMethods();
        const i = resolveRequestIndex(methods.length, idx);
        if (i === undefined) {
          throw new Error(
            `[${tc.name}] expected request method ${expected} on request index ${idx}, but only ${methods.length} requests were recorded`,
          );
        }
        expect(
          methods[i],
          `[${tc.name}] expected request method ${expected} on request index ${idx}, got ${methods[i]}`,
        ).toBe(expected);
        break;
      }

      case "requestBody": {
        const key = assertion.path!;
        const idx = assertion.index ?? 0;
        const bodies = tracker.requestBodies();
        const i = resolveRequestIndex(bodies.length, idx);
        if (i === undefined) {
          throw new Error(
            `[${tc.name}] expected request body key ${key} on request index ${idx}, but only ${bodies.length} requests were recorded`,
          );
        }
        const body = bodies[i];
        if (body === undefined) {
          throw new Error(
            `[${tc.name}] expected request body key ${key} on request index ${idx}, but the request had no JSON body`,
          );
        }
        const { present, value } = lookupBodyPath(body, key);
        if (!present) {
          throw new Error(
            `[${tc.name}] expected request body key ${key} on request index ${idx}, but it was absent (body: ${JSON.stringify(body)})`,
          );
        }
        expect(
          value,
          `[${tc.name}] expected request body ${key} = ${JSON.stringify(assertion.expected)} on request index ${idx}, got ${JSON.stringify(value)}`,
        ).toEqual(assertion.expected);
        break;
      }

      case "requestBodyAbsent": {
        const key = assertion.path!;
        const idx = assertion.index ?? 0;
        const bodies = tracker.requestBodies();
        const i = resolveRequestIndex(bodies.length, idx);
        if (i === undefined) {
          throw new Error(
            `[${tc.name}] expected request body key ${key} absent on request index ${idx}, but only ${bodies.length} requests were recorded`,
          );
        }
        const body = bodies[i];
        // No JSON body at all trivially satisfies absence.
        if (body !== undefined) {
          const { present, value } = lookupBodyPath(body, key);
          if (present) {
            throw new Error(
              `[${tc.name}] expected request body key ${key} absent on request index ${idx}, got ${JSON.stringify(value)}`,
            );
          }
        }
        break;
      }

      case "headerPresent": {
        const headerName = assertion.path!;
        const idx = assertion.index ?? 0;
        const headers = requestHeadersAt(tracker.requestHeaders(), idx);
        if (headers === undefined) {
          throw new Error(
            `[${tc.name}] expected header ${headerName} on request index ${idx}, but only ${tracker.requestCount()} requests were recorded`,
          );
        }
        const actual = headers[headerName.toLowerCase()];
        expect(
          actual,
          `[${tc.name}] expected header ${headerName} on request index ${idx}, but it was empty or missing`,
        ).toBeTruthy();
        break;
      }

      case "headerAbsent": {
        const headerName = assertion.path!;
        const idx = assertion.index ?? 0;
        const headers = requestHeadersAt(tracker.requestHeaders(), idx);
        if (headers === undefined) {
          throw new Error(
            `[${tc.name}] expected header ${headerName} absent on request index ${idx}, but only ${tracker.requestCount()} requests were recorded`,
          );
        }
        const actual = headers[headerName.toLowerCase()];
        expect(
          actual,
          `[${tc.name}] expected header ${headerName} absent on request index ${idx}, got "${actual}"`,
        ).toBeFalsy();
        break;
      }

      case "headerValue": {
        const headerName = assertion.path!;
        const expected = String(assertion.expected);
        const mockHeaders = tc.mockResponses[0]?.headers;
        expect(
          mockHeaders,
          `[${tc.name}] expected response header ${headerName}=${expected}, but no mock response headers defined`,
        ).toBeDefined();
        const actual = mockHeaders![headerName];
        expect(
          actual,
          `[${tc.name}] expected response header ${headerName}=${expected}, got ${actual}`,
        ).toBe(expected);
        break;
      }

      case "errorType": {
        const expectedType = String(assertion.expected);
        expect(
          result.error,
          `[${tc.name}] expected an error of type ${expectedType}`,
        ).toBeDefined();
        // Classify the error. A mapped BasecampError carries a canonical
        // `.code`. On the network path the TS SDK does NOT wrap the fetch
        // rejection into a BasecampError — it surfaces the raw transport
        // failure (a TypeError / "Failed to fetch") — so recognize that as
        // "network". Anything else is unknown: fail rather than silently pass.
        let actualType: string;
        if (result.error instanceof BasecampError) {
          actualType = result.error.code;
        } else if (isNetworkRejection(result.error)) {
          actualType = "network";
        } else {
          throw new Error(
            `[${tc.name}] expected error type "${expectedType}", got unrecognized error: ${String(result.error)}`,
          );
        }
        expect(
          actualType,
          `[${tc.name}] expected error type "${expectedType}", got "${actualType}"`,
        ).toBe(expectedType);
        break;
      }

      case "errorCode": {
        const expected = String(assertion.expected);
        if (!result.error) {
          throw new Error(`[${tc.name}] expected error code "${expected}", but got no error`);
        }
        if (result.error instanceof BasecampError) {
          expect(
            result.error.code,
            `[${tc.name}] expected error code "${expected}", got "${result.error.code}"`,
          ).toBe(expected);
        } else {
          throw new Error(`[${tc.name}] expected BasecampError with code "${expected}", got ${result.error.constructor.name}`);
        }
        break;
      }

      case "errorMessage": {
        const expected = String(assertion.expected);
        if (!result.error) {
          throw new Error(`[${tc.name}] expected error message containing "${expected}", but got no error`);
        }
        expect(
          result.error.message,
          `[${tc.name}] expected error message containing "${expected}"`,
        ).toContain(expected);
        break;
      }

      case "errorField": {
        const fieldPath = assertion.path!;
        if (!result.error) {
          throw new Error(`[${tc.name}] expected error field ${fieldPath}, but got no error`);
        }
        if (!(result.error instanceof BasecampError)) {
          throw new Error(`[${tc.name}] expected BasecampError for field ${fieldPath}, got ${result.error.constructor.name}`);
        }
        const err = result.error as BasecampError;
        let actual: unknown;
        switch (fieldPath) {
          case "httpStatus": actual = err.httpStatus; break;
          case "retryable": actual = err.retryable; break;
          case "requestId": actual = err.requestId; break;
          case "code": actual = err.code; break;
          case "message": actual = err.message; break;
          default:
            throw new Error(`[${tc.name}] unknown error field: ${fieldPath}`);
        }
        expect(
          actual,
          `[${tc.name}] expected error.${fieldPath} = ${JSON.stringify(assertion.expected)}, got ${JSON.stringify(actual)}`,
        ).toEqual(assertion.expected);
        break;
      }

      case "headerInjected": {
        const headerName = assertion.path!;
        const expected = String(assertion.expected);
        const idx = assertion.index ?? 0;
        const headers = requestHeadersAt(tracker.requestHeaders(), idx);
        if (headers === undefined) {
          throw new Error(
            `[${tc.name}] expected header ${headerName}="${expected}" on request index ${idx}, but only ${tracker.requestCount()} requests were recorded`,
          );
        }
        const actual = headers[headerName.toLowerCase()];
        expect(
          actual,
          `[${tc.name}] expected header ${headerName}="${expected}" on request index ${idx}, got "${actual}"`,
        ).toBe(expected);
        break;
      }

      case "requestScheme": {
        // HTTPS enforcement: SDK should refuse HTTP for non-localhost.
        // The errorCode assertion handles the specific error check.
        const expected = String(assertion.expected);
        if (expected === "https" && !result.error) {
          throw new Error(`[${tc.name}] expected HTTPS enforcement error, but request succeeded over HTTP`);
        }
        break;
      }

      case "urlOrigin": {
        // Cross-origin rejection: verified by requestCount=1 (link not followed).
        const expected = String(assertion.expected);
        if (expected === "rejected") {
          expect(
            tracker.requestCount(),
            `[${tc.name}] expected cross-origin URL rejection (1 request), but ${tracker.requestCount()} requests were made`,
          ).toBe(1);
        }
        break;
      }

      case "responseMeta": {
        const fieldPath = assertion.path!;
        const expected = assertion.expected;
        expect(
          result.meta,
          `[${tc.name}] expected response meta ${fieldPath}, but no metadata returned`,
        ).toBeDefined();
        const actual = result.meta![fieldPath];
        expect(
          actual,
          `[${tc.name}] expected meta.${fieldPath} = ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
        ).toEqual(expected);
        break;
      }

      case "responseStatus": {
        const expected = Number(assertion.expected);
        if (result.error) {
          if (
            result.error instanceof BasecampError &&
            result.error.httpStatus !== undefined &&
            result.error.httpStatus !== expected
          ) {
            throw new Error(
              `[${tc.name}] expected response status ${expected}, got ${result.error.httpStatus}`,
            );
          }
        } else if (expected >= 400) {
          throw new Error(
            `[${tc.name}] expected error with status ${expected}, but operation succeeded`,
          );
        }
        break;
      }

      case "responseBody": {
        const fieldPath = assertion.path!;
        const expected = assertion.expected;
        if (result.result === undefined || result.result === null) {
          throw new Error(`[${tc.name}] expected responseBody.${fieldPath}, but no result returned`);
        }
        const actual = (result.result as Record<string, unknown>)[fieldPath];
        expect(
          actual,
          `[${tc.name}] expected responseBody.${fieldPath} = ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
        ).toEqual(expected);
        break;
      }

      default:
        throw new Error(
          `[${tc.name}] unknown assertion type: ${assertion.type}`,
        );
    }
  }
}

// =============================================================================
// Load and run tests
// =============================================================================

function loadTestSuites(): { filename: string; tests: TestCase[] }[] {
  const files = fs
    .readdirSync(TESTS_DIR)
    .filter((f) => f.endsWith(".json"))
    .sort();

  return files
    .map((filename) => {
      const content = fs.readFileSync(path.join(TESTS_DIR, filename), "utf-8");
      const all = JSON.parse(content) as TestCase[];
      // Live tests are owned by live-runner.test.ts. Drop them here so they
      // never reach installMockHandlers / MSW.
      const tests = all.filter((tc) => (tc.mode ?? "mock") === "mock");
      return { filename, tests };
    })
    .filter((suite) => suite.tests.length > 0);
}

/**
 * Determine whether retry should be enabled for a given test case.
 *
 * Retry tests, idempotency tests, and network-retry tests need retry enabled.
 * Status-code tests generally need retry disabled to avoid interference,
 * except for the 429-retries-exhausted test which requires retry.
 */
function shouldEnableRetry(tc: TestCase, filename: string): boolean {
  if (
    filename === "retry.json" ||
    filename === "idempotency.json" ||
    filename === "network-retry.json" ||
    filename === "downloads.json"
  ) {
    // network-retry.json's CreateTodo safety case must run retry-ENABLED so it
    // actually proves the SDK doesn't re-send a non-idempotent POST on a network
    // error (with retry off, the requestCount:1 assertion would be vacuous).
    // downloads.json exercises the hop-1 retry policy (SPEC §14), and its
    // no-retry cases (500, redirect-no-Location) stay single-attempt because
    // those failures are outside the declared retry set, not because retry is
    // disabled.
    return true;
  }

  if (filename === "status-codes.json") {
    // The "429 Rate Limit error is surfaced after retries exhausted" test
    // needs retry enabled so the SDK exhausts retries and surfaces the 429.
    if (tc.tags?.includes("rate-limit") && tc.mockResponses.length > 1) {
      return true;
    }
    return false;
  }

  if (filename === "error-mapping.json") {
    // Rate-limit tests need retry to exhaust attempts
    if (tc.tags?.includes("rate-limit") && tc.mockResponses.length > 1) {
      return true;
    }
    return false;
  }

  // Path and pagination tests don't need retry
  return false;
}

// Generate test suites dynamically from JSON definitions
const suites = loadTestSuites();

for (const { filename, tests } of suites) {
  describe(`conformance/${filename}`, () => {
    for (const tc of tests) {
      if (tc.name in TS_SDK_SKIPS) {
        it.skip(`${tc.name} (${TS_SDK_SKIPS[tc.name]})`, () => {});
        continue;
      }

      it(tc.name, async () => {
        const enableRetry = shouldEnableRetry(tc, filename);
        const tracker = installMockHandlers(tc);

        // If configOverrides.baseUrl is set, use it for client construction.
        // The SDK may throw at construction time (e.g., HTTPS enforcement).
        const baseUrl = tc.configOverrides?.baseUrl
          ?? `http://localhost:9876/${TEST_ACCOUNT_ID}`;

        let result: { error?: BasecampError | Error; httpStatus?: number };
        try {
          const client = createBasecampClient({
            accountId: TEST_ACCOUNT_ID,
            accessToken: "conformance-test-token",
            baseUrl,
            enableRetry,
            maxPages: tc.configOverrides?.maxPages,
          });

          result = await executeOperation(client, tc);
        } catch (err) {
          if (err instanceof BasecampError) {
            result = { error: err, httpStatus: err.httpStatus };
          } else if (err instanceof Error) {
            result = { error: err };
          } else {
            result = { error: new Error(String(err)) };
          }
        }

        checkAssertions(tc, tracker, result);
      });
    }
  });
}
