/**
 * Basecamp TypeScript SDK Client
 *
 * Creates a type-safe client for the Basecamp 3 API using openapi-fetch.
 * Includes middleware for authentication, retry with exponential backoff,
 * and ETag-based caching.
 */

import createClient, { type Middleware } from "openapi-fetch";
import { createRequire } from "node:module";
import type { paths } from "./generated/schema.js";
import { PATH_TO_OPERATION } from "./generated/path-mapping.js";
import type { BasecampHooks, RequestInfo, RequestResult } from "./hooks.js";
import { BasecampError } from "./errors.js";
import { isLocalhost } from "./security.js";
import { parseNextLink, resolveURL, isSameOrigin } from "./pagination-utils.js";
import { type AuthStrategy, bearerAuth } from "./auth-strategy.js";

// Use createRequire for JSON import (Node 18+ compatible)
const require = createRequire(import.meta.url);
const metadata = require("./generated/metadata.json") as OperationMetadata;

// ============================================================================
// Services - Generated from OpenAPI spec (spec-driven, not hand-written)
// ============================================================================
import { ProjectsService } from "./generated/services/projects.js";
import { TodosService } from "./generated/services/todos.js";
import { TodolistsService } from "./generated/services/todolists.js";
import { TodosetsService } from "./generated/services/todosets.js";
import { PeopleService } from "./generated/services/people.js";
import { MessagesService } from "./generated/services/messages.js";
import { CommentsService } from "./generated/services/comments.js";
import { CampfiresService } from "./generated/services/campfires.js";
import { CardTablesService } from "./generated/services/card-tables.js";
import { CardsService } from "./generated/services/cards.js";
import { CardColumnsService } from "./generated/services/card-columns.js";
import { CardStepsService } from "./generated/services/card-steps.js";
import { MessageBoardsService } from "./generated/services/message-boards.js";
import { MessageTypesService } from "./generated/services/message-types.js";
import { ForwardsService } from "./generated/services/forwards.js";
import { CheckinsService } from "./generated/services/checkins.js";
import { ClientApprovalsService } from "./generated/services/client-approvals.js";
import { ClientCorrespondencesService } from "./generated/services/client-correspondences.js";
import { ClientRepliesService } from "./generated/services/client-replies.js";
import { WebhooksService } from "./generated/services/webhooks.js";
import { SubscriptionsService } from "./generated/services/subscriptions.js";
import { AttachmentsService } from "./generated/services/attachments.js";
import { VaultsService } from "./generated/services/vaults.js";
import { DocumentsService } from "./generated/services/documents.js";
import { UploadsService } from "./generated/services/uploads.js";
import { SchedulesService } from "./generated/services/schedules.js";
import { EventsService } from "./generated/services/events.js";
import { RecordingsService } from "./generated/services/recordings.js";
import { SearchService } from "./generated/services/search.js";
import { ReportsService } from "./generated/services/reports.js";
import { TemplatesService } from "./generated/services/templates.js";
import { LineupService } from "./generated/services/lineup.js";
import { TodolistGroupsService } from "./generated/services/todolist-groups.js";
import { ToolsService } from "./generated/services/tools.js";
import { TimesheetsService } from "./generated/services/timesheets.js";
import { TimelineService } from "./generated/services/timeline.js";
import { ClientVisibilityService } from "./generated/services/client-visibility.js";
import { BoostsService } from "./generated/services/boosts.js";

// ============================================================================
// Services - Hand-written (not spec-driven, e.g., OAuth flows)
// ============================================================================
import { AuthorizationService } from "./services/authorization.js";

// Re-export types for consumer convenience
export type { paths };

/**
 * Raw client type from openapi-fetch.
 * Use this when you need direct access to GET/POST/PUT/DELETE methods.
 */
export type RawClient = ReturnType<typeof createClient<paths>>;

/**
 * Enhanced Basecamp client with hooks support and service accessors.
 * Wraps the raw openapi-fetch client with observability features.
 */
export interface BasecampClient extends RawClient {
  /** The underlying raw client (for advanced use cases) */
  readonly raw: RawClient;
  /** Hooks for observability (if configured) */
  readonly hooks?: BasecampHooks;

