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
use basecamp.traits#basecampSensitive

/// Basecamp API
@restJson1
service Basecamp {
  version: "2026-01-26"
  operations: [
    ListProjects,
    GetProject,
    CreateProject,
    UpdateProject,
    TrashProject,
    ListTodos,
    GetTodo,
    CreateTodo,
    UpdateTodo,
    TrashTodo,
    CompleteTodo,
    UncompleteTodo,
    RepositionTodo,
    GetTodoset,
    ListTodolists,
    GetTodolistOrGroup,
    CreateTodolist,
    UpdateTodolistOrGroup,
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
    UpdateDocument,
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

    // Batch 4 - Campfires, Chatbots, Forwards/Inboxes (Real-time)
    ListCampfires,
    GetCampfire,
    ListCampfireLines,
    GetCampfireLine,
    CreateCampfireLine,
    DeleteCampfireLine,
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
    CreateForwardReply,

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
    CreateCardStep,
    UpdateCardStep,
    CompleteCardStep,
    UncompleteCardStep,
    RepositionCardStep,

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
    GetRecording,
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
    CloneTool,
    UpdateTool,
    DeleteTool,
    EnableTool,
    DisableTool,
    RepositionTool,
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
    GetUpcomingSchedule
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
  id: ProjectId
  status: ProjectStatus
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  name: ProjectName
  description: ProjectDescription
  purpose: String
  clients_enabled: Boolean
  bookmark_url: String
  url: String
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
  id: Long
  title: String
  name: String
  enabled: Boolean
  position: Integer
  url: String
  app_url: String
}

structure ClientCompany {
  id: Long
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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/todolists/{todolistId}/todos.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  todolistId: TodolistId

  @httpQuery("status")
  status: TodoStatus

  @httpQuery("completed")
  completed: Boolean
}

structure ListTodosOutput {

  todos: TodoItems
}

/// Get a single todo by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/todos/{todoId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  todoId: TodoId
}

structure GetTodoOutput {

  todo: Todo
}

/// Create a new todo in a todolist
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/todolists/{todolistId}/todos.json", code: 201)
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
  projectId: ProjectId

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

/// Update an existing todo
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/todos/{todoId}")
operation UpdateTodo {
  input: UpdateTodoInput
  output: UpdateTodoOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateTodoInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  todoId: TodoId

  content: TodoContent
  description: TodoDescription
  assignee_ids: PersonIdList
  completion_subscriber_ids: PersonIdList
  notify: Boolean
  due_on: ISO8601Date
  starts_on: ISO8601Date
}

structure UpdateTodoOutput {

  todo: Todo
}

/// Trash a todo (returns 204 No Content)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/todos/{todoId}")
operation TrashTodo {
  input: TrashTodoInput
  output: TrashTodoOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure TrashTodoInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  todoId: TodoId
}

structure TrashTodoOutput {}

/// Mark a todo as complete
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/todos/{todoId}/completion.json", code: 204)
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
  projectId: ProjectId

  @required
  @httpLabel
  todoId: TodoId
}

structure CompleteTodoOutput {}

/// Mark a todo as incomplete
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/todos/{todoId}/completion.json", code: 204)
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
  projectId: ProjectId

  @required
  @httpLabel
  todoId: TodoId
}

structure UncompleteTodoOutput {}

/// Reposition a todo within its todolist
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/todos/{todoId}/position.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  todoId: TodoId

  @required
  position: Integer
}

structure RepositionTodoOutput {}

// ===== Todoset Operations =====

/// Get a todoset (container for todolists in a project)
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/todosets/{todosetId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  todosetId: TodosetId
}

structure GetTodosetOutput {

  todoset: Todoset
}

// ===== Todolist Operations =====

/// List todolists in a todoset
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/todosets/{todosetId}/todolists.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  todosetId: TodosetId

  @httpQuery("status")
  status: TodolistStatus
}

structure ListTodolistsOutput {

  todolists: TodolistList
}

/// Get a single todolist or todolist group by id
/// The endpoint is polymorphic - the same URI returns either a Todolist or TodolistGroup
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/todolists/{id}")
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
  projectId: ProjectId

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
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/todosets/{todosetId}/todolists.json", code: 201)
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
  projectId: ProjectId

  @required
  @httpLabel
  todosetId: TodosetId

  @required
  name: TodolistName

  description: TodolistDescription
}

