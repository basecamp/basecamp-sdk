/**
 * Basecamp TypeScript SDK Client
 *
 * Creates a type-safe client for the Basecamp 3 API using openapi-fetch.
 * Includes middleware for authentication, retry with exponential backoff,
 * and ETag-based caching.
 */

import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./generated/schema.js";
import metadata from "./generated/metadata.json" with { type: "json" };

// Re-export types for consumer convenience
export type { paths };
export type BasecampClient = ReturnType<typeof createClient<paths>>;

/**
 * Token provider - either a static token string or an async function that returns a token.
 * Use an async function for token refresh scenarios.
 */
export type TokenProvider = string | (() => Promise<string>);

/**
 * Configuration options for creating a Basecamp client.
 */
export interface BasecampClientOptions {
  /** Basecamp account ID (found in your Basecamp URL) */
  accountId: string;
  /** OAuth access token or async function that returns one */
  accessToken: TokenProvider;
  /** Base URL override (defaults to https://3.basecampapi.com/{accountId}) */
  baseUrl?: string;
  /** User-Agent header (defaults to basecamp-sdk-ts/VERSION) */
  userAgent?: string;
  /** Enable ETag-based caching (defaults to true) */
  enableCache?: boolean;
  /** Enable automatic retry on 429/503 (defaults to true) */
  enableRetry?: boolean;
}

const VERSION = "0.1.0";
const DEFAULT_USER_AGENT = `basecamp-sdk-ts/${VERSION}`;

/**
 * Creates a type-safe Basecamp API client with built-in middleware for:
 * - Authentication (Bearer token)
 * - Retry with exponential backoff (respects Retry-After header)
 * - ETag-based HTTP caching
 *
 * @example
 * ```ts
 * import { createBasecampClient } from "@basecamp/sdk";
 *
 * const client = createBasecampClient({
 *   accountId: "12345",
 *   accessToken: process.env.BASECAMP_TOKEN!,
 * });
 *
 * const { data, error } = await client.GET("/projects.json");
 * ```
 */
export function createBasecampClient(options: BasecampClientOptions): BasecampClient {
  const {
    accountId,
    accessToken,
    baseUrl = `https://3.basecampapi.com/${accountId}`,
    userAgent = DEFAULT_USER_AGENT,
    enableCache = true,
    enableRetry = true,
  } = options;

  const client = createClient<paths>({ baseUrl });

  // Apply middleware in order: auth first, then cache, then retry
  client.use(createAuthMiddleware(accessToken, userAgent));

  if (enableCache) {
    client.use(createCacheMiddleware());
  }

  if (enableRetry) {
    client.use(createRetryMiddleware());
  }

  return client;
}

// =============================================================================
// Auth Middleware
// =============================================================================

function createAuthMiddleware(tokenProvider: TokenProvider, userAgent: string): Middleware {
  return {
    async onRequest({ request }) {
      const token =
        typeof tokenProvider === "function" ? await tokenProvider() : tokenProvider;

      request.headers.set("Authorization", `Bearer ${token}`);
      request.headers.set("User-Agent", userAgent);
      request.headers.set("Content-Type", "application/json");
      request.headers.set("Accept", "application/json");

      return request;
    },
  };
}

// =============================================================================
// Cache Middleware (ETag-based)
// =============================================================================

interface CacheEntry {
  etag: string;
  body: string;
}

const MAX_CACHE_ENTRIES = 1000;

function createCacheMiddleware(): Middleware {
  // Use Map for insertion-order iteration (approximates LRU)
  const cache = new Map<string, CacheEntry>();

  const evictOldest = () => {
    if (cache.size >= MAX_CACHE_ENTRIES) {
      // Delete oldest entry (first key in insertion order)
      const firstKey = cache.keys().next().value;
      if (firstKey) cache.delete(firstKey);
    }
  };

  return {
    async onRequest({ request }) {
      if (request.method !== "GET") return request;

      const cacheKey = getCacheKey(request.url);
      const entry = cache.get(cacheKey);

      if (entry?.etag) {
        request.headers.set("If-None-Match", entry.etag);
      }

      return request;
    },

    async onResponse({ request, response }) {
      if (request.method !== "GET") return response;

      const cacheKey = getCacheKey(request.url);

      // Handle 304 Not Modified - return cached body
      if (response.status === 304) {
        const entry = cache.get(cacheKey);
        if (entry) {
          return new Response(entry.body, {
            status: 200,
            headers: response.headers,
          });
        }
      }

      // Cache successful responses with ETag
      if (response.ok) {
        const etag = response.headers.get("ETag");
        if (etag) {
          const body = await response.clone().text();
          evictOldest();
          cache.set(cacheKey, { etag, body });
        }
      }

      return response;
    },
  };
}

