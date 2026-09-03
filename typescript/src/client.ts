/**
 * Basecamp TypeScript SDK Client
 *
 * Creates a type-safe client for the Basecamp API using openapi-fetch.
 * Includes middleware for authentication, retry with exponential backoff,
 * and ETag-based caching.
 */

import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./generated/schema.js";
import metadata from "./generated/metadata.js";
import { PATH_TO_OPERATION } from "./generated/path-mapping.js";
import type { BasecampHooks, RequestResult } from "./hooks.js";
import { BasecampError } from "./errors.js";
import { isLocalhost, requireSameOrigin } from "./security.js";
import { parseNextLink, resolveURL, isSameOrigin, DEFAULT_MAX_PAGES, assertValidMaxPages } from "./pagination-utils.js";
import { type AuthStrategy, bearerAuth } from "./auth-strategy.js";
import { createDownloadURL, type DownloadResult } from "./download.js";
import {
  DEFAULT_RETRY_CONFIG,
  NO_RETRY_CONFIG,
  TerminalRetryError,
  executeWithRetry,
  type RetryConfig,
  type RetryEmit,
} from "./retry.js";

// ============================================================================
// Services - Generated from OpenAPI spec (spec-driven, not hand-written)
// ============================================================================
import { ProjectsService } from "./generated/services/projects.js";
import { TodosService } from "./services/todos-extensions.js";
import { TodolistsService } from "./services/todolists-extensions.js";
import { TodosetsService } from "./generated/services/todosets.js";
import { HillChartsService } from "./generated/services/hill-charts.js";
import { PeopleService } from "./generated/services/people.js";
import { MessagesService } from "./generated/services/messages.js";
import { CommentsService } from "./generated/services/comments.js";
import { CampfiresService } from "./generated/services/campfires.js";
import { CardTablesService } from "./generated/services/card-tables.js";
import { CardsService } from "./services/cards-extensions.js";
import { CardColumnsService } from "./generated/services/card-columns.js";
import { CardStepsService } from "./generated/services/card-steps.js";
import { WormholesService } from "./generated/services/wormholes.js";
import { MessageBoardsService } from "./generated/services/message-boards.js";
import { MessageTypesService } from "./generated/services/message-types.js";
import { ForwardsService } from "./generated/services/forwards.js";
import { CheckinsService } from "./generated/services/checkins.js";
import { ClientApprovalsService } from "./generated/services/client-approvals.js";
import { ClientCorrespondencesService } from "./generated/services/client-correspondences.js";
import { ClientRepliesService } from "./generated/services/client-replies.js";
import { WebhooksService } from "./generated/services/webhooks.js";
import { BookmarksService } from "./generated/services/bookmarks.js";
import { BubbleUpsService } from "./generated/services/bubble-ups.js";
import { FoldersService } from "./generated/services/folders.js";
import { DraftsService } from "./generated/services/drafts.js";
import { CalendarsService } from "./generated/services/calendars.js";
import { MyNotesService } from "./generated/services/my-notes.js";
import { SubscriptionsService } from "./generated/services/subscriptions.js";
import { AttachmentsService } from "./generated/services/attachments.js";
import { VaultsService } from "./generated/services/vaults.js";
import { DocumentsService } from "./services/documents-extensions.js";
import { UploadsService } from "./services/uploads-extensions.js";
import { CloudFilesService } from "./generated/services/cloud-files.js";
import { GoogleDocumentsService } from "./generated/services/google-documents.js";
import { SchedulesService } from "./services/schedules-extensions.js";
import { EventsService } from "./generated/services/events.js";
import { RecordingsService } from "./generated/services/recordings.js";
import { SearchService } from "./generated/services/search.js";
import { ReportsService } from "./generated/services/reports.js";
import { TemplatesService } from "./generated/services/templates.js";
import { LineupService } from "./generated/services/lineup.js";
import { AutomationService } from "./generated/services/automation.js";
import { TodolistGroupsService } from "./generated/services/todolist-groups.js";
import { ToolsService } from "./generated/services/tools.js";
import { TimesheetsService } from "./generated/services/timesheets.js";
import { TimelineService } from "./generated/services/timeline.js";
import { EverythingService } from "./generated/services/everything.js";
import { ClientVisibilityService } from "./generated/services/client-visibility.js";
import { BoostsService } from "./generated/services/boosts.js";
import { AccountService } from "./generated/services/account.js";
import { GaugesService } from "./generated/services/gauges.js";
import { MyAssignmentsService } from "./generated/services/my-assignments.js";
import { MyNotificationsService } from "./generated/services/my-notifications.js";

// ============================================================================
// Services - Hand-written (not spec-driven, e.g., OAuth flows)
// ============================================================================
import { AuthorizationService } from "./services/authorization.js";