structure CreateTodolistOutput {

  todolist: Todolist
}

/// Update an existing todolist or todolist group
/// The endpoint is polymorphic - updates either a Todolist or TodolistGroup
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/todolists/{id}")
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
  projectId: ProjectId

  @required
  @httpLabel
  id: Long

  /// Name (required for both Todolist and TodolistGroup)
  name: TodolistName

  /// Description (Todolist only, ignored for groups)
  description: TodolistDescription
}

structure UpdateTodolistOrGroupOutput {

  result: TodolistOrGroup
}

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/todolists/{todolistId}/groups.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  todolistId: TodolistId
}

structure ListTodolistGroupsOutput {

  groups: TodolistGroupList
}

/// Create a new group in a todolist
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/todolists/{todolistId}/groups.json", code: 201)
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
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/todolists/{groupId}/position.json")
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
  projectId: ProjectId

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
  id: TodoId
  status: TodoStatus
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
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
  position: Integer
  parent: TodoParent
  bucket: TodoBucket
  creator: Person
  description: TodoDescription
  completed: Boolean
  content: TodoContent
  starts_on: ISO8601Date
  due_on: ISO8601Date
  assignees: PersonList
  completion_subscribers: PersonList
  completion_url: String
}

structure TodoParent {
  id: TodolistId
  title: String
  type: String
  url: String
  app_url: String
}

structure TodoBucket {
  id: ProjectId
  name: String
  type: String
}

structure Person {
  id: PersonId
  attachable_sgid: String

  @basecampSensitive(category: "pii", redact: true)
  name: PersonName

  @basecampSensitive(category: "pii", redact: true)
  email_address: EmailAddress

  personable_type: String

  @basecampSensitive(category: "pii", redact: false)
  title: PersonTitle

  @basecampSensitive(category: "pii", redact: false)
  bio: PersonBio

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
}

structure PersonCompany {
  id: Long
  name: CompanyName
}

list PersonList {
  member: Person
}

// ===== Todoset Shapes =====

long TodosetId
string TodosetName

structure Todoset {
  id: TodosetId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  position: Integer
  bucket: TodoBucket
  creator: Person
  name: TodosetName
  todolists_count: Integer
  todolists_url: String
  completed_ratio: String
  completed: Boolean
  completed_count: Integer
  on_schedule_count: Integer
  over_schedule_count: Integer
  app_todolists_url: String
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
  id: TodolistId
  status: TodolistStatus
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
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
  position: Integer
  parent: TodoParent
  bucket: TodoBucket
  creator: Person
  description: TodolistDescription
  completed: Boolean
  completed_ratio: String
  name: TodolistName
  todos_url: String
  groups_url: String
  app_todos_url: String
}

// ===== Todolist Group Shapes =====

long TodolistGroupId
string TodolistGroupName

list TodolistGroupList {
  member: TodolistGroup
}

structure TodolistGroup {
  id: TodolistGroupId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
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
  position: Integer
  parent: TodoParent
  bucket: TodoBucket
  creator: Person
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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/comments.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure ListCommentsOutput {

  comments: CommentList
}

/// Get a single comment by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/comments/{commentId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  commentId: CommentId
}

structure GetCommentOutput {

  comment: Comment
}

/// Create a new comment on a recording
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/comments.json", code: 201)
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
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/comments/{commentId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/message_boards/{boardId}/messages.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  boardId: MessageBoardId
}

structure ListMessagesOutput {

  messages: MessageList
}

/// Get a single message by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/messages/{messageId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  messageId: MessageId
}

structure GetMessageOutput {

  message: Message
}

/// Create a new message on a message board
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/message_boards/{boardId}/messages.json", code: 201)
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
  projectId: ProjectId

  @required
  @httpLabel
  boardId: MessageBoardId

  @required
  subject: MessageSubject

  content: MessageContent

  @documentation("active|drafted")
  status: String

  category_id: MessageTypeId
}

structure CreateMessageOutput {

  message: Message
}

/// Update an existing message
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/messages/{messageId}")
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
  projectId: ProjectId

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
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/recordings/{messageId}/pin.json", code: 204)
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
  projectId: ProjectId

  @required
  @httpLabel
  messageId: MessageId
}

