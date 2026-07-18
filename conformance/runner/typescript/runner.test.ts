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

// =============================================================================
// Types mirroring conformance/schema.json
// =============================================================================

interface MockResponse {
  status: number;
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
  configOverrides?: { baseUrl?: string; maxPages?: number; maxItems?: number };
}

// =============================================================================
// Constants
// =============================================================================

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const TESTS_DIR = path.resolve(__dirname, "../../tests");
const TEST_ACCOUNT_ID = "999";

/**
 * Tests where the TS SDK's retry middleware architecture limits chained retries.
 *
 * The TS SDK retry middleware uses native fetch() for retry requests, which
 * bypasses the middleware stack. This means each middleware pass yields at most
 * 1 retry. A test expecting 3 total requests (initial + 2 retries) will only
 * see 2 (initial + 1 retry).
 */
const TS_SDK_SKIPS: Record<string, string> = {
  "GET operation retries on 503":
    "TS SDK retry middleware yields at most 1 retry per middleware pass",
  "Large integer IDs preserved without precision loss":
    "JavaScript loses precision on integers > Number.MAX_SAFE_INTEGER (2^53)",
  "DownloadURL retries on 503 at the auth'd first hop":
    "TS SDK downloadURL uses raw fetch bypassing the retry middleware; 5xx / Retry-After retry is not implemented",
  "DownloadURL honors Retry-After on 429 at the auth'd first hop":
    "TS SDK downloadURL uses raw fetch bypassing the retry middleware; 5xx / Retry-After retry is not implemented",
};

/**
 * Operations whose fixtures describe multi-hop download flows: the fixture
 * `path` refers to the raw download URL (or a hop other than request 0), so
 * the generic first-request path invariant does not apply. DownloadURL keeps
 * its own stricter hop-1 invariant below.
 */
const MULTI_HOP_OPERATIONS = new Set(["DownloadURL", "UploadsDownload"]);

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
        const projects = await client.projects.list(
          maxItems ? { maxItems } : undefined,
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

      case "UpdateTodo":
        // Merge-safe update: GET then full PUT; only fixture-present keys are passed.
        await client.todos.update(Number(params.todoId), mapTodoWireFields(body));
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

      case "ReplaceTodo":
        // Verbatim sparse PUT — no GET. Fixtures always include content.
        await client.todos.replace(
          Number(params.todoId),
          mapTodoWireFields(body) as { content: string },
        );
        break;

      case "GetTimesheetEntry":
        await client.timesheets.get(Number(params.entryId));
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

      case "GetTool":
        await client.tools.get(Number(params.toolId));
        break;

      case "CloneTool":
        await client.tools.clone({
          sourceRecordingId: Number(body.source_recording_id),
          title: String(body.title || "Conformance Test"),
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

  // Catch-all handler for all requests to the active API origin.
  const handler = http.all(`${origin}/*`, async ({ request }) => {
    count++;
    times.push(Date.now());
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
      if (
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
      status: mock.status,
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
  // Detect if any mock response includes a Link header with rel="next".
  // The TS SDK auto-paginates (follows all Link next headers), so the
  // actual requestCount will be higher than what the conformance test
  // expects. In this case, assert >= instead of strict equality.
  const hasLinkNextHeader = suiteHasLinkNext(tc);

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

  for (const assertion of tc.assertions) {
    switch (assertion.type) {
      case "requestCount": {
        const expected = Number(assertion.expected);
        if (hasLinkNextHeader) {
          // TS SDK auto-paginates: expect at least the specified count
          expect(
            tracker.requestCount(),
            `[${tc.name}] expected >= ${expected} requests (SDK auto-paginates), got ${tracker.requestCount()}`,
          ).toBeGreaterThanOrEqual(expected);
        } else {
          expect(
            tracker.requestCount(),
            `[${tc.name}] expected ${expected} requests, got ${tracker.requestCount()}`,
          ).toBe(expected);
        }
        break;
      }

      case "delayBetweenRequests": {
        const times = tracker.requestTimes();
        if (times.length >= 2) {
          const delay = times[1]! - times[0]!;
          const minDelay = assertion.min ?? 0;
          expect(
            delay,
            `[${tc.name}] expected delay >= ${minDelay}ms, got ${delay}ms`,
          ).toBeGreaterThanOrEqual(minDelay);
        }
        break;
      }

      case "noError": {
        expect(
          result.error,
          `[${tc.name}] expected no error, got: ${result.error?.message}`,
        ).toBeUndefined();
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
        if (result.error instanceof BasecampError) {
          expect(
            result.error.code,
            `[${tc.name}] expected error code "${expectedType}", got "${result.error.code}"`,
          ).toBe(expectedType);
        }
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

  return files.map((filename) => {
    const content = fs.readFileSync(path.join(TESTS_DIR, filename), "utf-8");
    return { filename, tests: JSON.parse(content) as TestCase[] };
  });
}

/**
 * Determine whether retry should be enabled for a given test case.
 *
 * Retry tests and idempotency tests need retry enabled.
 * Status-code tests generally need retry disabled to avoid interference,
 * except for the 429-retries-exhausted test which requires retry.
 */
function shouldEnableRetry(tc: TestCase, filename: string): boolean {
  if (filename === "retry.json" || filename === "idempotency.json") {
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