function getCacheKey(url: string): string {
  return url;
}

// =============================================================================
// Retry Middleware
// =============================================================================

/**
 * Retry configuration matching x-basecamp-retry extension schema.
 */
interface RetryConfig {
  maxAttempts: number;
  baseDelayMs: number;
  backoff: "exponential" | "linear" | "constant";
  retryOn: number[];
}

/** Default retry config used when no operation-specific config is available */
const DEFAULT_RETRY_CONFIG: RetryConfig = {
  maxAttempts: 3,
  baseDelayMs: 1000,
  backoff: "exponential",
  retryOn: [429, 503],
};

const MAX_JITTER_MS = 100;

/**
 * Mapping from "METHOD:/path/pattern" to operation name.
 * Built from the OpenAPI paths definition.
 */
const PATH_TO_OPERATION: Record<string, string> = {
  "POST:/attachments.json": "CreateAttachment",
  "GET:/buckets/{projectId}/card_tables/cards/{cardId}": "GetCard",
  "PUT:/buckets/{projectId}/card_tables/cards/{cardId}": "UpdateCard",
  "POST:/buckets/{projectId}/card_tables/cards/{cardId}/moves.json": "MoveCard",
  "POST:/buckets/{projectId}/card_tables/cards/{cardId}/positions.json": "RepositionCardStep",
  "POST:/buckets/{projectId}/card_tables/cards/{cardId}/steps.json": "CreateCardStep",
  "GET:/buckets/{projectId}/card_tables/columns/{columnId}": "GetCardColumn",
  "PUT:/buckets/{projectId}/card_tables/columns/{columnId}": "UpdateCardColumn",
  "PUT:/buckets/{projectId}/card_tables/columns/{columnId}/color.json": "SetCardColumnColor",
  "POST:/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json": "EnableCardColumnOnHold",
  "DELETE:/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json": "DisableCardColumnOnHold",
  "GET:/buckets/{projectId}/card_tables/lists/{columnId}/cards.json": "ListCards",
  "POST:/buckets/{projectId}/card_tables/lists/{columnId}/cards.json": "CreateCard",
  "PUT:/buckets/{projectId}/card_tables/steps/{stepId}": "UpdateCardStep",
  "PUT:/buckets/{projectId}/card_tables/steps/{stepId}/completions.json": "CompleteCardStep",
  "DELETE:/buckets/{projectId}/card_tables/steps/{stepId}/completions.json": "UncompleteCardStep",
  "GET:/buckets/{projectId}/card_tables/{cardTableId}": "GetCardTable",
  "POST:/buckets/{projectId}/card_tables/{cardTableId}/columns.json": "CreateCardColumn",
  "POST:/buckets/{projectId}/card_tables/{cardTableId}/moves.json": "MoveCardColumn",
  "GET:/buckets/{projectId}/categories.json": "ListMessageTypes",
  "POST:/buckets/{projectId}/categories.json": "CreateMessageType",
  "GET:/buckets/{projectId}/categories/{typeId}": "GetMessageType",
  "PUT:/buckets/{projectId}/categories/{typeId}": "UpdateMessageType",
  "DELETE:/buckets/{projectId}/categories/{typeId}": "DeleteMessageType",
  "GET:/buckets/{projectId}/chats/{campfireId}": "GetCampfire",
  "GET:/buckets/{projectId}/chats/{campfireId}/integrations.json": "ListChatbots",
  "POST:/buckets/{projectId}/chats/{campfireId}/integrations.json": "CreateChatbot",
  "GET:/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}": "GetChatbot",
  "PUT:/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}": "UpdateChatbot",
  "DELETE:/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}": "DeleteChatbot",
  "GET:/buckets/{projectId}/chats/{campfireId}/lines.json": "ListCampfireLines",
  "POST:/buckets/{projectId}/chats/{campfireId}/lines.json": "CreateCampfireLine",
  "GET:/buckets/{projectId}/chats/{campfireId}/lines/{lineId}": "GetCampfireLine",
  "DELETE:/buckets/{projectId}/chats/{campfireId}/lines/{lineId}": "DeleteCampfireLine",
  "GET:/buckets/{projectId}/client/approvals.json": "ListClientApprovals",
  "GET:/buckets/{projectId}/client/approvals/{approvalId}": "GetClientApproval",
  "GET:/buckets/{projectId}/client/correspondences.json": "ListClientCorrespondences",
  "GET:/buckets/{projectId}/client/correspondences/{correspondenceId}": "GetClientCorrespondence",
  "GET:/buckets/{projectId}/client/recordings/{recordingId}/replies.json": "ListClientReplies",
  "GET:/buckets/{projectId}/client/replies/{replyId}": "GetClientReply",
  "GET:/buckets/{projectId}/comments/{commentId}": "GetComment",
  "PUT:/buckets/{projectId}/comments/{commentId}": "UpdateComment",
  "POST:/buckets/{projectId}/copy_tool/{toolId}": "CloneTool",
  "GET:/buckets/{projectId}/dock/{toolId}": "GetTool",
  "PUT:/buckets/{projectId}/dock/{toolId}": "UpdateTool",
  "DELETE:/buckets/{projectId}/dock/{toolId}": "DeleteTool",
  "PUT:/buckets/{projectId}/dock/{toolId}/position": "RepositionTool",
  "POST:/buckets/{projectId}/dock/{toolId}/enable": "EnableTool",
  "DELETE:/buckets/{projectId}/dock/{toolId}/enable": "DisableTool",
  "GET:/buckets/{projectId}/documents/{documentId}": "GetDocument",
  "PUT:/buckets/{projectId}/documents/{documentId}": "UpdateDocument",
  "GET:/buckets/{projectId}/inbox_forwards/{forwardId}": "GetForward",
  "GET:/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json": "ListForwardReplies",
  "POST:/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json": "CreateForwardReply",
  "GET:/buckets/{projectId}/inbox_replies/{replyId}": "GetForwardReply",
  "GET:/buckets/{projectId}/inboxes/{inboxId}": "GetInbox",
  "GET:/buckets/{projectId}/inboxes/{inboxId}/forwards.json": "ListForwards",
  "GET:/buckets/{projectId}/message_boards/{boardId}": "GetMessageBoard",
  "GET:/buckets/{projectId}/message_boards/{boardId}/messages.json": "ListMessages",
  "POST:/buckets/{projectId}/message_boards/{boardId}/messages.json": "CreateMessage",
  "GET:/buckets/{projectId}/messages/{messageId}": "GetMessage",
  "PUT:/buckets/{projectId}/messages/{messageId}": "UpdateMessage",
  "GET:/buckets/{projectId}/question_answers/{answerId}": "GetAnswer",
  "PUT:/buckets/{projectId}/question_answers/{answerId}": "UpdateAnswer",
  "GET:/buckets/{projectId}/questionnaires/{questionnaireId}": "GetQuestionnaire",
  "GET:/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json": "ListQuestions",
  "POST:/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json": "CreateQuestion",
  "GET:/buckets/{projectId}/questions/{questionId}": "GetQuestion",
  "PUT:/buckets/{projectId}/questions/{questionId}": "UpdateQuestion",
  "GET:/buckets/{projectId}/questions/{questionId}/answers.json": "ListAnswers",
  "POST:/buckets/{projectId}/questions/{questionId}/answers.json": "CreateAnswer",
  "POST:/buckets/{projectId}/recordings/{recordingId}/pin.json": "PinMessage",
  "DELETE:/buckets/{projectId}/recordings/{recordingId}/pin.json": "UnpinMessage",
  "GET:/buckets/{projectId}/recordings/{recordingId}": "GetRecording",
  "PUT:/buckets/{projectId}/recordings/{recordingId}/client_visibility": "SetClientVisibility",
  "GET:/buckets/{projectId}/recordings/{recordingId}/comments.json": "ListComments",
  "POST:/buckets/{projectId}/recordings/{recordingId}/comments.json": "CreateComment",
  "GET:/buckets/{projectId}/recordings/{recordingId}/events.json": "ListEvents",
  "PUT:/buckets/{projectId}/recordings/{recordingId}/status/active.json": "UnarchiveRecording",
  "PUT:/buckets/{projectId}/recordings/{recordingId}/status/archived.json": "ArchiveRecording",
  "PUT:/buckets/{projectId}/recordings/{recordingId}/status/trashed.json": "TrashRecording",
  "GET:/buckets/{projectId}/recordings/{recordingId}/subscription": "GetSubscription",
  "POST:/buckets/{projectId}/recordings/{recordingId}/subscription": "Subscribe",
  "PUT:/buckets/{projectId}/recordings/{recordingId}/subscription": "UpdateSubscription",
  "DELETE:/buckets/{projectId}/recordings/{recordingId}/subscription": "Unsubscribe",
  "GET:/buckets/{projectId}/recordings/{recordingId}/timesheet": "GetRecordingTimesheet",
  "GET:/buckets/{projectId}/schedule_entries/{entryId}": "GetScheduleEntry",
  "PUT:/buckets/{projectId}/schedule_entries/{entryId}": "UpdateScheduleEntry",
  "GET:/buckets/{projectId}/schedule_entries/{entryId}/occurrences/{occurrenceId}": "GetScheduleEntryOccurrence",
  "GET:/buckets/{projectId}/schedules/{scheduleId}": "GetSchedule",
  "PUT:/buckets/{projectId}/schedules/{scheduleId}/settings": "UpdateScheduleSettings",
  "GET:/buckets/{projectId}/schedules/{scheduleId}/entries.json": "ListScheduleEntries",
  "POST:/buckets/{projectId}/schedules/{scheduleId}/entries.json": "CreateScheduleEntry",
  "GET:/buckets/{projectId}/timesheet": "GetProjectTimesheet",
  "PUT:/buckets/{projectId}/todolist_groups/{groupId}/position": "RepositionTodolistGroup",
  "GET:/buckets/{projectId}/todolists/{todolistId}": "GetTodolistOrGroup",
  "PUT:/buckets/{projectId}/todolists/{todolistId}": "UpdateTodolistOrGroup",
  "GET:/buckets/{projectId}/todolists/{todolistId}/groups.json": "ListTodolistGroups",
  "POST:/buckets/{projectId}/todolists/{todolistId}/groups.json": "CreateTodolistGroup",
  "GET:/buckets/{projectId}/todolists/{todolistId}/todos.json": "ListTodos",
  "POST:/buckets/{projectId}/todolists/{todolistId}/todos.json": "CreateTodo",
  "GET:/buckets/{projectId}/todos/{todoId}": "GetTodo",
  "PUT:/buckets/{projectId}/todos/{todoId}": "UpdateTodo",
  "PUT:/buckets/{projectId}/todos/{todoId}/status/trashed.json": "TrashTodo",
  "POST:/buckets/{projectId}/todos/{todoId}/completion.json": "CompleteTodo",
  "DELETE:/buckets/{projectId}/todos/{todoId}/completion.json": "UncompleteTodo",
  "GET:/buckets/{projectId}/todosets/{todosetId}": "GetTodoset",
  "GET:/buckets/{projectId}/todosets/{todosetId}/todolists.json": "ListTodolists",
  "POST:/buckets/{projectId}/todosets/{todosetId}/todolists.json": "CreateTodolist",
  "GET:/buckets/{projectId}/uploads/{uploadId}": "GetUpload",
  "PUT:/buckets/{projectId}/uploads/{uploadId}": "UpdateUpload",
  "GET:/buckets/{projectId}/uploads/{uploadId}/versions.json": "ListUploadVersions",
  "GET:/buckets/{projectId}/vaults/{vaultId}": "GetVault",
  "PUT:/buckets/{projectId}/vaults/{vaultId}": "UpdateVault",
  "GET:/buckets/{projectId}/vaults/{vaultId}/documents.json": "ListDocuments",
  "POST:/buckets/{projectId}/vaults/{vaultId}/documents.json": "CreateDocument",
  "GET:/buckets/{projectId}/vaults/{vaultId}/uploads.json": "ListUploads",
  "POST:/buckets/{projectId}/vaults/{vaultId}/uploads.json": "CreateUpload",
  "GET:/buckets/{projectId}/vaults/{vaultId}/vaults.json": "ListVaults",
  "POST:/buckets/{projectId}/vaults/{vaultId}/vaults.json": "CreateVault",
  "GET:/buckets/{projectId}/webhooks.json": "ListWebhooks",
  "POST:/buckets/{projectId}/webhooks.json": "CreateWebhook",
  "GET:/buckets/{projectId}/webhooks/{webhookId}": "GetWebhook",
  "PUT:/buckets/{projectId}/webhooks/{webhookId}": "UpdateWebhook",
  "DELETE:/buckets/{projectId}/webhooks/{webhookId}": "DeleteWebhook",
  "GET:/chats.json": "ListCampfires",
  "GET:/circles/people.json": "ListPingablePeople",
  "POST:/my/lineup_markers.json": "CreateLineupMarker",
  "PUT:/my/lineup_markers/{markerId}": "UpdateLineupMarker",
  "DELETE:/my/lineup_markers/{markerId}": "DeleteLineupMarker",
  "GET:/my/profile.json": "GetMyProfile",
  "GET:/people.json": "ListPeople",
  "GET:/people/{personId}": "GetPerson",
  "GET:/projects.json": "ListProjects",
  "POST:/projects.json": "CreateProject",
  "GET:/projects/recordings.json": "ListRecordings",
  "GET:/projects/{projectId}": "GetProject",
  "PUT:/projects/{projectId}": "UpdateProject",
  "DELETE:/projects/{projectId}": "TrashProject",
  "GET:/projects/{projectId}/people.json": "ListProjectPeople",
  "PUT:/projects/{projectId}/people/users": "UpdateProjectAccess",
  "GET:/reports/timesheets": "GetTimesheetReport",
  "GET:/search.json": "Search",
  "GET:/search/metadata.json": "GetSearchMetadata",
  "GET:/templates.json": "ListTemplates",
  "POST:/templates.json": "CreateTemplate",
  "GET:/templates/{templateId}": "GetTemplate",
  "PUT:/templates/{templateId}": "UpdateTemplate",
  "DELETE:/templates/{templateId}": "DeleteTemplate",
  "POST:/templates/{templateId}/project_constructions.json": "CreateProjectFromTemplate",
  "GET:/templates/{templateId}/project_constructions/{constructionId}": "GetProjectConstruction",
};