structure PinMessageOutput {}

/// Unpin a message from the message board
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/recordings/{messageId}/pin.json", code: 204)
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/message_boards/{boardId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/categories.json")
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
  projectId: ProjectId
}

structure ListMessageTypesOutput {

  message_types: MessageTypeList
}

/// Get a single message type by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/categories/{typeId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  typeId: MessageTypeId
}

structure GetMessageTypeOutput {

  message_type: MessageType
}

/// Create a new message type in a project
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/categories.json", code: 201)
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
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/categories/{typeId}")
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
  projectId: ProjectId

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
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/categories/{typeId}", code: 204)
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/vaults/{vaultId}/vaults.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  vaultId: VaultId
}

structure ListVaultsOutput {

  vaults: VaultList
}

/// Get a single vault by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/vaults/{vaultId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  vaultId: VaultId
}

structure GetVaultOutput {

  vault: Vault
}

/// Create a new vault (subfolder) in a vault
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/vaults/{vaultId}/vaults.json", code: 201)
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
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/vaults/{vaultId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/vaults/{vaultId}/documents.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  vaultId: VaultId
}

structure ListDocumentsOutput {

  documents: DocumentList
}

/// Get a single document by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/documents/{documentId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  documentId: DocumentId
}

structure GetDocumentOutput {

  document: Document
}

/// Create a new document in a vault
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/vaults/{vaultId}/documents.json", code: 201)
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
  projectId: ProjectId

  @required
  @httpLabel
  vaultId: VaultId

  @required
  title: DocumentTitle

  content: DocumentContent

  @documentation("active|drafted")
  status: String
}

structure CreateDocumentOutput {

  document: Document
}

/// Update an existing document
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/documents/{documentId}")
operation UpdateDocument {
  input: UpdateDocumentInput
  output: UpdateDocumentOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateDocumentInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  documentId: DocumentId

  title: DocumentTitle
  content: DocumentContent
}

structure UpdateDocumentOutput {

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/vaults/{vaultId}/uploads.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  vaultId: VaultId
}

structure ListUploadsOutput {

  uploads: UploadList
}

/// Get a single upload by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/uploads/{uploadId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  uploadId: UploadId
}

structure GetUploadOutput {

  upload: Upload
}

/// Create a new upload in a vault
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/vaults/{vaultId}/uploads.json", code: 201)
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
  projectId: ProjectId

  @required
  @httpLabel
  vaultId: VaultId

  @required
  attachable_sgid: AttachableSgid

  description: UploadDescription
  base_name: UploadBaseName
}

structure CreateUploadOutput {

  upload: Upload
}

/// Update an existing upload
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/uploads/{uploadId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/uploads/{uploadId}/versions.json")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/schedules/{scheduleId}")
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
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/schedules/{scheduleId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/schedules/{scheduleId}/entries.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  scheduleId: ScheduleId

  @httpQuery("status")
  status: ScheduleEntryStatus
}

structure ListScheduleEntriesOutput {

  entries: ScheduleEntryList
}

/// Get a single schedule entry by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/schedule_entries/{entryId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/schedule_entries/{entryId}/occurrences/{date}")
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
  projectId: ProjectId

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
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/schedules/{scheduleId}/entries.json", code: 201)
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
  projectId: ProjectId

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
}

structure CreateScheduleEntryOutput {

  entry: ScheduleEntry
}

/// Update an existing schedule entry
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/schedule_entries/{entryId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  entryId: ScheduleEntryId

  summary: ScheduleEntrySummary
  starts_at: ISO8601Timestamp
  ends_at: ISO8601Timestamp
  description: ScheduleEntryDescription
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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/timesheet.json")
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
}

structure GetProjectTimesheetOutput {

  entries: TimesheetEntryList
}

/// Get timesheet for a specific recording
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/timesheet.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId

  @httpQuery("from")
  from: ISO8601Date

  @httpQuery("to")
  to: ISO8601Date

  @httpQuery("person_id")
  person_id: PersonId
}

structure GetRecordingTimesheetOutput {

  entries: TimesheetEntryList
}

// ===== Comment Shapes (Batch 1) =====

long CommentId
long RecordingId
string CommentContent

list CommentList {
  member: Comment
}

structure Comment {
  id: CommentId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  parent: RecordingParent
  bucket: TodoBucket
  creator: Person
  content: CommentContent
}

