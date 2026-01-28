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
import type { BasecampHooks, RequestInfo, RequestResult } from "./hooks.js";

// Use createRequire for JSON import (Node 18+ compatible)
const require = createRequire(import.meta.url);
const metadata = require("./generated/metadata.json") as OperationMetadata;

// Services
import { ProjectsService } from "./services/projects.js";
import { TodosService } from "./services/todos.js";
import { TodolistsService } from "./services/todolists.js";
import { TodosetsService } from "./services/todosets.js";
import { PeopleService } from "./services/people.js";
import { AuthorizationService } from "./services/authorization.js";
import { MessagesService } from "./services/messages.js";
import { CommentsService } from "./services/comments.js";
import { CampfiresService } from "./services/campfires.js";
import {
  CardTablesService,
  CardsService,
  CardColumnsService,
  CardStepsService,
} from "./services/cards.js";
import { MessageBoardsService } from "./services/message-boards.js";
import { MessageTypesService } from "./services/message-types.js";
import { ForwardsService } from "./services/forwards.js";
import { CheckinsService } from "./services/checkins.js";
import { ClientApprovalsService } from "./services/client-approvals.js";
import { ClientCorrespondencesService } from "./services/client-correspondences.js";
import { ClientRepliesService } from "./services/client-replies.js";
import { WebhooksService } from "./services/webhooks.js";
import { SubscriptionsService } from "./services/subscriptions.js";
import { AttachmentsService } from "./services/attachments.js";
import { VaultsService } from "./services/vaults.js";
import { DocumentsService } from "./services/documents.js";
import { UploadsService } from "./services/uploads.js";
import { SchedulesService } from "./services/schedules.js";
import { EventsService } from "./services/events.js";
import { RecordingsService } from "./services/recordings.js";
import { SearchService } from "./services/search.js";
import { ReportsService } from "./services/reports.js";
import { TemplatesService } from "./services/templates.js";
import { LineupService } from "./services/lineup.js";
import { TodolistGroupsService } from "./services/todolistGroups.js";
import { ToolsService } from "./services/tools.js";

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
  accessToken: TokenProvider;
  /** Base URL override (defaults to https://3.basecampapi.com/{accountId}) */
  baseUrl?: string;
  /** User-Agent header (defaults to basecamp-sdk-ts/VERSION) */
  userAgent?: string;
  /** Enable ETag-based caching (defaults to true) */
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
    baseUrl = `https://3.basecampapi.com/${accountId}`,
    userAgent = DEFAULT_USER_AGENT,
    enableCache = true,
    enableRetry = true,
    hooks,
  } = options;

  const client = createClient<paths>({ baseUrl });

  // Apply middleware in order: auth first, then hooks, then cache, then retry
  client.use(createAuthMiddleware(accessToken, userAgent));

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

  // Add lazy-initialized service accessors
  // Services are created on first access and cached
  const serviceCache: Record<string, unknown> = {};

  const defineService = <T>(name: string, factory: () => T) => {
    Object.defineProperty(enhancedClient, name, {
      get() {
        if (!serviceCache[name]) {
          serviceCache[name] = factory();
        }
        return serviceCache[name] as T;
      },
      enumerable: true,
      configurable: false,
    });
  };

  defineService("projects", () => new ProjectsService(client, hooks));
  defineService("todos", () => new TodosService(client, hooks));
  defineService("todolists", () => new TodolistsService(client, hooks));
  defineService("todosets", () => new TodosetsService(client, hooks));
  defineService("people", () => new PeopleService(client, hooks));
  defineService("authorization", () => new AuthorizationService(client, hooks, accessToken, userAgent));
  defineService("messages", () => new MessagesService(client, hooks));
  defineService("comments", () => new CommentsService(client, hooks));
  defineService("campfires", () => new CampfiresService(client, hooks));
  defineService("cardTables", () => new CardTablesService(client, hooks));
  defineService("cards", () => new CardsService(client, hooks));
  defineService("cardColumns", () => new CardColumnsService(client, hooks));
  defineService("cardSteps", () => new CardStepsService(client, hooks));
  defineService("messageBoards", () => new MessageBoardsService(client, hooks));
  defineService("messageTypes", () => new MessageTypesService(client, hooks));
  defineService("forwards", () => new ForwardsService(client, hooks));
  defineService("checkins", () => new CheckinsService(client, hooks));
  defineService("clientApprovals", () => new ClientApprovalsService(client, hooks));
  defineService("clientCorrespondences", () => new ClientCorrespondencesService(client, hooks));
  defineService("clientReplies", () => new ClientRepliesService(client, hooks));
  defineService("webhooks", () => new WebhooksService(client, hooks));
  defineService("subscriptions", () => new SubscriptionsService(client, hooks));
  defineService("attachments", () => new AttachmentsService(client, hooks));
  defineService("vaults", () => new VaultsService(client, hooks));
  defineService("documents", () => new DocumentsService(client, hooks));
  defineService("uploads", () => new UploadsService(client, hooks));
  defineService("schedules", () => new SchedulesService(client, hooks));
  defineService("events", () => new EventsService(client, hooks));
  defineService("recordings", () => new RecordingsService(client, hooks));
  defineService("search", () => new SearchService(client, hooks));
  defineService("reports", () => new ReportsService(client, hooks));
  defineService("templates", () => new TemplatesService(client, hooks));
  defineService("lineup", () => new LineupService(client, hooks));
  defineService("todolistGroups", () => new TodolistGroupsService(client, hooks));
  defineService("tools", () => new ToolsService(client, hooks));

  return enhancedClient;
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