/**
 * Normalizes a URL path by replacing numeric IDs with placeholder tokens.
 * For example: /buckets/123/todos/456 → /buckets/{projectId}/todos/{todoId}
 */
function normalizeUrlPath(url: string): string {
  // Parse the URL and extract the pathname
  const urlObj = new URL(url);
  let path = urlObj.pathname;

  // Remove .json suffix if present (we'll add it back for matching)
  const hasJsonSuffix = path.endsWith(".json");
  if (hasJsonSuffix) {
    path = path.slice(0, -5);
  }

  // Split path into segments
  const segments = path.split("/").filter(Boolean);

  // Map of resource names to their ID placeholder tokens
  const idMapping: Record<string, string> = {
    buckets: "{projectId}",
    projects: "{projectId}",
    templates: "{templateId}",
    card_tables: "{cardTableId}",
    cards: "{cardId}",
    columns: "{columnId}",
    lists: "{columnId}",
    steps: "{stepId}",
    categories: "{typeId}",
    chats: "{campfireId}",
    integrations: "{chatbotId}",
    lines: "{lineId}",
    approvals: "{approvalId}",
    correspondences: "{correspondenceId}",
    replies: "{replyId}",
    recordings: "{recordingId}",
    comments: "{commentId}",
    copy_tool: "{toolId}",
    dock: "{toolId}",
    documents: "{documentId}",
    inbox_forwards: "{forwardId}",
    inbox_replies: "{replyId}",
    inboxes: "{inboxId}",
    message_boards: "{boardId}",
    messages: "{messageId}",
    question_answers: "{answerId}",
    questionnaires: "{questionnaireId}",
    questions: "{questionId}",
    schedule_entries: "{entryId}",
    occurrences: "{occurrenceId}",
    schedules: "{scheduleId}",
    todolist_groups: "{groupId}",
    todolists: "{todolistId}",
    todos: "{todoId}",
    todosets: "{todosetId}",
    uploads: "{uploadId}",
    vaults: "{vaultId}",
    webhooks: "{webhookId}",
    people: "{personId}",
    lineup_markers: "{markerId}",
    project_constructions: "{constructionId}",
  };

  // Build normalized path by replacing numeric IDs based on context
  const normalized: string[] = [];
  let prevSegment: string | null = null;

  for (const segment of segments) {
    // Check if this segment is a numeric ID
    if (/^\d+$/.test(segment)) {
      // Map based on preceding segment
      const placeholder = prevSegment ? idMapping[prevSegment] : undefined;
      normalized.push(placeholder ?? "{id}");
    } else {
      normalized.push(segment);
    }
    prevSegment = segment;
  }

  // Reconstruct path
  let normalizedPath = "/" + normalized.join("/");
  if (hasJsonSuffix) {
    normalizedPath += ".json";
  }

  return normalizedPath;
}