structure RecordingParent {
  id: Long
  title: String
  type: String
  url: String
  app_url: String
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
  id: MessageId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
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
  parent: RecordingParent
  bucket: TodoBucket
  creator: Person
  subject: MessageSubject
  content: MessageContent
  category: MessageType
}

structure MessageBoard {
  id: MessageBoardId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  position: Integer
  bucket: TodoBucket
  creator: Person
  messages_count: Integer
  messages_url: String
  app_messages_url: String
}

list MessageTypeList {
  member: MessageType
}

structure MessageType {
  id: MessageTypeId
  name: MessageTypeName
  icon: MessageTypeIcon
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
}

// ===== Vault Shapes (Batch 2) =====

long VaultId
string VaultTitle

list VaultList {
  member: Vault
}

structure Vault {
  id: VaultId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: VaultTitle
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  position: Integer
  parent: RecordingParent
  bucket: TodoBucket
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
  id: DocumentId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: DocumentTitle
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  subscription_url: String
  comments_count: Integer
  comments_url: String
  position: Integer
  parent: RecordingParent
  bucket: TodoBucket
  creator: Person
  content: DocumentContent
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
  id: UploadId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
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
  position: Integer
  parent: RecordingParent
  bucket: TodoBucket
  creator: Person
  description: UploadDescription
  content_type: String
  byte_size: Long
  width: Integer
  height: Integer
  download_url: String
  filename: String
}

// ===== Schedule Shapes (Batch 3) =====

long ScheduleId
long ScheduleEntryId
string ScheduleEntrySummary
string ScheduleEntryDescription

@documentation("active|archived|trashed")
string ScheduleEntryStatus

structure Schedule {
  id: ScheduleId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  position: Integer
  bucket: TodoBucket
  creator: Person
  include_due_assignments: Boolean
  entries_count: Integer
  entries_url: String
}

list ScheduleEntryList {
  member: ScheduleEntry
}

structure ScheduleEntry {
  id: ScheduleEntryId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
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
  parent: RecordingParent
  bucket: TodoBucket
  creator: Person
  summary: ScheduleEntrySummary
  description: ScheduleEntryDescription
  all_day: Boolean
  starts_at: ISO8601Timestamp
  ends_at: ISO8601Timestamp
  participants: PersonList
}

// ===== Timesheet Shapes (Batch 3) =====

long TimesheetEntryId

list TimesheetEntryList {
  member: TimesheetEntry
}

structure TimesheetEntry {
  id: TimesheetEntryId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  parent: RecordingParent
  bucket: TodoBucket
  creator: Person
  date: ISO8601Date
  description: String
  hours: String
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
}

structure ListCampfiresOutput {

  campfires: CampfireList
}

/// Get a campfire by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/chats/{campfireId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/chats/{campfireId}/lines.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  campfireId: CampfireId
}

structure ListCampfireLinesOutput {

  lines: CampfireLineList
}

/// Get a campfire line by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/chats/{campfireId}/lines/{lineId}")
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
  projectId: ProjectId

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
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/chats/{campfireId}/lines.json", code: 201)
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
  projectId: ProjectId

  @required
  @httpLabel
  campfireId: CampfireId

  @required
  content: String
}

structure CreateCampfireLineOutput {

  line: CampfireLine
}

/// Delete a campfire line
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/chats/{campfireId}/lines/{lineId}", code: 204)
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
  projectId: ProjectId

  @required
  @httpLabel
  campfireId: CampfireId

  @required
  @httpLabel
  lineId: CampfireLineId
}

structure DeleteCampfireLineOutput {}

// ===== Chatbot Operations =====

/// List all chatbots for a campfire
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages. X-Total-Count header provides total count.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", totalCountHeader: "X-Total-Count", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations.json")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}")
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
  projectId: ProjectId

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
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations.json", code: 201)
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
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}")
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
  projectId: ProjectId

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
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}", code: 204)
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/inboxes/{inboxId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/inboxes/{inboxId}/forwards.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  inboxId: InboxId
}

structure ListForwardsOutput {

  forwards: ForwardList
}

/// Get a forward by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  forwardId: ForwardId
}

structure ListForwardRepliesOutput {

  replies: ForwardReplyList
}

