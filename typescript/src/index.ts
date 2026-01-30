/**
 * Basecamp TypeScript SDK
 *
 * Type-safe client for the Basecamp 3 API.
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
 * // High-level service methods
 * const projects = await client.projects.list();
 * const todo = await client.todos.create(projectId, todolistId, {
 *   content: "Ship the feature",
 *   assigneeIds: [userId],
 * });
 *
 * // Or use low-level typed API calls
 * const { data, error } = await client.GET("/projects.json");
 *
 * if (data) {
 *   console.log(data.map(p => p.name));
 * }
 * ```
 *
 * @packageDocumentation
 */

// Main client factory
export {
  createBasecampClient,
  type BasecampClient,
  type BasecampClientOptions,
  type TokenProvider,
  type RawClient,
} from "./client.js";

// Pagination helpers
export { fetchAllPages, paginateAll } from "./client.js";

// Errors
export {
  BasecampError,
  Errors,
  errorFromResponse,
  isBasecampError,
  isErrorCode,
  type ErrorCode,
  type BasecampErrorOptions,
} from "./errors.js";

// Hooks
export {
  chainHooks,
  consoleHooks,
  noopHooks,
  safeInvoke,
  type BasecampHooks,
  type OperationInfo,
  type RequestInfo,
  type RequestResult,
  type OperationResult,
  type ConsoleHooksOptions,
} from "./hooks.js";

// =============================================================================
// Services - Generated from OpenAPI spec (spec-driven)
// =============================================================================

// Base service (for extending) - hand-written infrastructure
export { BaseService, type FetchResponse } from "./services/base.js";

// Authorization service - hand-written (OAuth flows not in OpenAPI spec)
export {
  AuthorizationService,
  type Identity,
  type AuthorizedAccount,
  type AuthorizationInfo,
  type GetAuthorizationInfoOptions,
} from "./services/authorization.js";

// Core services - generated
export {
  ProjectsService,
  type Project,
  type ListProjectOptions,
  type CreateProjectRequest,
  type UpdateProjectRequest,
} from "./generated/services/projects.js";

export {
  TodosService,
  type Todo,
  type ListTodoOptions,
  type CreateTodoRequest,
  type UpdateTodoRequest,
  type RepositionTodoRequest,
} from "./generated/services/todos.js";

export {
  TodolistsService,
  type Todolist,
  type ListTodolistOptions,
  type CreateTodolistRequest,
  type UpdateTodolistRequest,
} from "./generated/services/todolists.js";

export {
  TodosetsService,
  type Todoset,
} from "./generated/services/todosets.js";

export {
  PeopleService,
  type Person,
  type UpdateProjectAccessPeopleRequest,
} from "./generated/services/people.js";

// Communication services - generated
export {
  MessagesService,
  type Message,
  type CreateMessageRequest,
  type UpdateMessageRequest,
} from "./generated/services/messages.js";

export {
  CommentsService,
  type Comment,
  type CreateCommentRequest,
  type UpdateCommentRequest,
} from "./generated/services/comments.js";

export {
  CampfiresService,
  type Campfire,
  type CampfireLine,
  type Chatbot,
  type CreateChatbotCampfireRequest,
  type UpdateChatbotCampfireRequest,
  type CreateLineCampfireRequest,
} from "./generated/services/campfires.js";

// Card services (kanban boards) - generated
export {
  CardTablesService,
  type CardTable,
} from "./generated/services/card-tables.js";

export {
  CardsService,
  type Card,
  type CreateCardRequest,
  type UpdateCardRequest,
  type MoveCardRequest,
} from "./generated/services/cards.js";

export {
  CardColumnsService,
  type CardColumn,
  type CreateCardColumnRequest,
  type UpdateCardColumnRequest,
  type MoveCardColumnRequest,
  type SetColorCardColumnRequest,
} from "./generated/services/card-columns.js";

export {
  CardStepsService,
  type CardStep,
  type CreateCardStepRequest,
  type UpdateCardStepRequest,
} from "./generated/services/card-steps.js";

// Message services - generated
export {
  MessageBoardsService,
  type MessageBoard,
} from "./generated/services/message-boards.js";

export {
  MessageTypesService,
  type MessageType,
  type CreateMessageTypeRequest,
  type UpdateMessageTypeRequest,
} from "./generated/services/message-types.js";

// Forwards service - generated
export {
  ForwardsService,
  type Inbox,
  type Forward,
  type ForwardReply,
  type CreateReplyForwardRequest,
} from "./generated/services/forwards.js";