/**
 * Gets the retry config for a specific request based on operation metadata.
 */
function getRetryConfigForRequest(method: string, url: string): RetryConfig {
  const normalizedPath = normalizeUrlPath(url);
  const key = `${method.toUpperCase()}:${normalizedPath}`;
  const operationName = PATH_TO_OPERATION[key];

  if (operationName) {
    const opMeta = metadata.operations[operationName as keyof typeof metadata.operations];
    if (opMeta?.retry) {
      return opMeta.retry as RetryConfig;
    }
  }

  return DEFAULT_RETRY_CONFIG;
}

function createRetryMiddleware(): Middleware {
  // Store request body clones keyed by a request identifier
  // This is needed because Request.body can only be read once
  const bodyCache = new Map<string, ArrayBuffer | null>();

  return {
    async onRequest({ request }) {
      // For methods that may have a body, clone it before the initial fetch
      // so we can use it for retries. Request.body can only be consumed once.
      const method = request.method.toUpperCase();
      if (method === "POST" || method === "PUT" || method === "PATCH") {
        const requestId = `${method}:${request.url}:${Date.now()}`;
        request.headers.set("X-Request-Id", requestId);

        if (request.body) {
          // Clone the body before it gets consumed
          const cloned = request.clone();
          bodyCache.set(requestId, await cloned.arrayBuffer());
        } else {
          bodyCache.set(requestId, null);
        }
      }

      return request;
    },

    async onResponse({ request, response }) {
      // Get operation-specific retry config from metadata
      const retryConfig = getRetryConfigForRequest(request.method, request.url);

      const requestId = request.headers.get("X-Request-Id");

      // Helper to clean up cached body
      const cleanupBody = () => {
        if (requestId) bodyCache.delete(requestId);
      };

      // Check if status code should trigger retry
      if (!retryConfig.retryOn.includes(response.status)) {
        cleanupBody();
        return response;
      }

      // Extract current retry attempt from custom header
      const attemptHeader = request.headers.get("X-Retry-Attempt");
      const attempt = attemptHeader ? parseInt(attemptHeader, 10) : 0;

      // Check if we've exhausted retries (maxAttempts is total attempts, not retries)
      // With maxAttempts=3: attempt 0 (initial), 1 (retry 1), 2 (retry 2) = 3 total
      if (attempt >= retryConfig.maxAttempts - 1) {
        cleanupBody();
        return response;
      }

      // Calculate delay
      let delay: number;

      // For 429, respect Retry-After header
      if (response.status === 429) {
        const retryAfter = response.headers.get("Retry-After");
        if (retryAfter) {
          const seconds = parseInt(retryAfter, 10);
          if (!isNaN(seconds)) {
            delay = seconds * 1000;
          } else {
            delay = calculateBackoffDelay(retryConfig, attempt);
          }
        } else {
          delay = calculateBackoffDelay(retryConfig, attempt);
        }
      } else {
        delay = calculateBackoffDelay(retryConfig, attempt);
      }

      // Wait before retry
      await sleep(delay);

      // Get cached body for methods that may have one
      let body: ArrayBuffer | null = null;
      if (requestId && bodyCache.has(requestId)) {
        const cachedBody = bodyCache.get(requestId);
        if (cachedBody) {
          body = cachedBody;
        }
      }

      // Create retry request with fresh body
      const retryRequest = new Request(request.url, {
        method: request.method,
        headers: new Headers(request.headers),
        body,
        signal: request.signal,
      });
      retryRequest.headers.set("X-Retry-Attempt", String(attempt + 1));

      // Retry using native fetch
      return fetch(retryRequest);
    },
  };
}