/// Get a forward reply by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}/replies/{replyId}")
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
  projectId: ProjectId

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

/// Create a reply to a forward
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json", code: 201)
operation CreateForwardReply {
  input: CreateForwardReplyInput
  output: CreateForwardReplyOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateForwardReplyInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  forwardId: ForwardId

  @required
  content: String
}

structure CreateForwardReplyOutput {

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
  id: CampfireId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  subscription_url: String
  position: Integer
  bucket: TodoBucket
  creator: Person
  topic: String
  lines_url: String
}

list CampfireLineList {
  member: CampfireLine
}

structure CampfireLine {
  id: CampfireLineId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  content: String
  parent: RecordingParent
  bucket: TodoBucket
  creator: Person
}

list ChatbotList {
  member: Chatbot
}

structure Chatbot {
  id: ChatbotId
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
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
  id: InboxId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  position: Integer
  bucket: TodoBucket
  creator: Person
  forwards_count: Integer
  forwards_url: String
}

list ForwardList {
  member: Forward
}

structure Forward {
  id: ForwardId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  subscription_url: String
  parent: RecordingParent
  bucket: TodoBucket
  creator: Person
  content: String
  subject: String
  from: String
  replies_count: Integer
  replies_url: String
}

list ForwardReplyList {
  member: ForwardReply
}

structure ForwardReply {
  id: ForwardReplyId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  parent: RecordingParent
  bucket: TodoBucket
  creator: Person
  content: String
}

// =============================================================================
// BATCH 5: CardTables, Cards, CardColumns, CardSteps (Kanban)
// =============================================================================

// ===== CardTable Operations =====

/// Get a card table by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/card_tables/{cardTableId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/card_tables/lists/{columnId}/cards.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure ListCardsOutput {

  cards: CardList
}

/// Get a card by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  cardId: CardId
}

structure GetCardOutput {

  card: Card
}

/// Create a card in a column
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/card_tables/lists/{columnId}/cards.json", code: 201)
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
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}")
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
  projectId: ProjectId

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
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}/moves.json", code: 204)
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
  projectId: ProjectId

  @required
  @httpLabel
  cardId: CardId

  @required
  column_id: CardColumnId
}

structure MoveCardOutput {}

// Note: Use TrashRecording to trash cards

// ===== CardColumn Operations =====

/// Get a card column by ID
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure GetCardColumnOutput {

  column: CardColumn
}

/// Create a column in a card table
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/card_tables/{cardTableId}/columns.json", code: 201)
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
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}")
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
  projectId: ProjectId

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
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/card_tables/{cardTableId}/moves.json", code: 204)
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
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}/color.json")
operation SetCardColumnColor {
  input: SetCardColumnColorInput
  output: SetCardColumnColorOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure SetCardColumnColorInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

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
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json")
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
  projectId: ProjectId

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
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json")
operation DisableCardColumnOnHold {
  input: DisableCardColumnOnHoldInput
  output: DisableCardColumnOnHoldOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure DisableCardColumnOnHoldInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

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
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/card_tables/lists/{columnId}/subscription.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure SubscribeToCardColumnOutput {}

/// Unsubscribe from a card column (stop watching for changes)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/card_tables/lists/{columnId}/subscription.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure UnsubscribeFromCardColumnOutput {}

// ===== CardStep Operations =====

/// Create a step on a card
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}/steps.json", code: 201)
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
  projectId: ProjectId

  @required
  @httpLabel
  cardId: CardId

  @required
  title: String

  due_on: ISO8601Date
  assignees: PersonIdList
}

structure CreateCardStepOutput {

  step: CardStep
}

/// Update an existing step
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/card_tables/steps/{stepId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  stepId: CardStepId

  title: String
  due_on: ISO8601Date
  assignees: PersonIdList
}

structure UpdateCardStepOutput {

  step: CardStep
}

/// Mark a step as completed
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/card_tables/steps/{stepId}/completions.json")
operation CompleteCardStep {
  input: CompleteCardStepInput
  output: CompleteCardStepOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure CompleteCardStepInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  stepId: CardStepId
}

structure CompleteCardStepOutput {

  step: CardStep
}