  // =========================================================================
  // Service Accessors
  // =========================================================================

  /** Projects service - list, get, create, update, and trash projects */
  readonly projects: ProjectsService;
  /** Todos service - list, get, create, update, complete, and manage todos */
  readonly todos: TodosService;
  /** Todolists service - list, get, create, and update todo lists */
  readonly todolists: TodolistsService;
  /** Todosets service - get todo sets (container for todo lists) */
  readonly todosets: TodosetsService;
  /** People service - list, get, and manage people in your account */
  readonly people: PeopleService;
  /** Authorization service - get authorization info and identity */
  readonly authorization: AuthorizationService;
  /** Messages service - list, get, create, update, pin/unpin messages */
  readonly messages: MessagesService;
  /** Comments service - list, get, create, and update comments */
  readonly comments: CommentsService;
  /** Campfires service - list, get campfires and manage lines */
  readonly campfires: CampfiresService;
  /** Card tables service - get card tables (kanban boards) */
  readonly cardTables: CardTablesService;
  /** Cards service - list, get, create, update, and move cards */
  readonly cards: CardsService;
  /** Card columns service - get, create, update, and manage columns */
  readonly cardColumns: CardColumnsService;
  /** Card steps service - create, update, complete, and manage card steps */
  readonly cardSteps: CardStepsService;
  /** Message boards service - get message boards */
  readonly messageBoards: MessageBoardsService;
  /** Message types service - list, get, create, update, delete message types */
  readonly messageTypes: MessageTypesService;
  /** Forwards service - manage email forwards and replies */
  readonly forwards: ForwardsService;
  /** Checkins service - manage questionnaires, questions, and answers */
  readonly checkins: CheckinsService;
  /** Client approvals service - list and get client approvals */
  readonly clientApprovals: ClientApprovalsService;
  /** Client correspondences service - list and get client correspondences */
  readonly clientCorrespondences: ClientCorrespondencesService;
  /** Client replies service - list and get client replies */
  readonly clientReplies: ClientRepliesService;
  /** Webhooks service - create, update, delete webhooks */
  readonly webhooks: WebhooksService;
  /** Subscriptions service - manage notification subscriptions */
  readonly subscriptions: SubscriptionsService;
  /** Attachments service - upload files for embedding in rich text */
  readonly attachments: AttachmentsService;
  /** Vaults service - manage folders in the Files tool */
  readonly vaults: VaultsService;
  /** Documents service - manage documents in vaults */
  readonly documents: DocumentsService;
  /** Uploads service - manage files in vaults */
  readonly uploads: UploadsService;
  /** Schedules service - manage schedules and calendar entries */
  readonly schedules: SchedulesService;
  /** Events service - view recording change events */
  readonly events: EventsService;
  /** Recordings service - manage recordings (base type for most content) */
  readonly recordings: RecordingsService;
  /** Search service - full-text search across all content */
  readonly search: SearchService;
  /** Reports service - timesheet and other reports */
  readonly reports: ReportsService;
  /** Templates service - manage project templates */
  readonly templates: TemplatesService;
  /** Lineup service - manage timeline markers */
  readonly lineup: LineupService;
  /** Todolist groups service - manage groups within todolists */
  readonly todolistGroups: TodolistGroupsService;
  /** Tools service - manage project dock tools */
  readonly tools: ToolsService;
  /** Timesheets service - get timesheet data */
  readonly timesheets: TimesheetsService;
  /** Timeline service - get project timeline */
  readonly timeline: TimelineService;
  /** Client visibility service - manage client visibility */
  readonly clientVisibility: ClientVisibilityService;
  /** Boosts service - manage recording boosts */
  readonly boosts: BoostsService;
}

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
  accessToken?: TokenProvider;
  /** Authentication strategy (alternative to accessToken for custom auth schemes) */
  auth?: AuthStrategy;
  /** Base URL override (defaults to https://3.basecampapi.com/{accountId}) */
  baseUrl?: string;
  /** User-Agent header (defaults to basecamp-sdk-ts/VERSION) */
  userAgent?: string;
  /** Enable ETag-based caching (defaults to false) */
  enableCache?: boolean;
  /** Enable automatic retry on 429/503 (defaults to true) */
  enableRetry?: boolean;
  /** Hooks for observability (logging, metrics, tracing) */
  hooks?: BasecampHooks;
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
    auth,
    baseUrl = `https://3.basecampapi.com/${accountId}`,
    userAgent = DEFAULT_USER_AGENT,
    enableCache = false,
    enableRetry = true,
    hooks,
  } = options;

  // Validate auth options: exactly one of auth or accessToken must be provided
  if (auth && accessToken) {
    throw new BasecampError("usage", "Provide either 'auth' or 'accessToken', not both");
  }
  if (!auth && !accessToken) {
    throw new BasecampError("usage", "Either 'auth' or 'accessToken' is required");
  }

  const authStrategy: AuthStrategy = auth ?? bearerAuth(accessToken!);

  // Validate configuration (skip HTTPS check for localhost in dev/test)
  if (baseUrl) {
    try {
      const parsed = new URL(baseUrl);
      if (parsed.protocol !== "https:" && !isLocalhost(parsed.hostname)) {
        throw new BasecampError("usage", `Base URL must use HTTPS: ${baseUrl}`);
      }
    } catch (err) {
      if (err instanceof BasecampError) throw err;
      throw new BasecampError("usage", `Invalid base URL: ${baseUrl}`);
    }
  }

  const client = createClient<paths>({ baseUrl });

  // Apply middleware in order: auth first, then hooks, then cache, then retry
  client.use(createAuthMiddleware(authStrategy, userAgent));

  if (hooks) {
    client.use(createHooksMiddleware(hooks));
  }

  if (enableCache) {
    client.use(createCacheMiddleware());
  }

  if (enableRetry) {
    client.use(createRetryMiddleware(hooks));
  }

  // Create enhanced client with additional properties
  const enhancedClient = client as BasecampClient;
  Object.defineProperty(enhancedClient, "raw", {
    value: client,
    writable: false,
    enumerable: false,
  });
  Object.defineProperty(enhancedClient, "hooks", {
    value: hooks,
    writable: false,
    enumerable: false,
  });

  // Create fetchPage closure for pagination — uses same auth & User-Agent as main client
  const fetchPage = async (url: string): Promise<Response> => {
    const headers = new Headers({
      "User-Agent": userAgent,
      Accept: "application/json",
    });
    await authStrategy.authenticate(headers);
    return fetch(url, { headers });
  };

  // Add lazy-initialized service accessors
  // Services are created on first access and cached
  // Uses nullish coalescing assignment for atomic check-and-set in single-threaded JS
  const serviceCache: Record<string, unknown> = {};

  const defineService = <T>(name: string, factory: () => T) => {
    Object.defineProperty(enhancedClient, name, {
      get() {
        // Nullish coalescing assignment is atomic in single-threaded JS.
        // This prevents duplicate service creation during async interleaving.
        return (serviceCache[name] ??= factory()) as T;
      },
      enumerable: true,
      configurable: false,
    });
  };

  defineService("projects", () => new ProjectsService(client, hooks, fetchPage));
  defineService("todos", () => new TodosService(client, hooks, fetchPage));
  defineService("todolists", () => new TodolistsService(client, hooks, fetchPage));
  defineService("todosets", () => new TodosetsService(client, hooks, fetchPage));
  defineService("people", () => new PeopleService(client, hooks, fetchPage));
  defineService("authorization", () => new AuthorizationService(client, hooks, authStrategy, userAgent));
  defineService("messages", () => new MessagesService(client, hooks, fetchPage));
  defineService("comments", () => new CommentsService(client, hooks, fetchPage));
  defineService("campfires", () => new CampfiresService(client, hooks, fetchPage));
  defineService("cardTables", () => new CardTablesService(client, hooks, fetchPage));
  defineService("cards", () => new CardsService(client, hooks, fetchPage));
  defineService("cardColumns", () => new CardColumnsService(client, hooks, fetchPage));
  defineService("cardSteps", () => new CardStepsService(client, hooks, fetchPage));
  defineService("messageBoards", () => new MessageBoardsService(client, hooks, fetchPage));
  defineService("messageTypes", () => new MessageTypesService(client, hooks, fetchPage));
  defineService("forwards", () => new ForwardsService(client, hooks, fetchPage));
  defineService("checkins", () => new CheckinsService(client, hooks, fetchPage));
  defineService("clientApprovals", () => new ClientApprovalsService(client, hooks, fetchPage));
  defineService("clientCorrespondences", () => new ClientCorrespondencesService(client, hooks, fetchPage));
  defineService("clientReplies", () => new ClientRepliesService(client, hooks, fetchPage));
  defineService("webhooks", () => new WebhooksService(client, hooks, fetchPage));
  defineService("subscriptions", () => new SubscriptionsService(client, hooks, fetchPage));
  defineService("attachments", () => new AttachmentsService(client, hooks, fetchPage));
  defineService("vaults", () => new VaultsService(client, hooks, fetchPage));
  defineService("documents", () => new DocumentsService(client, hooks, fetchPage));
  defineService("uploads", () => new UploadsService(client, hooks, fetchPage));
  defineService("schedules", () => new SchedulesService(client, hooks, fetchPage));
  defineService("events", () => new EventsService(client, hooks, fetchPage));
  defineService("recordings", () => new RecordingsService(client, hooks, fetchPage));
  defineService("search", () => new SearchService(client, hooks, fetchPage));
  defineService("reports", () => new ReportsService(client, hooks, fetchPage));
  defineService("templates", () => new TemplatesService(client, hooks, fetchPage));
  defineService("lineup", () => new LineupService(client, hooks, fetchPage));
  defineService("todolistGroups", () => new TodolistGroupsService(client, hooks, fetchPage));
  defineService("tools", () => new ToolsService(client, hooks, fetchPage));
  defineService("timesheets", () => new TimesheetsService(client, hooks, fetchPage));
  defineService("timeline", () => new TimelineService(client, hooks, fetchPage));
  defineService("clientVisibility", () => new ClientVisibilityService(client, hooks, fetchPage));
  defineService("boosts", () => new BoostsService(client, hooks, fetchPage));

  return enhancedClient;
}