// Re-export types for consumer convenience
export type { paths };

/**
 * Largest delay `AbortSignal.timeout` schedules faithfully: Node's timers are
 * backed by a signed 32-bit int, and anything above this is clamped to 1ms with
 * a TimeoutOverflowWarning rather than honored.
 */
const MAX_TIMEOUT_MS = 2_147_483_647;

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

  /** Projects service - list, get, create, update, trash, archive, and unarchive projects */
  readonly projects: ProjectsService;
  /** Todos service - list, get, create, update, complete, and manage todos */
  readonly todos: TodosService;
  /** Todolists service - list, get, create, update, edit, and replace todo lists */
  readonly todolists: TodolistsService;
  /** Todosets service - get todo sets (container for todo lists) */
  readonly todosets: TodosetsService;
  /** Hill charts service - get and update hill chart settings */
  readonly hillCharts: HillChartsService;
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
  /** Wormholes service - create, update, and delete card-table wormholes */
  readonly wormholes: WormholesService;
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
  /** Bookmarks service - the current user's personal bookmarks */
  readonly bookmarks: BookmarksService;
  /** Bubble Ups service - bubble a recording up and back down in the current user's readings */
  readonly bubbleUps: BubbleUpsService;

  /** Folders service - the current user's home-screen folders (wire type "Stack") */
  readonly folders: FoldersService;

  /** Drafts service - the current user's unpublished drafts */
  readonly drafts: DraftsService;

  /** Calendars service - per-account calendars (show + update) */
  readonly calendars: CalendarsService;

  /** MyNotes service - the current user's scratchpad note */
  readonly myNotes: MyNotesService;

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
  /** Cloud files service - manage links to files on external services in vaults */
  readonly cloudFiles: CloudFilesService;
  /** Google documents service - manage links to Google Workspace documents in vaults */
  readonly googleDocuments: GoogleDocumentsService;
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
  /** Automation service - lineup marker listings and other automation */
  readonly automation: AutomationService;
  /** Todolist groups service - manage groups within todolists */
  readonly todolistGroups: TodolistGroupsService;
  /** Tools service - manage project dock tools */
  readonly tools: ToolsService;
  /** Timesheets service - get timesheet data */
  readonly timesheets: TimesheetsService;
  /** Timeline service - get project timeline */
  readonly timeline: TimelineService;
  /** Everything service - account-wide aggregate listings */
  readonly everything: EverythingService;
  /** Client visibility service - manage client visibility */
  readonly clientVisibility: ClientVisibilityService;
  /** Boosts service - manage recording boosts */
  readonly boosts: BoostsService;
  /** Account service - get and update account settings */
  readonly account: AccountService;
  /** Gauges service - manage project progress gauges */
  readonly gauges: GaugesService;
  /** My assignments service - get current user's assignments */
  readonly myAssignments: MyAssignmentsService;
  /** My notifications service - get and manage notifications */
  readonly myNotifications: MyNotificationsService;
  /** Download file content from any API-routable download URL */
  downloadURL(rawURL: string): Promise<DownloadResult>;
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
  /** User-Agent header (defaults to basecamp-sdk-ts/VERSION (api:API_VERSION)) */
  userAgent?: string;
  /** Enable ETag-based caching (defaults to false) */
  enableCache?: boolean;
  /** Enable automatic retry on 429/503 (defaults to true) */
  enableRetry?: boolean;
  /** Request timeout in milliseconds (defaults to 30000) */
  requestTimeoutMs?: number;
  /** Hooks for observability (logging, metrics, tracing) */
  hooks?: BasecampHooks;
  /** Maximum pages to follow during auto-pagination (defaults to 10,000) */
  maxPages?: number;
}

export const VERSION = "0.15.0";
export const API_VERSION = "2026-09-02";
const DEFAULT_USER_AGENT = `basecamp-sdk-ts/${VERSION} (api:${API_VERSION})`;