/// Mark a step as incomplete
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/card_tables/steps/{stepId}/completions.json")
operation UncompleteCardStep {
  input: UncompleteCardStepInput
  output: UncompleteCardStepOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UncompleteCardStepInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  stepId: CardStepId
}

structure UncompleteCardStepOutput {

  step: CardStep
}

/// Reposition a step within a card
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}/positions.json")
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
  projectId: ProjectId

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

// ===== CardTable Shapes =====

long CardTableId
long CardId
long CardColumnId
long CardStepId

structure CardTable {
  id: CardTableId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  subscription_url: String
  bucket: TodoBucket
  creator: Person
  subscribers: PersonList
  lists: CardColumnList
}

list CardColumnList {
  member: CardColumn
}

structure CardColumn {
  id: CardColumnId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  position: Integer
  color: String
  description: String
  cards_count: Integer
  comments_count: Integer
  cards_url: String
  parent: RecordingParent
  bucket: TodoBucket
  creator: Person
  subscribers: PersonList
}

list CardList {
  member: Card
}

structure Card {
  id: CardId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  subscription_url: String
  position: Integer
  content: String
  description: String
  due_on: ISO8601Date
  completed: Boolean
  completed_at: ISO8601Timestamp
  comments_count: Integer
  comments_url: String
  completion_url: String
  parent: RecordingParent
  bucket: TodoBucket
  creator: Person
  completer: Person
  assignees: PersonList
  completion_subscribers: PersonList
  steps: CardStepList
}

list CardStepList {
  member: CardStep
}

structure CardStep {
  id: CardStepId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  position: Integer
  due_on: ISO8601Date
  completed: Boolean
  completed_at: ISO8601Timestamp
  parent: RecordingParent
  bucket: TodoBucket
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
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure GetSubscriptionOutput {

  subscription: Subscription
}

/// Subscribe the current user to a recording
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json")
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
  projectId: ProjectId

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
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json", code: 204)
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
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure UnsubscribeOutput {}

/// Update subscriptions by adding or removing specific users
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json")
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
  projectId: ProjectId

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
  subscribed: Boolean
  count: Integer
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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/client/approvals.json")
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
  projectId: ProjectId
}

structure ListClientApprovalsOutput {

  approvals: ClientApprovalList
}

/// Get a single client approval by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/client/approvals/{approvalId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/client/correspondences.json")
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
  projectId: ProjectId
}

structure ListClientCorrespondencesOutput {

  correspondences: ClientCorrespondenceList
}

/// Get a single client correspondence by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/client/correspondences/{correspondenceId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/client/recordings/{recordingId}/replies.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure ListClientRepliesOutput {

  replies: ClientReplyList
}

/// Get a single client reply by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/client/recordings/{recordingId}/replies/{replyId}")
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
  projectId: ProjectId

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
  id: ClientApprovalId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  subscription_url: String
  parent: RecordingParent
  bucket: RecordingBucket
  creator: Person
  content: String
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
  id: ClientCorrespondenceId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  subscription_url: String
  parent: RecordingParent
  bucket: RecordingBucket
  creator: Person
  content: String
  subject: String
  replies_count: Integer
  replies_url: String
}

list ClientReplyList {
  member: ClientReply
}

structure ClientReply {
  id: ClientReplyId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  parent: RecordingParent
  bucket: RecordingBucket
  creator: Person
  content: String
}

structure RecordingBucket {
  id: ProjectId
  name: String
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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/webhooks.json")
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
  projectId: ProjectId
}

structure ListWebhooksOutput {

  webhooks: WebhookList
}

/// Get a single webhook by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/webhooks/{webhookId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  webhookId: WebhookId
}

structure GetWebhookOutput {

  webhook: Webhook
}