function getCacheKey(url: string): string {
  return url;
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

/**
 * Mapping from "METHOD:/{accountId}/path/pattern" to operation name.
 * Built from the OpenAPI paths definition.
 * IMPORTANT: These paths MUST exactly match the OpenAPI spec paths for retry config to work.
 */
const PATH_TO_OPERATION: Record<string, string> = {
  // Attachments
  "POST:/{accountId}/attachments.json": "CreateAttachment",

  // Card Tables
  "GET:/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}": "GetCard",
  "PUT:/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}": "UpdateCard",
  "POST:/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}/moves.json": "MoveCard",
  "POST:/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}/positions.json": "RepositionCardStep",
  "POST:/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}/steps.json": "CreateCardStep",
  "GET:/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}": "GetCardColumn",
  "PUT:/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}": "UpdateCardColumn",
  "PUT:/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}/color.json": "SetCardColumnColor",
  "POST:/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json": "EnableCardColumnOnHold",
  "DELETE:/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json": "DisableCardColumnOnHold",
  "GET:/{accountId}/buckets/{projectId}/card_tables/lists/{columnId}/cards.json": "ListCards",
  "POST:/{accountId}/buckets/{projectId}/card_tables/lists/{columnId}/cards.json": "CreateCard",
  "PUT:/{accountId}/buckets/{projectId}/card_tables/steps/{stepId}": "UpdateCardStep",
  "PUT:/{accountId}/buckets/{projectId}/card_tables/steps/{stepId}/completions.json": "CompleteCardStep",
  "DELETE:/{accountId}/buckets/{projectId}/card_tables/steps/{stepId}/completions.json": "UncompleteCardStep",
  "GET:/{accountId}/buckets/{projectId}/card_tables/{cardTableId}": "GetCardTable",
  "POST:/{accountId}/buckets/{projectId}/card_tables/{cardTableId}/columns.json": "CreateCardColumn",
  "POST:/{accountId}/buckets/{projectId}/card_tables/{cardTableId}/moves.json": "MoveCardColumn",

  // Message Categories/Types
  "GET:/{accountId}/buckets/{projectId}/categories.json": "ListMessageTypes",
  "POST:/{accountId}/buckets/{projectId}/categories.json": "CreateMessageType",
  "GET:/{accountId}/buckets/{projectId}/categories/{typeId}": "GetMessageType",
  "PUT:/{accountId}/buckets/{projectId}/categories/{typeId}": "UpdateMessageType",
  "DELETE:/{accountId}/buckets/{projectId}/categories/{typeId}": "DeleteMessageType",

  // Campfires/Chats
  "GET:/{accountId}/buckets/{projectId}/chats/{campfireId}": "GetCampfire",
  "GET:/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations.json": "ListChatbots",
  "POST:/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations.json": "CreateChatbot",
  "GET:/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}": "GetChatbot",
  "PUT:/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}": "UpdateChatbot",
  "DELETE:/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}": "DeleteChatbot",
  "GET:/{accountId}/buckets/{projectId}/chats/{campfireId}/lines.json": "ListCampfireLines",
  "POST:/{accountId}/buckets/{projectId}/chats/{campfireId}/lines.json": "CreateCampfireLine",
  "GET:/{accountId}/buckets/{projectId}/chats/{campfireId}/lines/{lineId}": "GetCampfireLine",
  "DELETE:/{accountId}/buckets/{projectId}/chats/{campfireId}/lines/{lineId}": "DeleteCampfireLine",

  // Client Portal
  "GET:/{accountId}/buckets/{projectId}/client/approvals.json": "ListClientApprovals",
  "GET:/{accountId}/buckets/{projectId}/client/approvals/{approvalId}": "GetClientApproval",
  "GET:/{accountId}/buckets/{projectId}/client/correspondences.json": "ListClientCorrespondences",
  "GET:/{accountId}/buckets/{projectId}/client/correspondences/{correspondenceId}": "GetClientCorrespondence",
  "GET:/{accountId}/buckets/{projectId}/client/recordings/{recordingId}/replies.json": "ListClientReplies",
  "GET:/{accountId}/buckets/{projectId}/client/recordings/{recordingId}/replies/{replyId}": "GetClientReply",

  // Comments
  "GET:/{accountId}/buckets/{projectId}/comments/{commentId}": "GetComment",
  "PUT:/{accountId}/buckets/{projectId}/comments/{commentId}": "UpdateComment",

  // Dock/Tools (paths include /dock/tools/)
  "POST:/{accountId}/buckets/{projectId}/dock/tools/{toolId}/clone.json": "CloneTool",
  "GET:/{accountId}/buckets/{projectId}/dock/tools/{toolId}": "GetTool",
  "PUT:/{accountId}/buckets/{projectId}/dock/tools/{toolId}": "UpdateTool",
  "DELETE:/{accountId}/buckets/{projectId}/dock/tools/{toolId}": "DeleteTool",
  "PUT:/{accountId}/buckets/{projectId}/dock/tools/{toolId}/position.json": "RepositionTool",
  "POST:/{accountId}/buckets/{projectId}/dock/tools/{toolId}/position.json": "EnableTool",
  "DELETE:/{accountId}/buckets/{projectId}/dock/tools/{toolId}/position.json": "DisableTool",

  // Documents
  "GET:/{accountId}/buckets/{projectId}/documents/{documentId}": "GetDocument",
  "PUT:/{accountId}/buckets/{projectId}/documents/{documentId}": "UpdateDocument",

  // Forwards (Inbox)
  "GET:/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}": "GetForward",
  "GET:/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json": "ListForwardReplies",
  "POST:/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json": "CreateForwardReply",
  "GET:/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}/replies/{replyId}": "GetForwardReply",
  "GET:/{accountId}/buckets/{projectId}/inboxes/{inboxId}": "GetInbox",
  "GET:/{accountId}/buckets/{projectId}/inboxes/{inboxId}/forwards.json": "ListForwards",

  // Message Boards
  "GET:/{accountId}/buckets/{projectId}/message_boards/{boardId}": "GetMessageBoard",
  "GET:/{accountId}/buckets/{projectId}/message_boards/{boardId}/messages.json": "ListMessages",
  "POST:/{accountId}/buckets/{projectId}/message_boards/{boardId}/messages.json": "CreateMessage",

  // Messages
  "GET:/{accountId}/buckets/{projectId}/messages/{messageId}": "GetMessage",
  "PUT:/{accountId}/buckets/{projectId}/messages/{messageId}": "UpdateMessage",

  // Question & Answers (Checkins)
  "GET:/{accountId}/buckets/{projectId}/question_answers/{answerId}": "GetAnswer",
  "PUT:/{accountId}/buckets/{projectId}/question_answers/{answerId}": "UpdateAnswer",
  "GET:/{accountId}/buckets/{projectId}/questionnaires/{questionnaireId}": "GetQuestionnaire",
  "GET:/{accountId}/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json": "ListQuestions",
  "POST:/{accountId}/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json": "CreateQuestion",
  "GET:/{accountId}/buckets/{projectId}/questions/{questionId}": "GetQuestion",
  "PUT:/{accountId}/buckets/{projectId}/questions/{questionId}": "UpdateQuestion",
  "GET:/{accountId}/buckets/{projectId}/questions/{questionId}/answers.json": "ListAnswers",
  "POST:/{accountId}/buckets/{projectId}/questions/{questionId}/answers.json": "CreateAnswer",

  // Recordings
  "POST:/{accountId}/buckets/{projectId}/recordings/{recordingId}/pin.json": "PinMessage",
  "DELETE:/{accountId}/buckets/{projectId}/recordings/{recordingId}/pin.json": "UnpinMessage",
  "GET:/{accountId}/buckets/{projectId}/recordings/{recordingId}": "GetRecording",
  "PUT:/{accountId}/buckets/{projectId}/recordings/{recordingId}/client_visibility.json": "SetClientVisibility",
  "GET:/{accountId}/buckets/{projectId}/recordings/{recordingId}/comments.json": "ListComments",
  "POST:/{accountId}/buckets/{projectId}/recordings/{recordingId}/comments.json": "CreateComment",
  "GET:/{accountId}/buckets/{projectId}/recordings/{recordingId}/events.json": "ListEvents",
  "PUT:/{accountId}/buckets/{projectId}/recordings/{recordingId}/status/active.json": "UnarchiveRecording",
  "PUT:/{accountId}/buckets/{projectId}/recordings/{recordingId}/status/archived.json": "ArchiveRecording",
  "PUT:/{accountId}/buckets/{projectId}/recordings/{recordingId}/status/trashed.json": "TrashRecording",
  "GET:/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json": "GetSubscription",
  "POST:/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json": "Subscribe",
  "PUT:/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json": "UpdateSubscription",
  "DELETE:/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json": "Unsubscribe",
  "GET:/{accountId}/buckets/{projectId}/recordings/{recordingId}/timesheet.json": "GetRecordingTimesheet",

  // Schedules
  "GET:/{accountId}/buckets/{projectId}/schedule_entries/{entryId}": "GetScheduleEntry",
  "PUT:/{accountId}/buckets/{projectId}/schedule_entries/{entryId}": "UpdateScheduleEntry",
  "GET:/{accountId}/buckets/{projectId}/schedule_entries/{entryId}/occurrences/{date}": "GetScheduleEntryOccurrence",
  "GET:/{accountId}/buckets/{projectId}/schedules/{scheduleId}": "GetSchedule",
  "PUT:/{accountId}/buckets/{projectId}/schedules/{scheduleId}": "UpdateScheduleSettings",
  "GET:/{accountId}/buckets/{projectId}/schedules/{scheduleId}/entries.json": "ListScheduleEntries",
  "POST:/{accountId}/buckets/{projectId}/schedules/{scheduleId}/entries.json": "CreateScheduleEntry",

  // Timeline & Timesheet
  "GET:/{accountId}/buckets/{projectId}/timeline.json": "GetProjectTimeline",
  "GET:/{accountId}/buckets/{projectId}/timesheet.json": "GetProjectTimesheet",

  // Todolist Groups (all use {todolistId} for consistent normalization)
  "PUT:/{accountId}/buckets/{projectId}/todolists/{todolistId}/position.json": "RepositionTodolistGroup",
  "GET:/{accountId}/buckets/{projectId}/todolists/{todolistId}": "GetTodolistOrGroup",
  "PUT:/{accountId}/buckets/{projectId}/todolists/{todolistId}": "UpdateTodolistOrGroup",
  "GET:/{accountId}/buckets/{projectId}/todolists/{todolistId}/groups.json": "ListTodolistGroups",
  "POST:/{accountId}/buckets/{projectId}/todolists/{todolistId}/groups.json": "CreateTodolistGroup",

  // Todolists
  "GET:/{accountId}/buckets/{projectId}/todolists/{todolistId}/todos.json": "ListTodos",
  "POST:/{accountId}/buckets/{projectId}/todolists/{todolistId}/todos.json": "CreateTodo",

  // Todos
  "GET:/{accountId}/buckets/{projectId}/todos/{todoId}": "GetTodo",
  "PUT:/{accountId}/buckets/{projectId}/todos/{todoId}": "UpdateTodo",
  "DELETE:/{accountId}/buckets/{projectId}/todos/{todoId}": "TrashTodo",
  "POST:/{accountId}/buckets/{projectId}/todos/{todoId}/completion.json": "CompleteTodo",
  "DELETE:/{accountId}/buckets/{projectId}/todos/{todoId}/completion.json": "UncompleteTodo",
  "PUT:/{accountId}/buckets/{projectId}/todos/{todoId}/position.json": "RepositionTodo",

  // Todosets
  "GET:/{accountId}/buckets/{projectId}/todosets/{todosetId}": "GetTodoset",
  "GET:/{accountId}/buckets/{projectId}/todosets/{todosetId}/todolists.json": "ListTodolists",
  "POST:/{accountId}/buckets/{projectId}/todosets/{todosetId}/todolists.json": "CreateTodolist",

  // Uploads
  "GET:/{accountId}/buckets/{projectId}/uploads/{uploadId}": "GetUpload",
  "PUT:/{accountId}/buckets/{projectId}/uploads/{uploadId}": "UpdateUpload",
  "GET:/{accountId}/buckets/{projectId}/uploads/{uploadId}/versions.json": "ListUploadVersions",

  // Vaults
  "GET:/{accountId}/buckets/{projectId}/vaults/{vaultId}": "GetVault",
  "PUT:/{accountId}/buckets/{projectId}/vaults/{vaultId}": "UpdateVault",
  "GET:/{accountId}/buckets/{projectId}/vaults/{vaultId}/documents.json": "ListDocuments",
  "POST:/{accountId}/buckets/{projectId}/vaults/{vaultId}/documents.json": "CreateDocument",
  "GET:/{accountId}/buckets/{projectId}/vaults/{vaultId}/uploads.json": "ListUploads",
  "POST:/{accountId}/buckets/{projectId}/vaults/{vaultId}/uploads.json": "CreateUpload",
  "GET:/{accountId}/buckets/{projectId}/vaults/{vaultId}/vaults.json": "ListVaults",
  "POST:/{accountId}/buckets/{projectId}/vaults/{vaultId}/vaults.json": "CreateVault",

  // Webhooks
  "GET:/{accountId}/buckets/{projectId}/webhooks.json": "ListWebhooks",
  "POST:/{accountId}/buckets/{projectId}/webhooks.json": "CreateWebhook",
  "GET:/{accountId}/buckets/{projectId}/webhooks/{webhookId}": "GetWebhook",
  "PUT:/{accountId}/buckets/{projectId}/webhooks/{webhookId}": "UpdateWebhook",
  "DELETE:/{accountId}/buckets/{projectId}/webhooks/{webhookId}": "DeleteWebhook",

  // Campfires (global)
  "GET:/{accountId}/chats.json": "ListCampfires",

  // People
  "GET:/{accountId}/circles/people.json": "ListPingablePeople",
  "GET:/{accountId}/my/profile.json": "GetMyProfile",
  "GET:/{accountId}/people.json": "ListPeople",
  "GET:/{accountId}/people/{personId}": "GetPerson",

  // Lineup Markers
  "POST:/{accountId}/lineup/markers.json": "CreateLineupMarker",
  "PUT:/{accountId}/lineup/markers/{markerId}": "UpdateLineupMarker",
  "DELETE:/{accountId}/lineup/markers/{markerId}": "DeleteLineupMarker",

  // Projects
  "GET:/{accountId}/projects.json": "ListProjects",
  "POST:/{accountId}/projects.json": "CreateProject",
  "GET:/{accountId}/projects/recordings.json": "ListRecordings",
  "GET:/{accountId}/projects/{projectId}": "GetProject",
  "PUT:/{accountId}/projects/{projectId}": "UpdateProject",
  "DELETE:/{accountId}/projects/{projectId}": "TrashProject",
  "GET:/{accountId}/projects/{projectId}/people.json": "ListProjectPeople",
  "PUT:/{accountId}/projects/{projectId}/people/users.json": "UpdateProjectAccess",

  // Reports
  "GET:/{accountId}/reports/progress.json": "GetProgressReport",
  "GET:/{accountId}/reports/schedules/upcoming.json": "GetUpcomingSchedule",
  "GET:/{accountId}/reports/timesheet.json": "GetTimesheetReport",
  "GET:/{accountId}/reports/todos/assigned.json": "ListAssignablePeople",
  "GET:/{accountId}/reports/todos/assigned/{personId}": "GetAssignedTodos",
  "GET:/{accountId}/reports/todos/overdue.json": "GetOverdueTodos",
  "GET:/{accountId}/reports/users/progress/{personId}": "GetPersonProgress",

  // Search
  "GET:/{accountId}/search.json": "Search",
  "GET:/{accountId}/searches/metadata.json": "GetSearchMetadata",

  // Templates
  "GET:/{accountId}/templates.json": "ListTemplates",
  "POST:/{accountId}/templates.json": "CreateTemplate",
  "GET:/{accountId}/templates/{templateId}": "GetTemplate",
  "PUT:/{accountId}/templates/{templateId}": "UpdateTemplate",
  "DELETE:/{accountId}/templates/{templateId}": "DeleteTemplate",
  "POST:/{accountId}/templates/{templateId}/project_constructions.json": "CreateProjectFromTemplate",
  "GET:/{accountId}/templates/{templateId}/project_constructions/{constructionId}": "GetProjectConstruction",
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