// Checkins service - generated
export {
  CheckinsService,
  type Questionnaire,
  type Question,
  type Answer,
  type CreateQuestionCheckinRequest,
  type UpdateQuestionCheckinRequest,
  type CreateAnswerCheckinRequest,
  type UpdateAnswerCheckinRequest,
  type UpdateNotificationSettingsCheckinRequest,
} from "./generated/services/checkins.js";

// Client Portal services - generated
export {
  ClientApprovalsService,
  type ClientApproval,
} from "./generated/services/client-approvals.js";

export {
  ClientCorrespondencesService,
  type ClientCorrespondence,
} from "./generated/services/client-correspondences.js";

export {
  ClientRepliesService,
  type ClientReply,
} from "./generated/services/client-replies.js";

export {
  ClientVisibilityService,
} from "./generated/services/client-visibility.js";

// Automation services - generated
export {
  WebhooksService,
  type Webhook,
  type CreateWebhookRequest,
  type UpdateWebhookRequest,
} from "./generated/services/webhooks.js";

export {
  SubscriptionsService,
  type Subscription,
  type UpdateSubscriptionRequest,
} from "./generated/services/subscriptions.js";

export {
  EventsService,
  type Event,
} from "./generated/services/events.js";

// File services - generated
export {
  AttachmentsService,
} from "./generated/services/attachments.js";

export {
  VaultsService,
  type Vault,
  type CreateVaultRequest,
  type UpdateVaultRequest,
} from "./generated/services/vaults.js";

export {
  DocumentsService,
  type Document,
  type CreateDocumentRequest,
  type UpdateDocumentRequest,
} from "./generated/services/documents.js";

export {
  UploadsService,
  type Upload,
  type CreateUploadRequest,
  type UpdateUploadRequest,
} from "./generated/services/uploads.js";

// Schedule & Time services - generated
export {
  SchedulesService,
  type Schedule,
  type ScheduleEntry,
  type CreateEntryScheduleRequest,
  type UpdateEntryScheduleRequest,
  type UpdateSettingsScheduleRequest,
  type ListEntriesScheduleOptions,
} from "./generated/services/schedules.js";

export {
  TimesheetsService,
  type ForRecordingTimesheetOptions,
  type ForProjectTimesheetOptions,
  type ReportTimesheetOptions,
} from "./generated/services/timesheets.js";

export {
  TimelineService,
} from "./generated/services/timeline.js";

// Search & Reports services - generated
export {
  SearchService,
  type SearchSearchOptions,
} from "./generated/services/search.js";

export {
  ReportsService,
} from "./generated/services/reports.js";

// Recording services - generated
export {
  RecordingsService,
  type Recording,
  type ListRecordingOptions,
} from "./generated/services/recordings.js";

// Templates service - generated
export {
  TemplatesService,
  type Template,
  type ListTemplateOptions,
  type CreateTemplateRequest,
  type UpdateTemplateRequest,
  type CreateProjectTemplateRequest,
} from "./generated/services/templates.js";

// Lineup service - generated
export {
  LineupService,
  type LineupMarker,
  type CreateLineupRequest,
  type UpdateLineupRequest,
} from "./generated/services/lineup.js";

// Organization services - generated
export {
  TodolistGroupsService,
  type TodolistGroup,
  type CreateTodolistGroupRequest,
} from "./generated/services/todolist-groups.js";

export {
  ToolsService,
  type Tool,
} from "./generated/services/tools.js";

// OpenTelemetry hooks
export {
  otelHooks,
  type OtelHooksOptions,
} from "./hooks/otel.js";

// =============================================================================
// OAuth
// =============================================================================

// OAuth types
export type {
  OAuthConfig,
  OAuthToken,
  ExchangeRequest,
  RefreshRequest,
  RawTokenResponse,
  OAuthErrorResponse,
} from "./oauth/types.js";

// OAuth functions
export {
  discover,
  discoverLaunchpad,
  LAUNCHPAD_BASE_URL,
  type DiscoverOptions,
} from "./oauth/discovery.js";

export {
  exchangeCode,
  refreshToken,
  isTokenExpired,
  type TokenOptions,
} from "./oauth/exchange.js";

// PKCE utilities
export {
  generatePKCE,
  generateState,
  type PKCE,
} from "./oauth/pkce.js";

// =============================================================================
// Security Utilities
// =============================================================================

export {
  redactHeaders,
  redactHeadersRecord,
} from "./security.js";

// Re-export generated types
export type { paths } from "./generated/schema.js";