/// Create a new webhook for a project
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/webhooks.json", code: 201)
operation CreateWebhook {
  input: CreateWebhookInput
  output: CreateWebhookOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CreateWebhookInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/webhooks/{webhookId}")
operation UpdateWebhook {
  input: UpdateWebhookInput
  output: UpdateWebhookOutput
  errors: [NotFoundError, ValidationError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure UpdateWebhookInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

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
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/webhooks/{webhookId}", code: 204)
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/events.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
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
}

structure ListRecordingsOutput {

  recordings: RecordingList
}

/// Get a single recording by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}")
operation GetRecording {
  input: GetRecordingInput
  output: GetRecordingOutput
  errors: [NotFoundError, UnauthorizedError, ForbiddenError, InternalServerError]
}

structure GetRecordingInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure GetRecordingOutput {

  recording: Recording
}

/// Trash a recording
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", code: 204)
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
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure TrashRecordingOutput {}

/// Archive a recording
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/status/archived.json", code: 204)
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
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure ArchiveRecordingOutput {}

/// Unarchive a recording (restore to active status)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/status/active.json", code: 204)
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
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure UnarchiveRecordingOutput {}

/// Set client visibility for a recording
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/recordings/{recordingId}/client_visibility.json")
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
  projectId: ProjectId

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
  id: WebhookId
  active: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  payload_url: String
  types: WebhookTypeList
  url: String
  app_url: String
}

// ===== Event Shapes =====

long EventId

list EventList {
  member: Event
}

structure Event {
  id: EventId
  recording_id: RecordingId
  action: String
  details: EventDetails
  created_at: ISO8601Timestamp
  creator: Person
}

structure EventDetails {
  added_person_ids: PersonIdList
  removed_person_ids: PersonIdList
  notified_recipient_ids: PersonIdList
}

// ===== Recording Shapes =====

@documentation("Comment|Document|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule::Entry|Todo|Todolist|Upload|Vault")
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
  id: RecordingId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  parent: RecordingParent
  bucket: RecordingBucket
  creator: Person
}

// =============================================================================
// BATCH 9 - Checkins (Questionnaires, Questions, Answers)
// =============================================================================

// ===== Questionnaire Operations =====

/// Get a questionnaire (automatic check-ins container) by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/questionnaires/{questionnaireId}")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  questionnaireId: QuestionnaireId
}

structure ListQuestionsOutput {

  questions: QuestionList
}

/// Get a single question by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/questions/{questionId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  questionId: QuestionId
}

structure GetQuestionOutput {

  question: Question
}

/// Create a new question in a questionnaire
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json", code: 201)
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
  projectId: ProjectId

  @required
  @httpLabel
  questionnaireId: QuestionnaireId

  @required
  title: String

  @required
  schedule: QuestionSchedule
}

structure CreateQuestionOutput {

  question: Question
}

/// Update an existing question
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/questions/{questionId}")
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
  projectId: ProjectId

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
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/questions/{questionId}/pause.json")
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
  projectId: ProjectId

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
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/questions/{questionId}/pause.json")
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
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/questions/{questionId}/notification_settings.json")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/questions/{questionId}/answers.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  questionId: QuestionId
}

structure ListAnswersOutput {

  answers: QuestionAnswerList
}

/// Get a single answer by id
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/question_answers/{answerId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  answerId: AnswerId
}

structure GetAnswerOutput {

  answer: QuestionAnswer
}

/// Create a new answer for a question
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/questions/{questionId}/answers.json", code: 201)
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
  projectId: ProjectId

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
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/question_answers/{answerId}", code: 204)
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
  projectId: ProjectId

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
}

structure UpdateAnswerOutput {}

/// List all people who have answered a question (answerers)
///
/// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
/// to fetch additional pages.
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/questions/{questionId}/answers/by.json")
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
  projectId: ProjectId

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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/questions/{questionId}/answers/by/{personId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  questionId: QuestionId

  @required
  @httpLabel
  personId: PersonId
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
  id: QuestionnaireId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  questions_url: String
  questions_count: Integer
  name: String
  bucket: RecordingBucket
  creator: Person
}

// ===== Question Shapes =====

long QuestionId

list QuestionList {
  member: Question
}

structure Question {
  id: QuestionId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  subscription_url: String
  parent: RecordingParent
  bucket: RecordingBucket
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
  id: AnswerId
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
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
  content: String
  group_on: ISO8601Date
  parent: RecordingParent
  bucket: RecordingBucket
  creator: Person
}

// =============================================================================
// BATCH 10 - Utilities (Search, Templates, Tools, Lineup)
// =============================================================================

// ===== Search Operations =====

/// Search for content across the account
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
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
  @httpQuery("query")
  query: String

  @httpQuery("sort")
  sort: SearchSortField
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
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/dock/tools/{toolId}")
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
  projectId: ProjectId

  @required
  @httpLabel
  toolId: ToolId
}

structure GetToolOutput {

  tool: Tool
}

