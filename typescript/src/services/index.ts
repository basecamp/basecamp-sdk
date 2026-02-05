/**
 * Service layer exports.
 *
 * Re-exports all service classes and their types.
 */

// Base service
export { BaseService, type RawClient, type FetchResponse } from "./base.js";

// Core services
export {
  ProjectsService,
  type Project,
  type DockItem,
  type ProjectStatus,
  type ProjectListOptions,
  type CreateProjectRequest,
  type UpdateProjectRequest,
} from "./projects.js";

export {
  TodosService,
  type Todo,
  type Person,
  type TodoListOptions,
  type CreateTodoRequest,
  type UpdateTodoRequest,
} from "./todos.js";

export {
  TodolistsService,
  type Todolist,
  type TodolistListOptions,
  type CreateTodolistRequest,
  type UpdateTodolistRequest,
} from "./todolists.js";

// Communication services
export {
  MessagesService,
  type Message,
  type MessageType,
  type CreateMessageRequest,
  type UpdateMessageRequest,
} from "./messages.js";

export {
  CommentsService,
  type Comment,
  type CreateCommentRequest,
  type UpdateCommentRequest,
} from "./comments.js";

export {
  CampfiresService,
  type Campfire,
  type CampfireLine,
  type Chatbot,
  type CreateChatbotRequest,
  type UpdateChatbotRequest,
} from "./campfires.js";

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
} from "./cards.js";

// Authorization service
export {
  AuthorizationService,
  type Identity,
  type AuthorizedAccount,
  type AuthorizationInfo,
  type GetAuthorizationInfoOptions,
} from "./authorization.js";

// People service (re-export with additional types)
export {
  PeopleService,
  type Person as PersonFull,
  type PersonCompany,
  type CreatePersonRequest,
  type UpdateProjectAccessRequest,
  type UpdateProjectAccessResponse,
} from "./people.js";

// Todosets service
export {
  TodosetsService,
  type Todoset,
  type TodosetBucket,
  type TodosetCreator,
} from "./todosets.js";

// Message Boards service
export {
  MessageBoardsService,
  type MessageBoard,
  type PersonRef as MessageBoardPersonRef,
  type BucketRef as MessageBoardBucketRef,
} from "./message-boards.js";

// Message Types service
export {
  MessageTypesService,
  type MessageType as MessageTypeItem,
  type CreateMessageTypeRequest,
  type UpdateMessageTypeRequest,
} from "./message-types.js";

// Forwards service
export {
  ForwardsService,
  type Inbox,
  type Forward,
  type ForwardReply,
  type CreateForwardReplyRequest,
} from "./forwards.js";

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
} from "./checkins.js";

// Client Portal services
export {
  ClientApprovalsService,
  type ClientApproval,
  type ClientApprovalResponse,
} from "./client-approvals.js";

export {
  ClientCorrespondencesService,
  type ClientCorrespondence,
} from "./client-correspondences.js";

export {
  ClientRepliesService,
  type ClientReply,
} from "./client-replies.js";

// Automation services
export {
  WebhooksService,
  type Webhook,
  type CreateWebhookRequest,
  type UpdateWebhookRequest,
} from "./webhooks.js";

export {
  SubscriptionsService,
  type Subscription,
  type UpdateSubscriptionRequest,
} from "./subscriptions.js";

// File management services
export {
  AttachmentsService,
  type AttachmentResponse,
  type CreateAttachmentRequest,
} from "./attachments.js";

export {
  VaultsService,
  type Vault,
  type CreateVaultRequest,
  type UpdateVaultRequest,
} from "./vaults.js";

export {
  DocumentsService,
  type Document,
  type DocumentStatus,
  type CreateDocumentRequest,
  type UpdateDocumentRequest,
} from "./documents.js";

export {
  UploadsService,
  type Upload,
  type CreateUploadRequest,
  type UpdateUploadRequest,
} from "./uploads.js";

// Schedule services
export {
  SchedulesService,
  type Schedule,
  type ScheduleEntry,
  type CreateScheduleEntryRequest,
  type UpdateScheduleEntryRequest,
  type UpdateScheduleSettingsRequest,
} from "./schedules.js";

// Event services
export {
  EventsService,
  type Event,
  type EventDetails,
} from "./events.js";

// Recording services
export {
  RecordingsService,
  type Recording,
  type RecordingParent,
  type RecordingBucket,
  type RecordingType,
  type RecordingStatus,
  type RecordingSortField,
  type RecordingSortDirection,
  type RecordingsListOptions,
} from "./recordings.js";

// Search service
export {
  SearchService,
  type SearchResult,
  type SearchMetadata,
  type SearchProject,
  type SearchOptions,
} from "./search.js";

// Reports service
export {
  ReportsService,
  type TimesheetEntry,
  type TimesheetReportOptions,
} from "./reports.js";

// Templates service
export {
  TemplatesService,
  type Template,
  type ProjectConstruction,
  type CreateTemplateRequest,
  type UpdateTemplateRequest,
  type CreateProjectFromTemplateRequest,
} from "./templates.js";

// Lineup service
export {
  LineupService,
  type CreateMarkerRequest,
  type UpdateMarkerRequest,
} from "./lineup.js";

// Todolist Groups service
export {
  TodolistGroupsService,
  type TodolistGroup,
  type CreateTodolistGroupRequest,
  type UpdateTodolistGroupRequest,
} from "./todolistGroups.js";

// Tools service
export {
  ToolsService,
  type Tool,
} from "./tools.js";