/**
 * Creates a type-safe Basecamp API client with built-in middleware for:
 * - Authentication (Bearer token)
 * - Retry with exponential backoff (respects Retry-After header)
 * - ETag-based HTTP caching, opt-in via `enableCache` (defaults to false)
 *
 * @example
 * ```ts
 * import { createBasecampClient } from "@37signals/basecamp";
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
    requestTimeoutMs = 30000,
    hooks,
    maxPages,
  } = options;

  // Validate auth options: exactly one of auth or accessToken must be provided
  if (auth && accessToken) {
    throw new BasecampError("usage", "Provide either 'auth' or 'accessToken', not both");
  }
  if (!auth && !accessToken) {
    throw new BasecampError("usage", "Either 'auth' or 'accessToken' is required");
  }

  // AbortSignal.timeout accepts only a non-negative integer that fits a signed
  // 32-bit timer. Outside that range Node either throws a bare RangeError
  // (fractional, negative, or > 2^32-1) or — worse — silently clamps to 1ms with
  // a TimeoutOverflowWarning (2^31 .. 2^32-1), which would abort every request
  // almost immediately. Both would surface per request, far from the call that
  // misconfigured it, and both are behavior changes from the old setTimeout,
  // which coerced such values to 0. Fail fast at construction instead, like the
  // other config checks.
  if (
    !Number.isInteger(requestTimeoutMs) ||
    requestTimeoutMs < 0 ||
    requestTimeoutMs > MAX_TIMEOUT_MS
  ) {
    throw new BasecampError(
      "usage",
      `'requestTimeoutMs' must be an integer between 0 and ${MAX_TIMEOUT_MS}, got ${requestTimeoutMs}`
    );
  }

  // The cap the standalone helpers validate is the same cap the config carries,
  // and this is the wider door: `maxPages` here is handed to every service and
  // reached only later, inside `BaseService`'s `page < this.maxPages` loops, far
  // from the call that misconfigured it. Same predicate, same error, checked
  // once at construction — see assertValidMaxPages for what each rejected value
  // does to the bound. Only when the caller supplied one: an absent cap must
  // keep falling through to DEFAULT_MAX_PAGES.
  //
  // `!= null`, matching BaseService's guard exactly. This value is not consumed
  // here — it is handed straight to every service below, where `maxPages ??
  // DEFAULT_MAX_PAGES` treats an explicit `null` as absent. A `!== undefined`
  // test here made the two doors disagree on that single value: the factory
  // threw where the equivalent direct service construction defaulted. Whichever
  // way "not supplied" is defined, both guards have to define it the same way,
  // because the same `??` is downstream of both.
  if (maxPages != null) {
    assertValidMaxPages(maxPages);
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

  // One lifecycle, shared between the retrying fetch and the lifecycle
  // middleware: the fetch owns every attempt's start and each abandoned
  // attempt's end; the middleware finalizes the terminal outcome.
  const lifecycle = new RequestLifecycle(hooks);

  // The retry loop lives BENEATH the middleware chain, as the client's custom
  // fetch: openapi-fetch calls it exactly once per logical request, after every
  // onRequest middleware and before every onError/onResponse. Installed
  // unconditionally — with retry disabled it degenerates to a single attempt —
  // so hook emission is single-sourced regardless of config.
  const client = createClient<paths>({
    baseUrl,
    fetch: createRetryingFetch(lifecycle, authStrategy, enableRetry),
  });

  // Apply middleware in order: auth, lifecycle, cache.
  // onRequest runs in this order; onResponse and onError run in reverse, so the
  // lifecycle middleware finalizes the terminal attempt only after the cache
  // middleware has had its chance to transform the response.
  client.use(createAuthMiddleware(authStrategy, userAgent, requestTimeoutMs, baseUrl));

  // Registered even when no hooks are configured. It emits nothing in that case
  // (every call is optional-chained), but registering it unconditionally keeps
  // the finalize path identical whether or not anyone is listening.
  client.use(createLifecycleMiddleware(lifecycle));

  if (enableCache) {
    client.use(createCacheMiddleware());
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
    requireSameOrigin(url, baseUrl);
    const headers = new Headers({
      "User-Agent": userAgent,
      Accept: "application/json",
    });
    await authStrategy.authenticate(headers);
    return fetch(url, { headers });
  };

  // Authenticated fetch for multipart uploads — adds auth + User-Agent, caller controls body/method
  const authenticatedFetch = async (url: string, init: RequestInit): Promise<Response> => {
    requireSameOrigin(url, baseUrl);
    const headers = new Headers(init.headers);
    headers.set("User-Agent", userAgent);
    await authStrategy.authenticate(headers);
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), requestTimeoutMs);
    try {
      return await fetch(url, { ...init, headers, signal: controller.signal });
    } finally {
      clearTimeout(timeoutId);
    }
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

  // Wire downloadURL — raw fetch, not openapi-fetch (like fetchPage).
  // Defined before service factories so UploadsService can inject it.
  const downloadURLFn = createDownloadURL({
    authStrategy, userAgent, baseUrl, hooks, requestTimeoutMs, enableRetry,
  });
  Object.defineProperty(enhancedClient, "downloadURL", {
    value: downloadURLFn,
    writable: false,
    enumerable: false,
  });

  defineService("projects", () => new ProjectsService(client, hooks, fetchPage, maxPages));
  defineService("todos", () => new TodosService(client, hooks, fetchPage, maxPages));
  defineService("todolists", () => new TodolistsService(client, hooks, fetchPage, maxPages));
  defineService("todosets", () => new TodosetsService(client, hooks, fetchPage, maxPages));
  defineService("hillCharts", () => new HillChartsService(client, hooks, fetchPage, maxPages));
  defineService("people", () => new PeopleService(client, hooks, fetchPage, maxPages));
  defineService("authorization", () => new AuthorizationService(client, hooks, authStrategy, userAgent));
  defineService("messages", () => new MessagesService(client, hooks, fetchPage, maxPages));
  defineService("comments", () => new CommentsService(client, hooks, fetchPage, maxPages));
  defineService("campfires", () => new CampfiresService(client, hooks, fetchPage, maxPages));
  defineService("cardTables", () => new CardTablesService(client, hooks, fetchPage, maxPages));
  defineService("cards", () => new CardsService(client, hooks, fetchPage, maxPages));
  defineService("cardColumns", () => new CardColumnsService(client, hooks, fetchPage, maxPages));
  defineService("cardSteps", () => new CardStepsService(client, hooks, fetchPage, maxPages));
  defineService("wormholes", () => new WormholesService(client, hooks, fetchPage, maxPages));
  defineService("messageBoards", () => new MessageBoardsService(client, hooks, fetchPage, maxPages));
  defineService("messageTypes", () => new MessageTypesService(client, hooks, fetchPage, maxPages));
  defineService("forwards", () => new ForwardsService(client, hooks, fetchPage, maxPages));
  defineService("checkins", () => new CheckinsService(client, hooks, fetchPage, maxPages));
  defineService("clientApprovals", () => new ClientApprovalsService(client, hooks, fetchPage, maxPages));
  defineService("clientCorrespondences", () => new ClientCorrespondencesService(client, hooks, fetchPage, maxPages));
  defineService("clientReplies", () => new ClientRepliesService(client, hooks, fetchPage, maxPages));
  defineService("webhooks", () => new WebhooksService(client, hooks, fetchPage, maxPages));
  defineService("bookmarks", () => new BookmarksService(client, hooks, fetchPage, maxPages));
  defineService("bubbleUps", () => new BubbleUpsService(client, hooks, fetchPage, maxPages));
  defineService("folders", () => new FoldersService(client, hooks, fetchPage, maxPages));
  defineService("drafts", () => new DraftsService(client, hooks, fetchPage, maxPages));
  defineService("calendars", () => new CalendarsService(client, hooks, fetchPage, maxPages));
  defineService("myNotes", () => new MyNotesService(client, hooks, fetchPage, maxPages));
  defineService("subscriptions", () => new SubscriptionsService(client, hooks, fetchPage, maxPages));
  defineService("attachments", () => new AttachmentsService(client, hooks, fetchPage, maxPages));
  defineService("vaults", () => new VaultsService(client, hooks, fetchPage, maxPages));
  defineService("documents", () => new DocumentsService(client, hooks, fetchPage, maxPages));
  defineService("cloudFiles", () => new CloudFilesService(client, hooks, fetchPage, maxPages));
  defineService(
    "googleDocuments",
    () => new GoogleDocumentsService(client, hooks, fetchPage, maxPages),
  );
  defineService("uploads", () =>
    // Positional args mirror BaseService (incl. authenticatedFetch/baseUrl slots the
    // factory leaves unset), with downloadURLFn appended at the end.
    new UploadsService(client, hooks, fetchPage, maxPages, undefined, undefined, downloadURLFn),
  );
  defineService("schedules", () => new SchedulesService(client, hooks, fetchPage, maxPages));
  defineService("events", () => new EventsService(client, hooks, fetchPage, maxPages));
  defineService("recordings", () => new RecordingsService(client, hooks, fetchPage, maxPages));
  defineService("search", () => new SearchService(client, hooks, fetchPage, maxPages));
  defineService("reports", () => new ReportsService(client, hooks, fetchPage, maxPages));
  defineService("templates", () => new TemplatesService(client, hooks, fetchPage, maxPages));
  defineService("lineup", () => new LineupService(client, hooks, fetchPage, maxPages));
  defineService("automation", () => new AutomationService(client, hooks, fetchPage, maxPages));
  defineService("todolistGroups", () => new TodolistGroupsService(client, hooks, fetchPage, maxPages));
  defineService("tools", () => new ToolsService(client, hooks, fetchPage, maxPages));
  defineService("timesheets", () => new TimesheetsService(client, hooks, fetchPage, maxPages));
  defineService("timeline", () => new TimelineService(client, hooks, fetchPage, maxPages));
  defineService("everything", () => new EverythingService(client, hooks, fetchPage, maxPages));
  defineService("clientVisibility", () => new ClientVisibilityService(client, hooks, fetchPage, maxPages));
  defineService("boosts", () => new BoostsService(client, hooks, fetchPage, maxPages));
  defineService("account", () => new AccountService(client, hooks, fetchPage, maxPages, authenticatedFetch, baseUrl));
  defineService("gauges", () => new GaugesService(client, hooks, fetchPage, maxPages));
  defineService("myAssignments", () => new MyAssignmentsService(client, hooks, fetchPage, maxPages));
  defineService("myNotifications", () => new MyNotificationsService(client, hooks, fetchPage, maxPages));

  return enhancedClient;
}

// =============================================================================
// Auth Middleware
// =============================================================================

function createAuthMiddleware(authStrategy: AuthStrategy, userAgent: string, requestTimeoutMs: number, baseUrl: string): Middleware {
  return {
    async onRequest({ request }) {
      // Backstop: never attach credentials to a foreign origin.
      requireSameOrigin(request.url, baseUrl);
      await authStrategy.authenticate(request.headers);
      request.headers.set("User-Agent", userAgent);
      // Content-Type describes a request body, so set the JSON default only when a
      // body is present and not already typed (preserves binary uploads, etc.).
      // bc3 silently discards query params on GET requests that carry a Content-Type.
      if (request.body !== null && !request.headers.has("Content-Type")) {
        request.headers.set("Content-Type", "application/json");
      }
      request.headers.set("Accept", "application/json");

      // Apply request timeout, preserving any caller-supplied signal.
      // AbortSignal.timeout's timer is unref'd, so unlike a bare setTimeout it
      // never holds the event loop open after the request settles.
      const timeoutSignal = AbortSignal.timeout(requestTimeoutMs);
      const signal = request.signal
        ? AbortSignal.any([request.signal, timeoutSignal])
        : timeoutSignal;

      return new Request(request.url, {
        method: request.method,
        headers: request.headers,
        body: request.body,
        signal,
        duplex: request.body ? "half" : undefined,
      } as RequestInit);
    },
  };
}

// =============================================================================
// Request Lifecycle
// =============================================================================

/**
 * Per-attempt observability state for one logical request.
 *
 * `attempt` is the 1-based attempt currently in flight. `finalized` makes
 * onRequestEnd idempotent per attempt: two terminal paths can reach the same
 * attempt, and only the first may emit an end.
 *
 * Each attempt gets fresh state — begin() replaces this record — so the flag
 * guards one attempt against a double end, not the request. The retrying fetch
 * ends every attempt it abandons before it backs off; the lifecycle middleware
 * ends the terminal attempt. The fetch deliberately does not finalize the
 * response it returns, so that onResponse is free to record the outcome the
 * cache middleware has since transformed.
 */
