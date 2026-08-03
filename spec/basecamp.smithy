$version: "2"

// =============================================================================
// ARCHITECTURAL NOTE: Response Format Mappers
// =============================================================================
// The BC3 API returns bare values—arrays for list endpoints and objects for
// single-entity endpoints. Smithy's AWS restJson1 protocol requires outputs to
// be modeled as wrapped structures because @httpPayload only supports string,
// blob, structure, union, and document types—not arrays or bare references.
//
// As a result:
//   - This Smithy model uses wrapped outputs (e.g., ListProjectsOutput.projects,
//     GetProjectOutput.project)
//   - Two custom OpenApiMappers transform schemas during OpenAPI generation:
//     * BareArrayResponseMapper: List*ResponseContent → bare arrays
//     * BareObjectResponseMapper: Get*ResponseContent (single property, non-array) → bare $ref
//   - Generated SDK clients correctly handle bare responses
//
// Multi-field Get responses (e.g., GetAssignedTodosOutput) are left wrapped
// because the API genuinely returns an object with multiple top-level keys.
//
// This is a known protocol limitation, not a modeling error.
// =============================================================================

namespace basecamp

use smithy.api#documentation
use smithy.api#http
use smithy.api#httpLabel
use smithy.api#httpQuery
use smithy.api#httpPayload
use smithy.api#required
use smithy.api#readonly
use smithy.api#idempotent
use smithy.api#error
use smithy.api#httpError
use smithy.api#retryable
use smithy.api#sensitive
use smithy.api#deprecated
use aws.protocols#restJson1

// Bridge traits for OpenAPI x-basecamp-* extensions
use basecamp.traits#basecampRetry
use basecamp.traits#basecampPagination
use basecamp.traits#basecampIdempotent
use basecamp.traits#basecampWriteSemantics
use basecamp.traits#basecampMultipart
use basecamp.traits#basecampSensitive
use basecamp.traits#basecampAuthRoutableUrl

/// Basecamp API
@restJson1
service Basecamp {
  version: "2026-08-02"
  rename: {
    "smithy.api#Document": "JsonDocument"
  }
  operations: [
    ListProjects,
    GetProject,
    CreateProject,
    UpdateProject,
    TrashProject,
    ListTodos,
    GetTodo,
    CreateTodo,
    CreateTodosetTodo,
    ReplaceTodo,
    CompleteTodo,
    UncompleteTodo,
    RepositionTodo,
    GetTodoset,
    GetHillChart,
    UpdateHillChartSettings,
    ListTodolists,
    GetTodolistOrGroup,
    CreateTodolist,
    UpdateTodolistOrGroup,
    RepositionTodolist,
    ListTodolistGroups,
    CreateTodolistGroup,
    RepositionTodolistGroup,

    // Batch 1 - Comments, Messages, MessageBoards, MessageTypes
    ListComments,
    GetComment,
    CreateComment,
    UpdateComment,
    ListMessages,
    GetMessage,
    CreateMessage,
    UpdateMessage,
    PinMessage,
    UnpinMessage,
    GetMessageBoard,
    ListMessageTypes,
    GetMessageType,
    CreateMessageType,
    UpdateMessageType,
    DeleteMessageType,

    // Batch 2 - Vaults, Documents, Uploads, Attachments
    ListVaults,
    GetVault,
    CreateVault,
    UpdateVault,
    ListDocuments,
    GetDocument,
    CreateDocument,
    ReplaceDocument,
    ListUploads,
    GetUpload,
    CreateUpload,
    UpdateUpload,
    ListUploadVersions,
    CreateAttachment,

    // Batch 3 - Schedules, Timesheets
    GetSchedule,
    UpdateScheduleSettings,
    ListScheduleEntries,
    GetScheduleEntry,
    GetScheduleEntryOccurrence,
    CreateScheduleEntry,
    UpdateScheduleEntry,
    GetTimesheetReport,
    GetProjectTimesheet,
    GetRecordingTimesheet,
    GetTimesheetEntry,
    CreateTimesheetEntry,
    UpdateTimesheetEntry,

    // Batch 4 - Campfires, Chatbots, Forwards/Inboxes (Real-time)
    ListCampfires,
    GetCampfire,
    ListCampfireLines,
    GetCampfireLine,
    CreateCampfireLine,
    UpdateCampfireLine,
    DeleteCampfireLine,
    ListCampfireUploads,
    CreateCampfireUpload,
    ListChatbots,
    GetChatbot,
    CreateChatbot,
    UpdateChatbot,
    DeleteChatbot,
    GetInbox,
    ListForwards,
    GetForward,
    ListForwardReplies,
    GetForwardReply,

    // Batch 5 - CardTables, Cards, CardColumns, CardSteps (Kanban)
    GetCardTable,
    ListCards,
    GetCard,
    CreateCard,
    UpdateCard,
    MoveCard,
    GetCardColumn,
    CreateCardColumn,
    UpdateCardColumn,
    MoveCardColumn,
    SetCardColumnColor,
    EnableCardColumnOnHold,
    DisableCardColumnOnHold,
    SubscribeToCardColumn,
    UnsubscribeFromCardColumn,
    GetCardStep,
    CreateCardStep,
    UpdateCardStep,
    SetCardStepCompletion,
    RepositionCardStep,
    CreateWormhole,
    UpdateWormhole,
    DeleteWormhole,

    // Batch 6 - People, Subscriptions (People & Access)
    ListPeople,
    GetPerson,
    GetMyProfile,
    ListProjectPeople,
    ListPingablePeople,
    UpdateProjectAccess,
    GetSubscription,
    Subscribe,
    Unsubscribe,
    UpdateSubscription,

    // Batch 7 - ClientApprovals, ClientCorrespondences, ClientReplies (Client Features)
    ListClientApprovals,
    GetClientApproval,
    ListClientCorrespondences,
    GetClientCorrespondence,
    ListClientReplies,
    GetClientReply,

    // Batch 8 - Webhooks, Events, Recordings (Automation & Lifecycle)
    // Note: TrashRecording/ArchiveRecording/UnarchiveRecording are generic operations
    // that work on any recording type (comments, messages, documents, cards, etc.)
    ListWebhooks,
    GetWebhook,
    CreateWebhook,
    UpdateWebhook,
    DeleteWebhook,
    ListEvents,
    ListRecordings,
    TrashRecording,
    ArchiveRecording,
    UnarchiveRecording,
    SetClientVisibility,

    // Batch 9 - Questionnaires, Questions, Answers (Checkins)
    GetQuestionnaire,
    ListQuestions,
    GetQuestion,
    CreateQuestion,
    UpdateQuestion,
    PauseQuestion,
    ResumeQuestion,
    UpdateQuestionNotificationSettings,
    ListAnswers,
    GetAnswer,
    CreateAnswer,
    UpdateAnswer,
    ListQuestionAnswerers,
    GetAnswersByPerson,
    GetQuestionReminders,

    // Batch 10 - Search, Templates, Tools, Lineup (Utilities)
    Search,
    GetSearchMetadata,
    ListTemplates,
    GetTemplate,
    CreateTemplate,
    UpdateTemplate,
    DeleteTemplate,
    CreateProjectFromTemplate,
    GetProjectConstruction,
    GetTool,
    CreateTool,
    UpdateTool,
    DeleteTool,
    EnableTool,
    DisableTool,
    RepositionTool,
    ListLineupMarkers,
    CreateLineupMarker,
    UpdateLineupMarker,
    DeleteLineupMarker,

    // Batch 11 - Timeline, Reports (Activity & Reports)
    GetProgressReport,
    GetProjectTimeline,
    GetPersonProgress,
    ListAssignablePeople,
    GetAssignedTodos,
    GetOverdueTodos,
    GetUpcomingSchedule,

    // Batch 12 - Boosts
    ListRecordingBoosts,
    ListEventBoosts,
    GetBoost,
    CreateRecordingBoost,
    CreateEventBoost,
    DeleteBoost,

    // Batch 13 - Account
    GetAccount,
    UpdateAccountName,
    UpdateAccountLogo,
    RemoveAccountLogo,

    // Batch 14 - Gauges
    ListGauges,
    ListGaugeNeedles,
    GetGaugeNeedle,
    CreateGaugeNeedle,
    UpdateGaugeNeedle,
    DestroyGaugeNeedle,
    ToggleGauge,

    // Batch 15 - My Assignments
    GetMyAssignments,
    PrioritizeAssignment,
    DeprioritizeAssignment,
    ReorderUpNext,
    GetMyCompletedAssignments,
    GetMyDueAssignments,

    // Batch 15b - Everything Aggregates (flat family)
    GetEverythingMessages,
    GetEverythingComments,
    GetEverythingCheckins,
    GetEverythingForwards,
    GetEverythingFiles,
    GetEverythingOverdueTodos,
    GetEverythingOverdueCards,

    // Batch 15c - Everything Aggregates (bucket-grouped todo/card family)
    GetEverythingOpenTodos,
    GetEverythingCompletedTodos,
    GetEverythingUnassignedTodos,
    GetEverythingNoDueDateTodos,
    GetEverythingOpenCards,
    GetEverythingCompletedCards,
    GetEverythingUnassignedCards,
    GetEverythingNoDueDateCards,
    GetEverythingNotNowCards,

    // Batch 16 - My Notifications
    GetMyNotifications,
    GetBubbleUps,
    MarkAsRead,
    GetCalendar,
    UpdateCalendar,
    GetMyNote,
    UpdateMyNote,
    ListMyDrafts,
    ListMyBookmarks,
    GetBookmark,
    CreateBookmark,
    DeleteBookmark,

    // Batch 17 - Out of Office
    GetOutOfOffice,
    EnableOutOfOffice,
    DisableOutOfOffice,

    // Batch 18 - People (Profile & Preferences)
    UpdateMyProfile,
    GetMyPreferences,
    UpdateMyPreferences,

    // Batch 19 - Folders (wire type "Stack")
    ListFolders,
    GetFolder,
    CreateFolder,
    UpdateFolder,
    DeleteFolder
  ]
}

// ===== Error Shapes =====

@error("client")
@httpError(404)
structure NotFoundError {
  @required
  error: String
  message: String
}

@error("client")
@httpError(422)
structure ValidationError {
  @required
  error: String
  message: String
}

/// 404 with no response body — the bare `head :not_found` rendering. Used by
/// operations whose controllers emit no JSON payload on 404, so the generated
/// OpenAPI does not advertise a decodable body that the server never sends.
@error("client")
@httpError(404)
structure BareNotFoundError {}

/// 422 whose body is keyed by field ({"errors": {"color": ["is not a valid
/// color"]}}) — the Rails RecordInvalid rendering. Used by operations whose
/// controllers emit the field-keyed shape instead of the flat {error} body.
@error("client")
@httpError(422)
structure FieldValidationError {
  /// Single member so the OpenAPI unwrap resolves to FieldKeyedErrors — the
  /// {"errors": {...}} wire shape (the BookmarkStatus treatment; a direct map
  /// member would unwrap to the bare map and lose the errors key).
  @required
  field_errors: FieldKeyedErrors
}

/// The field-keyed 422 body: {"errors": {"color": ["is not a valid color"]}}.
structure FieldKeyedErrors {
  @required
  errors: FieldErrorMap
}

map FieldErrorMap {
  key: String
  value: FieldErrorMessages
}

list FieldErrorMessages {
  member: String
}

@error("client")
@retryable(throttling: true)
@httpError(429)
structure RateLimitError {
  @required
  error: String
  message: String
  retry_after: Integer
}

@error("client")
@httpError(401)
structure UnauthorizedError {
  @required
  error: String
  message: String
}

@error("client")
@httpError(403)
structure ForbiddenError {
  @required
  error: String
  message: String
}

@error("client")
@httpError(400)
structure BadRequestError {
  @required
  error: String
  message: String
}

@error("server")
@httpError(507)
structure WebhookLimitError {
  @required
  error: String
  message: String
}

@error("server")
@retryable
@httpError(500)
structure InternalServerError {
  @required
  error: String
  message: String
}

/// Basecamp account ID (numeric string)
@pattern("^[0-9]+$")
string AccountId

