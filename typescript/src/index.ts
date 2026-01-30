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
// Services
// =============================================================================

// Base service (for extending)
export { BaseService, type FetchResponse } from "./services/base.js";

// Core services
export {
  ProjectsService,
  type Project,
  type DockItem,
  type ProjectStatus,
  type ProjectListOptions,
  type CreateProjectRequest,
  type UpdateProjectRequest,
} from "./services/projects.js";

export {
  TodosService,
  type Todo,
  type Person,
  type TodoListOptions,
  type CreateTodoRequest,
  type UpdateTodoRequest,
} from "./services/todos.js";

export {
  TodolistsService,
  type Todolist,
  type TodolistListOptions,
  type CreateTodolistRequest,
  type UpdateTodolistRequest,
} from "./services/todolists.js";

export {
  TodosetsService,
  type Todoset,
  type TodosetBucket,
  type TodosetCreator,
} from "./services/todosets.js";

export {
  PeopleService,
  type Person as PersonFull,
  type PersonCompany,
  type CreatePersonRequest,
  type UpdateProjectAccessRequest,
  type UpdateProjectAccessResponse,
} from "./services/people.js";

export {
  AuthorizationService,
  type Identity,
  type AuthorizedAccount,
  type AuthorizationInfo,
  type GetAuthorizationInfoOptions,
} from "./services/authorization.js";

// Communication services
export {
  MessagesService,
  type Message,
  type MessageType,
  type CreateMessageRequest,
  type UpdateMessageRequest,
} from "./services/messages.js";

export {
  CommentsService,
  type Comment,
  type CreateCommentRequest,
  type UpdateCommentRequest,
} from "./services/comments.js";

export {
  CampfiresService,
  type Campfire,
  type CampfireLine,
  type Chatbot,
  type CreateChatbotRequest,
  type UpdateChatbotRequest,
} from "./services/campfires.js";

// Card services (kanban boards)
export {
  CardTablesService,
  CardsService,
  CardColumnsService,
  CardStepsService,
  type CardTable,
  type CardColumn,
  type Card,
  type CardStep,
  type ColumnColor,
  type CreateCardRequest,
  type UpdateCardRequest,
  type CreateColumnRequest,
  type UpdateColumnRequest,
  type MoveColumnRequest,
  type CreateStepRequest,
  type UpdateStepRequest,
} from "./services/cards.js";

// Message Boards service
export {
  MessageBoardsService,
  type MessageBoard,
} from "./services/message-boards.js";

// Message Types service
export {
  MessageTypesService,
  type MessageType as MessageTypeItem,
  type CreateMessageTypeRequest,
  type UpdateMessageTypeRequest,
} from "./services/message-types.js";

// Forwards service
export {
  ForwardsService,
  type Inbox,
  type Forward,
  type ForwardReply,
  type CreateForwardReplyRequest,
} from "./services/forwards.js";

// Checkins service
export {
  CheckinsService,
  type Questionnaire,
  type Question,
  type QuestionAnswer,
  type QuestionSchedule,
  type CreateQuestionRequest,
  type UpdateQuestionRequest,
  type CreateAnswerRequest,
  type UpdateAnswerRequest,
} from "./services/checkins.js";

// Client Portal services
export {
  ClientApprovalsService,
  type ClientApproval,
  type ClientApprovalResponse,
} from "./services/client-approvals.js";

export {
  ClientCorrespondencesService,
  type ClientCorrespondence,
} from "./services/client-correspondences.js";

export {
  ClientRepliesService,
  type ClientReply,
} from "./services/client-replies.js";

// Automation services
export {
  WebhooksService,
  type Webhook,
  type CreateWebhookRequest,
  type UpdateWebhookRequest,
} from "./services/webhooks.js";

export {
  SubscriptionsService,
  type Subscription,
  type UpdateSubscriptionRequest,
} from "./services/subscriptions.js";

// Search & Reports services
export {
  SearchService,
  type SearchResult,
  type SearchMetadata,
  type SearchProject,
  type SearchOptions,
} from "./services/search.js";

export {
  ReportsService,
  type TimesheetEntry,
  type TimesheetReportOptions,
} from "./services/reports.js";

// Templates service
export {
  TemplatesService,
  type Template,
  type ProjectConstruction,
  type CreateTemplateRequest,
  type UpdateTemplateRequest,
  type CreateProjectFromTemplateRequest,
} from "./services/templates.js";

// Time & Activity services
export {
  LineupService,
  type LineupMarker,
  type MarkerColor,
  type CreateMarkerRequest,
  type UpdateMarkerRequest,
} from "./services/lineup.js";

// Organization services
export {
  TodolistGroupsService,
  type TodolistGroup,
  type CreateTodolistGroupRequest,
  type UpdateTodolistGroupRequest,
} from "./services/todolistGroups.js";

export {
  ToolsService,
  type Tool,
} from "./services/tools.js";

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