interface AttemptState {
  startTime: number;
  attempt: number;
  finalized: boolean;
}

/**
 * Owns the onRequestStart / onRequestEnd / onRetry lifecycle for every attempt
 * of every in-flight request, keyed by the final Request object openapi-fetch
 * hands to its custom fetch.
 *
 * openapi-fetch passes that same object to onResponse and onError, so identity
 * holds from the retrying fetch (which begins every attempt) through to the
 * lifecycle middleware (which finalizes the terminal one). Nothing ever goes on
 * the wire — this keying replaced the old `X-SDK-Request-Id` and `X-Request-Id`
 * correlation headers. (`X-Request-Id` in particular is a spec-reserved BC3
 * *response* header; sending an SDK-internal value under that name was a
 * collision.)
 *
 * A WeakMap makes the release bookkeeping structural: once a request settles
 * and its Request is unreachable, the state is collectable — there is no
 * release() to forget.
 */
class RequestLifecycle {
  private readonly states = new WeakMap<Request, AttemptState>();

  constructor(private readonly hooks: BasecampHooks | undefined) {}

  /** Begin an attempt and fire onRequestStart for it. */
  begin(request: Request, method: string, url: string, attempt: number): void {
    this.states.set(request, { startTime: performance.now(), attempt, finalized: false });
    this.emit(() => this.hooks?.onRequestStart?.({ method, url, attempt }));
  }