/// Clone an existing tool to create a new one
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/dock/tools.json", code: 201)
operation CloneTool {
  input: CloneToolInput
  output: CloneToolOutput
  errors: [ValidationError, UnauthorizedError, ForbiddenError, RateLimitError, InternalServerError]
}

structure CloneToolInput {
  @required
  @httpLabel
  accountId: AccountId

  @required
  @httpLabel
  projectId: ProjectId

  @required
  source_recording_id: ToolId
}

structure CloneToolOutput {

  tool: Tool
}

/// Update (rename) an existing tool
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/dock/tools/{toolId}")
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
  projectId: ProjectId

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
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/dock/tools/{toolId}", code: 204)
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
  projectId: ProjectId

  @required
  @httpLabel
  toolId: ToolId
}

structure DeleteToolOutput {}

/// Enable a tool (show it on the project dock)
@basecampRetry(maxAttempts: 2, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@http(method: "POST", uri: "/{accountId}/buckets/{projectId}/recordings/{toolId}/position.json", code: 201)
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
  projectId: ProjectId

  @required
  @httpLabel
  toolId: ToolId
}

structure EnableToolOutput {}

/// Disable a tool (hide it from the project dock)
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "DELETE", uri: "/{accountId}/buckets/{projectId}/recordings/{toolId}/position.json", code: 204)
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
  projectId: ProjectId

  @required
  @httpLabel
  toolId: ToolId
}

structure DisableToolOutput {}

/// Reposition a tool on the project dock
@idempotent
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampIdempotent(natural: true)
@http(method: "PUT", uri: "/{accountId}/buckets/{projectId}/recordings/{toolId}/position.json")
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
  projectId: ProjectId

  @required
  @httpLabel
  toolId: ToolId

  @required
  position: Integer
}

structure RepositionToolOutput {}

// ===== Lineup Marker Operations =====

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
  title: String

  @required
  starts_on: ISO8601Date

  @required
  ends_on: ISO8601Date

  color: String
  description: String
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

  title: String
  starts_on: ISO8601Date
  ends_on: ISO8601Date
  color: String
  description: String
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
@basecampPagination(style: "link_header", maxPageSize: 50)
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
}

structure GetProgressReportOutput {
  events: TimelineEventList
}

/// Get project timeline
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
@basecampPagination(style: "link_header", maxPageSize: 50)
@http(method: "GET", uri: "/{accountId}/buckets/{projectId}/timeline.json")
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
}

structure GetProjectTimelineOutput {
  events: TimelineEventList
}

/// Get a person's activity timeline
@readonly
@basecampRetry(maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503])
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

/// Get upcoming schedule entries within a date window
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
  kind: String
  parent_recording_id: Long
  url: String
  app_url: String
  creator: Person
  action: String
  target: String
  title: String
  summary_excerpt: String
  bucket: TodoBucket
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

@documentation("created_at|updated_at")
string SearchSortField

list SearchResultList {
  member: SearchResult
}

structure SearchResult {
  id: Long
  status: String
  visible_to_clients: Boolean
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  inherits_status: Boolean
  type: String
  url: String
  app_url: String
  bookmark_url: String
  parent: RecordingParent
  bucket: RecordingBucket
  creator: Person
  content: String
  description: String
  subject: String
}

structure SearchMetadata {
  projects: SearchProjectList
}

list SearchProjectList {
  member: SearchProject
}

structure SearchProject {
  id: ProjectId
  name: String
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
  id: TemplateId
  status: String
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  name: String
  description: String
  url: String
  app_url: String
  dock: DockItemList
}

structure ProjectConstruction {
  id: ConstructionId
  status: String
  url: String
  project: Project
}

// ===== Tool Shapes =====

long ToolId

structure Tool {
  id: ToolId
  status: String
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  title: String
  name: String
  enabled: Boolean
  position: Integer
  url: String
  app_url: String
  bucket: RecordingBucket
}

// ===== Lineup Marker Shapes =====

long MarkerId

structure LineupMarker {
  id: MarkerId
  status: String
  color: String
  title: String
  starts_on: ISO8601Date
  ends_on: ISO8601Date
  description: String
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  type: String
  url: String
  app_url: String
  creator: Person
  parent: RecordingParent
  bucket: RecordingBucket
}