function calculateBackoffDelay(config: RetryConfig, attempt: number): number {
  const base = config.baseDelayMs;
  let delay: number;

  switch (config.backoff) {
    case "exponential":
      delay = base * Math.pow(2, attempt);
      break;
    case "linear":
      delay = base * (attempt + 1);
      break;
    case "constant":
    default:
      delay = base;
  }

  // Add jitter (0-100ms)
  const jitter = Math.random() * MAX_JITTER_MS;
  return delay + jitter;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// =============================================================================
// Pagination Helper
// =============================================================================

/**
 * Fetches all pages of a paginated resource using Link header pagination.
 * Automatically follows rel="next" links until no more pages exist.
 *
 * @example
 * ```ts
 * const response = await client.GET("/projects.json");
 *
 * const allProjects = await fetchAllPages(
 *   response.response,
 *   (r) => r.json()
 * );
 * ```
 */
export async function fetchAllPages<T>(
  initialResponse: Response,
  parse: (response: Response) => Promise<T[]>,
  authHeader?: string
): Promise<T[]> {
  const results: T[] = [];
  let response = initialResponse;

  while (true) {
    const items = await parse(response.clone());
    results.push(...items);

    const nextUrl = parseNextLink(response.headers.get("Link"));
    if (!nextUrl) break;

    const headers: Record<string, string> = { Accept: "application/json" };
    if (authHeader) {
      headers["Authorization"] = authHeader;
    }

    response = await fetch(nextUrl, { headers });
  }

  return results;
}

/**
 * Async generator that yields pages of results one at a time.
 * Useful for processing large datasets without loading everything into memory.
 *
 * @example
 * ```ts
 * for await (const page of paginateAll(response.response, (r) => r.json())) {
 *   console.log(`Processing ${page.length} items`);
 * }
 * ```
 */
export async function* paginateAll<T>(
  initialResponse: Response,
  parse: (response: Response) => Promise<T[]>,
  authHeader?: string
): AsyncGenerator<T[], void, unknown> {
  let response = initialResponse;

  while (true) {
    const items = await parse(response.clone());
    yield items;

    const nextUrl = parseNextLink(response.headers.get("Link"));
    if (!nextUrl) break;

    const headers: Record<string, string> = { Accept: "application/json" };
    if (authHeader) {
      headers["Authorization"] = authHeader;
    }

    response = await fetch(nextUrl, { headers });
  }
}

function parseNextLink(linkHeader: string | null): string | null {
  if (!linkHeader) return null;

  for (const part of linkHeader.split(",")) {
    const trimmed = part.trim();
    if (trimmed.includes('rel="next"')) {
      const match = trimmed.match(/<([^>]+)>/);
      return match?.[1] ?? null;
    }
  }

  return null;
}