  /**
   * Fire onRequestEnd for the in-flight attempt, exactly once.
   *
   * A network failure or timeout carries statusCode 0 and the originating error,
   * matching the multipart transport in services/base.ts and SPEC section 7.
   */
  finalize(
    request: Request,
    method: string,
    url: string,
    outcome: { statusCode: number; fromCache?: boolean; error?: Error },
  ): void {
    const state = this.states.get(request);
    if (!state || state.finalized) return;
    state.finalized = true;

    const durationMs = Math.round(performance.now() - state.startTime);
    const result: RequestResult = {
      statusCode: outcome.statusCode,
      durationMs,
      fromCache: outcome.fromCache ?? false,
      ...(outcome.error ? { error: outcome.error } : {}),
    };
    this.emit(() =>
      this.hooks?.onRequestEnd?.({ method, url, attempt: state.attempt }, result),
    );
  }

  /**
   * Fire onRetry between two attempts.
   *
   * SPEC section 7 splits the two numbers: RequestInfo.attempt is the attempt that
   * just FAILED, while the standalone argument is the UPCOMING attempt. Go,
   * Python, Ruby and Kotlin all pass (failed, failed + 1).
   */
  retrying(
    method: string,
    url: string,
    failedAttempt: number,
    error: Error,
    delayMs: number,
  ): void {
    this.emit(() =>
      this.hooks?.onRetry?.(
        { method, url, attempt: failedAttempt },
        failedAttempt + 1,
        error,
        delayMs,
      ),
    );
  }