// =============================================================================
// Auth Middleware
// =============================================================================

function createAuthMiddleware(authStrategy: AuthStrategy, userAgent: string): Middleware {
  return {
    async onRequest({ request }) {
      await authStrategy.authenticate(request.headers);
      request.headers.set("User-Agent", userAgent);
      // Only set Content-Type if not already set (preserves binary uploads, etc.)
      if (!request.headers.has("Content-Type")) {
        request.headers.set("Content-Type", "application/json");
      }
      request.headers.set("Accept", "application/json");

      return request;
    },
  };
}

// =============================================================================
// Hooks Middleware
// =============================================================================

/** Tracks request timing for hooks */
interface RequestTiming {
  startTime: number;
  attempt: number;
}

/** Counter for generating unique request IDs */
let requestIdCounter = 0;

function createHooksMiddleware(hooks: BasecampHooks): Middleware {
  // Track request timing by unique request ID
  const timings = new Map<string, RequestTiming>();

  return {
    async onRequest({ request }) {
      // Generate unique request ID to handle concurrent identical requests
      const requestId = `${++requestIdCounter}`;
      request.headers.set("X-SDK-Request-Id", requestId);

      const attemptHeader = request.headers.get("X-Retry-Attempt");
      const attempt = attemptHeader ? parseInt(attemptHeader, 10) + 1 : 1;

      timings.set(requestId, { startTime: performance.now(), attempt });

      const info: RequestInfo = {
        method: request.method,
        url: request.url,
        attempt,
      };

      try {
        hooks.onRequestStart?.(info);
      } catch {
        // Hooks should not interrupt the request
      }

      return request;
    },

    async onResponse({ request, response }) {
      const requestId = request.headers.get("X-SDK-Request-Id") ?? "";
      const timing = timings.get(requestId);
      const durationMs = timing ? Math.round(performance.now() - timing.startTime) : 0;
      const attempt = timing?.attempt ?? 1;

      timings.delete(requestId);

      const info: RequestInfo = {
        method: request.method,
        url: request.url,
        attempt,
      };

      // Check for cache hit via header set by cache middleware
      const fromCacheHeader = response.headers.get("X-From-Cache");
      const fromCache =
        fromCacheHeader === "1" ||
        response.status === 304;

      const result: RequestResult = {
        statusCode: response.status,
        durationMs,
        fromCache,
      };

      try {
        hooks.onRequestEnd?.(info, result);
      } catch {
        // Hooks should not interrupt the response
      }

      return response;
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

  // Store cache keys per-request without leaking them onto the wire.
  const cacheKeyStore = new WeakMap<Request, string>();

  // Derive a short token hash from the Authorization header for cache key isolation.
  // Different auth contexts must not share cached responses.
  // Re-computed per request so refreshed tokens produce new cache keys.
  //
  // Security: The map is bounded to MAX_TOKEN_HASH_ENTRIES to prevent unbounded growth.
  // LRU-like eviction removes oldest entries when the limit is reached.
  const MAX_TOKEN_HASH_ENTRIES = 100;
  const hashTokenMap = new Map<string, string>();
  // Track pending hash computations to coalesce concurrent requests for the same token.
  // This prevents duplicate crypto operations during async interleaving.
  const pendingHashes = new Map<string, Promise<string>>();

  const evictOldestHash = () => {
    if (hashTokenMap.size >= MAX_TOKEN_HASH_ENTRIES) {
      // Delete oldest entry (first key in insertion order)
      const firstKey = hashTokenMap.keys().next().value;
      if (firstKey) hashTokenMap.delete(firstKey);
    }
  };

  const getTokenHash = async (authHeader: string | null): Promise<string> => {
    if (!authHeader) return "";

    // Check completed cache first
    const cached = hashTokenMap.get(authHeader);
    if (cached) return cached;

    // Check if computation already in progress (coalesce concurrent requests)
    const pending = pendingHashes.get(authHeader);
    if (pending) return pending;

    // Start new computation with promise coalescing
    const promise = (async () => {
      const data = new TextEncoder().encode(authHeader);
      const hashBuffer = await crypto.subtle.digest("SHA-256", data);
      const hashArray = new Uint8Array(hashBuffer);
      const hash = Array.from(hashArray.slice(0, 8))
        .map((b) => b.toString(16).padStart(2, "0"))
        .join("");
      // Evict oldest before adding new entry
      evictOldestHash();
      hashTokenMap.set(authHeader, hash);
      return hash;
    })();

    pendingHashes.set(authHeader, promise);
    promise.finally(() => pendingHashes.delete(authHeader));

    return promise;
  };

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

      const tokenHash = await getTokenHash(request.headers.get("Authorization"));
      const cacheKey = getCacheKey(request.url, tokenHash);
      const entry = cache.get(cacheKey);

      if (entry?.etag) {
        request.headers.set("If-None-Match", entry.etag);
      }

      // Store cache key internally — not on the wire
      cacheKeyStore.set(request, cacheKey);

      return request;
    },

    async onResponse({ request, response }) {
      if (request.method !== "GET") return response;

      // Prefer stored key; fall back to recomputing from the Authorization header
      // (handles cases where middleware clones the Request, breaking WeakMap identity).
      const cacheKey =
        cacheKeyStore.get(request) ??
        getCacheKey(request.url, await getTokenHash(request.headers.get("Authorization")));

      // Handle 304 Not Modified - return cached body with cache indicator
      if (response.status === 304) {
        const entry = cache.get(cacheKey);
        if (entry) {
          const headers = new Headers(response.headers);
          headers.set("X-From-Cache", "1");
          return new Response(entry.body, {
            status: 200,
            headers,
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

function getCacheKey(url: string, tokenHash: string): string {
  return `${tokenHash}:${url}`;
}

// =============================================================================
// Retry Middleware
// =============================================================================

/**
 * Type for the metadata.json file structure.
 */
interface OperationMetadata {
  operations: Record<string, {
    retry?: RetryConfig;
    idempotent?: { natural: boolean };
  }>;
}

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

// PATH_TO_OPERATION is imported from generated/path-mapping.js

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
  // Note: Some paths have context-dependent placeholders, but we use consistent
  // placeholders that match our PATH_TO_OPERATION entries
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
    tools: "{toolId}",  // dock/tools/{toolId}
    documents: "{documentId}",
    inbox_forwards: "{forwardId}",
    inboxes: "{inboxId}",
    message_boards: "{boardId}",
    messages: "{messageId}",
    question_answers: "{answerId}",
    questionnaires: "{questionnaireId}",
    questions: "{questionId}",
    by: "{personId}",  // questions/{questionId}/answers/by/{personId}
    schedule_entries: "{entryId}",
    occurrences: "{date}",  // schedule_entries/{entryId}/occurrences/{date}
    schedules: "{scheduleId}",
    todolists: "{todolistId}",  // Also handles {id} and {groupId} via context
    groups: "{groupId}",  // todolists/{todolistId}/groups
    todos: "{todoId}",
    todosets: "{todosetId}",
    uploads: "{uploadId}",
    vaults: "{vaultId}",
    webhooks: "{webhookId}",
    people: "{personId}",
    markers: "{markerId}",  // lineup/markers/{markerId}
    project_constructions: "{constructionId}",
    assigned: "{personId}",  // reports/todos/assigned/{personId}
    progress: "{personId}",  // reports/users/progress/{personId}
    users: "{personId}",  // Alternative for users/progress
  };

  // Build normalized path by replacing IDs and dates based on context
  const normalized: string[] = [];
  let prevSegment: string | null = null;
  let isFirstSegment = true;

  // Pattern for ISO-8601 date (YYYY-MM-DD)
  const datePattern = /^\d{4}-\d{2}-\d{2}$/;

  for (const segment of segments) {
    // Check if this segment is a numeric ID
    if (/^\d+$/.test(segment)) {
      // First numeric segment is always the accountId
      if (isFirstSegment) {
        normalized.push("{accountId}");
      } else {
        // Map based on preceding segment
        const placeholder = prevSegment ? idMapping[prevSegment] : undefined;
        normalized.push(placeholder ?? "{id}");
      }
    } else if (datePattern.test(segment)) {
      // ISO-8601 date - map based on preceding segment (e.g., occurrences → {date})
      const placeholder = prevSegment ? idMapping[prevSegment] : undefined;
      normalized.push(placeholder ?? "{date}");
    } else {
      normalized.push(segment);
    }
    prevSegment = segment;
    isFirstSegment = false;
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

function createRetryMiddleware(hooks?: BasecampHooks): Middleware {
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

      // Notify hooks of retry
      if (hooks?.onRetry) {
        const info: RequestInfo = {
          method: request.method,
          url: request.url,
          attempt: attempt + 1,
        };
        const error = new Error(`HTTP ${response.status}: ${response.statusText || "Request failed"}`);
        try {
          hooks.onRetry(info, attempt + 1, error, delay);
        } catch {
          // Hooks should not interrupt the retry
        }
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

    const rawNextUrl = parseNextLink(response.headers.get("Link"));
    if (!rawNextUrl) break;

    // Resolve relative URLs against the current page URL (handles path-relative links)
    const nextUrl = resolveURL(response.url, rawNextUrl);

    // Validate same-origin to prevent SSRF / token leakage via poisoned Link headers
    if (!isSameOrigin(nextUrl, initialResponse.url)) {
      throw new Error(`Pagination Link header points to different origin: ${nextUrl}`);
    }

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

    const rawNextUrl = parseNextLink(response.headers.get("Link"));
    if (!rawNextUrl) break;

    // Resolve relative URLs against the current page URL (handles path-relative links)
    const nextUrl = resolveURL(response.url, rawNextUrl);

    // Validate same-origin to prevent SSRF / token leakage via poisoned Link headers
    if (!isSameOrigin(nextUrl, initialResponse.url)) {
      throw new Error(`Pagination Link header points to different origin: ${nextUrl}`);
    }

    const headers: Record<string, string> = { Accept: "application/json" };
    if (authHeader) {
      headers["Authorization"] = authHeader;
    }

    response = await fetch(nextUrl, { headers });
  }
}

// Re-export pagination utilities (defined in pagination-utils.ts to avoid circular deps)
export { parseNextLink, resolveURL, isSameOrigin };
