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

      const result: RequestResult = {
        statusCode: response.status,
        durationMs,
        fromCache: response.status === 304,
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
 * Mapping from "METHOD:/path/pattern" to operation name.
 * Built from the OpenAPI paths definition.
 * IMPORTANT: These paths MUST exactly match the OpenAPI spec paths for retry config to work.
 */
const PATH_TO_OPERATION: Record<string, string> = {
  // Attachments
  "POST:/attachments.json": "CreateAttachment",

  // Card Tables
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

  // Message Categories/Types
  "GET:/buckets/{projectId}/categories.json": "ListMessageTypes",
  "POST:/buckets/{projectId}/categories.json": "CreateMessageType",
  "GET:/buckets/{projectId}/categories/{typeId}": "GetMessageType",
  "PUT:/buckets/{projectId}/categories/{typeId}": "UpdateMessageType",
  "DELETE:/buckets/{projectId}/categories/{typeId}": "DeleteMessageType",

  // Campfires/Chats
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

  // Client Portal
  "GET:/buckets/{projectId}/client/approvals.json": "ListClientApprovals",
  "GET:/buckets/{projectId}/client/approvals/{approvalId}": "GetClientApproval",
  "GET:/buckets/{projectId}/client/correspondences.json": "ListClientCorrespondences",
  "GET:/buckets/{projectId}/client/correspondences/{correspondenceId}": "GetClientCorrespondence",
  "GET:/buckets/{projectId}/client/recordings/{recordingId}/replies.json": "ListClientReplies",
  "GET:/buckets/{projectId}/client/recordings/{recordingId}/replies/{replyId}": "GetClientReply",

  // Comments
  "GET:/buckets/{projectId}/comments/{commentId}": "GetComment",
  "PUT:/buckets/{projectId}/comments/{commentId}": "UpdateComment",

  // Dock/Tools (paths include /dock/tools/)
  "POST:/buckets/{projectId}/dock/tools/{toolId}/clone.json": "CloneTool",
  "GET:/buckets/{projectId}/dock/tools/{toolId}": "GetTool",
  "PUT:/buckets/{projectId}/dock/tools/{toolId}": "UpdateTool",
  "DELETE:/buckets/{projectId}/dock/tools/{toolId}": "DeleteTool",
  "PUT:/buckets/{projectId}/dock/tools/{toolId}/position.json": "RepositionTool",
  "POST:/buckets/{projectId}/dock/tools/{toolId}/position.json": "EnableTool",
  "DELETE:/buckets/{projectId}/dock/tools/{toolId}/position.json": "DisableTool",

  // Documents
  "GET:/buckets/{projectId}/documents/{documentId}": "GetDocument",
  "PUT:/buckets/{projectId}/documents/{documentId}": "UpdateDocument",

  // Forwards (Inbox)
  "GET:/buckets/{projectId}/inbox_forwards/{forwardId}": "GetForward",
  "GET:/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json": "ListForwardReplies",
  "POST:/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json": "CreateForwardReply",
  "GET:/buckets/{projectId}/inbox_forwards/{forwardId}/replies/{replyId}": "GetForwardReply",
  "GET:/buckets/{projectId}/inboxes/{inboxId}": "GetInbox",
  "GET:/buckets/{projectId}/inboxes/{inboxId}/forwards.json": "ListForwards",

  // Message Boards
  "GET:/buckets/{projectId}/message_boards/{boardId}": "GetMessageBoard",
  "GET:/buckets/{projectId}/message_boards/{boardId}/messages.json": "ListMessages",
  "POST:/buckets/{projectId}/message_boards/{boardId}/messages.json": "CreateMessage",

  // Messages
  "GET:/buckets/{projectId}/messages/{messageId}": "GetMessage",
  "PUT:/buckets/{projectId}/messages/{messageId}": "UpdateMessage",

  // Question & Answers (Checkins)
  "GET:/buckets/{projectId}/question_answers/{answerId}": "GetAnswer",
  "PUT:/buckets/{projectId}/question_answers/{answerId}": "UpdateAnswer",
  "GET:/buckets/{projectId}/questionnaires/{questionnaireId}": "GetQuestionnaire",
  "GET:/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json": "ListQuestions",
  "POST:/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json": "CreateQuestion",
  "GET:/buckets/{projectId}/questions/{questionId}": "GetQuestion",
  "PUT:/buckets/{projectId}/questions/{questionId}": "UpdateQuestion",
  "GET:/buckets/{projectId}/questions/{questionId}/answers.json": "ListAnswers",
  "POST:/buckets/{projectId}/questions/{questionId}/answers.json": "CreateAnswer",

  // Recordings
  "POST:/buckets/{projectId}/recordings/{recordingId}/pin.json": "PinMessage",
  "DELETE:/buckets/{projectId}/recordings/{recordingId}/pin.json": "UnpinMessage",
  "GET:/buckets/{projectId}/recordings/{recordingId}": "GetRecording",
  "PUT:/buckets/{projectId}/recordings/{recordingId}/client_visibility.json": "SetClientVisibility",
  "GET:/buckets/{projectId}/recordings/{recordingId}/comments.json": "ListComments",
  "POST:/buckets/{projectId}/recordings/{recordingId}/comments.json": "CreateComment",
  "GET:/buckets/{projectId}/recordings/{recordingId}/events.json": "ListEvents",
  "PUT:/buckets/{projectId}/recordings/{recordingId}/status/active.json": "UnarchiveRecording",
  "PUT:/buckets/{projectId}/recordings/{recordingId}/status/archived.json": "ArchiveRecording",
  "PUT:/buckets/{projectId}/recordings/{recordingId}/status/trashed.json": "TrashRecording",
  "GET:/buckets/{projectId}/recordings/{recordingId}/subscription.json": "GetSubscription",
  "POST:/buckets/{projectId}/recordings/{recordingId}/subscription.json": "Subscribe",
  "PUT:/buckets/{projectId}/recordings/{recordingId}/subscription.json": "UpdateSubscription",
  "DELETE:/buckets/{projectId}/recordings/{recordingId}/subscription.json": "Unsubscribe",
  "GET:/buckets/{projectId}/recordings/{recordingId}/timesheet.json": "GetRecordingTimesheet",

  // Schedules
  "GET:/buckets/{projectId}/schedule_entries/{entryId}": "GetScheduleEntry",
  "PUT:/buckets/{projectId}/schedule_entries/{entryId}": "UpdateScheduleEntry",
  "GET:/buckets/{projectId}/schedule_entries/{entryId}/occurrences/{date}": "GetScheduleEntryOccurrence",
  "GET:/buckets/{projectId}/schedules/{scheduleId}": "GetSchedule",
  "GET:/buckets/{projectId}/schedules/{scheduleId}/entries.json": "ListScheduleEntries",
  "POST:/buckets/{projectId}/schedules/{scheduleId}/entries.json": "CreateScheduleEntry",

  // Timeline & Timesheet
  "GET:/buckets/{projectId}/timeline.json": "GetProjectTimeline",
  "GET:/buckets/{projectId}/timesheet.json": "GetProjectTimesheet",

  // Todolist Groups (all use {todolistId} for consistent normalization)
  "PUT:/buckets/{projectId}/todolists/{todolistId}/position.json": "RepositionTodolistGroup",
  "GET:/buckets/{projectId}/todolists/{todolistId}": "GetTodolistOrGroup",
  "PUT:/buckets/{projectId}/todolists/{todolistId}": "UpdateTodolistOrGroup",
  "GET:/buckets/{projectId}/todolists/{todolistId}/groups.json": "ListTodolistGroups",
  "POST:/buckets/{projectId}/todolists/{todolistId}/groups.json": "CreateTodolistGroup",

  // Todolists
  "GET:/buckets/{projectId}/todolists/{todolistId}/todos.json": "ListTodos",
  "POST:/buckets/{projectId}/todolists/{todolistId}/todos.json": "CreateTodo",

  // Todos
  "GET:/buckets/{projectId}/todos/{todoId}": "GetTodo",
  "PUT:/buckets/{projectId}/todos/{todoId}": "UpdateTodo",
  "POST:/buckets/{projectId}/todos/{todoId}/completion.json": "CompleteTodo",
  "DELETE:/buckets/{projectId}/todos/{todoId}/completion.json": "UncompleteTodo",
  "PUT:/buckets/{projectId}/todos/{todoId}/position.json": "RepositionTodo",

  // Todosets
  "GET:/buckets/{projectId}/todosets/{todosetId}": "GetTodoset",
  "GET:/buckets/{projectId}/todosets/{todosetId}/todolists.json": "ListTodolists",
  "POST:/buckets/{projectId}/todosets/{todosetId}/todolists.json": "CreateTodolist",

  // Uploads
  "GET:/buckets/{projectId}/uploads/{uploadId}": "GetUpload",
  "PUT:/buckets/{projectId}/uploads/{uploadId}": "UpdateUpload",
  "GET:/buckets/{projectId}/uploads/{uploadId}/versions.json": "ListUploadVersions",

  // Vaults
  "GET:/buckets/{projectId}/vaults/{vaultId}": "GetVault",
  "PUT:/buckets/{projectId}/vaults/{vaultId}": "UpdateVault",
  "GET:/buckets/{projectId}/vaults/{vaultId}/documents.json": "ListDocuments",
  "POST:/buckets/{projectId}/vaults/{vaultId}/documents.json": "CreateDocument",
  "GET:/buckets/{projectId}/vaults/{vaultId}/uploads.json": "ListUploads",
  "POST:/buckets/{projectId}/vaults/{vaultId}/uploads.json": "CreateUpload",
  "GET:/buckets/{projectId}/vaults/{vaultId}/vaults.json": "ListVaults",
  "POST:/buckets/{projectId}/vaults/{vaultId}/vaults.json": "CreateVault",

  // Webhooks
  "GET:/buckets/{projectId}/webhooks.json": "ListWebhooks",
  "POST:/buckets/{projectId}/webhooks.json": "CreateWebhook",
  "GET:/buckets/{projectId}/webhooks/{webhookId}": "GetWebhook",
  "PUT:/buckets/{projectId}/webhooks/{webhookId}": "UpdateWebhook",
  "DELETE:/buckets/{projectId}/webhooks/{webhookId}": "DeleteWebhook",

  // Campfires (global)
  "GET:/chats.json": "ListCampfires",

  // People
  "GET:/circles/people.json": "ListPingablePeople",
  "GET:/my/profile.json": "GetMyProfile",
  "GET:/people.json": "ListPeople",
  "GET:/people/{personId}": "GetPerson",

  // Lineup Markers
  "POST:/lineup/markers.json": "CreateLineupMarker",
  "PUT:/lineup/markers/{markerId}": "UpdateLineupMarker",
  "DELETE:/lineup/markers/{markerId}": "DeleteLineupMarker",

  // Projects
  "GET:/projects.json": "ListProjects",
  "POST:/projects.json": "CreateProject",
  "GET:/projects/recordings.json": "ListRecordings",
  "GET:/projects/{projectId}": "GetProject",
  "PUT:/projects/{projectId}": "UpdateProject",
  "DELETE:/projects/{projectId}": "TrashProject",
  "GET:/projects/{projectId}/people.json": "ListProjectPeople",
  "PUT:/projects/{projectId}/people/users.json": "UpdateProjectAccess",

  // Reports
  "GET:/reports/progress.json": "GetProgress",
  "GET:/reports/schedules/upcoming.json": "GetUpcomingSchedule",
  "GET:/reports/timesheet.json": "GetTimesheetReport",
  "GET:/reports/todos/assigned.json": "GetAssignedTodos",
  "GET:/reports/todos/assigned/{personId}": "GetAssignedTodosForPerson",
  "GET:/reports/todos/overdue.json": "GetOverdueTodos",
  "GET:/reports/users/progress/{personId}": "GetPersonProgress",

  // Search
  "GET:/search.json": "Search",
  "GET:/searches/metadata.json": "GetSearchMetadata",

  // Templates
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