  private emit(fn: () => void): void {
    try {
      fn();
    } catch {
      // Hooks must never interrupt the request.
    }
  }
}

function createLifecycleMiddleware(lifecycle: RequestLifecycle): Middleware {
  return {
    async onResponse({ request, response }) {
      // The authoritative end for the terminal attempt: the retrying fetch
      // begins every attempt but deliberately does not finalize the response it
      // returns, so that this pass can record the outcome the cache middleware
      // has since transformed (a 304 rewritten into a cached 200). finalize
      // stays idempotent as a backstop.
      //
      // fromCache means "served out of the ETag cache", and only the header the
      // cache middleware sets proves that. A bare 304 does NOT: it reaches here
      // when the cache is disabled, or is enabled but holds no entry for this key,
      // and in both cases the caller's own conditional request went to the server.
      const fromCache = response.headers.get("X-From-Cache") === "1";

      lifecycle.finalize(request, request.method, request.url, {
        statusCode: response.status,
        fromCache,
      });

      return response;
    },

    async onError({ request, error }) {
      // A terminal throw from the retrying fetch — a network error, a timeout,
      // or a retry's auth refresh — skips onResponse entirely, so this is the
      // only place it can be observed.
      lifecycle.finalize(request, request.method, request.url, {
        statusCode: 0,
        error: error instanceof Error ? error : new Error(String(error)),
      });

      // Returning nothing preserves the original error's identity and rethrows it.
      return undefined;
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
// Retrying Fetch (the retry loop, beneath the middleware chain)
// =============================================================================

// The retry loop itself lives in retry.ts (executeWithRetry) so the raw-fetch
// download hop 1 shares it; this section owns what is client-specific — the
// operation-metadata config resolution and the openapi-fetch integration.

// PATH_TO_OPERATION is imported from generated/path-mapping.js

/**
 * Normalizes a URL path by replacing numeric IDs with placeholder tokens.
 * For example: /12345/todos/456 → /{accountId}/todos/{todoId}
 */
export function normalizeUrlPath(url: string): string {
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
    cloud_files: "{cloudFileId}",
    google_documents: "{googleDocumentId}",
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
    timesheet_entries: "{entryId}",
    people: "{personId}",
    markers: "{markerId}",  // lineup/markers/{markerId}
    project_constructions: "{constructionId}",
    assigned: "{personId}",  // reports/todos/assigned/{personId}
    progress: "{personId}",  // reports/users/progress/{personId}
    users: "{personId}",  // Alternative for users/progress
  };

  // Context-dependent overrides: when the segment following the ID matches a key,
  // override the placeholder from the default idMapping. This handles cases like
  // /buckets/{id}/webhooks → {bucketId} vs /buckets/{id}/timeline → {projectId}.
  const contextOverrides: Record<string, Record<string, string>> = {
    buckets: { webhooks: "{bucketId}", categories: "{bucketId}" },
  };

  // Build normalized path by replacing IDs and dates based on context
  const normalized: string[] = [];
  let prevSegment: string | null = null;
  let isFirstSegment = true;

  // Pattern for ISO-8601 date (YYYY-MM-DD)
  const datePattern = /^\d{4}-\d{2}-\d{2}$/;

  for (let i = 0; i < segments.length; i++) {
    const segment = segments[i]!;
    const nextSegment = i + 1 < segments.length ? segments[i + 1] : undefined;
    // Check if this segment is a numeric ID
    if (/^\d+$/.test(segment)) {
      // First numeric segment is always the accountId
      if (isFirstSegment) {
        normalized.push("{accountId}");
      } else {
        // Check context-dependent overrides first (look ahead to next segment)
        const overrides = prevSegment ? contextOverrides[prevSegment] : undefined;
        const override = overrides && nextSegment ? overrides[nextSegment] : undefined;
        // Fall back to default idMapping
        const placeholder = override ?? (prevSegment ? idMapping[prevSegment] : undefined);
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
 *
 * POST operations are NOT retried unless explicitly marked idempotent in
 * metadata (idempotent.natural === true). This prevents duplicate resource
 * creation on transient failures. GET, PUT, DELETE are naturally idempotent
 * and use the operation's retry config or the default.
 */
function getRetryConfigForRequest(method: string, url: string): RetryConfig {
  const upperMethod = method.toUpperCase();
  const normalizedPath = normalizeUrlPath(url);
  const key = `${upperMethod}:${normalizedPath}`;
  const operationName = PATH_TO_OPERATION[key];

  const opMeta = operationName
    ? metadata.operations[operationName as keyof typeof metadata.operations]
    : undefined;

  // POST operations must not be retried unless explicitly marked idempotent
  if (upperMethod === "POST") {
    if (opMeta?.idempotent?.natural) {
      return (opMeta.retry as RetryConfig) ?? DEFAULT_RETRY_CONFIG;
    }
    return NO_RETRY_CONFIG;
  }

  if (opMeta?.retry) {
    return opMeta.retry as RetryConfig;
  }

  return DEFAULT_RETRY_CONFIG;
}

/**
 * The retry loop, installed beneath the middleware chain as the client's custom
 * fetch (createClient's `fetch` option). openapi-fetch calls it exactly once
 * per logical request, after every onRequest middleware and before every
 * onError/onResponse — so the loop runs to the operation's full declared
 * maxAttempts, and no attempt ever leaves the chain, because the chain sits
 * above the loop.
 *
 * Division of labor with the lifecycle middleware: the loop begins EVERY
 * attempt and finalizes the attempts it abandons (before the backoff sleep);
 * the terminal outcome — the response it returns, or the error it throws — is
 * deliberately NOT finalized here. It flows up through the cache middleware,
 * which may rewrite a 304 into a cached 200 + X-From-Cache, to the lifecycle
 * middleware, which records the post-transform result. Finalizing in the loop
 * would freeze the pre-transformation status, and the idempotent finalize
 * would keep the hooks pass from correcting it.
 *
 * With retry disabled the loop degenerates to a single attempt but still owns
 * hook emission, so start/end events are single-sourced regardless of config.
 */
function createRetryingFetch(
  lifecycle: RequestLifecycle,
  authStrategy: AuthStrategy,
  enableRetry: boolean,
): (input: Request) => Promise<Response> {
  return async (request) => {
    const { method, url } = request;
    const retryConfig = getRetryConfigForRequest(method, url);
    const effectiveConfig: RetryConfig = {
      ...retryConfig,
      maxAttempts: enableRetry ? retryConfig.maxAttempts : 1,
    };

    // Serialize the body once, before the first send, because Request.body is
    // a stream that can only be consumed once and a retry needs to replay it.
    // clone() tees the stream, so attempt 1 sends the original untouched and
    // replays read from this buffer. Requests that can never retry — including
    // every non-idempotent POST, via NO_RETRY_CONFIG — skip the buffering.
    const upperMethod = method.toUpperCase();
    let bodyBuffer: ArrayBuffer | null = null;
    if (
      effectiveConfig.maxAttempts > 1 &&
      (upperMethod === "POST" || upperMethod === "PUT" || upperMethod === "PATCH") &&
      request.body
    ) {
      bodyBuffer = await request.clone().arrayBuffer();
    }

    let attemptRequest = request;
    const makeAttempt = async (attempt: number): Promise<Response> => {
      if (attempt > 1) {
        // Rebuild from the original request: the headers carry everything the
        // onRequest middleware attached (auth, User-Agent, If-None-Match), and
        // the signal keeps the caller's cancellation and the per-request
        // timeout — one budget shared by all attempts and backoffs.
        attemptRequest = new Request(url, {
          method,
          headers: new Headers(request.headers),
          body: bodyBuffer,
          signal: request.signal,
        });

        // Refresh auth (the token may have rotated since the last attempt).
        // The attempt is already begun, so a throwing refresh lands on a live
        // attempt; the terminal marker carries it raw past the loop's retry
        // classification to the lifecycle middleware's onError.
        try {
          await authStrategy.authenticate(attemptRequest.headers);
        } catch (error) {
          throw new TerminalRetryError(error);
        }
      }

      // globalThis.fetch resolved per attempt rather than captured at client
      // creation, so test interceptors that patch it (MSW) are honored.
      return globalThis.fetch(attemptRequest);
    };

    const emit: RetryEmit = {
      begin: (attempt) => lifecycle.begin(request, method, url, attempt),
      finalize: (outcome) => lifecycle.finalize(request, method, url, outcome),
      retrying: (failedAttempt, error, delayMs) =>
        lifecycle.retrying(method, url, failedAttempt, error, delayMs),
    };

    try {
      return await executeWithRetry(makeAttempt, effectiveConfig, emit, request.signal);
    } catch (error) {
      // Unwrap the terminal marker so the original error's identity survives
      // to the lifecycle middleware's onError (and to the caller).
      throw error instanceof TerminalRetryError ? error.reason : error;
    }
  };
}

// =============================================================================
// Pagination Helper
// =============================================================================


/**
 * Fetches all pages of a paginated resource using Link header pagination.
 * Automatically follows rel="next" links until no more pages exist, or until
 * `maxPages` pages have been consumed.
 *
 * The cap is not optional in spirit: a Link header naming the page it was
 * served from makes "until no more pages exist" never true, and the header is
 * attacker-influenced (that is why `isSameOrigin` is checked below). Every
 * other pagination loop in this SDK, and in the other five, is bounded the
 * same way. Reaching the cap stops quietly — there is no meta channel on a
 * bare `T[]` to report it, and throwing would break callers who legitimately
 * have that many pages.
 *
 * @param maxPages - Safety cap on pages CONSUMED. Must be a positive integer;
 *   `0`, negatives, `NaN`, `Infinity` and non-integers all throw
 *   `BasecampError("usage")` before any request is made. Defaults to
 *   `DEFAULT_MAX_PAGES`.
 * @throws {BasecampError} with `code: "usage"` if `maxPages` is not a positive
 *   integer.
 *
 * @example
 * ```ts
 * const response = await client.GET("/projects.json");
 *
 * const allProjects = await fetchAllPages(
 *   response.response,
 *   (r) => r.json() as Promise<any[]>
 * );
 * ```
 */
export async function fetchAllPages<T>(
  initialResponse: Response,
  parse: (response: Response) => Promise<T[]>,
  authHeader?: string,
  maxPages: number = DEFAULT_MAX_PAGES
): Promise<T[]> {
  assertValidMaxPages(maxPages);

  const results: T[] = [];
  let response = initialResponse;

  for (let page = 1; page <= maxPages; page++) {
    const items = await parse(response.clone());
    results.push(...items);

    // Before reading the Link header, not after fetching it. The loop consumes
    // the current page at the head of the iteration and fetches the next at the
    // tail, so testing the cap only in the `for` condition would let the final
    // allowed iteration issue a request whose response is never parsed and
    // never returned — and that request goes to a URL taken from an
    // attacker-influenceable header. `maxPages` counts pages CONSUMED, matching
    // BaseService.followPagination.
    if (page === maxPages) break;

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
 * Bounded by `maxPages` for the same reason as {@link fetchAllPages}: a Link
 * header that points at its own page would otherwise yield forever.
 *
 * Validation is EAGER, which is why this is a plain function returning a
 * generator rather than an `async function*`. The body of an `async function*`
 * does not run until something iterates it, so a check written inside one would
 * surface a bad `maxPages` at some later `for await` — possibly in a different
 * function, on a generator that was passed along. A usage error is a programmer
 * error and belongs at the call site that made it.
 *
 * @param maxPages - Safety cap on pages CONSUMED. Must be a positive integer;
 *   `0`, negatives, `NaN`, `Infinity` and non-integers all throw
 *   `BasecampError("usage")` from this call, before the generator is created
 *   and before any request is made. Defaults to `DEFAULT_MAX_PAGES`.
 * @throws {BasecampError} with `code: "usage"` if `maxPages` is not a positive
 *   integer.
 *
 * @example
 * ```ts
 * for await (const page of paginateAll(response.response, (r) => r.json() as Promise<any[]>)) {
 *   console.log(`Processing ${page.length} items`);
 * }
 * ```
 */
export function paginateAll<T>(
  initialResponse: Response,
  parse: (response: Response) => Promise<T[]>,
  authHeader?: string,
  maxPages: number = DEFAULT_MAX_PAGES
): AsyncGenerator<T[], void, unknown> {
  assertValidMaxPages(maxPages);

  return paginatePages(initialResponse, parse, authHeader, maxPages);
}

/**
 * The generator body behind {@link paginateAll}. Split out only so the cap can
 * be validated before the generator object exists; `maxPages` is already known
 * good here.
 */
async function* paginatePages<T>(
  initialResponse: Response,
  parse: (response: Response) => Promise<T[]>,
  authHeader: string | undefined,
  maxPages: number
): AsyncGenerator<T[], void, unknown> {
  let response = initialResponse;

  for (let page = 1; page <= maxPages; page++) {
    const items = await parse(response.clone());
    yield items;

    // See {@link fetchAllPages}: breaking here rather than relying on the loop
    // condition is what keeps the last allowed iteration from issuing a fetch
    // whose response is never yielded.
    if (page === maxPages) break;

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