/// List projects (active by default; optionally archived/trashed)
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/projects.json")
operation ListProjects {
  input: ListProjectsInput
  output: ListProjectsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListProjectsInput {
  @required
  @httpLabel
  accountId: AccountId

  @httpQuery("status")
  status: ProjectStatus

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListProjectsOutput {

  projects: ProjectList
}

/// Get a single project by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/projects/{projectId}")
operation GetProject {
  input: GetProjectInput
  output: GetProjectOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetProjectInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId
}

structure GetProjectOutput {

  project: Project
}

/// Create a new project
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/projects.json", code: 201)
operation CreateProject {
  input: CreateProjectInput
  output: CreateProjectOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateProjectInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  name: ProjectName
  description: ProjectDescription
}

structure CreateProjectOutput {

  project: Project
}

/// Update an existing project
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/projects/{projectId}")
operation UpdateProject {
  input: UpdateProjectInput
  output: UpdateProjectOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateProjectInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @required
  name: ProjectName
  description: ProjectDescription
  admissions: AdmissionsPolicy
  schedule_attributes: ScheduleAttributes
}

structure UpdateProjectOutput {

  project: Project
}

/// Trash a project (returns 204 No Content)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/projects/{projectId}", code: 204)
operation TrashProject {
  input: TrashProjectInput
  output: TrashProjectOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure TrashProjectInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId
}

structure TrashProjectOutput {}


// ===== Sensitive Types (PII) =====

@sensitive
string PersonName

@sensitive
string EmailAddress

@sensitive
string PersonTitle

@sensitive
string PersonBio

@sensitive
string PersonLocation

@sensitive
string AvatarUrl

@sensitive
string CompanyName

// ===== Shapes =====


long ProjectId
string ProjectName
string ProjectDescription
string ISO8601Timestamp
string ISO8601Date

@documentation("active|archived|trashed")
string ProjectStatus

@documentation("invite|employee|team")
string AdmissionsPolicy

structure ScheduleAttributes {
  start_date: ISO8601Date
  end_date: ISO8601Date
}

list ProjectList {
  member: Project
}

structure Project {
  @required
  id: ProjectId
  @required
  status: ProjectStatus
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  name: ProjectName
  description: ProjectDescription
  purpose: String
  start_date: ISO8601Date
  end_date: ISO8601Date
  clients_enabled: Boolean
  bookmark_url: String
  @required
  url: String
  @required
  app_url: String
  dock: DockItemList
  bookmarked: Boolean
  client_company: ClientCompany
  @deprecated(message: "Use Client Visibility feature instead", since: "2024-01")
  clientside: ClientSide
}

list DockItemList {
  member: DockItem
}

structure DockItem {
  @required
  id: Long
  @required
  title: String
  @required
  name: String
  @required
  enabled: Boolean
  position: Integer
  @required
  url: String
  @required
  app_url: String
}

structure ClientCompany {
  @required
  id: Long
  @required
  name: String
}

@deprecated(message: "Use Client Visibility feature instead", since: "2024-01")
structure ClientSide {
  url: String
  app_url: String
}

// ===== Todo Operations =====

/// List todos in a todolist
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/todolists/{todolistId}/todos.json")
operation ListTodos {
  input: ListTodosInput
  output: ListTodosOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListTodosInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todolistId: TodolistId

  @httpQuery("status")
  status: TodoStatus

  @httpQuery("completed")
  completed: Boolean

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListTodosOutput {

  todos: TodoItems
}

/// Get a single todo by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/todos/{todoId}")
operation GetTodo {
  input: GetTodoInput
  output: GetTodoOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetTodoInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todoId: TodoId
}

structure GetTodoOutput {

  todo: Todo
}

/// Create a new todo in a todolist
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/todolists/{todolistId}/todos.json", code: 201)
operation CreateTodo {
  input: CreateTodoInput
  output: CreateTodoOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateTodoInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todolistId: TodolistId

  @required
  content: TodoContent

  description: TodoDescription
  assignee_ids: PersonIdList
  completion_subscriber_ids: PersonIdList
  notify: Boolean
  due_on: ISO8601Date
  starts_on: ISO8601Date
}

structure CreateTodoOutput {

  todo: Todo
}

/// Create a to-do directly under a project's to-do set, outside any to-do list.
/// This form exists only project-scoped (no account-scoped variant); parameters
/// and response match the to-do-list create. Find a project's to-do set id via
/// GetTodoset.
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{bucketId}/todosets/{todosetId}/todos.json", code: 201)
operation CreateTodosetTodo {
  input: CreateTodosetTodoInput
  output: CreateTodoOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateTodosetTodoInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  todosetId: TodosetId

  @required
  content: TodoContent

  description: TodoDescription
  assignee_ids: PersonIdList
  completion_subscriber_ids: PersonIdList
  notify: Boolean
  due_on: ISO8601Date
  starts_on: ISO8601Date
}

/// Replace a todo with a new complete representation.
/// The request body is the todo's full writable state: any writable field
/// omitted from the request is cleared server-side (empty/missing
/// assignee_ids clears assignees, missing description clears it, and so
/// on). content is required — a request without it is rejected.
/// To set some fields while preserving the rest, use the SDK's merge-safe
/// update or edit methods, which GET the current todo and PUT the full
/// representation back. Those read-modify-write helpers are not atomic:
/// a concurrent write between the GET and PUT is overwritten (last write
/// wins for the whole representation; the window is one round-trip).
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@basecampWriteSemantics(mode: "replace", clearsOmitted: true)
@http(method: "PUT", uri: "/{accountId}/todos/{todoId}")
operation ReplaceTodo {
  input: ReplaceTodoInput
  output: ReplaceTodoOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure ReplaceTodoInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todoId: TodoId

  @required
  content: TodoContent

  description: TodoDescription
  assignee_ids: PersonIdList
  completion_subscriber_ids: PersonIdList
  notify: Boolean
  due_on: ISO8601Date
  starts_on: ISO8601Date
}

structure ReplaceTodoOutput {

  todo: Todo
}

/// Mark a todo as complete
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "POST", uri: "/{accountId}/todos/{todoId}/completion.json", code: 204)
operation CompleteTodo {
  input: CompleteTodoInput
  output: CompleteTodoOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CompleteTodoInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todoId: TodoId
}

structure CompleteTodoOutput {}

/// Mark a todo as incomplete
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/todos/{todoId}/completion.json", code: 204)
operation UncompleteTodo {
  input: UncompleteTodoInput
  output: UncompleteTodoOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UncompleteTodoInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todoId: TodoId
}

structure UncompleteTodoOutput {}

/// Reposition a todo within its todolist
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/todos/{todoId}/position.json")
operation RepositionTodo {
  input: RepositionTodoInput
  output: RepositionTodoOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure RepositionTodoInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todoId: TodoId

  @required
  position: Integer

  /// Optional todolist ID to move the todo to a different parent
  parent_id: TodolistId
}

structure RepositionTodoOutput {}

// ===== Todoset Operations =====

/// Get a todoset (container for todolists in a project)
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/todosets/{todosetId}")
operation GetTodoset {
  input: GetTodosetInput
  output: GetTodosetOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetTodosetInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todosetId: TodosetId
}

structure GetTodosetOutput {

  todoset: Todoset
}

// ===== Hill Chart Operations =====

/// Get the hill chart for a todoset
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/todosets/{todosetId}/hill.json")
operation GetHillChart {
  input: GetHillChartInput
  output: GetHillChartOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetHillChartInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todosetId: TodosetId
}

structure GetHillChartOutput {

  hillChart: HillChart
}

/// Track or untrack todolists on a hill chart
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/todosets/{todosetId}/hills/settings.json")
operation UpdateHillChartSettings {
  input: UpdateHillChartSettingsInput
  output: UpdateHillChartSettingsOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateHillChartSettingsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todosetId: TodosetId

  tracked: TodolistIdList
  untracked: TodolistIdList
}

structure UpdateHillChartSettingsOutput {

  hillChart: HillChart
}

// ===== Hill Chart Shapes =====

structure HillChart {
  @required
  enabled: Boolean

  @required
  stale: Boolean

  updated_at: ISO8601Timestamp

  app_update_url: String

  app_versions_url: String

  dots: HillChartDotList
}

structure HillChartDot {
  @required
  id: Long

  @required
  label: String

  @required
  color: String

  @required
  position: Integer

  url: String

  app_url: String
}

list HillChartDotList {
  member: HillChartDot
}

list TodolistIdList {
  member: TodolistId
}

// ===== Todolist Operations =====

/// List todolists in a todoset
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/todosets/{todosetId}/todolists.json")
operation ListTodolists {
  input: ListTodolistsInput
  output: ListTodolistsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListTodolistsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todosetId: TodosetId

  @httpQuery("status")
  status: TodolistStatus

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListTodolistsOutput {

  todolists: TodolistList
}

/// Get a single todolist or todolist group by id
/// The endpoint is polymorphic - the same URI returns either a Todolist or TodolistGroup
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/todolists/{id}")
operation GetTodolistOrGroup {
  input: GetTodolistOrGroupInput
  output: GetTodolistOrGroupOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetTodolistOrGroupInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  id: Long
}

structure GetTodolistOrGroupOutput {

  result: TodolistOrGroup
}

/// Union type for polymorphic todolist endpoint
union TodolistOrGroup {
  todolist: Todolist
  group: TodolistGroup
}

/// Create a new todolist in a todoset
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/todosets/{todosetId}/todolists.json", code: 201)
operation CreateTodolist {
  input: CreateTodolistInput
  output: CreateTodolistOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateTodolistInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todosetId: TodosetId

  @required
  name: TodolistName

  description: TodolistDescription

  visible_to_clients: Boolean
}

structure CreateTodolistOutput {

  todolist: Todolist
}

/// Replace a todolist (or todolist group) with a new complete representation.
/// The endpoint is polymorphic - it addresses either a Todolist or a TodolistGroup.
/// The request body is the recordable's full writable state: TodolistsController#update
/// builds a brand-new Todolist from the permitted params and swaps it in, so any
/// writable field omitted from the request is cleared server-side (a request that
/// omits description erases the description). name is required - it is
/// presence-validated on the model, so a request without it is rejected.
/// To set some fields while preserving the rest, use the SDK's merge-safe
/// update or edit methods, which GET the current list and PUT the full
/// representation back. Those read-modify-write helpers are not atomic:
/// a concurrent write between the GET and PUT is overwritten (last write
/// wins for the whole representation; the window is one round-trip).
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@basecampWriteSemantics(mode: "replace", clearsOmitted: true)
@http(method: "PUT", uri: "/{accountId}/todolists/{id}")
operation UpdateTodolistOrGroup {
  input: UpdateTodolistOrGroupInput
  output: UpdateTodolistOrGroupOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateTodolistOrGroupInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  id: Long

  /// Name (required for both Todolist and TodolistGroup) - presence-validated server-side, so omitting it is a 422, not a preserve
  @required
  name: TodolistName

  /// Description (rich text HTML) - writable for a todolist group as well as a todolist, and omitting it clears it either way
  description: TodolistDescription
}

structure UpdateTodolistOrGroupOutput {

  result: TodolistOrGroup
}

/// Reposition a to-do list within its to-do set.
/// position is the 1-based index among the to-do lists the caller can see; the server
/// translates it relative to loose to-dos and hidden completed lists. Shifts siblings.
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/todosets/todolists/{todolistId}/position.json", code: 204)
operation RepositionTodolist {
  input: RepositionTodolistInput
  output: RepositionTodolistOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure RepositionTodolistInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todolistId: TodolistId

  @required
  position: Integer
}

structure RepositionTodolistOutput {}

// ===== Todolist Group Operations =====
// Note: GetTodolistGroup and UpdateTodolistGroup are consolidated into
// GetTodolistOrGroup and UpdateTodolistOrGroup above (polymorphic endpoints)
// TrashTodolist and TrashTodolistGroup use generic TrashRecording operation

/// List groups in a todolist
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/todolists/{todolistId}/groups.json")
operation ListTodolistGroups {
  input: ListTodolistGroupsInput
  output: ListTodolistGroupsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListTodolistGroupsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todolistId: TodolistId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListTodolistGroupsOutput {

  groups: TodolistGroupList
}

/// Create a new group in a todolist
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/todolists/{todolistId}/groups.json", code: 201)
operation CreateTodolistGroup {
  input: CreateTodolistGroupInput
  output: CreateTodolistGroupOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateTodolistGroupInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  todolistId: TodolistId

  @required
  name: TodolistGroupName
}

structure CreateTodolistGroupOutput {

  group: TodolistGroup
}

/// Reposition a todolist group
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/todolists/groups/{groupId}/position.json")
operation RepositionTodolistGroup {
  input: RepositionTodolistGroupInput
  output: RepositionTodolistGroupOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure RepositionTodolistGroupInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  groupId: TodolistGroupId

  @required
  position: Integer
}

structure RepositionTodolistGroupOutput {}

// ===== Todo Shapes =====

long TodoId
long TodolistId
long PersonId
string TodoContent
string TodoDescription

@documentation("active|archived|trashed")
string TodoStatus

list TodoItems {
  member: Todo
}

list PersonIdList {
  member: PersonId
}

structure Todo {
  @required
  id: TodoId
  @required
  status: TodoStatus
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  comments_count: Integer
  comments_url: String
  position: Integer
  @required
  parent: TodoParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  description: TodoDescription
  completed: Boolean
  @required
  content: TodoContent
  @required
  description_attachments: RichTextAttachmentList
  starts_on: ISO8601Date
  due_on: ISO8601Date
  assignees: PersonList
  completion_subscribers: PersonList
  completion_url: String
  boosts_count: Integer
  boosts_url: String

  /// Steps embedded in the Todo response (BC5 addition). The shared
  /// `steps/step` jbuilder partial emits the same shape as `CardStep`,
  /// so the existing `CardStepList` is reused.
  steps: CardStepList
}

list RichTextAttachmentList {
  member: RichTextAttachment
}

/// Structured metadata for a downloadable file attachment embedded in a
/// rich text attribute. Every rich text attribute in an API response is
/// accompanied by a corresponding `*_attachments` array named after the
/// attribute (a Todo's `description_attachments` for its `description`).
/// Mentions, remote images, and opengraph embeds are excluded — only
/// downloadable file attachments appear.
structure RichTextAttachment {
  @required
  id: Long
  @required
  sgid: String
  @required
  filename: String
  @required
  content_type: String
  @required
  byte_size: Long
  @required
  @basecampAuthRoutableUrl
  download_url: String

  /// Pixel dimensions, present as keys on every attachment but null for
  /// non-image blobs, and the BC3 API may serialize them float-spelled
  /// (`1024.0`) — hence optional/nullable rather than `@required` (the enhance
  /// pass marks them `nullable: true` in the OpenAPI). All SDKs decode both
  /// forms faithfully and type the nullable value statically: Go `types.FlexInt`
  /// → `*int32`, Kotlin `Int?` via `FlexibleIntSerializer`, Swift `Int32?`,
  /// TypeScript `number | null`, Python `Optional[int | float]` (raw JSON keeps
  /// the float), Ruby nilable. See SPEC.md §10 Type Fidelity.
  width: Integer
  /// See `width` — same nullable/float-spelled behavior and cross-SDK note.
  height: Integer

  @required
  previewable: Boolean
  @required
  preview_url: String
  @required
  thumbnail_url: String
}

structure TodoParent {
  @required
  id: TodolistId
  @required
  title: String
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
}

structure TodoBucket {
  @required
  id: ProjectId
  @required
  name: String
  @required
  type: String
}

structure Person {
  @required
  id: PersonId
  attachable_sgid: String

  @required
  @basecampSensitive(category: "pii", redact: true)
  name: PersonName

  @basecampSensitive(category: "pii", redact: true)
  email_address: EmailAddress

  personable_type: String

  @basecampSensitive(category: "pii", redact: false)
  title: PersonTitle

  @basecampSensitive(category: "pii", redact: false)
  bio: PersonBio

  /// Alias of `bio` introduced in BC5. BC3 emits both keys with identical content;
  /// older BC4 responses may omit `tagline`. Prefer `bio` for cross-version reads.
  @basecampSensitive(category: "pii", redact: false)
  tagline: PersonBio

  @basecampSensitive(category: "pii", redact: false)
  location: PersonLocation

  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  admin: Boolean
  owner: Boolean
  client: Boolean
  employee: Boolean
  time_zone: String

  @basecampSensitive(category: "pii", redact: true)
  avatar_url: AvatarUrl

  company: PersonCompany
  can_manage_projects: Boolean
  can_manage_people: Boolean
  can_ping: Boolean
  can_access_timesheet: Boolean
  can_access_hill_charts: Boolean
}

structure PersonCompany {
  @required
  id: Long
  @required
  name: CompanyName
}

list PersonList {
  member: Person
}

// ===== Todoset Shapes =====

long TodosetId
string TodosetName

structure Todoset {
  @required
  id: TodosetId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  position: Integer
  @required
  bucket: TodoBucket
  @required
  creator: Person
  @required
  name: TodosetName
  todolists_count: Integer
  todolists_url: String
  completed_ratio: String
  completed: Boolean
  app_todolists_url: String

  /// Total count of todos across all todolists in this todoset (BC5 addition).
  todos_count: Integer

  /// Count of completed loose todos at the todoset level (BC5 addition).
  completed_loose_todos_count: Integer

  /// API URL for listing todos directly under this todoset (BC5 addition).
  todos_url: String

  /// In-app URL for viewing the todoset's todos (BC5 addition).
  app_todos_url: String
}

// ===== Todolist Shapes =====

string TodolistName
string TodolistDescription

@documentation("active|archived|trashed")
string TodolistStatus

list TodolistList {
  member: Todolist
}

structure Todolist {
  @required
  id: TodolistId
  @required
  status: TodolistStatus
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String

  /// URL of the Bubble Up record for this recording (BC5 addition). Required:
  /// `todolists/_todolist.json.jbuilder` renders the shared recording partial
  /// with `bubbleupable: true` unconditionally, and every list, show, and group
  /// path renders that partial — so the key is present on every projection of
  /// this shape.
  @required
  bubble_up_url: String
  comments_count: Integer
  comments_url: String
  position: Integer
  @required
  parent: TodoParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  description: TodolistDescription
  @required
  description_attachments: RichTextAttachmentList
  completed: Boolean
  completed_ratio: String
  @required
  name: TodolistName
  todos_url: String
  groups_url: String
  app_todos_url: String
  boosts_count: Integer
  boosts_url: String
}

// ===== Todolist Group Shapes =====

long TodolistGroupId
string TodolistGroupName

list TodolistGroupList {
  member: TodolistGroup
}

structure TodolistGroup {
  @required
  id: TodolistGroupId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String

  /// URL of the Bubble Up record for this recording (BC5 addition). Required:
  /// `todolists/_todolist.json.jbuilder` renders the shared recording partial
  /// with `bubbleupable: true` unconditionally, and every list, show, and group
  /// path renders that partial — so the key is present on every projection of
  /// this shape.
  @required
  bubble_up_url: String
  comments_count: Integer
  comments_url: String
  position: Integer
  @required
  parent: TodoParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  @required
  name: TodolistGroupName
  completed: Boolean
  completed_ratio: String
  todos_url: String
  app_todos_url: String
}

// ===== Comment Operations (Batch 1) =====

/// List comments on a recording
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/recordings/{recordingId}/comments.json")
operation ListComments {
  input: ListCommentsInput
  output: ListCommentsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListCommentsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListCommentsOutput {

  comments: CommentList
}

/// Get a single comment by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/comments/{commentId}")
operation GetComment {
  input: GetCommentInput
  output: GetCommentOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetCommentInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  commentId: CommentId
}

structure GetCommentOutput {

  comment: Comment
}

/// Create a new comment on a recording
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/recordings/{recordingId}/comments.json", code: 201)
operation CreateComment {
  input: CreateCommentInput
  output: CreateCommentOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateCommentInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId

  @required
  content: CommentContent
}

structure CreateCommentOutput {

  comment: Comment
}

/// Update an existing comment
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/comments/{commentId}")
operation UpdateComment {
  input: UpdateCommentInput
  output: UpdateCommentOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateCommentInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  commentId: CommentId

  @required
  content: CommentContent
}

structure UpdateCommentOutput {

  comment: Comment
}

// Note: Use TrashRecording to trash comments

// ===== Message Operations (Batch 1) =====

/// List messages on a message board
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/message_boards/{boardId}/messages.json")
operation ListMessages {
  input: ListMessagesInput
  output: ListMessagesOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListMessagesInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  boardId: MessageBoardId

  @httpQuery("sort")
  sort: RecordingSortField

  @httpQuery("direction")
  direction: SortDirection

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListMessagesOutput {

  messages: MessageList
}

/// Get a single message by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/messages/{messageId}")
operation GetMessage {
  input: GetMessageInput
  output: GetMessageOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetMessageInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  messageId: MessageId
}

structure GetMessageOutput {

  message: Message
}

/// Create a new message on a message board
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/message_boards/{boardId}/messages.json", code: 201)
operation CreateMessage {
  input: CreateMessageInput
  output: CreateMessageOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateMessageInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  boardId: MessageBoardId

  @required
  subject: MessageSubject

  content: MessageContent

  @documentation("active|drafted")
  status: String

  category_id: MessageTypeId

  subscriptions: PersonIdList

  visible_to_clients: Boolean
}

structure CreateMessageOutput {

  message: Message
}

/// Update an existing message
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/messages/{messageId}")
operation UpdateMessage {
  input: UpdateMessageInput
  output: UpdateMessageOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateMessageInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  messageId: MessageId

  subject: MessageSubject
  content: MessageContent

  @documentation("active|drafted")
  status: String

  category_id: MessageTypeId
}

structure UpdateMessageOutput {

  message: Message
}

/// Pin a message to the top of the message board
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/recordings/{messageId}/pin.json", code: 204)
operation PinMessage {
  input: PinMessageInput
  output: PinMessageOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure PinMessageInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  messageId: MessageId
}

structure PinMessageOutput {}

/// Unpin a message from the message board
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/recordings/{messageId}/pin.json", code: 204)
operation UnpinMessage {
  input: UnpinMessageInput
  output: UnpinMessageOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UnpinMessageInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  messageId: MessageId
}

structure UnpinMessageOutput {}

// Note: Use TrashRecording/ArchiveRecording/UnarchiveRecording for message lifecycle

// ===== Message Board Operations (Batch 1) =====

/// Get a message board
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/message_boards/{boardId}")
operation GetMessageBoard {
  input: GetMessageBoardInput
  output: GetMessageBoardOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetMessageBoardInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  boardId: MessageBoardId
}

structure GetMessageBoardOutput {

  message_board: MessageBoard
}

// ===== Message Type Operations (Batch 1) =====

/// List message types in a project
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/buckets/{bucketId}/categories.json")
operation ListMessageTypes {
  input: ListMessageTypesInput
  output: ListMessageTypesOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListMessageTypesInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId
}

structure ListMessageTypesOutput {

  message_types: MessageTypeList
}

/// Get a single message type by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{bucketId}/categories/{typeId}")
operation GetMessageType {
  input: GetMessageTypeInput
  output: GetMessageTypeOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetMessageTypeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  typeId: MessageTypeId
}

structure GetMessageTypeOutput {

  message_type: MessageType
}

/// Create a new message type in a project
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{bucketId}/categories.json", code: 201)
operation CreateMessageType {
  input: CreateMessageTypeInput
  output: CreateMessageTypeOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateMessageTypeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  name: MessageTypeName

  @required
  icon: MessageTypeIcon
}

structure CreateMessageTypeOutput {

  message_type: MessageType
}

/// Update an existing message type
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{bucketId}/categories/{typeId}")
operation UpdateMessageType {
  input: UpdateMessageTypeInput
  output: UpdateMessageTypeOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateMessageTypeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  typeId: MessageTypeId

  name: MessageTypeName
  icon: MessageTypeIcon
}

structure UpdateMessageTypeOutput {

  message_type: MessageType
}

/// Delete a message type
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/buckets/{bucketId}/categories/{typeId}", code: 204)
operation DeleteMessageType {
  input: DeleteMessageTypeInput
  output: DeleteMessageTypeOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure DeleteMessageTypeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  typeId: MessageTypeId
}

structure DeleteMessageTypeOutput {}

// ===== Vault Operations (Batch 2) =====

/// List vaults (subfolders) in a vault
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/vaults/{vaultId}/vaults.json")
operation ListVaults {
  input: ListVaultsInput
  output: ListVaultsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListVaultsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  vaultId: VaultId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListVaultsOutput {

  vaults: VaultList
}

/// Get a single vault by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/vaults/{vaultId}")
operation GetVault {
  input: GetVaultInput
  output: GetVaultOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetVaultInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  vaultId: VaultId
}

structure GetVaultOutput {

  vault: Vault
}

/// Create a new vault (subfolder) in a vault
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/vaults/{vaultId}/vaults.json", code: 201)
operation CreateVault {
  input: CreateVaultInput
  output: CreateVaultOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateVaultInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  vaultId: VaultId

  @required
  title: VaultTitle
}

structure CreateVaultOutput {

  vault: Vault
}

/// Update an existing vault
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/vaults/{vaultId}")
operation UpdateVault {
  input: UpdateVaultInput
  output: UpdateVaultOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateVaultInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  vaultId: VaultId

  title: VaultTitle
}

structure UpdateVaultOutput {

  vault: Vault
}

// ===== Document Operations (Batch 2) =====

/// List documents in a vault
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/vaults/{vaultId}/documents.json")
operation ListDocuments {
  input: ListDocumentsInput
  output: ListDocumentsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListDocumentsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  vaultId: VaultId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListDocumentsOutput {

  documents: DocumentList
}

/// Get a single document by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/documents/{documentId}")
operation GetDocument {
  input: GetDocumentInput
  output: GetDocumentOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetDocumentInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  documentId: DocumentId
}

structure GetDocumentOutput {

  document: Document
}

/// Create a new document in a vault
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/vaults/{vaultId}/documents.json", code: 201)
operation CreateDocument {
  input: CreateDocumentInput
  output: CreateDocumentOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateDocumentInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  vaultId: VaultId

  @required
  title: DocumentTitle

  content: DocumentContent

  @documentation("active|drafted")
  status: String

  subscriptions: PersonIdList

  visible_to_clients: Boolean
}

structure CreateDocumentOutput {

  document: Document
}

/// Replace a document with a new complete representation.
/// The request body is the document's full writable state: any writable field
/// omitted from the request is cleared server-side. Omitting content clears it;
/// omitting title clears it too, and the document then reads back as
/// "Untitled" (Document#title falls back when blank).
/// Neither field is required. BC3 builds a brand-new Document from the
/// permitted params and swaps the recordable wholesale, and neither attribute
/// carries a presence validation — so an omission is a 200 that clears, not a
/// 422. What BC3 does require is the wrapping document object, which Rails
/// synthesizes from a flat body, so a request naming neither field is a 400.
/// Publishing a draft (status: "active") is not modeled: the SDK sends only
/// title and content, and BC3 rejects a status-only update for the same
/// reason it 400s an empty body.
/// Subscribers are the one exception to omission-clears. A drafted document
/// keeps its current subscribers when the request addresses neither
/// subscriptions nor notify, so a full-representation PUT that mentions
/// neither is safe on a draft.
/// To set some fields while preserving the rest, use the SDK's merge-safe
/// update or edit methods, which GET the current document and PUT the full
/// representation back. Those read-modify-write helpers are not atomic:
/// a concurrent write between the GET and PUT is overwritten (last write
/// wins for the whole representation; the window is one round-trip).
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@basecampWriteSemantics(mode: "replace", clearsOmitted: true)
@http(method: "PUT", uri: "/{accountId}/documents/{documentId}")
operation ReplaceDocument {
  input: ReplaceDocumentInput
  output: ReplaceDocumentOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure ReplaceDocumentInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  documentId: DocumentId

  title: DocumentTitle
  content: DocumentContent
}

structure ReplaceDocumentOutput {

  document: Document
}

// Note: Use TrashRecording to trash documents

// ===== Upload Operations (Batch 2) =====

/// List uploads in a vault
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/vaults/{vaultId}/uploads.json")
operation ListUploads {
  input: ListUploadsInput
  output: ListUploadsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListUploadsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  vaultId: VaultId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListUploadsOutput {

  uploads: UploadList
}

/// Get a single upload by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/uploads/{uploadId}")
operation GetUpload {
  input: GetUploadInput
  output: GetUploadOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetUploadInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  uploadId: UploadId
}

structure GetUploadOutput {

  upload: Upload
}

/// Create a new upload in a vault
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/vaults/{vaultId}/uploads.json", code: 201)
operation CreateUpload {
  input: CreateUploadInput
  output: CreateUploadOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateUploadInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  vaultId: VaultId

  @required
  attachable_sgid: AttachableSgid

  description: UploadDescription
  base_name: UploadBaseName

  subscriptions: PersonIdList

  visible_to_clients: Boolean
}

structure CreateUploadOutput {

  upload: Upload
}

/// Update an existing upload
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/uploads/{uploadId}")
operation UpdateUpload {
  input: UpdateUploadInput
  output: UpdateUploadOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateUploadInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  uploadId: UploadId

  description: UploadDescription
  base_name: UploadBaseName
}

structure UpdateUploadOutput {

  upload: Upload
}

// Note: Use TrashRecording to trash uploads

/// List versions of an upload
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/uploads/{uploadId}/versions.json")
operation ListUploadVersions {
  input: ListUploadVersionsInput
  output: ListUploadVersionsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListUploadVersionsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  uploadId: UploadId
}

structure ListUploadVersionsOutput {

  uploads: UploadList
}

// ===== Attachment Operations (Batch 2) =====

/// Create an attachment (upload a file for embedding)
@basecampRetry(maxAttempts: 3, baseDelayMs: 2000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/attachments.json", code: 201)
operation CreateAttachment {
  input: CreateAttachmentInput
  output: CreateAttachmentOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateAttachmentInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpQuery("name")
  name: AttachmentFilename

  @required
  @httpPayload
  data: Blob
}

structure CreateAttachmentOutput {
  attachable_sgid: AttachableSgid
}

// ===== Schedule Operations (Batch 3) =====

/// Get a schedule
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/schedules/{scheduleId}")
operation GetSchedule {
  input: GetScheduleInput
  output: GetScheduleOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetScheduleInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  scheduleId: ScheduleId
}

structure GetScheduleOutput {

  schedule: Schedule
}

/// Update schedule settings
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/schedules/{scheduleId}")
operation UpdateScheduleSettings {
  input: UpdateScheduleSettingsInput
  output: UpdateScheduleSettingsOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateScheduleSettingsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  scheduleId: ScheduleId

  @required
  include_due_assignments: Boolean
}

structure UpdateScheduleSettingsOutput {

  schedule: Schedule
}

/// List entries on a schedule
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/schedules/{scheduleId}/entries.json")
operation ListScheduleEntries {
  input: ListScheduleEntriesInput
  output: ListScheduleEntriesOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListScheduleEntriesInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  scheduleId: ScheduleId

  @httpQuery("status")
  status: ScheduleEntryStatus

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListScheduleEntriesOutput {

  entries: ScheduleEntryList
}

/// Get a single schedule entry by id.
/// Note: Recurring entries will redirect (302) to their recordable URL.
/// Use GetScheduleEntryOccurrence for recurring entries instead.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/schedule_entries/{entryId}")
operation GetScheduleEntry {
  input: GetScheduleEntryInput
  output: GetScheduleEntryOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetScheduleEntryInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  entryId: ScheduleEntryId
}

structure GetScheduleEntryOutput {

  entry: ScheduleEntry
}

/// Get a specific occurrence of a recurring schedule entry
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/schedule_entries/{entryId}/occurrences/{date}")
operation GetScheduleEntryOccurrence {
  input: GetScheduleEntryOccurrenceInput
  output: GetScheduleEntryOccurrenceOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetScheduleEntryOccurrenceInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  entryId: ScheduleEntryId

  @required
  @httpLabel
  date: ISO8601Date
}

structure GetScheduleEntryOccurrenceOutput {

  entry: ScheduleEntry
}

/// Create a new schedule entry
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/schedules/{scheduleId}/entries.json", code: 201)
operation CreateScheduleEntry {
  input: CreateScheduleEntryInput
  output: CreateScheduleEntryOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateScheduleEntryInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  scheduleId: ScheduleId

  @required
  summary: ScheduleEntrySummary

  @required
  starts_at: ISO8601Timestamp

  @required
  ends_at: ISO8601Timestamp

  description: ScheduleEntryDescription
  participant_ids: PersonIdList
  all_day: Boolean
  notify: Boolean

  subscriptions: PersonIdList

  visible_to_clients: Boolean
}

structure CreateScheduleEntryOutput {

  entry: ScheduleEntry
}

/// Update an existing schedule entry
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/schedule_entries/{entryId}")
operation UpdateScheduleEntry {
  input: UpdateScheduleEntryInput
  output: UpdateScheduleEntryOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateScheduleEntryInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  entryId: ScheduleEntryId

  summary: ScheduleEntrySummary
  starts_at: ISO8601Timestamp
  ends_at: ISO8601Timestamp
  description: ScheduleEntryDescription
  /// Replaces the entry's participants.
  ///
  /// Omitting this member preserves the current participants; sending an empty
  /// array clears them. That guarantee is BC3-side and recent: until
  /// basecamp/bc3#12425, `Schedules::EntriesController#update` called
  /// `replace_participants` unconditionally, so any update omitting the key —
  /// including the shape in BC3's own "Update a schedule entry" doc example —
  /// silently removed every participant and notified each one. The controller
  /// now guards on the request actually addressing participants.
  participant_ids: PersonIdList
  all_day: Boolean
  notify: Boolean
}

structure UpdateScheduleEntryOutput {

  entry: ScheduleEntry
}

// Note: Use TrashRecording to trash schedule entries

// ===== Timesheet Operations (Batch 3) =====

/// Get account-wide timesheet report
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/reports/timesheet.json")
operation GetTimesheetReport {
  input: GetTimesheetReportInput
  output: GetTimesheetReportOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetTimesheetReportInput {
  @required
  @httpLabel
  accountId: AccountId

  @httpQuery("from")
  from: ISO8601Date

  @httpQuery("to")
  to: ISO8601Date

  @httpQuery("person_id")
  person_id: PersonId
}

structure GetTimesheetReportOutput {

  entries: TimesheetEntryList
}

/// Get timesheet for a specific project
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/projects/{projectId}/timesheet.json")
operation GetProjectTimesheet {
  input: GetProjectTimesheetInput
  output: GetProjectTimesheetOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetProjectTimesheetInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @httpQuery("from")
  from: ISO8601Date

  @httpQuery("to")
  to: ISO8601Date

  @httpQuery("person_id")
  person_id: PersonId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetProjectTimesheetOutput {

  entries: TimesheetEntryList
}

/// Get timesheet for a specific recording
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/recordings/{recordingId}/timesheet.json")
operation GetRecordingTimesheet {
  input: GetRecordingTimesheetInput
  output: GetRecordingTimesheetOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetRecordingTimesheetInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId

  @httpQuery("from")
  from: ISO8601Date

  @httpQuery("to")
  to: ISO8601Date

  @httpQuery("person_id")
  person_id: PersonId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetRecordingTimesheetOutput {

  entries: TimesheetEntryList
}

/// Get a single timesheet entry
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/timesheet_entries/{entryId}")
operation GetTimesheetEntry {
  input: GetTimesheetEntryInput
  output: GetTimesheetEntryOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetTimesheetEntryInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  entryId: TimesheetEntryId
}

structure GetTimesheetEntryOutput {
  entry: TimesheetEntry
}

/// Create a timesheet entry on a recording
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/recordings/{recordingId}/timesheet/entries.json", code: 201)
operation CreateTimesheetEntry {
  input: CreateTimesheetEntryInput
  output: CreateTimesheetEntryOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateTimesheetEntryInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId

  @required
  date: ISO8601Date

  @required
  hours: String

  description: String

  person_id: PersonId
}

structure CreateTimesheetEntryOutput {
  entry: TimesheetEntry
}

/// Update a timesheet entry
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/timesheet_entries/{entryId}")
operation UpdateTimesheetEntry {
  input: UpdateTimesheetEntryInput
  output: UpdateTimesheetEntryOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateTimesheetEntryInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  entryId: TimesheetEntryId

  date: ISO8601Date

  hours: String

  description: String

  person_id: PersonId
}

structure UpdateTimesheetEntryOutput {
  entry: TimesheetEntry
}

// Note: Use TrashRecording to trash timesheet entries

// ===== Comment Shapes (Batch 1) =====

long CommentId
long RecordingId
string CommentContent

list CommentList {
  member: Comment
}

structure Comment {
  @required
  id: CommentId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  @required
  content: CommentContent
  @required
  content_attachments: RichTextAttachmentList
  boosts_count: Integer
  boosts_url: String
}

structure RecordingParent {
  @required
  id: Long
  @required
  title: String
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  // Optional project context. Absent for a recording's `parent` reference (same
  // bucket as the recording); populated when a boost feed embeds the boosted
  // recording (my/boosts) so callers can identify its project.
  bucket: RecordingBucket
}

// ===== Message Shapes (Batch 1) =====

long MessageId
long MessageBoardId
long MessageTypeId
string MessageSubject
string MessageContent
string MessageTypeName
string MessageTypeIcon

list MessageList {
  member: Message
}

structure Message {
  @required
  id: MessageId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  comments_count: Integer
  comments_url: String
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  @required
  subject: MessageSubject
  @required
  content: MessageContent
  @required
  content_attachments: RichTextAttachmentList
  category: MessageType
  boosts_count: Integer
  boosts_url: String
}

structure MessageBoard {
  @required
  id: MessageBoardId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  position: Integer
  @required
  bucket: TodoBucket
  @required
  creator: Person
  messages_count: Integer
  messages_url: String
  app_messages_url: String
}

list MessageTypeList {
  member: MessageType
}

structure MessageType {
  @required
  id: MessageTypeId
  @required
  name: MessageTypeName
  @required
  icon: MessageTypeIcon
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
}

// ===== Vault Shapes (Batch 2) =====

long VaultId
string VaultTitle

list VaultList {
  member: Vault
}

structure Vault {
  @required
  id: VaultId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: VaultTitle
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  position: Integer
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  documents_count: Integer
  documents_url: String
  uploads_count: Integer
  uploads_url: String
  vaults_count: Integer
  vaults_url: String
}

// ===== Document Shapes (Batch 2) =====

long DocumentId
string DocumentTitle
string DocumentContent

list DocumentList {
  member: Document
}

structure Document {
  @required
  id: DocumentId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: DocumentTitle
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  comments_count: Integer
  comments_url: String
  position: Integer
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  content: DocumentContent
  @required
  content_attachments: RichTextAttachmentList
  boosts_count: Integer
  boosts_url: String
}

// ===== Upload Shapes (Batch 2) =====

long UploadId
string UploadDescription
string UploadBaseName
string AttachableSgid
string AttachmentFilename

list UploadList {
  member: Upload
}

structure Upload {
  @required
  id: UploadId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  comments_count: Integer
  comments_url: String
  position: Integer
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  description: UploadDescription
  @required
  description_attachments: RichTextAttachmentList
  content_type: String
  byte_size: Long
  width: Integer
  height: Integer
  @basecampAuthRoutableUrl
  download_url: String
  filename: String
  boosts_count: Integer
  boosts_url: String
}

// ===== Schedule Shapes (Batch 3) =====

long ScheduleId
long ScheduleEntryId
string ScheduleEntrySummary
string ScheduleEntryDescription

@documentation("active|archived|trashed")
string ScheduleEntryStatus

structure Schedule {
  @required
  id: ScheduleId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  position: Integer
  @required
  bucket: TodoBucket
  @required
  creator: Person
  include_due_assignments: Boolean
  entries_count: Integer
  entries_url: String
}

list ScheduleEntryList {
  member: ScheduleEntry
}

structure ScheduleEntry {
  @required
  id: ScheduleEntryId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  comments_count: Integer
  comments_url: String
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  @required
  summary: ScheduleEntrySummary
  description: ScheduleEntryDescription
  @required
  description_attachments: RichTextAttachmentList
  all_day: Boolean
  starts_at: ISO8601Timestamp
  ends_at: ISO8601Timestamp
  participants: PersonList
  boosts_count: Integer
  boosts_url: String
}

// ===== Timesheet Shapes (Batch 3) =====

long TimesheetEntryId

list TimesheetEntryList {
  member: TimesheetEntry
}

structure TimesheetEntry {
  @required
  id: TimesheetEntryId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  date: ISO8601Date
  description: String
  hours: String

  /// The person the time is logged for (distinct from creator)
  person: Person
}

// =============================================================================
// BATCH 4: Campfires, Chatbots, Forwards/Inboxes (Real-time)
// =============================================================================

// ===== Campfire Operations =====

/// List all campfires across the account
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/chats.json")
operation ListCampfires {
  input: ListCampfiresInput
  output: ListCampfiresOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListCampfiresInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListCampfiresOutput {

  campfires: CampfireList
}

/// Get a campfire by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/chats/{campfireId}")
operation GetCampfire {
  input: GetCampfireInput
  output: GetCampfireOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetCampfireInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  campfireId: CampfireId
}

structure GetCampfireOutput {

  campfire: Campfire
}

/// List all lines (messages) in a campfire
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/chats/{campfireId}/lines.json")
operation ListCampfireLines {
  input: ListCampfireLinesInput
  output: ListCampfireLinesOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListCampfireLinesInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  campfireId: CampfireId

  @httpQuery("sort")
  sort: RecordingSortField

  @httpQuery("direction")
  direction: SortDirection

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListCampfireLinesOutput {

  lines: CampfireLineList
}

/// Get a campfire line by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/chats/{campfireId}/lines/{lineId}")
operation GetCampfireLine {
  input: GetCampfireLineInput
  output: GetCampfireLineOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetCampfireLineInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  campfireId: CampfireId

  @required
  @httpLabel
  lineId: CampfireLineId
}

structure GetCampfireLineOutput {

  line: CampfireLine
}

/// Create a new line (message) in a campfire
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/chats/{campfireId}/lines.json", code: 201)
operation CreateCampfireLine {
  input: CreateCampfireLineInput
  output: CreateCampfireLineOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateCampfireLineInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  campfireId: CampfireId

  @required
  content: String

  content_type: String
}

structure CreateCampfireLineOutput {

  line: CampfireLine
}

/// Update an existing campfire line; the content is always treated as rich text (HTML).
/// The server coerces every edited line to rich text and ignores any content
/// type hint. Only the line's creator may edit it, and only text and
/// rich-text lines are editable.
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/chats/{campfireId}/lines/{lineId}", code: 204)
operation UpdateCampfireLine {
  input: UpdateCampfireLineInput
  output: UpdateCampfireLineOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure UpdateCampfireLineInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  campfireId: CampfireId

  @required
  @httpLabel
  lineId: CampfireLineId

  /// The new line content, interpreted as rich text (HTML)
  @required
  content: String
}

structure UpdateCampfireLineOutput {}

/// Delete a campfire line; allowed for the line's creator or an admin.
/// The API responds 403 Forbidden otherwise.
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/chats/{campfireId}/lines/{lineId}", code: 204)
operation DeleteCampfireLine {
  input: DeleteCampfireLineInput
  output: DeleteCampfireLineOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure DeleteCampfireLineInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  campfireId: CampfireId

  @required
  @httpLabel
  lineId: CampfireLineId
}

structure DeleteCampfireLineOutput {}

// ===== Campfire Upload Operations =====

/// List uploaded files in a campfire
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/chats/{campfireId}/uploads.json")
operation ListCampfireUploads {
  input: ListCampfireUploadsInput
  output: ListCampfireUploadsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListCampfireUploadsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  campfireId: CampfireId

  @httpQuery("sort")
  sort: RecordingSortField

  @httpQuery("direction")
  direction: SortDirection

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListCampfireUploadsOutput {

  uploads: CampfireLineList
}

/// Upload a file to a campfire
@basecampRetry(maxAttempts: 3, baseDelayMs: 2000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/chats/{campfireId}/uploads.json", code: 201)
operation CreateCampfireUpload {
  input: CreateCampfireUploadInput
  output: CreateCampfireUploadOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateCampfireUploadInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  campfireId: CampfireId

  /// Filename for the uploaded file (e.g. "report.pdf").
  @required
  @httpQuery("name")
  name: String

  /// Raw binary content of the file. Set the Content-Type header to match
  /// the file's media type (e.g. "image/png", "application/pdf").
  @required
  @httpPayload
  data: Blob
}

structure CreateCampfireUploadOutput {

  upload: CampfireLine
}

// ===== Chatbot Operations =====

/// List all chatbots for a campfire
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/buckets/{bucketId}/chats/{campfireId}/integrations.json")
operation ListChatbots {
  input: ListChatbotsInput
  output: ListChatbotsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListChatbotsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  campfireId: CampfireId
}

structure ListChatbotsOutput {

  chatbots: ChatbotList
}

/// Get a chatbot by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{bucketId}/chats/{campfireId}/integrations/{chatbotId}")
operation GetChatbot {
  input: GetChatbotInput
  output: GetChatbotOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetChatbotInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  campfireId: CampfireId

  @required
  @httpLabel
  chatbotId: ChatbotId
}

structure GetChatbotOutput {

  chatbot: Chatbot
}

/// Create a new chatbot for a campfire
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{bucketId}/chats/{campfireId}/integrations.json", code: 201)
operation CreateChatbot {
  input: CreateChatbotInput
  output: CreateChatbotOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateChatbotInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  campfireId: CampfireId

  @required
  service_name: String

  command_url: String
}

structure CreateChatbotOutput {

  chatbot: Chatbot
}

/// Update an existing chatbot
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{bucketId}/chats/{campfireId}/integrations/{chatbotId}")
operation UpdateChatbot {
  input: UpdateChatbotInput
  output: UpdateChatbotOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateChatbotInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  campfireId: CampfireId

  @required
  @httpLabel
  chatbotId: ChatbotId

  @required
  service_name: String

  command_url: String
}

structure UpdateChatbotOutput {

  chatbot: Chatbot
}

/// Delete a chatbot
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/buckets/{bucketId}/chats/{campfireId}/integrations/{chatbotId}", code: 204)
operation DeleteChatbot {
  input: DeleteChatbotInput
  output: DeleteChatbotOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure DeleteChatbotInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  campfireId: CampfireId

  @required
  @httpLabel
  chatbotId: ChatbotId
}

structure DeleteChatbotOutput {}

// ===== Inbox Operations =====

/// Get an inbox by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/inboxes/{inboxId}")
operation GetInbox {
  input: GetInboxInput
  output: GetInboxOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetInboxInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  inboxId: InboxId
}

structure GetInboxOutput {

  inbox: Inbox
}

// ===== Forward Operations =====

/// List all forwards in an inbox
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/inboxes/{inboxId}/inbox_forwards.json")
operation ListForwards {
  input: ListForwardsInput
  output: ListForwardsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListForwardsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  inboxId: InboxId

  @httpQuery("sort")
  sort: RecordingSortField

  @httpQuery("direction")
  direction: SortDirection

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListForwardsOutput {

  forwards: ForwardList
}

/// Get a forward by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/inbox_forwards/{forwardId}")
operation GetForward {
  input: GetForwardInput
  output: GetForwardOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetForwardInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  forwardId: ForwardId
}

structure GetForwardOutput {

  forward: Forward
}

/// List all replies to a forward
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/inbox_forwards/{forwardId}/replies.json")
operation ListForwardReplies {
  input: ListForwardRepliesInput
  output: ListForwardRepliesOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListForwardRepliesInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  forwardId: ForwardId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListForwardRepliesOutput {

  replies: ForwardReplyList
}

/// Get a forward reply by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/inbox_forwards/{forwardId}/replies/{replyId}")
operation GetForwardReply {
  input: GetForwardReplyInput
  output: GetForwardReplyOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetForwardReplyInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  forwardId: ForwardId

  @required
  @httpLabel
  replyId: ForwardReplyId
}

structure GetForwardReplyOutput {

  reply: ForwardReply
}

// ===== Campfire Shapes =====

long CampfireId
long CampfireLineId
long ChatbotId

list CampfireList {
  member: Campfire
}

structure Campfire {
  @required
  id: CampfireId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  position: Integer
  @required
  bucket: TodoBucket
  @required
  creator: Person
  topic: String
  lines_url: String
  files_url: String
}

list CampfireLineList {
  member: CampfireLine
}

structure CampfireLine {
  @required
  id: CampfireLineId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  content: String
  attachments: CampfireLineAttachmentList
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  boosts_count: Integer
  boosts_url: String
}

list CampfireLineAttachmentList {
  member: CampfireLineAttachment
}

structure CampfireLineAttachment {
  title: String
  url: String
  filename: String
  content_type: String
  byte_size: Long
  @basecampAuthRoutableUrl
  download_url: String
}

list ChatbotList {
  member: Chatbot
}

structure Chatbot {
  @required
  id: ChatbotId
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  service_name: String
  command_url: String
  url: String
  app_url: String
  lines_url: String
}

// ===== Inbox/Forward Shapes =====

long InboxId
long ForwardId
long ForwardReplyId

structure Inbox {
  @required
  id: InboxId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  position: Integer
  @required
  bucket: TodoBucket
  @required
  creator: Person
  forwards_count: Integer
  forwards_url: String
}

list ForwardList {
  member: Forward
}

structure Forward {
  @required
  id: ForwardId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  content: String
  @required
  content_attachments: RichTextAttachmentList
  @required
  subject: String
  from: String
  replies_count: Integer
  replies_url: String
}

list ForwardReplyList {
  member: ForwardReply
}

structure ForwardReply {
  @required
  id: ForwardReplyId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  @required
  content: String
  @required
  content_attachments: RichTextAttachmentList
  boosts_count: Integer
  boosts_url: String
}

// =============================================================================
// BATCH 5: CardTables, Cards, CardColumns, CardSteps (Kanban)
// =============================================================================

// ===== CardTable Operations =====

/// Get a card table by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/card_tables/{cardTableId}")
operation GetCardTable {
  input: GetCardTableInput
  output: GetCardTableOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetCardTableInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  cardTableId: CardTableId
}

structure GetCardTableOutput {

  card_table: CardTable
}

// ===== Card Operations =====

/// List cards in a column
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/card_tables/lists/{columnId}/cards.json")
operation ListCards {
  input: ListCardsInput
  output: ListCardsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListCardsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  columnId: CardColumnId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListCardsOutput {

  cards: CardList
}

/// Get a card by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/card_tables/cards/{cardId}")
operation GetCard {
  input: GetCardInput
  output: GetCardOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetCardInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  cardId: CardId
}

structure GetCardOutput {

  card: Card
}

/// Create a card in a column
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/card_tables/lists/{columnId}/cards.json", code: 201)
operation CreateCard {
  input: CreateCardInput
  output: CreateCardOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateCardInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  columnId: CardColumnId

  @required
  title: String

  content: String
  due_on: ISO8601Date
  notify: Boolean
}

structure CreateCardOutput {

  card: Card
}

/// Update an existing card
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/card_tables/cards/{cardId}")
operation UpdateCard {
  input: UpdateCardInput
  output: UpdateCardOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateCardInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  cardId: CardId

  title: String
  content: String
  due_on: ISO8601Date
  assignee_ids: PersonIdList
}

structure UpdateCardOutput {

  card: Card
}

/// Move a card to a different column
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/card_tables/cards/{cardId}/moves.json", code: 204)
operation MoveCard {
  input: MoveCardInput
  output: MoveCardOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure MoveCardInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  cardId: CardId

  @required
  column_id: CardColumnId

  /// 1-indexed position within the destination column. Defaults to 1 (top).
  position: Integer
}

structure MoveCardOutput {}

// Note: Use TrashRecording to trash cards

// ===== CardColumn Operations =====

/// Get a card column by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/card_tables/columns/{columnId}")
operation GetCardColumn {
  input: GetCardColumnInput
  output: GetCardColumnOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetCardColumnInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure GetCardColumnOutput {

  column: CardColumn
}

/// Create a column in a card table
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/card_tables/{cardTableId}/columns.json", code: 201)
operation CreateCardColumn {
  input: CreateCardColumnInput
  output: CreateCardColumnOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateCardColumnInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  cardTableId: CardTableId

  @required
  title: String

  description: String
}

structure CreateCardColumnOutput {

  column: CardColumn
}

/// Update an existing column
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/card_tables/columns/{columnId}")
operation UpdateCardColumn {
  input: UpdateCardColumnInput
  output: UpdateCardColumnOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateCardColumnInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  columnId: CardColumnId

  title: String
  description: String
}

structure UpdateCardColumnOutput {

  column: CardColumn
}

/// Move a column within a card table
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/card_tables/{cardTableId}/moves.json", code: 204)
operation MoveCardColumn {
  input: MoveCardColumnInput
  output: MoveCardColumnOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure MoveCardColumnInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  cardTableId: CardTableId

  @required
  source_id: CardColumnId

  @required
  target_id: CardColumnId

  position: Integer
}

structure MoveCardColumnOutput {}

/// Set the color of a column
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{bucketId}/card_tables/columns/{columnId}/color.json")
operation SetCardColumnColor {
  input: SetCardColumnColorInput
  output: SetCardColumnColorOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure SetCardColumnColorInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId

  @required
  @documentation("Valid colors: white, red, orange, yellow, green, blue, aqua, purple, gray, pink, brown")
  color: String
}

structure SetCardColumnColorOutput {

  column: CardColumn
}

/// Enable on-hold section in a column
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "POST", uri: "/{accountId}/buckets/{bucketId}/card_tables/columns/{columnId}/on_hold.json")
operation EnableCardColumnOnHold {
  input: EnableCardColumnOnHoldInput
  output: EnableCardColumnOnHoldOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure EnableCardColumnOnHoldInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure EnableCardColumnOnHoldOutput {

  column: CardColumn
}

/// Disable on-hold section in a column
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/buckets/{bucketId}/card_tables/columns/{columnId}/on_hold.json")
operation DisableCardColumnOnHold {
  input: DisableCardColumnOnHoldInput
  output: DisableCardColumnOnHoldOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure DisableCardColumnOnHoldInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure DisableCardColumnOnHoldOutput {

  column: CardColumn
}

/// Subscribe to a card column (watch for changes)
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "POST", uri: "/{accountId}/card_tables/lists/{columnId}/subscription.json", code: 204)
operation SubscribeToCardColumn {
  input: SubscribeToCardColumnInput
  output: SubscribeToCardColumnOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure SubscribeToCardColumnInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure SubscribeToCardColumnOutput {}

/// Unsubscribe from a card column (stop watching for changes)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/card_tables/lists/{columnId}/subscription.json", code: 204)
operation UnsubscribeFromCardColumn {
  input: UnsubscribeFromCardColumnInput
  output: UnsubscribeFromCardColumnOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UnsubscribeFromCardColumnInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure UnsubscribeFromCardColumnOutput {}

// ===== CardStep Operations =====

/// Get a step by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/card_tables/steps/{stepId}")
operation GetCardStep {
  input: GetCardStepInput
  output: GetCardStepOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetCardStepInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  stepId: CardStepId
}

structure GetCardStepOutput {
  step: CardStep
}

/// Create a step on a card
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/card_tables/cards/{cardId}/steps.json", code: 201)
operation CreateCardStep {
  input: CreateCardStepInput
  output: CreateCardStepOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateCardStepInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  cardId: CardId

  @required
  title: String

  due_on: ISO8601Date
  assignee_ids: PersonIdList
}

structure CreateCardStepOutput {

  step: CardStep
}

/// Update an existing step
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/card_tables/steps/{stepId}")
operation UpdateCardStep {
  input: UpdateCardStepInput
  output: UpdateCardStepOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateCardStepInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  stepId: CardStepId

  title: String
  due_on: ISO8601Date
  assignee_ids: PersonIdList
}

structure UpdateCardStepOutput {

  step: CardStep
}

/// Set card step completion status (PUT with completion: "on" to complete, "" to uncomplete)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/card_tables/steps/{stepId}/completions.json")
operation SetCardStepCompletion {
  input: SetCardStepCompletionInput
  output: SetCardStepCompletionOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure SetCardStepCompletionInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  stepId: CardStepId

  /// Set to "on" to complete the step, "" (empty) to uncomplete
  @required
  completion: String
}

structure SetCardStepCompletionOutput {

  step: CardStep
}

/// Reposition a step within a card
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/card_tables/cards/{cardId}/positions.json")
operation RepositionCardStep {
  input: RepositionCardStepInput
  output: RepositionCardStepOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure RepositionCardStepInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  cardId: CardId

  @required
  source_id: CardStepId

  @required
  @documentation("0-indexed position")
  position: Integer
}

structure RepositionCardStepOutput {}

// Note: Use TrashRecording to delete card steps

// ===== Wormhole Operations =====

/// Create a wormhole linking this card table to a column on another card table.
///
/// A wormhole is the only mechanism for moving a card to a different project: its
/// id is a valid `column_id` for MoveCard, teleporting the card across projects.
/// `destinationRecordingId` is the id of a column on another accessible card table.
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{bucketId}/card_tables/{cardTableId}/wormholes.json", code: 201)
operation CreateWormhole {
  input: CreateWormholeInput
  output: CreateWormholeOutput
  errors: [ValidationError, NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateWormholeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  cardTableId: CardTableId

  /// Id of the destination column (on another accessible card table) to link to.
  @required
  destination_recording_id: CardColumnId
}

structure CreateWormholeOutput {

  wormhole: Wormhole
}

/// Update a wormhole's destination column
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{bucketId}/card_tables/wormholes/{wormholeId}")
operation UpdateWormhole {
  input: UpdateWormholeInput
  output: UpdateWormholeOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateWormholeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  wormholeId: WormholeId

  /// Id of the new destination column (on another accessible card table).
  @required
  destination_recording_id: CardColumnId
}

structure UpdateWormholeOutput {

  wormhole: Wormhole
}

/// Delete a wormhole
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/buckets/{bucketId}/card_tables/wormholes/{wormholeId}", code: 204)
operation DeleteWormhole {
  input: DeleteWormholeInput
  output: DeleteWormholeOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure DeleteWormholeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  wormholeId: WormholeId
}

structure DeleteWormholeOutput {}

// ===== CardTable Shapes =====

long CardTableId
long CardId
long CardColumnId
long CardStepId
long WormholeId

structure CardTable {
  @required
  id: CardTableId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  @required
  bucket: TodoBucket
  @required
  creator: Person
  subscribers: PersonList
  lists: CardColumnList
  wormholes: WormholeList
}

list CardColumnList {
  member: CardColumn
}

list WormholeList {
  member: Wormhole
}

/// A wormhole links this card table to a column on another card table, enabling
/// cards to move across projects. It carries the full recording representation
/// plus the destination-linkage fields. The wormhole's own `url`/`app_url`/`parent`
/// point at the *source* board; `destination_url` is the only field identifying
/// the destination column.
structure Wormhole {
  @required
  id: WormholeId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  /// Wormhole color; always emitted on the wire (`json.color recording.color`),
  /// `null` when unset. Like destination_url, `@required` models the presence and
  /// the nullability is layered on in the OpenAPI (smithy-build.json jsonAdd ->
  /// type: ["string","null"]). Go types it *string because the value is nullable,
  /// not because it is optional — the field is @required and carries no omitempty.
  @required
  color: String
  /// True only while the destination column, its board, and its bucket are all
  /// active; false once the destination is unlinked. Always emitted.
  @required
  linked: Boolean
  /// URL of the destination column; always present on the wire, `null` for an
  /// unlinked wormhole. `@required` models the presence; the nullability of the
  /// value is layered on in the OpenAPI (smithy-build.json jsonAdd -> type:
  /// ["string","null"]) since Smithy has no native
  /// required-and-nullable — exactly the SearchType.key treatment. SDKs model it
  /// as required-but-nullable (`string | null`, not `string | null | undefined`).
  @required
  destination_url: String
}

structure CardColumn {
  @required
  id: CardColumnId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  position: Integer
  color: String
  description: String
  cards_count: Integer
  comments_count: Integer
  cards_url: String
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  subscribers: PersonList
  on_hold: CardColumnOnHold
}

structure CardColumnOnHold {
  @required
  id: RecordingId
  @required
  status: String
  @required
  inherits_status: Boolean
  @required
  title: String
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  cards_count: Integer
  @required
  cards_url: String
}

list CardList {
  member: Card
}

structure Card {
  @required
  id: CardId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  position: Integer
  content: String
  description: String
  @required
  description_attachments: RichTextAttachmentList
  due_on: ISO8601Date
  completed: Boolean
  completed_at: ISO8601Timestamp
  comments_count: Integer
  comments_url: String
  completion_url: String
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  completer: Person
  assignees: PersonList
  completion_subscribers: PersonList
  steps: CardStepList
  boosts_count: Integer
  boosts_url: String
}

list CardStepList {
  member: CardStep
}

structure CardStep {
  @required
  id: CardStepId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  position: Integer
  due_on: ISO8601Date
  completed: Boolean
  completed_at: ISO8601Timestamp
  @required
  parent: RecordingParent
  @required
  bucket: TodoBucket
  @required
  creator: Person
  completer: Person
  assignees: PersonList
  completion_url: String
}

// =============================================================================
// BATCH 6: People, Subscriptions (People & Access)
// =============================================================================

// ===== People Operations =====

/// List all people visible to the current user
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/people.json")
operation ListPeople {
  input: ListPeopleInput
  output: ListPeopleOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListPeopleInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListPeopleOutput {

  people: PersonList
}

/// Get a person by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/people/{personId}")
operation GetPerson {
  input: GetPersonInput
  output: GetPersonOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetPersonInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  personId: PersonId
}

structure GetPersonOutput {

  person: Person
}

/// Get the current authenticated user's profile
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/my/profile.json")
operation GetMyProfile {
  input: GetMyProfileInput
  output: GetMyProfileOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetMyProfileInput {
  @required
  @httpLabel
  accountId: AccountId
}

structure GetMyProfileOutput {

  person: Person
}

/// Update the current authenticated user's profile (returns 204 No Content)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/my/profile.json", code: 204)
operation UpdateMyProfile {
  input: UpdateMyProfileInput
  output: UpdateMyProfileOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateMyProfileInput {
  @required
  @httpLabel
  accountId: AccountId

  name: PersonName
  email_address: EmailAddress
  title: PersonTitle
  bio: PersonBio
  location: PersonLocation
  time_zone_name: String
  first_week_day: FirstWeekDay
  time_format: String
}

enum FirstWeekDay {
  SUNDAY = "Sunday"
  MONDAY = "Monday"
  TUESDAY = "Tuesday"
  WEDNESDAY = "Wednesday"
  THURSDAY = "Thursday"
  FRIDAY = "Friday"
  SATURDAY = "Saturday"
}

structure UpdateMyProfileOutput {}

/// List all active people on a project
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/projects/{projectId}/people.json")
operation ListProjectPeople {
  input: ListProjectPeopleInput
  output: ListProjectPeopleOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListProjectPeopleInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListProjectPeopleOutput {

  people: PersonList
}

/// List all account users who can be pinged
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/circles/people.json")
operation ListPingablePeople {
  input: ListPingablePeopleInput
  output: ListPingablePeopleOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListPingablePeopleInput {
  @required
  @httpLabel
  accountId: AccountId
}

structure ListPingablePeopleOutput {

  people: PersonList
}

/// Update project access (grant/revoke/create people)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/projects/{projectId}/people/users.json")
operation UpdateProjectAccess {
  input: UpdateProjectAccessInput
  output: UpdateProjectAccessOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure UpdateProjectAccessInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  grant: PersonIdList
  revoke: PersonIdList
  create: CreatePersonRequestList
}

list CreatePersonRequestList {
  member: CreatePersonRequest
}

structure CreatePersonRequest {
  @required
  name: PersonName

  @required
  email_address: EmailAddress

  title: PersonTitle
  company_name: CompanyName
}

structure UpdateProjectAccessOutput {

  result: ProjectAccessResult
}

structure ProjectAccessResult {
  granted: PersonList
  revoked: PersonList
}

// ===== Subscription Operations =====

/// Get subscription information for a recording
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/recordings/{recordingId}/subscription.json")
operation GetSubscription {
  input: GetSubscriptionInput
  output: GetSubscriptionOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetSubscriptionInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure GetSubscriptionOutput {

  subscription: Subscription
}

/// Subscribe the current user to a recording
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "POST", uri: "/{accountId}/recordings/{recordingId}/subscription.json")
operation Subscribe {
  input: SubscribeInput
  output: SubscribeOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure SubscribeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure SubscribeOutput {

  subscription: Subscription
}

/// Unsubscribe the current user from a recording
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/recordings/{recordingId}/subscription.json", code: 204)
operation Unsubscribe {
  input: UnsubscribeInput
  output: UnsubscribeOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UnsubscribeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure UnsubscribeOutput {}

/// Update subscriptions by adding or removing specific users
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/recordings/{recordingId}/subscription.json")
operation UpdateSubscription {
  input: UpdateSubscriptionInput
  output: UpdateSubscriptionOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateSubscriptionInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId

  subscriptions: PersonIdList
  unsubscriptions: PersonIdList
}

structure UpdateSubscriptionOutput {

  subscription: Subscription
}

// ===== Subscription Shapes =====

structure Subscription {
  @required
  subscribed: Boolean
  @required
  count: Integer
  @required
  url: String
  subscribers: PersonList
}

// =============================================================================
// BATCH 7 - Client Features (ClientApprovals, ClientCorrespondences, ClientReplies)
// =============================================================================

// ===== Client Approval Operations =====

/// List all client approvals in a project
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/buckets/{bucketId}/client/approvals.json")
operation ListClientApprovals {
  input: ListClientApprovalsInput
  output: ListClientApprovalsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListClientApprovalsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @httpQuery("sort")
  sort: RecordingSortField

  @httpQuery("direction")
  direction: SortDirection

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListClientApprovalsOutput {

  approvals: ClientApprovalList
}

/// Get a single client approval by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/client/approvals/{approvalId}")
operation GetClientApproval {
  input: GetClientApprovalInput
  output: GetClientApprovalOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetClientApprovalInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  approvalId: ClientApprovalId
}

structure GetClientApprovalOutput {

  approval: ClientApproval
}

// ===== Client Correspondence Operations =====

/// List all client correspondences in a project
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/buckets/{bucketId}/client/correspondences.json")
operation ListClientCorrespondences {
  input: ListClientCorrespondencesInput
  output: ListClientCorrespondencesOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListClientCorrespondencesInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @httpQuery("sort")
  sort: RecordingSortField

  @httpQuery("direction")
  direction: SortDirection

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListClientCorrespondencesOutput {

  correspondences: ClientCorrespondenceList
}

/// Get a single client correspondence by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/client/correspondences/{correspondenceId}")
operation GetClientCorrespondence {
  input: GetClientCorrespondenceInput
  output: GetClientCorrespondenceOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetClientCorrespondenceInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  correspondenceId: ClientCorrespondenceId
}

structure GetClientCorrespondenceOutput {

  correspondence: ClientCorrespondence
}

// ===== Client Reply Operations =====

/// List all client replies for a recording (correspondence or approval)
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/buckets/{bucketId}/client/recordings/{recordingId}/replies.json")
operation ListClientReplies {
  input: ListClientRepliesInput
  output: ListClientRepliesOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListClientRepliesInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListClientRepliesOutput {

  replies: ClientReplyList
}

/// Get a single client reply by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{bucketId}/client/recordings/{recordingId}/replies/{replyId}")
operation GetClientReply {
  input: GetClientReplyInput
  output: GetClientReplyOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetClientReplyInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId

  @required
  @httpLabel
  replyId: ClientReplyId
}

structure GetClientReplyOutput {

  reply: ClientReply
}

// ===== Client Feature Shapes =====

long ClientApprovalId
long ClientCorrespondenceId
long ClientReplyId

list ClientApprovalList {
  member: ClientApproval
}

structure ClientApproval {
  @required
  id: ClientApprovalId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  @required
  parent: RecordingParent
  @required
  bucket: RecordingBucket
  @required
  creator: Person
  content: String
  @required
  content_attachments: RichTextAttachmentList
  subject: String
  due_on: ISO8601Date
  replies_count: Integer
  replies_url: String
  approval_status: String
  approver: Person
  responses: ClientApprovalResponseList
}

list ClientApprovalResponseList {
  member: ClientApprovalResponse
}

structure ClientApprovalResponse {
  id: Long
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  app_url: String
  bookmark_url: String
  parent: RecordingParent
  bucket: RecordingBucket
  creator: Person
  content: String
  approved: Boolean
}

list ClientCorrespondenceList {
  member: ClientCorrespondence
}

structure ClientCorrespondence {
  @required
  id: ClientCorrespondenceId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  @required
  parent: RecordingParent
  @required
  bucket: RecordingBucket
  @required
  creator: Person
  content: String
  @required
  content_attachments: RichTextAttachmentList
  @required
  subject: String
  replies_count: Integer
  replies_url: String
}

list ClientReplyList {
  member: ClientReply
}

structure ClientReply {
  @required
  id: ClientReplyId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  @required
  parent: RecordingParent
  @required
  bucket: RecordingBucket
  @required
  creator: Person
  @required
  content: String
  @required
  content_attachments: RichTextAttachmentList
}

structure RecordingBucket {
  @required
  id: ProjectId
  @required
  name: String
  @required
  type: String
}

// =============================================================================
// BATCH 8 - Automation (Webhooks, Events, Recordings)
// =============================================================================

// ===== Webhook Operations =====

/// List all webhooks for a project
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/buckets/{bucketId}/webhooks.json")
operation ListWebhooks {
  input: ListWebhooksInput
  output: ListWebhooksOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListWebhooksInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId
}

structure ListWebhooksOutput {

  webhooks: WebhookList
}

/// Get a single webhook by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/webhooks/{webhookId}")
operation GetWebhook {
  input: GetWebhookInput
  output: GetWebhookOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetWebhookInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  webhookId: WebhookId
}

structure GetWebhookOutput {

  webhook: Webhook
}

/// Create a new webhook for a project
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{bucketId}/webhooks.json", code: 201)
operation CreateWebhook {
  input: CreateWebhookInput
  output: CreateWebhookOutput
  errors: [BadRequestError, WebhookLimitError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateWebhookInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  @required
  payload_url: String

  @required
  types: WebhookTypeList

  active: Boolean
}

structure CreateWebhookOutput {

  webhook: Webhook
}

/// Update an existing webhook
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/webhooks/{webhookId}")
operation UpdateWebhook {
  input: UpdateWebhookInput
  output: UpdateWebhookOutput
  errors: [NotFoundError, BadRequestError, WebhookLimitError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateWebhookInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  webhookId: WebhookId

  payload_url: String
  types: WebhookTypeList
  active: Boolean
}

structure UpdateWebhookOutput {

  webhook: Webhook
}

/// Delete a webhook
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/webhooks/{webhookId}", code: 204)
operation DeleteWebhook {
  input: DeleteWebhookInput
  output: DeleteWebhookOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure DeleteWebhookInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  webhookId: WebhookId
}

structure DeleteWebhookOutput {}

// ===== Event Operations =====

/// List all events for a recording
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/recordings/{recordingId}/events.json")
operation ListEvents {
  input: ListEventsInput
  output: ListEventsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListEventsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListEventsOutput {

  events: EventList
}

// ===== Recording Operations =====

/// List recordings of a given type across projects
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/projects/recordings.json")
operation ListRecordings {
  input: ListRecordingsInput
  output: ListRecordingsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListRecordingsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpQuery("type")
  type: RecordingType

  @httpQuery("bucket")
  bucket: String

  @httpQuery("status")
  status: RecordingStatus

  @httpQuery("sort")
  sort: RecordingSortField

  @httpQuery("direction")
  direction: SortDirection

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListRecordingsOutput {

  recordings: RecordingList
}

/// Trash a recording
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/recordings/{recordingId}/status/trashed.json", code: 204)
operation TrashRecording {
  input: TrashRecordingInput
  output: TrashRecordingOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure TrashRecordingInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure TrashRecordingOutput {}

/// Archive a recording
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/recordings/{recordingId}/status/archived.json", code: 204)
operation ArchiveRecording {
  input: ArchiveRecordingInput
  output: ArchiveRecordingOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure ArchiveRecordingInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure ArchiveRecordingOutput {}

/// Unarchive a recording (restore to active status)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/recordings/{recordingId}/status/active.json", code: 204)
operation UnarchiveRecording {
  input: UnarchiveRecordingInput
  output: UnarchiveRecordingOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UnarchiveRecordingInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure UnarchiveRecordingOutput {}

/// Set client visibility for a recording
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/recordings/{recordingId}/client_visibility.json")
operation SetClientVisibility {
  input: SetClientVisibilityInput
  output: SetClientVisibilityOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure SetClientVisibilityInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId

  @required
  visible_to_clients: Boolean
}

structure SetClientVisibilityOutput {

  recording: Recording
}

// ===== Webhook Shapes =====

long WebhookId

list WebhookList {
  member: Webhook
}

list WebhookTypeList {
  member: String
}

structure Webhook {
  @required
  id: WebhookId
  active: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  payload_url: String
  types: WebhookTypeList
  @required
  url: String
  @required
  app_url: String

  /// Up to the 25 most recent delivery exchanges, most recent first.
  /// Empty when the webhook hasn't delivered anything yet.
  recent_deliveries: WebhookDeliveryList
}

/// The event payload delivered to webhook URLs.
/// This is the body of an outbound webhook HTTP request.
/// Also appears as the body field in WebhookDelivery.request.
structure WebhookEvent {
  id: Long
  kind: String
  details: smithy.api#Document
  created_at: ISO8601Timestamp
  recording: Recording
  creator: Person
  copy: WebhookCopy
}

/// Reference to a copied/moved recording in copy events.
structure WebhookCopy {
  id: Long
  url: String
  app_url: String
  bucket: WebhookCopyBucket
}

structure WebhookCopyBucket {
  id: ProjectId
}

structure WebhookDelivery {
  id: Long
  created_at: ISO8601Timestamp
  request: WebhookDeliveryRequest
  response: WebhookDeliveryResponse
}

structure WebhookDeliveryRequest {
  headers: WebhookHeadersMap
  body: WebhookEvent
}

structure WebhookDeliveryResponse {
  headers: WebhookHeadersMap
  code: Integer
  message: String
}

map WebhookHeadersMap {
  key: String
  value: String
}

list WebhookDeliveryList {
  member: WebhookDelivery
}

// ===== Event Shapes =====

long EventId

list EventList {
  member: Event
}

structure Event {
  @required
  id: EventId
  @required
  recording_id: RecordingId
  @required
  action: String
  details: EventDetails
  @required
  created_at: ISO8601Timestamp
  @required
  creator: Person
  boosts_count: Integer
  boosts_url: String
}

structure EventDetails {
  added_person_ids: PersonIdList
  removed_person_ids: PersonIdList
  notified_recipient_ids: PersonIdList
}

// ===== Recording Shapes =====

@documentation("Comment|Document|Door|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule::Entry|Todo|Todolist|Upload|Vault")
string RecordingType

@documentation("active|archived|trashed")
string RecordingStatus

@documentation("created_at|updated_at")
string RecordingSortField

@documentation("asc|desc")
string SortDirection

list RecordingList {
  member: Recording
}

structure Recording {
  @required
  id: RecordingId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  /// API URL of the recording. Exception: in the `type=Door` (external-link)
  /// projection, `url` is the door's **external destination address** (e.g. the
  /// Figma/Dropbox URL) and `app_url` is the Basecamp redirector — see the
  /// door-specific `service`/`description` fields below.
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String

  /// URL of the Bubble Up record for this recording (BC5 addition). Optional
  /// here because this is a polymorphic projection:
  /// `recordings/_recording.json.jbuilder` emits the key only when the caller
  /// passes `local_assigns[:bubbleupable]`, and `todolists/_todolist` is the
  /// only partial that does. So a Todolist-shaped instance carries it and the
  /// other recording types do not.
  bubble_up_url: String
  content: String
  /// Rich-text companion arrays carried through the generic recording
  /// projection (`to_recordable_partial_path` renders the full type-specific
  /// partial). A given recording is one type, so it carries only the array
  /// matching its rich-text attribute (`content_attachments` for a
  /// Comment/Message, `description_attachments` for a Todo/Card); a
  /// webhook-sourced recording (base partial) carries neither. Optional (no
  /// `@required`), non-nullable.
  content_attachments: RichTextAttachmentList
  /// See `content_attachments` — the description-attribute companion array.
  description_attachments: RichTextAttachmentList
  comments_count: Integer
  comments_url: String
  subscription_url: String

  /// Boost count/URL. Carried on boostable recordings — notably the account-wide
  /// aggregate feeds (`/messages.json`, `/comments.json`), whose type-specific
  /// partials render with `boostable: true`. Optional (absent on non-boostable
  /// recordings and on the base/webhook partial).
  boosts_count: Integer
  boosts_url: String

  /// Message subject. Present on `Message` recordings — notably the account-wide
  /// `/messages.json` aggregate feed, whose message partial renders `subject`.
  subject: String

  /// Message category (type), when the message is filed under one. Rendered by
  /// the shared recordings/_category partial (present on categorized `Message`
  /// recordings, e.g. in the `/messages.json` aggregate feed).
  category: RecordingCategory

  /// Check-in grouping date (YYYY-MM-DD). Present on automatic check-in answer
  /// (`Question::Answer`) recordings — notably the `/checkins.json` aggregate
  /// feed, whose answer partial renders `group_on`.
  group_on: String

  /// Sender of an inbox forward. Present on `Inbox::Forward` recordings — notably
  /// the `/forwards.json` aggregate feed, whose forward partial renders `from`.
  from: String

  /// Reply count/URL for an inbox forward. Present on `Inbox::Forward`
  /// recordings — notably the `/forwards.json` aggregate feed.
  replies_count: Integer
  replies_url: String

  /// Ordinal position within the project's External links section. Present on
  /// `Door` (external-link) recordings.
  position: Integer

  /// Rich-text (HTML) description shown beneath an external link. Present only
  /// on `Door` recordings returned by the `type=Door` recordings query (the only
  /// endpoint that returns the full door shape). The external destination
  /// address is `url` (not this field); `app_url` is the Basecamp redirector.
  /// See `spec/api-gaps/external-links-doors.md`.
  description: String

  /// Metadata about the recognized external service the link's `url` points to
  /// (Figma, Dropbox, GitHub, …). Present only on `Door` recordings.
  service: DoorService

  /// Parent container of the recording. Optional because `Door` (external-link)
  /// recordings have no parent recording — the `type=Door` projection omits it,
  /// so a strict decoder must tolerate its absence.
  parent: RecordingParent
  @required
  bucket: RecordingBucket
  @required
  creator: Person
}

/// A message category (type) as rendered by the shared recordings/_category
/// partial: id, display name, and icon. Present on categorized Message
/// recordings.
structure RecordingCategory {
  @required
  id: Long
  @required
  name: String
  icon: String
}

/// Metadata describing the recognized external service backing an external link
/// (`Door` recording): its display name, a canonical example URL, a short code,
/// the URL patterns Basecamp recognizes for it, and human supporting text. `code`
/// is `other` for a generic link.
structure DoorService {
  name: String
  example_url: String
  code: String
  valid_patterns: StringList
  supporting_text: String
}

// =============================================================================
// BATCH 9 - Checkins (Questionnaires, Questions, Answers)
// =============================================================================

// ===== Questionnaire Operations =====

/// Get a questionnaire (automatic check-ins container) by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/questionnaires/{questionnaireId}")
operation GetQuestionnaire {
  input: GetQuestionnaireInput
  output: GetQuestionnaireOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetQuestionnaireInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionnaireId: QuestionnaireId
}

structure GetQuestionnaireOutput {

  questionnaire: Questionnaire
}

// ===== Question Operations =====

/// List all questions in a questionnaire
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/questionnaires/{questionnaireId}/questions.json")
operation ListQuestions {
  input: ListQuestionsInput
  output: ListQuestionsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListQuestionsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionnaireId: QuestionnaireId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListQuestionsOutput {

  questions: QuestionList
}

/// Get a single question by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/questions/{questionId}")
operation GetQuestion {
  input: GetQuestionInput
  output: GetQuestionOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetQuestionInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionId: QuestionId
}

structure GetQuestionOutput {

  question: Question
}

/// Create a new question in a questionnaire
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/questionnaires/{questionnaireId}/questions.json", code: 201)
operation CreateQuestion {
  input: CreateQuestionInput
  output: CreateQuestionOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateQuestionInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionnaireId: QuestionnaireId

  @required
  title: String

  @required
  schedule: QuestionSchedule

  visible_to_clients: Boolean
}

structure CreateQuestionOutput {

  question: Question
}

/// Update an existing question
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/questions/{questionId}")
operation UpdateQuestion {
  input: UpdateQuestionInput
  output: UpdateQuestionOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateQuestionInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionId: QuestionId

  title: String
  schedule: QuestionSchedule
  paused: Boolean
}

structure UpdateQuestionOutput {

  question: Question
}

/// Pause a check-in question (stops sending reminders)
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "POST", uri: "/{accountId}/questions/{questionId}/pause.json")
operation PauseQuestion {
  input: PauseQuestionInput
  output: PauseQuestionOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure PauseQuestionInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionId: QuestionId
}

structure PauseQuestionOutput {
  paused: Boolean
}

/// Resume a paused check-in question (resumes sending reminders)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/questions/{questionId}/pause.json")
operation ResumeQuestion {
  input: ResumeQuestionInput
  output: ResumeQuestionOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure ResumeQuestionInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionId: QuestionId
}

structure ResumeQuestionOutput {
  paused: Boolean
}

/// Update notification settings for a check-in question
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/questions/{questionId}/notification_settings.json")
operation UpdateQuestionNotificationSettings {
  input: UpdateQuestionNotificationSettingsInput
  output: UpdateQuestionNotificationSettingsOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateQuestionNotificationSettingsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionId: QuestionId

  /// Notify when someone answers
  notify_on_answer: Boolean

  /// Include unanswered in digest
  digest_include_unanswered: Boolean
}

structure UpdateQuestionNotificationSettingsOutput {
  responding: Boolean
  subscribed: Boolean
}

// ===== Answer Operations =====

/// List all answers for a question
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/questions/{questionId}/answers.json")
operation ListAnswers {
  input: ListAnswersInput
  output: ListAnswersOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListAnswersInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionId: QuestionId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListAnswersOutput {

  answers: QuestionAnswerList
}

/// Get a single answer by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/question_answers/{answerId}")
operation GetAnswer {
  input: GetAnswerInput
  output: GetAnswerOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetAnswerInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  answerId: AnswerId
}

structure GetAnswerOutput {

  answer: QuestionAnswer
}

/// Create a new answer for a question
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/questions/{questionId}/answers.json", code: 201)
operation CreateAnswer {
  input: CreateAnswerInput
  output: CreateAnswerOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateAnswerInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionId: QuestionId

  @required
  @httpPayload
  question_answer: QuestionAnswerPayload
}

structure QuestionAnswerPayload {
  @required
  content: String

  group_on: ISO8601Date
}

structure CreateAnswerOutput {

  answer: QuestionAnswer
}

/// Update an existing answer
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/question_answers/{answerId}", code: 204)
operation UpdateAnswer {
  input: UpdateAnswerInput
  output: UpdateAnswerOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateAnswerInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  answerId: AnswerId

  @required
  @httpPayload
  question_answer: QuestionAnswerUpdatePayload
}

structure QuestionAnswerUpdatePayload {
  @required
  content: String

  group_on: ISO8601Date
}

structure UpdateAnswerOutput {}

/// List all people who have answered a question (answerers)
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/questions/{questionId}/answers/by.json")
operation ListQuestionAnswerers {
  input: ListQuestionAnswerersInput
  output: ListQuestionAnswerersOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListQuestionAnswerersInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionId: QuestionId
}

structure ListQuestionAnswerersOutput {

  people: PersonList
}

/// Get all answers from a specific person for a question
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/questions/{questionId}/answers/by/{personId}")
operation GetAnswersByPerson {
  input: GetAnswersByPersonInput
  output: GetAnswersByPersonOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetAnswersByPersonInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  questionId: QuestionId

  @required
  @httpLabel
  personId: PersonId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetAnswersByPersonOutput {

  answers: QuestionAnswerList
}

/// Get pending check-in reminders for the current user
///
/// Returns questions that are pending a response from the authenticated user.
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/my/question_reminders.json")
operation GetQuestionReminders {
  input: GetQuestionRemindersInput
  output: GetQuestionRemindersOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetQuestionRemindersInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetQuestionRemindersOutput {

  reminders: QuestionReminderList
}

// ===== Question Reminder Shapes =====

list QuestionReminderList {
  member: QuestionReminder
}

structure QuestionReminder {
  reminder_id: Long
  remind_at: ISO8601Timestamp
  group_on: ISO8601Date
  question: Question
}

// ===== Questionnaire Shapes =====

long QuestionnaireId

structure Questionnaire {
  @required
  id: QuestionnaireId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  questions_url: String
  questions_count: Integer
  @required
  name: String
  @required
  bucket: RecordingBucket
  @required
  creator: Person
}

// ===== Question Shapes =====

long QuestionId

list QuestionList {
  member: Question
}

structure Question {
  @required
  id: QuestionId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  @required
  parent: RecordingParent
  @required
  bucket: RecordingBucket
  @required
  creator: Person
  paused: Boolean
  schedule: QuestionSchedule
  answers_count: Integer
  answers_url: String
}

structure QuestionSchedule {
  frequency: String
  days: IntegerList
  hour: Integer
  minute: Integer
  week_instance: Integer
  week_interval: Integer
  month_interval: Integer
  start_date: ISO8601Date
  end_date: ISO8601Date
}

list IntegerList {
  member: Integer
}

// ===== Answer Shapes =====

long AnswerId

list QuestionAnswerList {
  member: QuestionAnswer
}

structure QuestionAnswer {
  @required
  id: AnswerId
  @required
  status: String
  @required
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String
  subscription_url: String
  comments_count: Integer
  comments_url: String
  @required
  content: String
  @required
  content_attachments: RichTextAttachmentList
  group_on: ISO8601Date
  @required
  parent: RecordingParent
  @required
  bucket: RecordingBucket
  @required
  creator: Person
  boosts_count: Integer
  boosts_url: String
}

// =============================================================================
// BATCH 10 - Utilities (Search, Templates, Tools, Lineup)
// =============================================================================

// ===== Search Operations =====

/// Search for content across the account
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/search.json")
operation Search {
  input: SearchInput
  output: SearchOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure SearchInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpQuery("q")
  query: String

  /// Recording types to include. Use `key` values from the metadata
  /// endpoint's `recording_search_types`. Available since Basecamp 5.
  @httpQuery("type_names[]")
  typeNames: SearchTypeNameList

  /// Project IDs to filter by. Available since Basecamp 5.
  @httpQuery("bucket_ids[]")
  bucketIds: SearchBucketIdList

  /// Creator person IDs to filter by. Available since Basecamp 5.
  @httpQuery("creator_ids[]")
  creatorIds: SearchCreatorIdList

  /// Filter attachments by type. Use `key` values from the metadata
  /// endpoint's `file_search_types`.
  @httpQuery("file_type")
  fileType: String

  /// Set to true to exclude chat results.
  @httpQuery("exclude_chat")
  excludeChat: Boolean

  @httpQuery("since")
  since: SearchSinceField

  @httpQuery("sort")
  sort: SearchSortField

  /// Deprecated: prefer type_names[].
  @deprecated(message: "Use typeNames (type_names[]) instead", since: "2026-07")
  @httpQuery("type")
  type: String

  /// Deprecated: prefer bucket_ids[].
  @deprecated(message: "Use bucketIds (bucket_ids[]) instead", since: "2026-07")
  @httpQuery("bucket_id")
  bucketId: ProjectId

  /// Deprecated: prefer creator_ids[].
  @deprecated(message: "Use creatorIds (creator_ids[]) instead", since: "2026-07")
  @httpQuery("creator_id")
  creatorId: PersonId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure SearchOutput {

  results: SearchResultList
}

/// Get search metadata (available filter options)
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/searches/metadata.json")
operation GetSearchMetadata {
  input: GetSearchMetadataInput
  output: GetSearchMetadataOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetSearchMetadataInput {
  @required
  @httpLabel
  accountId: AccountId
}

structure GetSearchMetadataOutput {

  metadata: SearchMetadata
}

// ===== Template Operations =====

/// List all templates visible to the current user
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/templates.json")
operation ListTemplates {
  input: ListTemplatesInput
  output: ListTemplatesOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListTemplatesInput {
  @required
  @httpLabel
  accountId: AccountId

  @httpQuery("status")
  status: TemplateStatus

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListTemplatesOutput {

  templates: TemplateList
}

/// Get a single template by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/templates/{templateId}")
operation GetTemplate {
  input: GetTemplateInput
  output: GetTemplateOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetTemplateInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  templateId: TemplateId
}

structure GetTemplateOutput {

  template: Template
}

/// Create a new template
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/templates.json", code: 201)
operation CreateTemplate {
  input: CreateTemplateInput
  output: CreateTemplateOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateTemplateInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  name: String

  description: String
}

structure CreateTemplateOutput {

  template: Template
}

/// Update an existing template
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/templates/{templateId}")
operation UpdateTemplate {
  input: UpdateTemplateInput
  output: UpdateTemplateOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateTemplateInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  templateId: TemplateId

  name: String

  description: String
}

structure UpdateTemplateOutput {

  template: Template
}

/// Delete a template (trash it)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/templates/{templateId}", code: 204)
operation DeleteTemplate {
  input: DeleteTemplateInput
  output: DeleteTemplateOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure DeleteTemplateInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  templateId: TemplateId
}

structure DeleteTemplateOutput {}

/// Create a project from a template (asynchronous)
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/templates/{templateId}/project_constructions.json", code: 201)
operation CreateProjectFromTemplate {
  input: CreateProjectFromTemplateInput
  output: CreateProjectFromTemplateOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateProjectFromTemplateInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  templateId: TemplateId

  @required
  project: ProjectConstructionAttributes
}

structure ProjectConstructionAttributes {
  @required
  name: String

  description: String
}

structure CreateProjectFromTemplateOutput {

  construction: ProjectConstruction
}

/// Get the status of a project construction
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/templates/{templateId}/project_constructions/{constructionId}")
operation GetProjectConstruction {
  input: GetProjectConstructionInput
  output: GetProjectConstructionOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetProjectConstructionInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  templateId: TemplateId

  @required
  @httpLabel
  constructionId: ConstructionId
}

structure GetProjectConstructionOutput {

  construction: ProjectConstruction
}

// ===== Tool Operations =====

/// Get a dock tool by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/dock/tools/{toolId}")
operation GetTool {
  input: GetToolInput
  output: GetToolOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetToolInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  toolId: ToolId
}

structure GetToolOutput {

  tool: Tool
}

/// Create a tool in a project dock
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{bucketId}/dock/tools.json", code: 201)
operation CreateTool {
  input: CreateToolInput
  output: CreateToolOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateToolInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  bucketId: ProjectId

  /// Tool type to add to the project dock. Values: Chat::Transcript|Inbox|Kanban::Board|Message::Board|Questionnaire|Schedule|Todoset|Vault.
  @required
  tool_type: String

  /// Title for the new tool. When omitted, Basecamp assigns the next available default title for the tool type.
  title: String

  /// Create the tool already visible to clients. Honored only for tool types that manage their own client visibility (Chat::Transcript, Kanban::Board), which otherwise start hidden; every other tool type ignores it and inherits the project default.
  visible_to_clients: Boolean
}

structure CreateToolOutput {

  tool: Tool
}

/// Update (rename) an existing tool
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/dock/tools/{toolId}")
operation UpdateTool {
  input: UpdateToolInput
  output: UpdateToolOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateToolInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  toolId: ToolId

  @required
  title: String
}

structure UpdateToolOutput {

  tool: Tool
}

/// Delete a tool (trash it)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/dock/tools/{toolId}", code: 204)
operation DeleteTool {
  input: DeleteToolInput
  output: DeleteToolOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure DeleteToolInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  toolId: ToolId
}

structure DeleteToolOutput {}

/// Enable a tool (show it on the project dock)
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/recordings/{toolId}/position.json", code: 201)
operation EnableTool {
  input: EnableToolInput
  output: EnableToolOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure EnableToolInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  toolId: ToolId
}

structure EnableToolOutput {}

/// Disable a tool (hide it from the project dock)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/recordings/{toolId}/position.json", code: 204)
operation DisableTool {
  input: DisableToolInput
  output: DisableToolOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure DisableToolInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  toolId: ToolId
}

structure DisableToolOutput {}

/// Reposition a tool on the project dock
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/recordings/{toolId}/position.json")
operation RepositionTool {
  input: RepositionToolInput
  output: RepositionToolOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure RepositionToolInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  toolId: ToolId

  @required
  position: Integer
}

structure RepositionToolOutput {}

// ===== Lineup Marker Operations =====

/// List all lineup markers for the account
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/lineup/markers.json")
operation ListLineupMarkers {
  input: ListLineupMarkersInput
  output: ListLineupMarkersOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListLineupMarkersInput {
  @required
  @httpLabel
  accountId: AccountId
}

list LineupMarkerList {
  member: LineupMarker
}

structure ListLineupMarkersOutput {
  markers: LineupMarkerList
}

/// Create a new lineup marker
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/lineup/markers.json", code: 201)
operation CreateLineupMarker {
  input: CreateLineupMarkerInput
  output: CreateLineupMarkerOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateLineupMarkerInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  name: String

  @required
  date: ISO8601Date
}

structure CreateLineupMarkerOutput {}

/// Update an existing lineup marker
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/lineup/markers/{markerId}")
operation UpdateLineupMarker {
  input: UpdateLineupMarkerInput
  output: UpdateLineupMarkerOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateLineupMarkerInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  markerId: MarkerId

  name: String
  date: ISO8601Date
}

structure UpdateLineupMarkerOutput {}

/// Delete a lineup marker
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/lineup/markers/{markerId}", code: 204)
operation DeleteLineupMarker {
  input: DeleteLineupMarkerInput
  output: DeleteLineupMarkerOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure DeleteLineupMarkerInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  markerId: MarkerId
}

structure DeleteLineupMarkerOutput {}

// ===== Timeline Operations =====

/// Get account-wide activity feed (progress report)
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/reports/progress.json")
operation GetProgressReport {
  input: GetProgressReportInput
  output: GetProgressReportOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetProgressReportInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetProgressReportOutput {
  events: TimelineEventList
}

/// Get project timeline
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/projects/{projectId}/timeline.json")
operation GetProjectTimeline {
  input: GetProjectTimelineInput
  output: GetProjectTimelineOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetProjectTimelineInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetProjectTimelineOutput {
  events: TimelineEventList
}

/// Get a person's activity timeline
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50, key: "events")
@http(method: "GET", uri: "/{accountId}/reports/users/progress/{personId}")
operation GetPersonProgress {
  input: GetPersonProgressInput
  output: GetPersonProgressOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetPersonProgressInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  personId: PersonId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetPersonProgressOutput {
  person: Person
  events: TimelineEventList
}

// ===== Reports Operations =====

/// List people who can be assigned todos
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/reports/todos/assigned.json")
operation ListAssignablePeople {
  input: ListAssignablePeopleInput
  output: ListAssignablePeopleOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListAssignablePeopleInput {
  @required
  @httpLabel
  accountId: AccountId
}

structure ListAssignablePeopleOutput {
  people: PersonList
}

/// Get todos assigned to a specific person
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/reports/todos/assigned/{personId}")
operation GetAssignedTodos {
  input: GetAssignedTodosInput
  output: GetAssignedTodosOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetAssignedTodosInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  personId: PersonId

  /// Group by "bucket" or "date"
  @httpQuery("group_by")
  group_by: String
}

structure GetAssignedTodosOutput {
  person: Person
  grouped_by: String
  todos: TodoItems
}

/// Get overdue todos grouped by lateness
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/reports/todos/overdue.json")
operation GetOverdueTodos {
  input: GetOverdueTodosInput
  output: GetOverdueTodosOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetOverdueTodosInput {
  @required
  @httpLabel
  accountId: AccountId
}

structure GetOverdueTodosOutput {
  under_a_week_late: TodoItems
  over_a_week_late: TodoItems
  over_a_month_late: TodoItems
  over_three_months_late: TodoItems
}

/// Get upcoming schedule entries and assignable items within a date window.
/// This endpoint is preserved as the canonical API path on BC5;
/// the BC5 `/calendar` web view is HTML-only.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/reports/schedules/upcoming.json")
operation GetUpcomingSchedule {
  input: GetUpcomingScheduleInput
  output: GetUpcomingScheduleOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetUpcomingScheduleInput {
  @required
  @httpLabel
  accountId: AccountId

  @httpQuery("window_starts_on")
  window_starts_on: ISO8601Date

  @httpQuery("window_ends_on")
  window_ends_on: ISO8601Date
}

structure GetUpcomingScheduleOutput {
  schedule_entries: ScheduleEntryList
  recurring_schedule_entry_occurrences: ScheduleEntryList
  assignables: AssignableList
}

// ===== Timeline Shapes =====

list TimelineEventList {
  member: TimelineEvent
}

structure TimelineEvent {
  id: Long
  created_at: ISO8601Timestamp

  /// What kind of activity the event records. Open, non-exhaustive vocabulary —
  /// BC3 documents "common values include" and adds new kinds over time, so
  /// treat unrecognized values as valid. Common values include message_created,
  /// comment_created, todo_created, todo_completed, upload_created,
  /// document_created, google_document_created, schedule_entry_created,
  /// schedule_entry_rescheduled, question_created, question_answer_created,
  /// chat_transcript_rollup, kanban_card_created, kanban_card_completed,
  /// inbox_forward_created, client_correspondence_created, dock_created, and
  /// project_access_changed.
  kind: String

  parent_recording_id: Long
  url: String
  app_url: String
  creator: Person
  action: String
  target: String
  title: String
  summary_excerpt: String

  /// Avatar URLs of participants — populated for chat_transcript_rollup events
  /// (the people summarized in the rollup); an empty array otherwise.
  avatars_sample: StringList

  bucket: TodoBucket

  /// Event-specific payload. Present only for schedule_entry_created and
  /// schedule_entry_rescheduled events, where it carries the entry's timing.
  data: TimelineEventData

  /// Files attached to the event's recording, when it has any. Heterogeneous:
  /// an upload-kind recording contributes its full Upload shape, while other
  /// recordings contribute rich-text attachment/blob partials. Modeled as an
  /// optional-field superset so a single element type decodes either variant;
  /// consumers should treat the per-variant fields as present-or-absent.
  attachments: TimelineAttachmentList
}

/// Schedule-entry timing carried on schedule_entry_* timeline events. starts_at
/// and ends_at are date-or-timestamp: a full ISO 8601 timestamp for timed
/// entries, or a bare date (YYYY-MM-DD) when all_day is true. Modeled as
/// ISO8601Timestamp (mirroring ScheduleEntry), with the Go enhancement pass
/// mapping them to types.FlexibleTime so date-only values decode; the other
/// SDKs type them as plain strings.
structure TimelineEventData {
  /// Whether the entry is all-day. BC3 emits all three members unconditionally
  /// whenever the data object is present (schedule_entry_* events), so they are
  /// required within this struct.
  @required
  all_day: Boolean
  @required
  starts_at: ISO8601Timestamp
  @required
  ends_at: ISO8601Timestamp
}

list TimelineAttachmentList {
  member: TimelineAttachment
}

/// A single timeline-event attachment. This is an optional-field superset over
/// two wire variants — a full Upload recording (upload-kind recordings, rendered
/// by BC3's uploads/_upload partial: the complete recording projection + rich-
/// text description + the upload body) and a rich-text attachment/blob partial
/// (all other recordings) — so one element type decodes either. Every field is
/// optional; a given instance populates only the fields of the variant it
/// represents. The upload-recording variant enumerates the full documented
/// projection so no documented field is silently dropped on decode.
structure TimelineAttachment {
  /// Attachment or upload-recording id.
  id: Long

  // ----- shared by both variants -----
  /// MIME type of the file.
  content_type: String
  /// Size of the file in bytes.
  byte_size: Long
  /// Original filename.
  filename: String
  /// Authenticated download URL for the file.
  @basecampAuthRoutableUrl
  download_url: String
  /// Pixel width; null for non-image blobs and may be float-spelled (1024.0).
  width: Integer
  /// Pixel height; null for non-image blobs and may be float-spelled (1024.0).
  height: Integer

  // ----- upload-recording variant (full uploads/_upload projection) -----
  /// Recording type, e.g. "Upload" (upload-recording variant).
  type: String
  /// Title of the upload recording.
  title: String
  /// Publication status of the upload recording (e.g. "active").
  status: String
  /// Whether the recording inherits its status from its parent.
  inherits_status: Boolean
  /// When the upload recording was created.
  created_at: ISO8601Timestamp
  /// When the upload recording was last updated.
  updated_at: ISO8601Timestamp
  /// API URL of the upload recording.
  url: String
  /// Web URL of the upload recording.
  app_url: String
  /// Personal bookmark toggle URL for the current user.
  bookmark_url: String
  /// Subscription URL; present only when the recording is subscribable.
  subscription_url: String
  /// Number of comments on the recording.
  comments_count: Integer
  /// API URL for the recording's comments.
  comments_url: String
  /// Number of boosts on the recording.
  boosts_count: Integer
  /// API URL for the recording's boosts.
  boosts_url: String
  /// Position within its parent; present only when the recording is positioned.
  position: Integer
  /// The recording's parent (message board, todolist, vault, …).
  parent: RecordingParent
  /// The bucket (project) the recording lives in.
  bucket: TodoBucket
  /// The person who created the recording.
  creator: Person
  /// Rich-text description of the upload (HTML), when present.
  description: UploadDescription
  /// Rich-text attachments referenced by the description.
  description_attachments: RichTextAttachmentList
  /// Web download URL (upload-recording variant).
  app_download_url: String
  /// Whether the upload recording is visible to clients.
  visible_to_clients: Boolean

  // ----- rich-text attachment/blob variant -----
  /// Signed global id of the attachable (attachment variant).
  attachable_sgid: String
  /// Signed global id of the attachment (attachment variant).
  sgid: String
  /// URL to poll attachment processing status (attachment variant).
  status_url: String
  /// Caption text, if any (attachment variant).
  caption: String
  /// Storage key of the underlying blob (attachment variant).
  key: String
  /// Whether the blob can be previewed (attachment variant).
  previewable: Boolean
  /// Full-size preview URL (attachment variant).
  preview_url: String
  /// Thumbnail preview URL (attachment variant).
  thumbnail_url: String
}

// ===== Reports Shapes =====

list AssignableList {
  member: Assignable
}

structure Assignable {
  id: Long
  title: String
  type: String
  url: String
  app_url: String
  bucket: TodoBucket
  parent: TodoParent
  due_on: ISO8601Date
  starts_on: ISO8601Date
  assignees: PersonList
}

// ===== Search Shapes =====

@documentation("best_match|recency")
string SearchSortField

@documentation("last_7_days|last_30_days|last_90_days|last_12_months|forever")
string SearchSinceField

list SearchTypeNameList {
  member: String
}

list SearchBucketIdList {
  member: ProjectId
}

list SearchCreatorIdList {
  member: PersonId
}

list SearchResultList {
  member: SearchResult
}

structure SearchResult {
  @required
  id: Long
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  @required
  title: String
  inherits_status: Boolean
  @required
  type: String
  @required
  url: String
  @required
  app_url: String
  bookmark_url: String

  /// URL of the Bubble Up record for this recording (BC5 addition). Optional
  /// here because this is a polymorphic projection:
  /// `recordings/_recording.json.jbuilder` emits the key only when the caller
  /// passes `local_assigns[:bubbleupable]`, and `todolists/_todolist` is the
  /// only partial that does. So a Todolist-shaped instance carries it and the
  /// other recording types do not.
  bubble_up_url: String
  parent: RecordingParent
  bucket: RecordingBucket
  creator: Person
  /// Always present, always null. `api/searches/show.json.jbuilder` renders the
  /// recording's own partial and then unconditionally overwrites `content` with
  /// `nil` to strip the large HTML body out of the search payload. The key is
  /// therefore guaranteed present on every result — required — and its value is
  /// guaranteed null. Read `plain_text_content` instead.
  @required
  content: String
  /// Always present, always null — the description-attribute counterpart to
  /// `content`. Read `plain_text_description` instead.
  @required
  description: String
  /// A highlighted, truncated excerpt of the recording's content — **not** plain
  /// text despite the name. `excerpt_and_highlight_matches` converts the rich
  /// text with `to_plain_text`, escapes it with `html_escape_once`, then wraps
  /// each query match in `<mark class="circled-text"><span></span>…</mark>` and
  /// truncates the result to 300 characters. Treat it as an HTML fragment.
  ///
  /// Optional and non-nullable: emitted only when the underlying recordable
  /// responds to `content`, so a result whose type has no content attribute
  /// omits the key entirely rather than sending null.
  plain_text_content: String
  /// The description-attribute counterpart to `plain_text_content`, with the
  /// same highlighting, escaping, and 300-character truncation. Optional and
  /// non-nullable — omitted when the recordable has no description attribute.
  plain_text_description: String
  /// Rich-text companion arrays carried through the polymorphic search
  /// projection. A given result is one recording type, so it carries only
  /// the array matching its rich-text attribute (`content_attachments` for a
  /// Comment/Message, `description_attachments` for a Todo); a webhook-sourced
  /// result carries neither. Optional (no `@required`), non-nullable.
  ///
  /// Search results additionally repeat this same array under a generic
  /// `attachments` key. It is a redundant projection, not a distinct
  /// aggregate: `searches/show.json.jbuilder` emits
  /// `recording.downloadable_attachments`, which delegates to the recordable's
  /// sole `rich_text_content`, through the same `attachments/_attachment`
  /// partial that `recordings/_rich_text.json.jbuilder` uses to build the
  /// companion array. `RichText.rich_text_attribute` permits exactly one
  /// rich-text attribute per model, so the two keys always carry identical
  /// elements. Modeling `attachments` would duplicate the field, so it is
  /// deliberately not modeled.
  content_attachments: RichTextAttachmentList
  /// See `content_attachments` — the description-attribute companion array.
  description_attachments: RichTextAttachmentList
  subject: String
}

structure SearchMetadata {
  @required
  recording_search_types: SearchTypeList
  @required
  file_search_types: SearchTypeList
  @required
  default_creator_label: String
  @required
  default_bucket_label: String
  @required
  default_circle_label: String
  @required
  default_file_type_label: String
  @required
  default_type_label: String
}

list SearchTypeList {
  member: SearchType
}

/// A selectable search filter option. `key` is the value passed back as a
/// filter parameter (null represents the default "everything" option); `value`
/// is the human-readable label.
structure SearchType {
  /// Always present on the wire; `null` for the default "everything" option.
  /// `@required` models the presence; nullability of the value is layered on in
  /// the OpenAPI (smithy-build.json jsonAdd -> type: ["string", "null"]) since
  /// Smithy has no native required-and-nullable.
  @required
  key: String
  @required
  value: String
}

// ===== Template Shapes =====

long TemplateId
long ConstructionId

@documentation("active|archived|trashed")
string TemplateStatus

list TemplateList {
  member: Template
}

structure Template {
  @required
  id: TemplateId
  status: String
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  name: String
  description: String
  url: String
  app_url: String
  dock: DockItemList
}

structure ProjectConstruction {
  @required
  id: ConstructionId
  @required
  status: String
  url: String
  project: Project
}

// ===== Tool Shapes =====

long ToolId

structure Tool {
  @required
  id: ToolId
  status: String
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  title: String
  @required
  name: String
  @required
  enabled: Boolean
  position: Integer
  url: String
  app_url: String
  bucket: RecordingBucket
}

// ===== Lineup Marker Shapes =====

long MarkerId

structure LineupMarker {
  @required
  id: MarkerId
  @required
  name: String
  @required
  date: ISO8601Date
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
}

// =============================================================================
// BATCH 12: Boosts
// =============================================================================

// ===== Boost Operations =====

long BoostId

/// List boosts on a recording
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/recordings/{recordingId}/boosts.json")
operation ListRecordingBoosts {
  input: ListRecordingBoostsInput
  output: ListRecordingBoostsOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure ListRecordingBoostsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListRecordingBoostsOutput {
  boosts: BoostList
}

/// List boosts on a specific event within a recording
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/recordings/{recordingId}/events/{eventId}/boosts.json")
operation ListEventBoosts {
  input: ListEventBoostsInput
  output: ListEventBoostsOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure ListEventBoostsInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId

  @required
  @httpLabel
  eventId: EventId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListEventBoostsOutput {
  boosts: BoostList
}

/// Get a single boost
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/boosts/{boostId}")
operation GetBoost {
  input: GetBoostInput
  output: GetBoostOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetBoostInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  boostId: BoostId
}

structure GetBoostOutput {
  boost: Boost
}

/// Create a boost on a recording
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/recordings/{recordingId}/boosts.json", code: 201)
operation CreateRecordingBoost {
  input: CreateRecordingBoostInput
  output: CreateRecordingBoostOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateRecordingBoostInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId

  @required
  @length(max: 16)
  content: String
}

structure CreateRecordingBoostOutput {
  boost: Boost
}

/// Create a boost on a specific event within a recording
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/recordings/{recordingId}/events/{eventId}/boosts.json", code: 201)
operation CreateEventBoost {
  input: CreateEventBoostInput
  output: CreateEventBoostOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateEventBoostInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId

  @required
  @httpLabel
  eventId: EventId

  @required
  @length(max: 16)
  content: String
}

structure CreateEventBoostOutput {
  boost: Boost
}

/// Delete a boost
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/boosts/{boostId}", code: 204)
operation DeleteBoost {
  input: DeleteBoostInput
  output: DeleteBoostOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure DeleteBoostInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  boostId: BoostId
}

structure DeleteBoostOutput {}

// ===== Boost Shapes =====

list BoostList {
  member: Boost
}

structure Boost {
  @required
  id: BoostId
  content: String
  @required
  created_at: ISO8601Timestamp
  booster: Person
  // The boosted recording on the feeds that embed it (my/boosts); absent on
  // the per-recording boosts list. Kept as the shared RecordingParent
  // (unchanged public type) so this additive coverage does not break existing
  // Boost callers; RecordingParent carries an optional `bucket` so a
  // feed-embedded boost keeps its project context.
  recording: RecordingParent
}

// =============================================================================
// BATCH 13 - Account
// =============================================================================

// ===== Account Operations =====

/// Get the account for the current access token
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/account.json")
operation GetAccount {
  input: GetAccountInput
  output: GetAccountOutput
  errors: [UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetAccountInput {
  @required
  @httpLabel
  accountId: AccountId
}

structure GetAccountOutput {

  account: Account
}

/// Rename the current account. Only account owners can use this endpoint.
@idempotent
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/account/name.json")
operation UpdateAccountName {
  input: UpdateAccountNameInput
  output: UpdateAccountNameOutput
  errors: [BadRequestError, ForbiddenError, UnauthorizedError, RateLimitError, InternalServerError]
}

structure UpdateAccountNameInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  name: String
}

structure UpdateAccountNameOutput {

  account: Account
}

/// Remove the account logo. Only administrators and account owners can use this endpoint.
@idempotent
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/account/logo.json", code: 204)
operation RemoveAccountLogo {
  input: RemoveAccountLogoInput
  output: RemoveAccountLogoOutput
  errors: [ForbiddenError, UnauthorizedError, RateLimitError, InternalServerError]
}

structure RemoveAccountLogoInput {
  @required
  @httpLabel
  accountId: AccountId
}

structure RemoveAccountLogoOutput {}

/// Upload or replace the account logo.
/// Accepted formats: PNG, JPEG, GIF, WebP, AVIF, HEIC. Maximum 5 MB.
/// Owners and admins only.
@idempotent
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@basecampMultipart(field: "logo")
@http(method: "PUT", uri: "/{accountId}/account/logo.json", code: 204)
operation UpdateAccountLogo {
  input: UpdateAccountLogoInput
  output: UpdateAccountLogoOutput
  errors: [ValidationError, ForbiddenError, UnauthorizedError, RateLimitError, InternalServerError]
}

structure UpdateAccountLogoInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpPayload
  data: Blob
}

structure UpdateAccountLogoOutput {}

// ===== Account Shapes =====

structure Account {
  @required
  id: Long
  @required
  name: String
  owner_name: String
  active: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  trial: Boolean
  trial_ends_on: ISO8601Date
  frozen: Boolean
  paused: Boolean
  limits: AccountLimits
  subscription: AccountSubscription
  settings: AccountSettings
  logo: AccountLogo
}

structure AccountLimits {
  can_create_projects: Boolean
  can_pin_projects: Boolean
  can_create_users: Boolean
  can_upload_files: Boolean
}

structure AccountSubscription {
  short_name: String
  proper_name: String
  project_limit: Integer
  teams: Boolean
  clients: Boolean
  templates: Boolean
  logo: Boolean
  timesheet: Boolean
}

structure AccountSettings {
  company_hq_enabled: Boolean
  teams_enabled: Boolean
  projects_enabled: Boolean
}

structure AccountLogo {
  url: String
}

// =============================================================================
// BATCH 14 - Gauges
// =============================================================================

// ===== Gauge Operations =====

/// List gauges across all projects the authenticated user has access to.
/// Gauges are sorted by risk level (red, yellow, green), then alphabetically.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/reports/gauges.json")
operation ListGauges {
  input: ListGaugesInput
  output: ListGaugesOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListGaugesInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Comma-separated list of project IDs. When provided, results are returned
  /// in the order specified instead of by risk level.
  @httpQuery("bucket_ids")
  bucket_ids: String

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListGaugesOutput {

  gauges: GaugeList
}

/// List gauge needles for a project, ordered newest first.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/projects/{projectId}/gauge/needles.json")
operation ListGaugeNeedles {
  input: ListGaugeNeedlesInput
  output: ListGaugeNeedlesOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListGaugeNeedlesInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListGaugeNeedlesOutput {

  needles: GaugeNeedleList
}

/// Get a gauge needle by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/gauge_needles/{needleId}")
operation GetGaugeNeedle {
  input: GetGaugeNeedleInput
  output: GetGaugeNeedleOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetGaugeNeedleInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  needleId: GaugeNeedleId
}

structure GetGaugeNeedleOutput {

  needle: GaugeNeedle
}

/// Create a gauge needle (progress update) for a project
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/projects/{projectId}/gauge/needles.json", code: 201)
operation CreateGaugeNeedle {
  input: CreateGaugeNeedleInput
  output: CreateGaugeNeedleOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateGaugeNeedleInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @required
  gauge_needle: GaugeNeedlePayload

  /// Who to notify: "everyone", "working_on", "custom", or omit for nobody
  notify: String

  /// Array of people IDs to notify (only used when notify is "custom")
  subscriptions: PersonIdList
}

structure GaugeNeedlePayload {
  /// Position of the needle (0-100)
  @required
  position: Integer

  /// Status color: green (default), yellow, or red
  color: String

  /// Rich text (HTML) description of the progress update
  description: String
}

structure CreateGaugeNeedleOutput {

  needle: GaugeNeedle
}

/// Update a gauge needle's description. Position and color are immutable.
@idempotent
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/gauge_needles/{needleId}")
operation UpdateGaugeNeedle {
  input: UpdateGaugeNeedleInput
  output: UpdateGaugeNeedleOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure UpdateGaugeNeedleInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  needleId: GaugeNeedleId

  gauge_needle: GaugeNeedleUpdatePayload
}

structure GaugeNeedleUpdatePayload {
  /// Rich text (HTML) description
  description: String
}

structure UpdateGaugeNeedleOutput {

  needle: GaugeNeedle
}

/// Destroy a gauge needle
@idempotent
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/gauge_needles/{needleId}", code: 204)
operation DestroyGaugeNeedle {
  input: DestroyGaugeNeedleInput
  output: DestroyGaugeNeedleOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure DestroyGaugeNeedleInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  needleId: GaugeNeedleId
}

structure DestroyGaugeNeedleOutput {}

/// Enable or disable the gauge for a project. Only project admins can toggle gauges.
@idempotent
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/projects/{projectId}/gauge.json")
operation ToggleGauge {
  input: ToggleGaugeInput
  output: ToggleGaugeOutput
  errors: [ForbiddenError, UnauthorizedError, RateLimitError, InternalServerError]
}

structure ToggleGaugeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @required
  gauge: GaugeTogglePayload
}

structure GaugeTogglePayload {
  @required
  enabled: Boolean
}

structure ToggleGaugeOutput {}

// ===== Gauge Shapes =====

long GaugeNeedleId

list GaugeList {
  member: Gauge
}

list GaugeNeedleList {
  member: GaugeNeedle
}

structure Gauge {
  @required
  id: Long
  status: String
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  bucket: RecordingBucket
  creator: Person
  description: String
  /// Optional (no `@required`): the type-specific partial renders the
  /// companion array only when the gauge has needles (bc3 `if
  /// gauge.any_needles?`), so a needle-less gauge omits the key entirely.
  /// Non-nullable — never served as JSON `null`.
  description_attachments: RichTextAttachmentList
  enabled: Boolean
  last_needle_color: String
  last_needle_position: Integer
  previous_needle_position: Integer
}

structure GaugeNeedle {
  @required
  id: GaugeNeedleId
  status: String
  visible_to_clients: Boolean
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  subscription_url: String
  comments_count: Integer
  comments_url: String
  boosts_count: Integer
  boosts_url: String
  parent: RecordingParent
  bucket: RecordingBucket
  creator: Person
  description: String
  @required
  description_attachments: RichTextAttachmentList
  color: String
  position: Integer
}

// =============================================================================
// BATCH 15 - My Assignments
// =============================================================================

// ===== My Assignment Operations =====

/// Get the current user's active assignments grouped into priorities and non_priorities.
/// Card table steps are normalized to their parent card with steps as children.
/// This endpoint is not paginated.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/my/assignments.json")
operation GetMyAssignments {
  input: GetMyAssignmentsInput
  output: GetMyAssignmentsOutput
  errors: [UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetMyAssignmentsInput {
  @required
  @httpLabel
  accountId: AccountId
}

structure GetMyAssignmentsOutput {
  priorities: MyAssignmentList
  non_priorities: MyAssignmentList
}

/// Add a recording to Up Next — the current user's ordered list of prioritized
/// assignments (the priorities returned by GetMyAssignments). Identify the item
/// by the recording id that carries the priority; for a card table step
/// surfaced under its parent card, that is the entry's priority_recording_id.
/// Idempotent: re-prioritizing an already-prioritized recording is a no-op.
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "POST", uri: "/{accountId}/my/priorities.json", code: 204)
operation PrioritizeAssignment {
  input: PrioritizeAssignmentInput
  output: PrioritizeAssignmentOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure PrioritizeAssignmentInput {
  @required
  @httpLabel
  accountId: AccountId

  /// The recording id to prioritize.
  @required
  id: RecordingId
}

structure PrioritizeAssignmentOutput {}

/// Remove a recording from Up Next (returns 204 No Content). Exact-target:
/// only the priority carried by the identified recording is cleared, and
/// deleting an absent priority is a no-op 204 — so the DELETE is idempotent
/// and safe to retry (BC3 #12483). Address a surfaced card table step by its
/// priority_recording_id, not its parent card's id.
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/my/priorities/{recordingId}", code: 204)
operation DeprioritizeAssignment {
  input: DeprioritizeAssignmentInput
  output: DeprioritizeAssignmentOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure DeprioritizeAssignmentInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure DeprioritizeAssignmentOutput {}

/// Move an already-prioritized recording to a new 1-based position in Up Next
/// (returns 204 No Content). NOT idempotent: a positional move's meaning
/// shifts as the list changes, so a retry can land the item somewhere else —
/// no retry gating is declared. Errors: 400 for a missing or non-integer
/// position, 422 (flat {error} body) for an out-of-range position or an
/// unprioritized recording, and a bare bodyless 404 for an inaccessible
/// recording.
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/my/priority_moves.json", code: 204)
operation ReorderUpNext {
  input: ReorderUpNextInput
  output: ReorderUpNextOutput
  errors: [BareNotFoundError, BadRequestError, ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ReorderUpNextInput {
  @required
  @httpLabel
  accountId: AccountId

  /// The recording id to move, chosen the same way as when prioritizing.
  @required
  source_id: RecordingId

  /// The 1-based position to move it to.
  @required
  position: Integer
}

structure ReorderUpNextOutput {}

/// Get the current user's completed assignments.
/// Archived and trashed recordings are excluded. This endpoint is not paginated.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/my/assignments/completed.json")
operation GetMyCompletedAssignments {
  input: GetMyCompletedAssignmentsInput
  output: GetMyCompletedAssignmentsOutput
  errors: [UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetMyCompletedAssignmentsInput {
  @required
  @httpLabel
  accountId: AccountId
}

structure GetMyCompletedAssignmentsOutput {

  assignments: MyAssignmentList
}

/// Get the current user's assignments filtered by due date scope.
/// Defaults to overdue when no scope is provided. This endpoint is not paginated.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/my/assignments/due.json")
operation GetMyDueAssignments {
  input: GetMyDueAssignmentsInput
  output: GetMyDueAssignmentsOutput
  errors: [BadRequestError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetMyDueAssignmentsInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Filter by due date range: overdue, due_today, due_tomorrow,
  /// due_later_this_week, due_next_week, due_later
  @httpQuery("scope")
  scope: String
}

structure GetMyDueAssignmentsOutput {

  assignments: MyAssignmentList
}

// =============================================================================
// BATCH 15b - Everything Aggregates (flat family)
// =============================================================================
//
// Account-wide recording listings served by the everything/*_controller.rb
// namespace under flat top-level paths (the /everything/... segment is the
// Rails controller namespace, not part of the URL). Documented by BC3 #11627 in
// doc/api/sections/everything.md. This is the flat family: six recency-ordered,
// Link-paginated roots plus two unpaginated oldest-first overdue lists. The
// bucket-grouped todo/card filter sub-routes are a separate family. Never model
// the bare /todos.json or /cards.json roots (HTML shells) or the internal
// /<resource>/recent.json feeds.

/// Get every message across all accessible projects, newest-first (paginated).
/// Each item embeds its `bucket` for project context.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/messages.json")
operation GetEverythingMessages {
  input: GetEverythingMessagesInput
  output: GetEverythingMessagesOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetEverythingMessagesInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetEverythingMessagesOutput {
  recordings: RecordingList
}

/// Get every comment across all accessible projects, newest-first (paginated).
/// Each item embeds its `bucket`.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/comments.json")
operation GetEverythingComments {
  input: GetEverythingCommentsInput
  output: GetEverythingCommentsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetEverythingCommentsInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetEverythingCommentsOutput {
  recordings: RecordingList
}

/// Get every automatic check-in answer across all accessible projects, newest-first.
/// Paginated; each item embeds its `bucket`.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/checkins.json")
operation GetEverythingCheckins {
  input: GetEverythingCheckinsInput
  output: GetEverythingCheckinsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetEverythingCheckinsInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetEverythingCheckinsOutput {
  recordings: RecordingList
}

/// Get every inbox forward across all accessible projects, newest-first (paginated).
/// Each item embeds its `bucket`.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/forwards.json")
operation GetEverythingForwards {
  input: GetEverythingForwardsInput
  output: GetEverythingForwardsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetEverythingForwardsInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetEverythingForwardsOutput {
  recordings: RecordingList
}

/// Get every file recording across all accessible projects, newest-first (paginated).
/// Heterogeneous: uploads and Basecamp documents carry their
/// standard recording shapes, while rich-text attachments are wrapped in a
/// recording envelope plus an `attachable_sgid` and blob metadata. Modeled as
/// an optional-field superset (EverythingFile) so one element type decodes any
/// variant.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/files.json")
operation GetEverythingFiles {
  input: GetEverythingFilesInput
  output: GetEverythingFilesOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetEverythingFilesInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Filter by file kind: all (default), images, pdfs, documents, or videos.
  @httpQuery("kind")
  kind: String

  /// Restrict to files created by the given people (repeatable).
  @httpQuery("people_ids[]")
  people_ids: PersonIdList

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetEverythingFilesOutput {
  files: EverythingFileList
}

/// Get every overdue to-do across all accessible projects, oldest-due-date-first.
/// A complete, unpaginated array; each item embeds its `bucket`.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/todos/overdue.json")
operation GetEverythingOverdueTodos {
  input: GetEverythingOverdueTodosInput
  output: GetEverythingOverdueTodosOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetEverythingOverdueTodosInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Restrict to tasks assigned to at least one of the given people (repeatable).
  /// Assignees on nested steps are not considered.
  @httpQuery("assignee_ids[]")
  assignee_ids: PersonIdList

  /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
  @httpQuery("due")
  due: String
}

structure GetEverythingOverdueTodosOutput {
  todos: TodoItems
}

/// Get every overdue card across all accessible projects, oldest-due-date-first.
/// A complete, unpaginated array; each item embeds its `bucket`.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/cards/overdue.json")
operation GetEverythingOverdueCards {
  input: GetEverythingOverdueCardsInput
  output: GetEverythingOverdueCardsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetEverythingOverdueCardsInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Restrict to tasks assigned to at least one of the given people (repeatable).
  /// Assignees on nested steps are not considered.
  @httpQuery("assignee_ids[]")
  assignee_ids: PersonIdList

  /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
  @httpQuery("due")
  due: String
}

structure GetEverythingOverdueCardsOutput {
  cards: CardList
}

// ===== Everything File Shapes =====

list EverythingFileList {
  member: EverythingFile
}

/// A single item in the /files.json feed. An optional-field superset over three
/// wire variants — a full Upload recording, a Basecamp Document recording, and a
/// rich-text attachment wrapped in a recording envelope (distinguished by
/// `attachable_sgid` and blob metadata). Every field is optional; a given
/// instance populates only the fields of the variant it represents. Unknown
/// fields are ignored by every SDK decoder, so the superset need not enumerate
/// every field of the Upload/Document recordings.
structure EverythingFile {
  /// Recording (Upload/Document) or attachment id.
  id: Long
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  /// "Upload", "Document", or "Attachment".
  type: String
  url: String
  app_url: String
  bookmark_url: String
  subscription_url: String
  comments_count: Integer
  comments_url: String
  boosts_count: Integer
  boosts_url: String
  position: Integer
  parent: RecordingParent
  bucket: RecordingBucket
  creator: Person

  /// Present on the rich-text attachment variant: signed global id of the
  /// attachment (uploads/documents omit it).
  attachable_sgid: String

  // ----- blob/file metadata (uploads and attachments) -----
  content_type: String
  byte_size: Long
  filename: String
  @basecampAuthRoutableUrl
  download_url: String
  app_download_url: String
  /// Pixel width; null for non-image blobs and may be float-spelled (1024.0).
  width: Integer
  /// Pixel height; null for non-image blobs and may be float-spelled (1024.0).
  height: Integer

  /// Rich-text description (upload/document variants).
  description: String
  description_attachments: RichTextAttachmentList

  /// Rich-text body of the Document variant (uploads/attachments omit it).
  content: DocumentContent
  content_attachments: RichTextAttachmentList
}

// =============================================================================
// BATCH 15c - Everything Aggregates (bucket-grouped todo/card filter family)
// =============================================================================
//
// The todo/card filter sub-routes return a paginated array of buckets, each
// entry grouping the matching recordings (with their steps) under their parent
// project. Documented by BC3 #11627 in doc/api/sections/everything.md. The bare
// /todos.json and /cards.json roots stay HTML shells and are never modeled.

/// Active, incomplete to-dos across all accessible projects, grouped by project (paginated).
/// Each bucket entry carries the matching to-dos and their steps.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 5)
@http(method: "GET", uri: "/{accountId}/todos/open.json")
operation GetEverythingOpenTodos {
  input: EverythingTodosFilterInput
  output: EverythingTodosGroupOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

/// Completed to-dos across all accessible projects, grouped by project (paginated).
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 5)
@http(method: "GET", uri: "/{accountId}/todos/completed.json")
operation GetEverythingCompletedTodos {
  input: EverythingTodosFilterInput
  output: EverythingTodosGroupOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

/// Open, unassigned to-dos across all accessible projects, grouped by project (paginated).
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 5)
@http(method: "GET", uri: "/{accountId}/todos/unassigned.json")
operation GetEverythingUnassignedTodos {
  input: EverythingTodosFilterInput
  output: EverythingTodosGroupOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

/// Open to-dos with no due date across all accessible projects, grouped by project (paginated).
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 5)
@http(method: "GET", uri: "/{accountId}/todos/no_due_date.json")
operation GetEverythingNoDueDateTodos {
  input: EverythingTodosFilterInput
  output: EverythingTodosGroupOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

/// Incomplete cards in active columns across all accessible projects, grouped by project (paginated).
/// Each bucket entry carries the matching cards and their steps.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 5)
@http(method: "GET", uri: "/{accountId}/cards/open.json")
operation GetEverythingOpenCards {
  input: EverythingCardsFilterInput
  output: EverythingCardsGroupOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

/// Completed cards across all accessible projects, grouped by project (paginated).
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 5)
@http(method: "GET", uri: "/{accountId}/cards/completed.json")
operation GetEverythingCompletedCards {
  input: EverythingCardsFilterInput
  output: EverythingCardsGroupOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

/// Open, unassigned cards across all accessible projects, grouped by project (paginated).
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 5)
@http(method: "GET", uri: "/{accountId}/cards/unassigned.json")
operation GetEverythingUnassignedCards {
  input: EverythingCardsFilterInput
  output: EverythingCardsGroupOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

/// Open cards with no due date across all accessible projects, grouped by project (paginated).
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 5)
@http(method: "GET", uri: "/{accountId}/cards/no_due_date.json")
operation GetEverythingNoDueDateCards {
  input: EverythingCardsFilterInput
  output: EverythingCardsGroupOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

/// Cards parked in a project's "Not now" column across all accessible projects, grouped by project (paginated).
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 5)
@http(method: "GET", uri: "/{accountId}/cards/not_now.json")
operation GetEverythingNotNowCards {
  input: EverythingCardsFilterInput
  output: EverythingCardsGroupOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure EverythingTodosFilterInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Restrict to tasks assigned to at least one of the given people (repeatable).
  /// Assignees on nested steps are not considered.
  @httpQuery("assignee_ids[]")
  assignee_ids: PersonIdList

  /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
  @httpQuery("due")
  due: String

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure EverythingTodosGroupOutput {
  buckets: BucketTodosGroupList
}

structure EverythingCardsFilterInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Restrict to tasks assigned to at least one of the given people (repeatable).
  /// Assignees on nested steps are not considered.
  @httpQuery("assignee_ids[]")
  assignee_ids: PersonIdList

  /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
  @httpQuery("due")
  due: String

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure EverythingCardsGroupOutput {
  buckets: BucketCardsGroupList
}

// ===== Everything Bucket-Group Shapes =====

list BucketTodosGroupList {
  member: BucketTodosGroup
}

/// One project's slice of a filtered to-do listing: the parent project and the
/// matching to-dos (each carrying its steps).
structure BucketTodosGroup {
  @required
  bucket: RecordingBucket
  @required
  todos: TodoItems
}

list BucketCardsGroupList {
  member: BucketCardsGroup
}

/// One project's slice of a filtered card listing: the parent project and the
/// matching cards (each carrying its steps).
structure BucketCardsGroup {
  @required
  bucket: RecordingBucket
  @required
  cards: CardList
}

// ===== My Assignment Shapes =====

list MyAssignmentList {
  member: MyAssignment
}

structure MyAssignment {
  @required
  id: Long
  app_url: String
  content: String
  starts_on: ISO8601Date
  due_on: ISO8601Date
  bucket: MyAssignmentBucket
  completed: Boolean
  type: String
  assignees: MyAssignmentAssigneeList
  comments_count: Integer
  has_description: Boolean
  /// Present on priority items
  priority_recording_id: Long
  parent: MyAssignmentParent
  children: MyAssignmentList
}

structure MyAssignmentBucket {
  @required
  id: Long
  name: String
  app_url: String
}

structure MyAssignmentParent {
  @required
  id: Long
  title: String
  app_url: String
}

structure MyAssignmentAssignee {
  @required
  id: PersonId
  name: PersonName
  avatar_url: AvatarUrl
}

list MyAssignmentAssigneeList {
  member: MyAssignmentAssignee
}

// =============================================================================
// BATCH 16 - My Notifications
// =============================================================================

// ===== Notification Operations =====

/// Get the current user's notification inbox (the "Hey!" menu).
/// Notifications are grouped into unreads, reads, bubble-ups, and
/// scheduled bubble-ups (`memories` remains as an always-empty
/// placeholder on BC5). Reads are paginated (50 per page). Unreads are
/// capped at 100. Bubble-ups are capped per `limit_bubble_ups`.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/my/readings.json")
operation GetMyNotifications {
  input: GetMyNotificationsInput
  output: GetMyNotificationsOutput
  errors: [UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetMyNotificationsInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number for paginating through read items. Defaults to 1. This
  /// operation is not auto-paginated in any SDK, so a page is returned as
  /// asked for and later pages are not followed.
  @httpQuery("page")
  page: Integer

  /// Set to true to cap `bubble_ups` at 2 current bubble-ups and omit the
  /// `scheduled_bubble_ups` key entirely. Defaults to false. Use the dedicated
  /// bubble-ups endpoint (GetBubbleUps) to page through all current and
  /// scheduled bubble-ups.
  @httpQuery("limit_bubble_ups")
  limit_bubble_ups: Boolean
}

structure GetMyNotificationsOutput {
  unreads: NotificationList
  reads: NotificationList

  /// Total number of current bubble-ups, for notification UI counts
  /// (independent of the `limit_bubble_ups` cap on the `bubble_ups` array).
  @required
  bubble_ups_count: Integer

  /// Total number of scheduled bubble-ups, for notification UI counts
  /// (present even when `limit_bubble_ups` omits the `scheduled_bubble_ups`
  /// array).
  @required
  scheduled_bubble_ups_count: Integer

  /// Legacy "save forever" collection. Permanently `[]` on BC5 by documented
  /// contract (`doc/api/sections/my_notifications.md`, codified by BC3 #11628):
  /// an always-empty placeholder superseded by `bubble_ups`. BC4 (the `four`
  /// branch) still populates it — an accepted BC4→BC5 subtractive delta
  /// recorded in `spec/api-gaps/memories-emptied-regression.md`. New
  /// integrations should use `bubble_ups` / `scheduled_bubble_ups` and must
  /// not rely on `memories` on BC5.
  memories: NotificationList

  /// Items the user has saved with Bubble Up (BC5 addition). Roughly the
  /// successor to `memories` but with optional scheduling — see
  /// `scheduled_bubble_ups` for the time-deferred subset.
  bubble_ups: NotificationList

  /// Bubble Ups scheduled to resurface in the future (BC5 addition).
  scheduled_bubble_ups: NotificationList
}

/// Mark specified items as read
@idempotent
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/my/unreads.json")
operation MarkAsRead {
  input: MarkAsReadInput
  output: MarkAsReadOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure MarkAsReadInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Array of readable_sgid values identifying the items to mark as read
  @required
  readables: StringList
}

structure MarkAsReadOutput {}

/// Get the current user's current and scheduled bubble-ups (paginated, 50 per page).
/// Current bubble-ups are returned first, ordered by most recently bubbled up;
/// scheduled bubble-ups follow, ordered by scheduled bubble-up time. Each item
/// uses the same notification object shape as GetMyNotifications.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/my/readings/bubble_ups.json")
operation GetBubbleUps {
  input: GetBubbleUpsInput
  output: GetBubbleUpsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetBubbleUpsInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure GetBubbleUpsOutput {
  bubble_ups: NotificationList
}

// ===== Calendar Operations =====

/// Get a calendar by its bucket id. A Calendar is a top-level BC5 bucketable
/// (distinct from a project) exposing display metadata and a link to its
/// underlying schedule resource. Shipped scope is show + update only.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/calendars/{calendarId}")
operation GetCalendar {
  input: GetCalendarInput
  output: CalendarOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetCalendarInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  calendarId: Long
}

/// Update a calendar's display color. An unknown color returns 422 with a JSON
/// errors payload keyed by field ({"errors": {"color": ["is not a valid
/// color"]}}) — the controller rejects invalid enum values up front.
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/calendars/{calendarId}")
operation UpdateCalendar {
  input: UpdateCalendarInput
  output: CalendarOutput
  errors: [NotFoundError, FieldValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure UpdateCalendarInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  calendarId: Long

  @required
  calendar: CalendarAttributes
}

/// The writable calendar payload — the wire body is the nested
/// {calendar: {color}} envelope.
structure CalendarAttributes {
  /// One of: white, red, orange, yellow, green, blue, aqua, purple, gray,
  /// pink, brown.
  @required
  color: String
}

structure CalendarOutput {
  calendar: Calendar
}

/// A per-account calendar (wire type Calendar), keyed by its own bucket id.
structure Calendar {
  @required
  id: Long
  @required
  type: String
  @required
  name: String
  /// One of: white, red, orange, yellow, green, blue, aqua, purple, gray,
  /// pink, brown.
  @required
  color: String
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  url: String
  @required
  app_url: String
  /// API URL of the calendar's underlying schedule resource.
  @required
  schedule_url: String
}

// ===== My Notes (Scratchpad) Operations =====

/// Get the authenticated user's note — a per-person notebook singleton at
/// /my/notes.json. If the user has not yet written anything, the shape is the
/// same with empty content and null id/created_at/updated_at; the record is
/// created on first update.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/my/notes.json")
operation GetMyNote {
  input: GetMyNoteInput
  output: MyNoteOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetMyNoteInput {
  @required
  @httpLabel
  accountId: AccountId
}

/// Replace the note's content, recording a new revision server-side.
/// The first update also creates the underlying notebook if the user did not
/// have one yet. Returns the updated note. Rejections arrive as a field-keyed
/// 422 ({"errors": {"content": ["can't be blank"]}}), not the flat {error}
/// body.
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/my/notes.json")
operation UpdateMyNote {
  input: UpdateMyNoteInput
  output: MyNoteOutput
  errors: [FieldValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure UpdateMyNoteInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  note: MyNoteAttributes
}

/// The writable note payload — the wire body is the nested {note: {content}}
/// envelope, the ProjectConstructionAttributes treatment.
structure MyNoteAttributes {
  /// The note's rich-text body (HTML).
  @required
  content: String
}

structure MyNoteOutput {
  note: MyNote
}

/// The per-user notebook note (wire type Notebook::Note). Before the first
/// write, id/created_at/updated_at are present-but-null (required-nullable,
/// layered in the OpenAPI via jsonAdd) and content is empty.
structure MyNote {
  /// Null until the note is first written.
  @required
  id: Long
  @required
  type: String
  /// Null until the note is first written.
  @required
  created_at: ISO8601Timestamp
  /// Null until the note is first written.
  @required
  updated_at: ISO8601Timestamp
  @required
  content: String
  @required
  content_attachments: RichTextAttachmentList
  @required
  url: String
  @required
  app_url: String
}

// ===== My Drafts Operations =====

/// List the current user's drafts across their active projects, most recently
/// updated first (paginated, capped at 250 like /my/assignments). Five draft
/// kinds are returned: messages, documents, uploads, client approvals, and
/// client correspondences. Drafts under archived or trashed projects are
/// excluded.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/my/drafts.json")
operation ListMyDrafts {
  input: ListMyDraftsInput
  output: ListMyDraftsOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListMyDraftsInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListMyDraftsOutput {
  drafts: DraftList
}

list DraftList {
  member: Draft
}

/// A draft envelope: a message, document, upload, or client approval/
/// correspondence saved but not yet published. Flat and purpose-built —
/// NOT the shared recording projection.
structure Draft {
  @required
  id: Long
  @required
  app_url: String
  @required
  title: String
  /// Short recordable name: message, document, upload, client_approval,
  /// or client_correspondence.
  @required
  type: String
  @required
  bucket: DraftBucket
  /// Parent recording the draft is filed under. Always present on the wire,
  /// `null` for drafts filed directly under their bucket. `@required` models
  /// the presence; value nullability is layered in the OpenAPI
  /// (smithy-build.json jsonAdd) — the Wormhole.destination_url treatment.
  @required
  parent: DraftParent
  /// Up to 300 characters of plain text; empty string when the draft has no body.
  @required
  excerpt: String
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  /// Always present; `null` unless the draft is scheduled to publish later.
  /// Required-presence with nullable value, like `parent`.
  @required
  scheduled_posting_at: ISO8601Timestamp
}

/// The project a draft lives in (drafts-specific projection: id, name, app_url).
structure DraftBucket {
  @required
  id: Long
  @required
  name: String
  @required
  app_url: String
}

/// The parent recording a draft is filed under (id, title, app_url).
structure DraftParent {
  @required
  id: Long
  @required
  title: String
  @required
  app_url: String
}

// ===== My Bookmarks Operations =====

/// List the current user's bookmarks, most recently bookmarked first (paginated).
/// A bookmark is a personal link between the current user and a single recording,
/// visible only to its creator; each entry wraps the shared recording projection.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/my/bookmarks.json")
operation ListMyBookmarks {
  input: ListMyBookmarksInput
  output: ListMyBookmarksOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListMyBookmarksInput {
  @required
  @httpLabel
  accountId: AccountId

  /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
  @httpQuery("page")
  page: Integer
}

structure ListMyBookmarksOutput {
  bookmarks: BookmarkList
}

list BookmarkList {
  member: Bookmark
}

/// A personal bookmark: the current user's link to a single recording.
/// The wrapped recording is the shared recording projection, whose `parent`
/// is optional (docked recordings and doors omit it).
structure Bookmark {
  @required
  id: Long
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp
  @required
  recording: Recording
}

/// Report whether the current user has bookmarked the recording.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/recordings/{recordingId}/bookmark.json")
operation GetBookmark {
  input: GetBookmarkInput
  output: GetBookmarkOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetBookmarkInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure GetBookmarkOutput {
  bookmark_status: BookmarkStatus
}

/// The current user's bookmark state for one recording.
structure BookmarkStatus {
  @required
  bookmarked: Boolean
}

/// Bookmark a recording for the current user.
/// Idempotent: re-bookmarking returns the existing bookmark, never a duplicate.
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "POST", uri: "/{accountId}/recordings/{recordingId}/bookmark.json", code: 201)
operation CreateBookmark {
  input: CreateBookmarkInput
  output: CreateBookmarkOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateBookmarkInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure CreateBookmarkOutput {
  bookmark: Bookmark
}

/// Remove the current user's bookmark from a recording (returns 204 No Content).
/// Idempotent: deleting an absent bookmark also returns 204.
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/recordings/{recordingId}/bookmark.json", code: 204)
operation DeleteBookmark {
  input: DeleteBookmarkInput
  output: DeleteBookmarkOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure DeleteBookmarkInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure DeleteBookmarkOutput {}

// ===== Notification Shapes =====

list NotificationList {
  member: Notification
}

list StringList {
  member: String
}

structure Notification {
  @required
  id: Long
  @required
  created_at: ISO8601Timestamp
  @required
  updated_at: ISO8601Timestamp

  /// The notification category: `inbox`, `chats`, `pings`, `bubbles`,
  /// or `mentions`.
  section: String
  unread_count: Integer
  unread_at: ISO8601Timestamp
  read_at: ISO8601Timestamp
  readable_sgid: String
  readable_identifier: String
  title: String
  type: String
  bucket_name: String
  creator: Person
  content_excerpt: String
  app_url: String
  unread_url: String
  bookmark_url: String
  memory_url: String

  /// URL for the Bubble Up record covering this notification (BC5 addition).
  /// Eligibility-gated — only present on items the current user can bubble up.
  bubble_up_url: String

  /// Scheduled resurfacing time when this item is queued as a scheduled
  /// Bubble Up (BC5 addition). Absent when there is no scheduled time.
  bubble_up_at: ISO8601Timestamp

  subscription_url: String
  subscribed: Boolean
  previewable_attachments: PreviewableAttachmentList
  /// Present on ping notifications
  participants: PersonList
  /// Whether the ping has a custom name (pings only)
  named: Boolean
  /// Custom image URL (pings only)
  image_url: String
}

list PreviewableAttachmentList {
  member: PreviewableAttachment
}

structure PreviewableAttachment {
  id: Long
  url: String
  app_url: String
  content_type: String
  filename: String
  filesize: Long
  width: Integer
  height: Integer
}

// =============================================================================
// BATCH 17 - Out of Office
// =============================================================================

// ===== Out of Office Operations =====

/// Get the out of office status for a person
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/people/{personId}/out_of_office.json")
operation GetOutOfOffice {
  input: GetOutOfOfficeInput
  output: GetOutOfOfficeOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetOutOfOfficeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  personId: PersonId
}

structure GetOutOfOfficeOutput {

  outOfOffice: OutOfOffice
}

/// Enable or replace out of office for a person.
/// Admins on Pro Pack accounts can manage others; otherwise self only.
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/people/{personId}/out_of_office.json")
operation EnableOutOfOffice {
  input: EnableOutOfOfficeInput
  output: EnableOutOfOfficeOutput
  errors: [ValidationError, ForbiddenError, UnauthorizedError, RateLimitError, InternalServerError]
}

structure EnableOutOfOfficeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  personId: PersonId

  @required
  out_of_office: OutOfOfficePayload
}

structure OutOfOfficePayload {
  /// Start date in ISO 8601 format (YYYY-MM-DD)
  @required
  start_date: ISO8601Date

  /// End date in ISO 8601 format (YYYY-MM-DD)
  @required
  end_date: ISO8601Date
}

structure EnableOutOfOfficeOutput {

  outOfOffice: OutOfOffice
}

/// Disable out of office for a person.
/// Admins on Pro Pack accounts can manage others; otherwise self only.
@idempotent
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/people/{personId}/out_of_office.json", code: 204)
operation DisableOutOfOffice {
  input: DisableOutOfOfficeInput
  output: DisableOutOfOfficeOutput
  errors: [ForbiddenError, UnauthorizedError, RateLimitError, InternalServerError]
}

structure DisableOutOfOfficeInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  personId: PersonId
}

structure DisableOutOfOfficeOutput {}

// ===== Out of Office Shapes =====

/// When out of office is not enabled, `enabled` is `false` and
/// `start_date`, `end_date`, and `back_on_date` are omitted.
structure OutOfOffice {
  person: OutOfOfficePerson
  enabled: Boolean
  ongoing: Boolean
  start_date: ISO8601Date
  end_date: ISO8601Date

  /// First working day after the out-of-office window ends.
  /// Omitted when out of office is not enabled.
  back_on_date: ISO8601Date
}

structure OutOfOfficePerson {
  @required
  id: PersonId
  name: PersonName
  avatar_url: AvatarUrl
}

// =============================================================================
// BATCH 18 - People (Profile & Preferences)
// =============================================================================

// ===== Profile & Preferences Operations =====

/// Get the current user's preferences
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/my/preferences.json")
operation GetMyPreferences {
  input: GetMyPreferencesInput
  output: GetMyPreferencesOutput
  errors: [UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetMyPreferencesInput {
  @required
  @httpLabel
  accountId: AccountId
}

structure GetMyPreferencesOutput {

  preferences: Preferences
}

/// Update the current user's preferences.
/// Rejections arrive as a field-keyed 422
/// ({"errors": {"time_zone_name": ["is not included in the list"]}}), not the
/// flat {error} body.
@idempotent
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/my/preferences.json")
operation UpdateMyPreferences {
  input: UpdateMyPreferencesInput
  output: UpdateMyPreferencesOutput
  errors: [FieldValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure UpdateMyPreferencesInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  person: PreferencesPayload
}

structure PreferencesPayload {
  /// Time zone name. Accepts any valid Rails time zone name (e.g.
  /// "London", "UTC") as well as IANA identifiers (e.g.
  /// "America/Chicago"), which are normalized to the matching
  /// Rails-style name before saving.
  time_zone_name: String

  /// First day of the week: Sunday, Monday, Tuesday, etc.
  first_week_day: String

  /// Time display format: twelve_hour or twenty_four_hour
  time_format: String
}

structure UpdateMyPreferencesOutput {

  preferences: Preferences
}

// ===== Preferences Shapes =====

structure Preferences {
  url: String
  app_url: String

  /// Returned as a Rails-style name (e.g. "Central Time (US & Canada)").
  time_zone_name: String
  first_week_day: String
  time_format: String
}


// ===== Folders Operations =====
//
// Product noun vs. wire noun: the product calls these **folders**, the wire
// still says **stack**. Both spellings are load-bearing and neither is a typo —
// the operations, structures and generated methods use `Folder`, while the URI
// segment (`/stacks.json`) and the `type` discriminator (`"Stack"`) keep the
// original name. Anything matching on `type` must match `"Stack"`.
//
// Folders are per-user: they group projects on one person's home screen, and
// filing a project away for yourself changes nothing for anyone else. There is
// no account-wide folder, which is why the collection is flat rather than
// bucket-scoped.

/// List the authenticated user's folders in home-screen order.
///
/// Returns a bare array with no pagination envelope. Items are the base folder
/// shape: they carry `bucket_ids` but **not** the expanded `projects`, which
/// only the single-folder operations return.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/stacks.json")
operation ListFolders {
  input: ListFoldersInput
  output: ListFoldersOutput
  errors: [UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure ListFoldersInput {
  @required
  @httpLabel
  accountId: AccountId
}

structure ListFoldersOutput {
  folders: FolderList
}

/// Get one folder, with the projects grouped inside it expanded under `projects`.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/stacks/{folderId}")
operation GetFolder {
  input: GetFolderInput
  output: GetFolderOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure GetFolderInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  folderId: FolderId
}

structure GetFolderOutput {
  folder: FolderWithProjects
}

/// Create a folder for the authenticated user and file the given projects into it.
///
/// Returns 201 with the new folder and its expanded `projects`, placed at the
/// top of the home screen. Filing an all-access project the user has not joined
/// **grants** them access to it. Every id is preflighted: if any is archived,
/// trashed, or an invitation-only project the user is not on, the whole request
/// fails with 404 and nothing is created — there is no partial success.
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/stacks.json", code: 201)
operation CreateFolder {
  input: CreateFolderInput
  output: CreateFolderOutput
  errors: [NotFoundError, FieldValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateFolderInput {
  @required
  @httpLabel
  accountId: AccountId

  /// The folder's name. Defaults to `New folder` when blank, null, or omitted.
  name: String

  /// IDs of the projects to file into the folder — the same ids the folder
  /// reports back as `bucket_ids` and expands as `projects`. This does not
  /// round-trip under its own name. Omit it, or send null or an empty array,
  /// for an empty folder.
  project_ids: ProjectIdList
}

structure CreateFolderOutput {
  folder: FolderWithProjects
}

/// Rename a folder.
///
/// `name` is the only writable attribute; a folder's projects, ordering, and
/// image are managed elsewhere and an image parameter sent here is ignored.
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/stacks/{folderId}")
operation UpdateFolder {
  input: UpdateFolderInput
  output: UpdateFolderOutput
  errors: [NotFoundError, FieldValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateFolderInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  folderId: FolderId

  /// The folder's new name. Blank is rejected with 422 — unlike create, update
  /// does not fall back to a default name.
  @required
  name: String
}

structure UpdateFolderOutput {
  folder: FolderWithProjects
}

/// Delete a folder and unpin its projects from the home screen (returns 204 No Content).
///
/// The projects themselves are not deleted and are not moved back out onto the
/// home screen; they simply stop appearing there until pinned again.
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/stacks/{folderId}", code: 204)
operation DeleteFolder {
  input: DeleteFolderInput
  output: DeleteFolderOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure DeleteFolderInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  folderId: FolderId
}

structure DeleteFolderOutput {}

// ===== Folder Shapes =====

long FolderId

list ProjectIdList {
  member: ProjectId
}

list FolderList {
  member: Folder
}

/// A folder as the list returns it: the base shape, without expanded projects.
///
/// Deliberately distinct from FolderWithProjects. A single shape with an
/// optional `projects` member would make every list item declare a field the
/// list response never populates.
structure Folder {
  @required
  id: FolderId

  @required
  name: String

  /// Always the string `Stack` — the wire type kept its pre-rename name.
  @required
  type: String

  @required
  created_at: ISO8601Timestamp

  @required
  updated_at: ISO8601Timestamp

  /// IDs of the projects filed into this folder. Same ids as `project_ids` on
  /// create, and the ids FolderWithProjects expands under `projects`.
  @required
  bucket_ids: ProjectIdList

  @required
  is_emoji_only_name: Boolean

  @required
  star_url: String

  /// Gauges URL covering this folder's projects; always emitted, `null` when
  /// none of them is gauged. `@required` models the presence — the nullability
  /// is layered on in the OpenAPI (smithy-build.json jsonAdd -> type:
  /// ["string","null"]), so Go types it *string because the value is nullable,
  /// not because the key is optional.
  @required
  gauges_url: String

  /// The viewer's colour customization for this folder; always emitted, `null`
  /// when unset. Required-and-nullable, like gauges_url.
  @required
  color: String

  /// The viewer's folder image; always emitted, `null` when unset. Read-only:
  /// there is no image create or update in v1. Required-and-nullable, like
  /// gauges_url.
  @required
  image_url: String

  @required
  url: String
}

/// One folder plus the projects grouped inside it, as get/create/update return it.
///
/// The `projects` entries are the shared project projection, minus the
/// `bookmarked` flag that only the projects index adds.
structure FolderWithProjects {
  @required
  id: FolderId

  @required
  name: String

  /// Always the string `Stack` — the wire type kept its pre-rename name.
  @required
  type: String

  @required
  created_at: ISO8601Timestamp

  @required
  updated_at: ISO8601Timestamp

  /// IDs of the projects filed into this folder — the same set `projects`
  /// expands.
  @required
  bucket_ids: ProjectIdList

  @required
  is_emoji_only_name: Boolean

  @required
  star_url: String

  /// Gauges URL covering this folder's projects; always emitted, `null` when
  /// none of them is gauged. Required-and-nullable (see Folder.gauges_url).
  @required
  gauges_url: String

  /// The viewer's colour customization for this folder; always emitted, `null`
  /// when unset. Required-and-nullable.
  @required
  color: String

  /// The viewer's folder image; always emitted, `null` when unset. Read-only.
  /// Required-and-nullable.
  @required
  image_url: String

  @required
  url: String

  /// The projects filed into this folder, expanded. Always emitted; empty for
  /// an empty folder.
  @required
  projects: ProjectList
}
