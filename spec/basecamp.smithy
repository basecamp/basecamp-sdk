$version: "2"

namespace basecamp

use smithy.api#documentation
use smithy.api#http
use smithy.api#httpLabel
use smithy.api#httpQuery
use smithy.api#httpPayload
use smithy.api#required

/// Basecamp API
service Basecamp {
  version: "2026-01-25"
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
    GetTodoset,
    ListTodolists,
    GetTodolist,
    CreateTodolist,
    UpdateTodolist,
    TrashTodolist,
    ListTodolistGroups,
    GetTodolistGroup,
    CreateTodolistGroup,
    UpdateTodolistGroup,
    RepositionTodolistGroup,
    TrashTodolistGroup,

    // Batch 1 - Comments, Messages, MessageBoards, MessageTypes
    ListComments,
    GetComment,
    CreateComment,
    UpdateComment,
    TrashComment,
    ListMessages,
    GetMessage,
    CreateMessage,
    UpdateMessage,
    PinMessage,
    UnpinMessage,
    TrashMessage,
    ArchiveMessage,
    UnarchiveMessage,
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
    TrashDocument,
    ListUploads,
    GetUpload,
    CreateUpload,
    UpdateUpload,
    TrashUpload,
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
    TrashScheduleEntry,
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
    TrashCard,
    GetCardColumn,
    CreateCardColumn,
    UpdateCardColumn,
    MoveCardColumn,
    SetCardColumnColor,
    EnableCardColumnOnHold,
    DisableCardColumnOnHold,
    WatchCardColumn,
    UnwatchCardColumn,
    CreateCardStep,
    UpdateCardStep,
    CompleteCardStep,
    UncompleteCardStep,
    RepositionCardStep,
    DeleteCardStep,

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

    // Batch 8 - Webhooks, Events, Recordings (Automation)
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
    ListAnswers,
    GetAnswer,
    CreateAnswer,
    UpdateAnswer,

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
    DeleteLineupMarker
  ]
}

/// List projects (active by default; optionally archived/trashed)
@http(method: "GET", uri: "/projects.json")
operation ListProjects {
  input: ListProjectsInput
  output: ListProjectsOutput
}

structure ListProjectsInput {
  @httpQuery("status")
  status: ProjectStatus
}

structure ListProjectsOutput {
  @httpPayload
  projects: ProjectList
}

/// Get a single project by id
@http(method: "GET", uri: "/projects/{projectId}.json")
operation GetProject {
  input: GetProjectInput
  output: GetProjectOutput
}

structure GetProjectInput {
  @required
  @httpLabel
  projectId: ProjectId
}

structure GetProjectOutput {
  @httpPayload
  project: Project
}

/// Create a new project
@http(method: "POST", uri: "/projects.json")
operation CreateProject {
  input: CreateProjectInput
  output: CreateProjectOutput
}

structure CreateProjectInput {
  @required
  name: ProjectName
  description: ProjectDescription
}

structure CreateProjectOutput {
  @httpPayload
  project: Project
}

/// Update an existing project
@http(method: "PUT", uri: "/projects/{projectId}.json")
operation UpdateProject {
  input: UpdateProjectInput
  output: UpdateProjectOutput
}

structure UpdateProjectInput {
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
  @httpPayload
  project: Project
}

/// Trash a project (returns 204 No Content)
@http(method: "DELETE", uri: "/projects/{projectId}.json")
operation TrashProject {
  input: TrashProjectInput
  output: TrashProjectOutput
}

structure TrashProjectInput {
  @required
  @httpLabel
  projectId: ProjectId
}

structure TrashProjectOutput {}

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

structure ClientSide {
  url: String
  app_url: String
}

// ===== Todo Operations =====

/// List todos in a todolist
@http(method: "GET", uri: "/buckets/{projectId}/todolists/{todolistId}/todos.json")
operation ListTodos {
  input: ListTodosInput
  output: ListTodosOutput
}

structure ListTodosInput {
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
  @httpPayload
  todos: TodoList
}

/// Get a single todo by id
@http(method: "GET", uri: "/buckets/{projectId}/todos/{todoId}.json")
operation GetTodo {
  input: GetTodoInput
  output: GetTodoOutput
}

structure GetTodoInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  todoId: TodoId
}

structure GetTodoOutput {
  @httpPayload
  todo: Todo
}

/// Create a new todo in a todolist
@http(method: "POST", uri: "/buckets/{projectId}/todolists/{todolistId}/todos.json")
operation CreateTodo {
  input: CreateTodoInput
  output: CreateTodoOutput
}

structure CreateTodoInput {
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
  @httpPayload
  todo: Todo
}

/// Update an existing todo
@http(method: "PUT", uri: "/buckets/{projectId}/todos/{todoId}.json")
operation UpdateTodo {
  input: UpdateTodoInput
  output: UpdateTodoOutput
}

structure UpdateTodoInput {
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
  @httpPayload
  todo: Todo
}

/// Trash a todo (returns 204 No Content)
@http(method: "DELETE", uri: "/buckets/{projectId}/todos/{todoId}.json")
operation TrashTodo {
  input: TrashTodoInput
  output: TrashTodoOutput
}

structure TrashTodoInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  todoId: TodoId
}

structure TrashTodoOutput {}

/// Mark a todo as complete
@http(method: "POST", uri: "/buckets/{projectId}/todos/{todoId}/completion.json")
operation CompleteTodo {
  input: CompleteTodoInput
  output: CompleteTodoOutput
}

structure CompleteTodoInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  todoId: TodoId
}

structure CompleteTodoOutput {}

/// Mark a todo as incomplete
@http(method: "DELETE", uri: "/buckets/{projectId}/todos/{todoId}/completion.json")
operation UncompleteTodo {
  input: UncompleteTodoInput
  output: UncompleteTodoOutput
}

structure UncompleteTodoInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  todoId: TodoId
}

structure UncompleteTodoOutput {}

// ===== Todoset Operations =====

/// Get a todoset (container for todolists in a project)
@http(method: "GET", uri: "/buckets/{projectId}/todosets/{todosetId}.json")
operation GetTodoset {
  input: GetTodosetInput
  output: GetTodosetOutput
}

structure GetTodosetInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  todosetId: TodosetId
}

structure GetTodosetOutput {
  @httpPayload
  todoset: Todoset
}

// ===== Todolist Operations =====

/// List todolists in a todoset
@http(method: "GET", uri: "/buckets/{projectId}/todosets/{todosetId}/todolists.json")
operation ListTodolists {
  input: ListTodolistsInput
  output: ListTodolistsOutput
}

structure ListTodolistsInput {
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
  @httpPayload
  todolists: TodolistList
}

/// Get a single todolist by id
@http(method: "GET", uri: "/buckets/{projectId}/todolists/{todolistId}.json")
operation GetTodolist {
  input: GetTodolistInput
  output: GetTodolistOutput
}

structure GetTodolistInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  todolistId: TodolistId
}

structure GetTodolistOutput {
  @httpPayload
  todolist: Todolist
}

/// Create a new todolist in a todoset
@http(method: "POST", uri: "/buckets/{projectId}/todosets/{todosetId}/todolists.json")
operation CreateTodolist {
  input: CreateTodolistInput
  output: CreateTodolistOutput
}

structure CreateTodolistInput {
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
  @httpPayload
  todolist: Todolist
}

/// Update an existing todolist
@http(method: "PUT", uri: "/buckets/{projectId}/todolists/{todolistId}.json")
operation UpdateTodolist {
  input: UpdateTodolistInput
  output: UpdateTodolistOutput
}

structure UpdateTodolistInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  todolistId: TodolistId

  name: TodolistName
  description: TodolistDescription
}

structure UpdateTodolistOutput {
  @httpPayload
  todolist: Todolist
}

/// Trash a todolist
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{todolistId}/status/trashed.json")
operation TrashTodolist {
  input: TrashTodolistInput
  output: TrashTodolistOutput
}

structure TrashTodolistInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  todolistId: TodolistId
}

structure TrashTodolistOutput {}

// ===== Todolist Group Operations =====

/// List groups in a todolist
@http(method: "GET", uri: "/buckets/{projectId}/todolists/{todolistId}/groups.json")
operation ListTodolistGroups {
  input: ListTodolistGroupsInput
  output: ListTodolistGroupsOutput
}

structure ListTodolistGroupsInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  todolistId: TodolistId
}

structure ListTodolistGroupsOutput {
  @httpPayload
  groups: TodolistGroupList
}

/// Get a single todolist group by id
@http(method: "GET", uri: "/buckets/{projectId}/todolists/{groupId}.json")
operation GetTodolistGroup {
  input: GetTodolistGroupInput
  output: GetTodolistGroupOutput
}

structure GetTodolistGroupInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  groupId: TodolistGroupId
}

structure GetTodolistGroupOutput {
  @httpPayload
  group: TodolistGroup
}

/// Create a new group in a todolist
@http(method: "POST", uri: "/buckets/{projectId}/todolists/{todolistId}/groups.json")
operation CreateTodolistGroup {
  input: CreateTodolistGroupInput
  output: CreateTodolistGroupOutput
}

structure CreateTodolistGroupInput {
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
  @httpPayload
  group: TodolistGroup
}

/// Update an existing todolist group
@http(method: "PUT", uri: "/buckets/{projectId}/todolists/{groupId}.json")
operation UpdateTodolistGroup {
  input: UpdateTodolistGroupInput
  output: UpdateTodolistGroupOutput
}

structure UpdateTodolistGroupInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  groupId: TodolistGroupId

  name: TodolistGroupName
}

structure UpdateTodolistGroupOutput {
  @httpPayload
  group: TodolistGroup
}

/// Reposition a todolist group
@http(method: "PUT", uri: "/buckets/{projectId}/todolists/{groupId}/position.json")
operation RepositionTodolistGroup {
  input: RepositionTodolistGroupInput
  output: RepositionTodolistGroupOutput
}

structure RepositionTodolistGroupInput {
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

/// Trash a todolist group
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{groupId}/status/trashed.json")
operation TrashTodolistGroup {
  input: TrashTodolistGroupInput
  output: TrashTodolistGroupOutput
}

structure TrashTodolistGroupInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  groupId: TodolistGroupId
}

structure TrashTodolistGroupOutput {}

// ===== Todo Shapes =====

long TodoId
long TodolistId
long PersonId
string TodoContent
string TodoDescription

@documentation("active|archived|trashed")
string TodoStatus

list TodoList {
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
  name: String
  email_address: String
  personable_type: String
  title: String
  bio: String
  location: String
  created_at: ISO8601Timestamp
  updated_at: ISO8601Timestamp
  admin: Boolean
  owner: Boolean
  client: Boolean
  employee: Boolean
  time_zone: String
  avatar_url: String
  company: PersonCompany
  can_manage_projects: Boolean
  can_manage_people: Boolean
}

structure PersonCompany {
  id: Long
  name: String
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
@http(method: "GET", uri: "/buckets/{projectId}/recordings/{recordingId}/comments.json")
operation ListComments {
  input: ListCommentsInput
  output: ListCommentsOutput
}

structure ListCommentsInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure ListCommentsOutput {
  @httpPayload
  comments: CommentList
}

/// Get a single comment by id
@http(method: "GET", uri: "/buckets/{projectId}/comments/{commentId}.json")
operation GetComment {
  input: GetCommentInput
  output: GetCommentOutput
}

structure GetCommentInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  commentId: CommentId
}

structure GetCommentOutput {
  @httpPayload
  comment: Comment
}

/// Create a new comment on a recording
@http(method: "POST", uri: "/buckets/{projectId}/recordings/{recordingId}/comments.json")
operation CreateComment {
  input: CreateCommentInput
  output: CreateCommentOutput
}

structure CreateCommentInput {
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
  @httpPayload
  comment: Comment
}

/// Update an existing comment
@http(method: "PUT", uri: "/buckets/{projectId}/comments/{commentId}.json")
operation UpdateComment {
  input: UpdateCommentInput
  output: UpdateCommentOutput
}

structure UpdateCommentInput {
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
  @httpPayload
  comment: Comment
}

/// Trash a comment
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{commentId}/status/trashed.json")
operation TrashComment {
  input: TrashCommentInput
  output: TrashCommentOutput
}

structure TrashCommentInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  commentId: CommentId
}

structure TrashCommentOutput {}

// ===== Message Operations (Batch 1) =====

/// List messages on a message board
@http(method: "GET", uri: "/buckets/{projectId}/message_boards/{boardId}/messages.json")
operation ListMessages {
  input: ListMessagesInput
  output: ListMessagesOutput
}

structure ListMessagesInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  boardId: MessageBoardId
}

structure ListMessagesOutput {
  @httpPayload
  messages: MessageList
}

/// Get a single message by id
@http(method: "GET", uri: "/buckets/{projectId}/messages/{messageId}.json")
operation GetMessage {
  input: GetMessageInput
  output: GetMessageOutput
}

structure GetMessageInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  messageId: MessageId
}

structure GetMessageOutput {
  @httpPayload
  message: Message
}

/// Create a new message on a message board
@http(method: "POST", uri: "/buckets/{projectId}/message_boards/{boardId}/messages.json")
operation CreateMessage {
  input: CreateMessageInput
  output: CreateMessageOutput
}

structure CreateMessageInput {
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
  @httpPayload
  message: Message
}

/// Update an existing message
@http(method: "PUT", uri: "/buckets/{projectId}/messages/{messageId}.json")
operation UpdateMessage {
  input: UpdateMessageInput
  output: UpdateMessageOutput
}

structure UpdateMessageInput {
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
  @httpPayload
  message: Message
}

/// Pin a message to the top of the message board
@http(method: "POST", uri: "/buckets/{projectId}/recordings/{messageId}/pin.json")
operation PinMessage {
  input: PinMessageInput
  output: PinMessageOutput
}

structure PinMessageInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  messageId: MessageId
}

structure PinMessageOutput {}

/// Unpin a message from the message board
@http(method: "DELETE", uri: "/buckets/{projectId}/recordings/{messageId}/pin.json")
operation UnpinMessage {
  input: UnpinMessageInput
  output: UnpinMessageOutput
}

structure UnpinMessageInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  messageId: MessageId
}

structure UnpinMessageOutput {}

/// Trash a message
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{messageId}/status/trashed.json")
operation TrashMessage {
  input: TrashMessageInput
  output: TrashMessageOutput
}

structure TrashMessageInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  messageId: MessageId
}

structure TrashMessageOutput {}

/// Archive a message
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{messageId}/status/archived.json")
operation ArchiveMessage {
  input: ArchiveMessageInput
  output: ArchiveMessageOutput
}

structure ArchiveMessageInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  messageId: MessageId
}

structure ArchiveMessageOutput {}

/// Unarchive a message (restore to active)
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{messageId}/status/active.json")
operation UnarchiveMessage {
  input: UnarchiveMessageInput
  output: UnarchiveMessageOutput
}

structure UnarchiveMessageInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  messageId: MessageId
}

structure UnarchiveMessageOutput {}

// ===== Message Board Operations (Batch 1) =====

/// Get a message board
@http(method: "GET", uri: "/buckets/{projectId}/message_boards/{boardId}.json")
operation GetMessageBoard {
  input: GetMessageBoardInput
  output: GetMessageBoardOutput
}

structure GetMessageBoardInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  boardId: MessageBoardId
}

structure GetMessageBoardOutput {
  @httpPayload
  message_board: MessageBoard
}

// ===== Message Type Operations (Batch 1) =====

/// List message types in a project
@http(method: "GET", uri: "/buckets/{projectId}/categories.json")
operation ListMessageTypes {
  input: ListMessageTypesInput
  output: ListMessageTypesOutput
}

structure ListMessageTypesInput {
  @required
  @httpLabel
  projectId: ProjectId
}

structure ListMessageTypesOutput {
  @httpPayload
  message_types: MessageTypeList
}

/// Get a single message type by id
@http(method: "GET", uri: "/buckets/{projectId}/categories/{typeId}.json")
operation GetMessageType {
  input: GetMessageTypeInput
  output: GetMessageTypeOutput
}

structure GetMessageTypeInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  typeId: MessageTypeId
}

structure GetMessageTypeOutput {
  @httpPayload
  message_type: MessageType
}

/// Create a new message type in a project
@http(method: "POST", uri: "/buckets/{projectId}/categories.json")
operation CreateMessageType {
  input: CreateMessageTypeInput
  output: CreateMessageTypeOutput
}

structure CreateMessageTypeInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  name: MessageTypeName

  @required
  icon: MessageTypeIcon
}

structure CreateMessageTypeOutput {
  @httpPayload
  message_type: MessageType
}

/// Update an existing message type
@http(method: "PUT", uri: "/buckets/{projectId}/categories/{typeId}.json")
operation UpdateMessageType {
  input: UpdateMessageTypeInput
  output: UpdateMessageTypeOutput
}

structure UpdateMessageTypeInput {
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
  @httpPayload
  message_type: MessageType
}

/// Delete a message type
@http(method: "DELETE", uri: "/buckets/{projectId}/categories/{typeId}.json")
operation DeleteMessageType {
  input: DeleteMessageTypeInput
  output: DeleteMessageTypeOutput
}

structure DeleteMessageTypeInput {
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
@http(method: "GET", uri: "/buckets/{projectId}/vaults/{vaultId}/vaults.json")
operation ListVaults {
  input: ListVaultsInput
  output: ListVaultsOutput
}

structure ListVaultsInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  vaultId: VaultId
}

structure ListVaultsOutput {
  @httpPayload
  vaults: VaultList
}

/// Get a single vault by id
@http(method: "GET", uri: "/buckets/{projectId}/vaults/{vaultId}.json")
operation GetVault {
  input: GetVaultInput
  output: GetVaultOutput
}

structure GetVaultInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  vaultId: VaultId
}

structure GetVaultOutput {
  @httpPayload
  vault: Vault
}

/// Create a new vault (subfolder) in a vault
@http(method: "POST", uri: "/buckets/{projectId}/vaults/{vaultId}/vaults.json")
operation CreateVault {
  input: CreateVaultInput
  output: CreateVaultOutput
}

structure CreateVaultInput {
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
  @httpPayload
  vault: Vault
}

/// Update an existing vault
@http(method: "PUT", uri: "/buckets/{projectId}/vaults/{vaultId}.json")
operation UpdateVault {
  input: UpdateVaultInput
  output: UpdateVaultOutput
}

structure UpdateVaultInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  vaultId: VaultId

  title: VaultTitle
}

structure UpdateVaultOutput {
  @httpPayload
  vault: Vault
}

// ===== Document Operations (Batch 2) =====

/// List documents in a vault
@http(method: "GET", uri: "/buckets/{projectId}/vaults/{vaultId}/documents.json")
operation ListDocuments {
  input: ListDocumentsInput
  output: ListDocumentsOutput
}

structure ListDocumentsInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  vaultId: VaultId
}

structure ListDocumentsOutput {
  @httpPayload
  documents: DocumentList
}

/// Get a single document by id
@http(method: "GET", uri: "/buckets/{projectId}/documents/{documentId}.json")
operation GetDocument {
  input: GetDocumentInput
  output: GetDocumentOutput
}

structure GetDocumentInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  documentId: DocumentId
}

structure GetDocumentOutput {
  @httpPayload
  document: Document
}

/// Create a new document in a vault
@http(method: "POST", uri: "/buckets/{projectId}/vaults/{vaultId}/documents.json")
operation CreateDocument {
  input: CreateDocumentInput
  output: CreateDocumentOutput
}

structure CreateDocumentInput {
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
  @httpPayload
  document: Document
}

/// Update an existing document
@http(method: "PUT", uri: "/buckets/{projectId}/documents/{documentId}.json")
operation UpdateDocument {
  input: UpdateDocumentInput
  output: UpdateDocumentOutput
}

structure UpdateDocumentInput {
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
  @httpPayload
  document: Document
}

/// Trash a document
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{documentId}/status/trashed.json")
operation TrashDocument {
  input: TrashDocumentInput
  output: TrashDocumentOutput
}

structure TrashDocumentInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  documentId: DocumentId
}

structure TrashDocumentOutput {}

// ===== Upload Operations (Batch 2) =====

/// List uploads in a vault
@http(method: "GET", uri: "/buckets/{projectId}/vaults/{vaultId}/uploads.json")
operation ListUploads {
  input: ListUploadsInput
  output: ListUploadsOutput
}

structure ListUploadsInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  vaultId: VaultId
}

structure ListUploadsOutput {
  @httpPayload
  uploads: UploadList
}

/// Get a single upload by id
@http(method: "GET", uri: "/buckets/{projectId}/uploads/{uploadId}.json")
operation GetUpload {
  input: GetUploadInput
  output: GetUploadOutput
}

structure GetUploadInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  uploadId: UploadId
}

structure GetUploadOutput {
  @httpPayload
  upload: Upload
}

/// Create a new upload in a vault
@http(method: "POST", uri: "/buckets/{projectId}/vaults/{vaultId}/uploads.json")
operation CreateUpload {
  input: CreateUploadInput
  output: CreateUploadOutput
}

structure CreateUploadInput {
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
  @httpPayload
  upload: Upload
}

/// Update an existing upload
@http(method: "PUT", uri: "/buckets/{projectId}/uploads/{uploadId}.json")
operation UpdateUpload {
  input: UpdateUploadInput
  output: UpdateUploadOutput
}

structure UpdateUploadInput {
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
  @httpPayload
  upload: Upload
}

/// Trash an upload
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{uploadId}/status/trashed.json")
operation TrashUpload {
  input: TrashUploadInput
  output: TrashUploadOutput
}

structure TrashUploadInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  uploadId: UploadId
}

structure TrashUploadOutput {}

/// List versions of an upload
@http(method: "GET", uri: "/buckets/{projectId}/uploads/{uploadId}/versions.json")
operation ListUploadVersions {
  input: ListUploadVersionsInput
  output: ListUploadVersionsOutput
}

structure ListUploadVersionsInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  uploadId: UploadId
}

structure ListUploadVersionsOutput {
  @httpPayload
  uploads: UploadList
}

// ===== Attachment Operations (Batch 2) =====

/// Create an attachment (upload a file for embedding)
@http(method: "POST", uri: "/attachments.json")
operation CreateAttachment {
  input: CreateAttachmentInput
  output: CreateAttachmentOutput
}

structure CreateAttachmentInput {
  @required
  @httpQuery("name")
  name: AttachmentFilename

  @required
  @httpHeader("Content-Type")
  contentType: String

  @required
  @httpPayload
  data: Blob
}

structure CreateAttachmentOutput {
  attachable_sgid: AttachableSgid
}

// ===== Schedule Operations (Batch 3) =====

/// Get a schedule
@http(method: "GET", uri: "/buckets/{projectId}/schedules/{scheduleId}.json")
operation GetSchedule {
  input: GetScheduleInput
  output: GetScheduleOutput
}

structure GetScheduleInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  scheduleId: ScheduleId
}

structure GetScheduleOutput {
  @httpPayload
  schedule: Schedule
}

/// Update schedule settings
@http(method: "PUT", uri: "/buckets/{projectId}/schedules/{scheduleId}.json")
operation UpdateScheduleSettings {
  input: UpdateScheduleSettingsInput
  output: UpdateScheduleSettingsOutput
}

structure UpdateScheduleSettingsInput {
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
  @httpPayload
  schedule: Schedule
}

/// List entries on a schedule
@http(method: "GET", uri: "/buckets/{projectId}/schedules/{scheduleId}/entries.json")
operation ListScheduleEntries {
  input: ListScheduleEntriesInput
  output: ListScheduleEntriesOutput
}

structure ListScheduleEntriesInput {
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
  @httpPayload
  entries: ScheduleEntryList
}

/// Get a single schedule entry by id
@http(method: "GET", uri: "/buckets/{projectId}/schedule_entries/{entryId}.json")
operation GetScheduleEntry {
  input: GetScheduleEntryInput
  output: GetScheduleEntryOutput
}

structure GetScheduleEntryInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  entryId: ScheduleEntryId
}

structure GetScheduleEntryOutput {
  @httpPayload
  entry: ScheduleEntry
}

/// Get a specific occurrence of a recurring schedule entry
@http(method: "GET", uri: "/buckets/{projectId}/schedule_entries/{entryId}/occurrences/{date}.json")
operation GetScheduleEntryOccurrence {
  input: GetScheduleEntryOccurrenceInput
  output: GetScheduleEntryOccurrenceOutput
}

structure GetScheduleEntryOccurrenceInput {
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
  @httpPayload
  entry: ScheduleEntry
}

/// Create a new schedule entry
@http(method: "POST", uri: "/buckets/{projectId}/schedules/{scheduleId}/entries.json")
operation CreateScheduleEntry {
  input: CreateScheduleEntryInput
  output: CreateScheduleEntryOutput
}

structure CreateScheduleEntryInput {
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
  @httpPayload
  entry: ScheduleEntry
}

/// Update an existing schedule entry
@http(method: "PUT", uri: "/buckets/{projectId}/schedule_entries/{entryId}.json")
operation UpdateScheduleEntry {
  input: UpdateScheduleEntryInput
  output: UpdateScheduleEntryOutput
}

structure UpdateScheduleEntryInput {
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
  @httpPayload
  entry: ScheduleEntry
}

/// Trash a schedule entry
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{entryId}/status/trashed.json")
operation TrashScheduleEntry {
  input: TrashScheduleEntryInput
  output: TrashScheduleEntryOutput
}

structure TrashScheduleEntryInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  entryId: ScheduleEntryId
}

structure TrashScheduleEntryOutput {}

// ===== Timesheet Operations (Batch 3) =====

/// Get account-wide timesheet report
@http(method: "GET", uri: "/reports/timesheet.json")
operation GetTimesheetReport {
  input: GetTimesheetReportInput
  output: GetTimesheetReportOutput
}

structure GetTimesheetReportInput {
  @httpQuery("from")
  from: ISO8601Date

  @httpQuery("to")
  to: ISO8601Date

  @httpQuery("person_id")
  person_id: PersonId
}

structure GetTimesheetReportOutput {
  @httpPayload
  entries: TimesheetEntryList
}

/// Get timesheet for a specific project
@http(method: "GET", uri: "/buckets/{projectId}/timesheet.json")
operation GetProjectTimesheet {
  input: GetProjectTimesheetInput
  output: GetProjectTimesheetOutput
}

structure GetProjectTimesheetInput {
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
  @httpPayload
  entries: TimesheetEntryList
}

/// Get timesheet for a specific recording
@http(method: "GET", uri: "/buckets/{projectId}/recordings/{recordingId}/timesheet.json")
operation GetRecordingTimesheet {
  input: GetRecordingTimesheetInput
  output: GetRecordingTimesheetOutput
}

structure GetRecordingTimesheetInput {
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
  @httpPayload
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
@http(method: "GET", uri: "/chats.json")
operation ListCampfires {
  input: ListCampfiresInput
  output: ListCampfiresOutput
}

structure ListCampfiresInput {}

structure ListCampfiresOutput {
  @httpPayload
  campfires: CampfireList
}

/// Get a campfire by ID
@http(method: "GET", uri: "/buckets/{projectId}/chats/{campfireId}.json")
operation GetCampfire {
  input: GetCampfireInput
  output: GetCampfireOutput
}

structure GetCampfireInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  campfireId: CampfireId
}

structure GetCampfireOutput {
  @httpPayload
  campfire: Campfire
}

/// List all lines (messages) in a campfire
@http(method: "GET", uri: "/buckets/{projectId}/chats/{campfireId}/lines.json")
operation ListCampfireLines {
  input: ListCampfireLinesInput
  output: ListCampfireLinesOutput
}

structure ListCampfireLinesInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  campfireId: CampfireId
}

structure ListCampfireLinesOutput {
  @httpPayload
  lines: CampfireLineList
}

/// Get a campfire line by ID
@http(method: "GET", uri: "/buckets/{projectId}/chats/{campfireId}/lines/{lineId}.json")
operation GetCampfireLine {
  input: GetCampfireLineInput
  output: GetCampfireLineOutput
}

structure GetCampfireLineInput {
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
  @httpPayload
  line: CampfireLine
}

/// Create a new line (message) in a campfire
@http(method: "POST", uri: "/buckets/{projectId}/chats/{campfireId}/lines.json")
operation CreateCampfireLine {
  input: CreateCampfireLineInput
  output: CreateCampfireLineOutput
}

structure CreateCampfireLineInput {
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
  @httpPayload
  line: CampfireLine
}

/// Delete a campfire line
@http(method: "DELETE", uri: "/buckets/{projectId}/chats/{campfireId}/lines/{lineId}.json")
operation DeleteCampfireLine {
  input: DeleteCampfireLineInput
  output: DeleteCampfireLineOutput
}

structure DeleteCampfireLineInput {
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
@http(method: "GET", uri: "/buckets/{projectId}/chats/{campfireId}/integrations.json")
operation ListChatbots {
  input: ListChatbotsInput
  output: ListChatbotsOutput
}

structure ListChatbotsInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  campfireId: CampfireId
}

structure ListChatbotsOutput {
  @httpPayload
  chatbots: ChatbotList
}

/// Get a chatbot by ID
@http(method: "GET", uri: "/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}.json")
operation GetChatbot {
  input: GetChatbotInput
  output: GetChatbotOutput
}

structure GetChatbotInput {
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
  @httpPayload
  chatbot: Chatbot
}

/// Create a new chatbot for a campfire
@http(method: "POST", uri: "/buckets/{projectId}/chats/{campfireId}/integrations.json")
operation CreateChatbot {
  input: CreateChatbotInput
  output: CreateChatbotOutput
}

structure CreateChatbotInput {
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
  @httpPayload
  chatbot: Chatbot
}

/// Update an existing chatbot
@http(method: "PUT", uri: "/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}.json")
operation UpdateChatbot {
  input: UpdateChatbotInput
  output: UpdateChatbotOutput
}

structure UpdateChatbotInput {
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
  @httpPayload
  chatbot: Chatbot
}

/// Delete a chatbot
@http(method: "DELETE", uri: "/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}.json")
operation DeleteChatbot {
  input: DeleteChatbotInput
  output: DeleteChatbotOutput
}

structure DeleteChatbotInput {
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
@http(method: "GET", uri: "/buckets/{projectId}/inboxes/{inboxId}.json")
operation GetInbox {
  input: GetInboxInput
  output: GetInboxOutput
}

structure GetInboxInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  inboxId: InboxId
}

structure GetInboxOutput {
  @httpPayload
  inbox: Inbox
}

// ===== Forward Operations =====

/// List all forwards in an inbox
@http(method: "GET", uri: "/buckets/{projectId}/inboxes/{inboxId}/forwards.json")
operation ListForwards {
  input: ListForwardsInput
  output: ListForwardsOutput
}

structure ListForwardsInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  inboxId: InboxId
}

structure ListForwardsOutput {
  @httpPayload
  forwards: ForwardList
}

/// Get a forward by ID
@http(method: "GET", uri: "/buckets/{projectId}/inbox_forwards/{forwardId}.json")
operation GetForward {
  input: GetForwardInput
  output: GetForwardOutput
}

structure GetForwardInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  forwardId: ForwardId
}

structure GetForwardOutput {
  @httpPayload
  forward: Forward
}

/// List all replies to a forward
@http(method: "GET", uri: "/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json")
operation ListForwardReplies {
  input: ListForwardRepliesInput
  output: ListForwardRepliesOutput
}

structure ListForwardRepliesInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  forwardId: ForwardId
}

structure ListForwardRepliesOutput {
  @httpPayload
  replies: ForwardReplyList
}

/// Get a forward reply by ID
@http(method: "GET", uri: "/buckets/{projectId}/inbox_forwards/{forwardId}/replies/{replyId}.json")
operation GetForwardReply {
  input: GetForwardReplyInput
  output: GetForwardReplyOutput
}

structure GetForwardReplyInput {
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
  @httpPayload
  reply: ForwardReply
}

/// Create a reply to a forward
@http(method: "POST", uri: "/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json")
operation CreateForwardReply {
  input: CreateForwardReplyInput
  output: CreateForwardReplyOutput
}

structure CreateForwardReplyInput {
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
  @httpPayload
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
@http(method: "GET", uri: "/buckets/{projectId}/card_tables/{cardTableId}.json")
operation GetCardTable {
  input: GetCardTableInput
  output: GetCardTableOutput
}

structure GetCardTableInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  cardTableId: CardTableId
}

structure GetCardTableOutput {
  @httpPayload
  card_table: CardTable
}

// ===== Card Operations =====

/// List cards in a column
@http(method: "GET", uri: "/buckets/{projectId}/card_tables/lists/{columnId}/cards.json")
operation ListCards {
  input: ListCardsInput
  output: ListCardsOutput
}

structure ListCardsInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure ListCardsOutput {
  @httpPayload
  cards: CardList
}

/// Get a card by ID
@http(method: "GET", uri: "/buckets/{projectId}/card_tables/cards/{cardId}.json")
operation GetCard {
  input: GetCardInput
  output: GetCardOutput
}

structure GetCardInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  cardId: CardId
}

structure GetCardOutput {
  @httpPayload
  card: Card
}

/// Create a card in a column
@http(method: "POST", uri: "/buckets/{projectId}/card_tables/lists/{columnId}/cards.json")
operation CreateCard {
  input: CreateCardInput
  output: CreateCardOutput
}

structure CreateCardInput {
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
  @httpPayload
  card: Card
}

/// Update an existing card
@http(method: "PUT", uri: "/buckets/{projectId}/card_tables/cards/{cardId}.json")
operation UpdateCard {
  input: UpdateCardInput
  output: UpdateCardOutput
}

structure UpdateCardInput {
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
  @httpPayload
  card: Card
}

/// Move a card to a different column
@http(method: "POST", uri: "/buckets/{projectId}/card_tables/cards/{cardId}/moves.json")
operation MoveCard {
  input: MoveCardInput
  output: MoveCardOutput
}

structure MoveCardInput {
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

/// Trash a card
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{cardId}/status/trashed.json")
operation TrashCard {
  input: TrashCardInput
  output: TrashCardOutput
}

structure TrashCardInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  cardId: CardId
}

structure TrashCardOutput {}

// ===== CardColumn Operations =====

/// Get a card column by ID
@http(method: "GET", uri: "/buckets/{projectId}/card_tables/columns/{columnId}.json")
operation GetCardColumn {
  input: GetCardColumnInput
  output: GetCardColumnOutput
}

structure GetCardColumnInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure GetCardColumnOutput {
  @httpPayload
  column: CardColumn
}

/// Create a column in a card table
@http(method: "POST", uri: "/buckets/{projectId}/card_tables/{cardTableId}/columns.json")
operation CreateCardColumn {
  input: CreateCardColumnInput
  output: CreateCardColumnOutput
}

structure CreateCardColumnInput {
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
  @httpPayload
  column: CardColumn
}

/// Update an existing column
@http(method: "PUT", uri: "/buckets/{projectId}/card_tables/columns/{columnId}.json")
operation UpdateCardColumn {
  input: UpdateCardColumnInput
  output: UpdateCardColumnOutput
}

structure UpdateCardColumnInput {
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
  @httpPayload
  column: CardColumn
}

/// Move a column within a card table
@http(method: "POST", uri: "/buckets/{projectId}/card_tables/{cardTableId}/moves.json")
operation MoveCardColumn {
  input: MoveCardColumnInput
  output: MoveCardColumnOutput
}

structure MoveCardColumnInput {
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
@http(method: "PUT", uri: "/buckets/{projectId}/card_tables/columns/{columnId}/color.json")
operation SetCardColumnColor {
  input: SetCardColumnColorInput
  output: SetCardColumnColorOutput
}

structure SetCardColumnColorInput {
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
  @httpPayload
  column: CardColumn
}

/// Enable on-hold section in a column
@http(method: "POST", uri: "/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json")
operation EnableCardColumnOnHold {
  input: EnableCardColumnOnHoldInput
  output: EnableCardColumnOnHoldOutput
}

structure EnableCardColumnOnHoldInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure EnableCardColumnOnHoldOutput {
  @httpPayload
  column: CardColumn
}

/// Disable on-hold section in a column
@http(method: "DELETE", uri: "/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json")
operation DisableCardColumnOnHold {
  input: DisableCardColumnOnHoldInput
  output: DisableCardColumnOnHoldOutput
}

structure DisableCardColumnOnHoldInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure DisableCardColumnOnHoldOutput {
  @httpPayload
  column: CardColumn
}

/// Watch (subscribe to) a card column
@http(method: "POST", uri: "/buckets/{projectId}/recordings/{columnId}/subscription.json")
operation WatchCardColumn {
  input: WatchCardColumnInput
  output: WatchCardColumnOutput
}

structure WatchCardColumnInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure WatchCardColumnOutput {
  @httpPayload
  subscription: Subscription
}

/// Unwatch (unsubscribe from) a card column
@http(method: "DELETE", uri: "/buckets/{projectId}/recordings/{columnId}/subscription.json")
operation UnwatchCardColumn {
  input: UnwatchCardColumnInput
  output: UnwatchCardColumnOutput
}

structure UnwatchCardColumnInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  columnId: CardColumnId
}

structure UnwatchCardColumnOutput {}

// ===== CardStep Operations =====

/// Create a step on a card
@http(method: "POST", uri: "/buckets/{projectId}/card_tables/cards/{cardId}/steps.json")
operation CreateCardStep {
  input: CreateCardStepInput
  output: CreateCardStepOutput
}

structure CreateCardStepInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  cardId: CardId

  @required
  title: String

  due_on: ISO8601Date
  @documentation("Comma-separated list of person IDs")
  assignees: String
}

structure CreateCardStepOutput {
  @httpPayload
  step: CardStep
}

/// Update an existing step
@http(method: "PUT", uri: "/buckets/{projectId}/card_tables/steps/{stepId}.json")
operation UpdateCardStep {
  input: UpdateCardStepInput
  output: UpdateCardStepOutput
}

structure UpdateCardStepInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  stepId: CardStepId

  title: String
  due_on: ISO8601Date
  @documentation("Comma-separated list of person IDs")
  assignees: String
}

structure UpdateCardStepOutput {
  @httpPayload
  step: CardStep
}

/// Mark a step as completed
@http(method: "PUT", uri: "/buckets/{projectId}/card_tables/steps/{stepId}/completions.json")
operation CompleteCardStep {
  input: CompleteCardStepInput
  output: CompleteCardStepOutput
}

structure CompleteCardStepInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  stepId: CardStepId
}

structure CompleteCardStepOutput {
  @httpPayload
  step: CardStep
}

/// Mark a step as incomplete
@http(method: "DELETE", uri: "/buckets/{projectId}/card_tables/steps/{stepId}/completions.json")
operation UncompleteCardStep {
  input: UncompleteCardStepInput
  output: UncompleteCardStepOutput
}

structure UncompleteCardStepInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  stepId: CardStepId
}

structure UncompleteCardStepOutput {
  @httpPayload
  step: CardStep
}

/// Reposition a step within a card
@http(method: "POST", uri: "/buckets/{projectId}/card_tables/cards/{cardId}/positions.json")
operation RepositionCardStep {
  input: RepositionCardStepInput
  output: RepositionCardStepOutput
}

structure RepositionCardStepInput {
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

/// Delete a step (move to trash)
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{stepId}/status/trashed.json")
operation DeleteCardStep {
  input: DeleteCardStepInput
  output: DeleteCardStepOutput
}

structure DeleteCardStepInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  stepId: CardStepId
}

structure DeleteCardStepOutput {}

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
  comment_count: Integer
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
  comment_count: Integer
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
@http(method: "GET", uri: "/people.json")
operation ListPeople {
  input: ListPeopleInput
  output: ListPeopleOutput
}

structure ListPeopleInput {}

structure ListPeopleOutput {
  @httpPayload
  people: PersonList
}

/// Get a person by ID
@http(method: "GET", uri: "/people/{personId}.json")
operation GetPerson {
  input: GetPersonInput
  output: GetPersonOutput
}

structure GetPersonInput {
  @required
  @httpLabel
  personId: PersonId
}

structure GetPersonOutput {
  @httpPayload
  person: Person
}

/// Get the current authenticated user's profile
@http(method: "GET", uri: "/my/profile.json")
operation GetMyProfile {
  input: GetMyProfileInput
  output: GetMyProfileOutput
}

structure GetMyProfileInput {}

structure GetMyProfileOutput {
  @httpPayload
  person: Person
}

/// List all active people on a project
@http(method: "GET", uri: "/projects/{projectId}/people.json")
operation ListProjectPeople {
  input: ListProjectPeopleInput
  output: ListProjectPeopleOutput
}

structure ListProjectPeopleInput {
  @required
  @httpLabel
  projectId: ProjectId
}

structure ListProjectPeopleOutput {
  @httpPayload
  people: PersonList
}

/// List all account users who can be pinged
@http(method: "GET", uri: "/circles/people.json")
operation ListPingablePeople {
  input: ListPingablePeopleInput
  output: ListPingablePeopleOutput
}

structure ListPingablePeopleInput {}

structure ListPingablePeopleOutput {
  @httpPayload
  people: PersonList
}

/// Update project access (grant/revoke/create people)
@http(method: "PUT", uri: "/projects/{projectId}/people/users.json")
operation UpdateProjectAccess {
  input: UpdateProjectAccessInput
  output: UpdateProjectAccessOutput
}

structure UpdateProjectAccessInput {
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
  name: String

  @required
  email_address: String

  title: String
  company_name: String
}

structure UpdateProjectAccessOutput {
  @httpPayload
  result: ProjectAccessResult
}

structure ProjectAccessResult {
  granted: PersonList
  revoked: PersonList
}

// ===== Subscription Operations =====

/// Get subscription information for a recording
@http(method: "GET", uri: "/buckets/{projectId}/recordings/{recordingId}/subscription.json")
operation GetSubscription {
  input: GetSubscriptionInput
  output: GetSubscriptionOutput
}

structure GetSubscriptionInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure GetSubscriptionOutput {
  @httpPayload
  subscription: Subscription
}

/// Subscribe the current user to a recording
@http(method: "POST", uri: "/buckets/{projectId}/recordings/{recordingId}/subscription.json")
operation Subscribe {
  input: SubscribeInput
  output: SubscribeOutput
}

structure SubscribeInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure SubscribeOutput {
  @httpPayload
  subscription: Subscription
}

/// Unsubscribe the current user from a recording
@http(method: "DELETE", uri: "/buckets/{projectId}/recordings/{recordingId}/subscription.json")
operation Unsubscribe {
  input: UnsubscribeInput
  output: UnsubscribeOutput
}

structure UnsubscribeInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure UnsubscribeOutput {}

/// Update subscriptions by adding or removing specific users
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{recordingId}/subscription.json")
operation UpdateSubscription {
  input: UpdateSubscriptionInput
  output: UpdateSubscriptionOutput
}

structure UpdateSubscriptionInput {
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
  @httpPayload
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
@http(method: "GET", uri: "/buckets/{projectId}/client/approvals.json")
operation ListClientApprovals {
  input: ListClientApprovalsInput
  output: ListClientApprovalsOutput
}

structure ListClientApprovalsInput {
  @required
  @httpLabel
  projectId: ProjectId
}

structure ListClientApprovalsOutput {
  @httpPayload
  approvals: ClientApprovalList
}

/// Get a single client approval by id
@http(method: "GET", uri: "/buckets/{projectId}/client/approvals/{approvalId}.json")
operation GetClientApproval {
  input: GetClientApprovalInput
  output: GetClientApprovalOutput
}

structure GetClientApprovalInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  approvalId: ClientApprovalId
}

structure GetClientApprovalOutput {
  @httpPayload
  approval: ClientApproval
}

// ===== Client Correspondence Operations =====

/// List all client correspondences in a project
@http(method: "GET", uri: "/buckets/{projectId}/client/correspondences.json")
operation ListClientCorrespondences {
  input: ListClientCorrespondencesInput
  output: ListClientCorrespondencesOutput
}

structure ListClientCorrespondencesInput {
  @required
  @httpLabel
  projectId: ProjectId
}

structure ListClientCorrespondencesOutput {
  @httpPayload
  correspondences: ClientCorrespondenceList
}

/// Get a single client correspondence by id
@http(method: "GET", uri: "/buckets/{projectId}/client/correspondences/{correspondenceId}.json")
operation GetClientCorrespondence {
  input: GetClientCorrespondenceInput
  output: GetClientCorrespondenceOutput
}

structure GetClientCorrespondenceInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  correspondenceId: ClientCorrespondenceId
}

structure GetClientCorrespondenceOutput {
  @httpPayload
  correspondence: ClientCorrespondence
}

// ===== Client Reply Operations =====

/// List all client replies for a recording (correspondence or approval)
@http(method: "GET", uri: "/buckets/{projectId}/client/recordings/{recordingId}/replies.json")
operation ListClientReplies {
  input: ListClientRepliesInput
  output: ListClientRepliesOutput
}

structure ListClientRepliesInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure ListClientRepliesOutput {
  @httpPayload
  replies: ClientReplyList
}

/// Get a single client reply by id
@http(method: "GET", uri: "/buckets/{projectId}/client/recordings/{recordingId}/replies/{replyId}.json")
operation GetClientReply {
  input: GetClientReplyInput
  output: GetClientReplyOutput
}

structure GetClientReplyInput {
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
  @httpPayload
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
@http(method: "GET", uri: "/buckets/{projectId}/webhooks.json")
operation ListWebhooks {
  input: ListWebhooksInput
  output: ListWebhooksOutput
}

structure ListWebhooksInput {
  @required
  @httpLabel
  projectId: ProjectId
}

structure ListWebhooksOutput {
  @httpPayload
  webhooks: WebhookList
}

/// Get a single webhook by id
@http(method: "GET", uri: "/buckets/{projectId}/webhooks/{webhookId}.json")
operation GetWebhook {
  input: GetWebhookInput
  output: GetWebhookOutput
}

structure GetWebhookInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  webhookId: WebhookId
}

structure GetWebhookOutput {
  @httpPayload
  webhook: Webhook
}

/// Create a new webhook for a project
@http(method: "POST", uri: "/buckets/{projectId}/webhooks.json")
operation CreateWebhook {
  input: CreateWebhookInput
  output: CreateWebhookOutput
}

structure CreateWebhookInput {
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
  @httpPayload
  webhook: Webhook
}

/// Update an existing webhook
@http(method: "PUT", uri: "/buckets/{projectId}/webhooks/{webhookId}.json")
operation UpdateWebhook {
  input: UpdateWebhookInput
  output: UpdateWebhookOutput
}

structure UpdateWebhookInput {
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
  @httpPayload
  webhook: Webhook
}

/// Delete a webhook
@http(method: "DELETE", uri: "/buckets/{projectId}/webhooks/{webhookId}.json")
operation DeleteWebhook {
  input: DeleteWebhookInput
  output: DeleteWebhookOutput
}

structure DeleteWebhookInput {
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
@http(method: "GET", uri: "/buckets/{projectId}/recordings/{recordingId}/events.json")
operation ListEvents {
  input: ListEventsInput
  output: ListEventsOutput
}

structure ListEventsInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure ListEventsOutput {
  @httpPayload
  events: EventList
}

// ===== Recording Operations =====

/// List recordings of a given type across projects
@http(method: "GET", uri: "/projects/recordings.json")
operation ListRecordings {
  input: ListRecordingsInput
  output: ListRecordingsOutput
}

structure ListRecordingsInput {
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
  @httpPayload
  recordings: RecordingList
}

/// Get a single recording by id
@http(method: "GET", uri: "/buckets/{projectId}/recordings/{recordingId}.json")
operation GetRecording {
  input: GetRecordingInput
  output: GetRecordingOutput
}

structure GetRecordingInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure GetRecordingOutput {
  @httpPayload
  recording: Recording
}

/// Trash a recording
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{recordingId}/status/trashed.json")
operation TrashRecording {
  input: TrashRecordingInput
  output: TrashRecordingOutput
}

structure TrashRecordingInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure TrashRecordingOutput {}

/// Archive a recording
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{recordingId}/status/archived.json")
operation ArchiveRecording {
  input: ArchiveRecordingInput
  output: ArchiveRecordingOutput
}

structure ArchiveRecordingInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure ArchiveRecordingOutput {}

/// Unarchive a recording (restore to active status)
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{recordingId}/status/active.json")
operation UnarchiveRecording {
  input: UnarchiveRecordingInput
  output: UnarchiveRecordingOutput
}

structure UnarchiveRecordingInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  recordingId: RecordingId
}

structure UnarchiveRecordingOutput {}

/// Set client visibility for a recording
@http(method: "PUT", uri: "/buckets/{projectId}/recordings/{recordingId}/client_visibility.json")
operation SetClientVisibility {
  input: SetClientVisibilityInput
  output: SetClientVisibilityOutput
}

structure SetClientVisibilityInput {
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
  @httpPayload
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
@http(method: "GET", uri: "/buckets/{projectId}/questionnaires/{questionnaireId}.json")
operation GetQuestionnaire {
  input: GetQuestionnaireInput
  output: GetQuestionnaireOutput
}

structure GetQuestionnaireInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  questionnaireId: QuestionnaireId
}

structure GetQuestionnaireOutput {
  @httpPayload
  questionnaire: Questionnaire
}

// ===== Question Operations =====

/// List all questions in a questionnaire
@http(method: "GET", uri: "/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json")
operation ListQuestions {
  input: ListQuestionsInput
  output: ListQuestionsOutput
}

structure ListQuestionsInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  questionnaireId: QuestionnaireId
}

structure ListQuestionsOutput {
  @httpPayload
  questions: QuestionList
}

/// Get a single question by id
@http(method: "GET", uri: "/buckets/{projectId}/questions/{questionId}.json")
operation GetQuestion {
  input: GetQuestionInput
  output: GetQuestionOutput
}

structure GetQuestionInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  questionId: QuestionId
}

structure GetQuestionOutput {
  @httpPayload
  question: Question
}

/// Create a new question in a questionnaire
@http(method: "POST", uri: "/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json")
operation CreateQuestion {
  input: CreateQuestionInput
  output: CreateQuestionOutput
}

structure CreateQuestionInput {
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
  @httpPayload
  question: Question
}

/// Update an existing question
@http(method: "PUT", uri: "/buckets/{projectId}/questions/{questionId}.json")
operation UpdateQuestion {
  input: UpdateQuestionInput
  output: UpdateQuestionOutput
}

structure UpdateQuestionInput {
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
  @httpPayload
  question: Question
}

// ===== Answer Operations =====

/// List all answers for a question
@http(method: "GET", uri: "/buckets/{projectId}/questions/{questionId}/answers.json")
operation ListAnswers {
  input: ListAnswersInput
  output: ListAnswersOutput
}

structure ListAnswersInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  questionId: QuestionId
}

structure ListAnswersOutput {
  @httpPayload
  answers: QuestionAnswerList
}

/// Get a single answer by id
@http(method: "GET", uri: "/buckets/{projectId}/question_answers/{answerId}.json")
operation GetAnswer {
  input: GetAnswerInput
  output: GetAnswerOutput
}

structure GetAnswerInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  answerId: AnswerId
}

structure GetAnswerOutput {
  @httpPayload
  answer: QuestionAnswer
}

/// Create a new answer for a question
@http(method: "POST", uri: "/buckets/{projectId}/questions/{questionId}/answers.json")
operation CreateAnswer {
  input: CreateAnswerInput
  output: CreateAnswerOutput
}

structure CreateAnswerInput {
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
  @httpPayload
  answer: QuestionAnswer
}

/// Update an existing answer
@http(method: "PUT", uri: "/buckets/{projectId}/question_answers/{answerId}.json")
operation UpdateAnswer {
  input: UpdateAnswerInput
  output: UpdateAnswerOutput
}

structure UpdateAnswerInput {
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

structure UpdateAnswerOutput {
  @httpPayload
  answer: QuestionAnswer
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
@http(method: "GET", uri: "/search.json")
operation Search {
  input: SearchInput
  output: SearchOutput
}

structure SearchInput {
  @required
  @httpQuery("query")
  query: String

  @httpQuery("sort")
  sort: SearchSortField
}

structure SearchOutput {
  @httpPayload
  results: SearchResultList
}

/// Get search metadata (available filter options)
@http(method: "GET", uri: "/searches/metadata.json")
operation GetSearchMetadata {
  input: GetSearchMetadataInput
  output: GetSearchMetadataOutput
}

structure GetSearchMetadataInput {}

structure GetSearchMetadataOutput {
  @httpPayload
  metadata: SearchMetadata
}

// ===== Template Operations =====

/// List all templates visible to the current user
@http(method: "GET", uri: "/templates.json")
operation ListTemplates {
  input: ListTemplatesInput
  output: ListTemplatesOutput
}

structure ListTemplatesInput {
  @httpQuery("status")
  status: TemplateStatus
}

structure ListTemplatesOutput {
  @httpPayload
  templates: TemplateList
}

/// Get a single template by id
@http(method: "GET", uri: "/templates/{templateId}.json")
operation GetTemplate {
  input: GetTemplateInput
  output: GetTemplateOutput
}

structure GetTemplateInput {
  @required
  @httpLabel
  templateId: TemplateId
}

structure GetTemplateOutput {
  @httpPayload
  template: Template
}

/// Create a new template
@http(method: "POST", uri: "/templates.json")
operation CreateTemplate {
  input: CreateTemplateInput
  output: CreateTemplateOutput
}

structure CreateTemplateInput {
  @required
  name: String

  description: String
}

structure CreateTemplateOutput {
  @httpPayload
  template: Template
}

/// Update an existing template
@http(method: "PUT", uri: "/templates/{templateId}.json")
operation UpdateTemplate {
  input: UpdateTemplateInput
  output: UpdateTemplateOutput
}

structure UpdateTemplateInput {
  @required
  @httpLabel
  templateId: TemplateId

  name: String

  description: String
}

structure UpdateTemplateOutput {
  @httpPayload
  template: Template
}

/// Delete a template (trash it)
@http(method: "DELETE", uri: "/templates/{templateId}.json")
operation DeleteTemplate {
  input: DeleteTemplateInput
  output: DeleteTemplateOutput
}

structure DeleteTemplateInput {
  @required
  @httpLabel
  templateId: TemplateId
}

structure DeleteTemplateOutput {}

/// Create a project from a template (asynchronous)
@http(method: "POST", uri: "/templates/{templateId}/project_constructions.json")
operation CreateProjectFromTemplate {
  input: CreateProjectFromTemplateInput
  output: CreateProjectFromTemplateOutput
}

structure CreateProjectFromTemplateInput {
  @required
  @httpLabel
  templateId: TemplateId

  @required
  name: String

  description: String
}

structure CreateProjectFromTemplateOutput {
  @httpPayload
  construction: ProjectConstruction
}

/// Get the status of a project construction
@http(method: "GET", uri: "/templates/{templateId}/project_constructions/{constructionId}.json")
operation GetProjectConstruction {
  input: GetProjectConstructionInput
  output: GetProjectConstructionOutput
}

structure GetProjectConstructionInput {
  @required
  @httpLabel
  templateId: TemplateId

  @required
  @httpLabel
  constructionId: ConstructionId
}

structure GetProjectConstructionOutput {
  @httpPayload
  construction: ProjectConstruction
}

// ===== Tool Operations =====

/// Get a dock tool by id
@http(method: "GET", uri: "/buckets/{projectId}/dock/tools/{toolId}.json")
operation GetTool {
  input: GetToolInput
  output: GetToolOutput
}

structure GetToolInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  toolId: ToolId
}

structure GetToolOutput {
  @httpPayload
  tool: Tool
}

/// Clone an existing tool to create a new one
@http(method: "POST", uri: "/buckets/{projectId}/dock/tools/{sourceToolId}/clone.json")
operation CloneTool {
  input: CloneToolInput
  output: CloneToolOutput
}

structure CloneToolInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  sourceToolId: ToolId
}

structure CloneToolOutput {
  @httpPayload
  tool: Tool
}

/// Update (rename) an existing tool
@http(method: "PUT", uri: "/buckets/{projectId}/dock/tools/{toolId}.json")
operation UpdateTool {
  input: UpdateToolInput
  output: UpdateToolOutput
}

structure UpdateToolInput {
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
  @httpPayload
  tool: Tool
}

/// Delete a tool (trash it)
@http(method: "DELETE", uri: "/buckets/{projectId}/dock/tools/{toolId}.json")
operation DeleteTool {
  input: DeleteToolInput
  output: DeleteToolOutput
}

structure DeleteToolInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  toolId: ToolId
}

structure DeleteToolOutput {}

/// Enable a tool (show it on the project dock)
@http(method: "POST", uri: "/buckets/{projectId}/dock/tools/{toolId}/position.json")
operation EnableTool {
  input: EnableToolInput
  output: EnableToolOutput
}

structure EnableToolInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  toolId: ToolId
}

structure EnableToolOutput {}

/// Disable a tool (hide it from the project dock)
@http(method: "DELETE", uri: "/buckets/{projectId}/dock/tools/{toolId}/position.json")
operation DisableTool {
  input: DisableToolInput
  output: DisableToolOutput
}

structure DisableToolInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  toolId: ToolId
}

structure DisableToolOutput {}

/// Reposition a tool on the project dock
@http(method: "PUT", uri: "/buckets/{projectId}/dock/tools/{toolId}/position.json")
operation RepositionTool {
  input: RepositionToolInput
  output: RepositionToolOutput
}

structure RepositionToolInput {
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
@http(method: "POST", uri: "/lineup/markers.json")
operation CreateLineupMarker {
  input: CreateLineupMarkerInput
  output: CreateLineupMarkerOutput
}

structure CreateLineupMarkerInput {
  @required
  title: String

  @required
  starts_on: ISO8601Date

  @required
  ends_on: ISO8601Date

  color: String
  description: String
}

structure CreateLineupMarkerOutput {
  @httpPayload
  marker: LineupMarker
}

/// Update an existing lineup marker
@http(method: "PUT", uri: "/lineup/markers/{markerId}.json")
operation UpdateLineupMarker {
  input: UpdateLineupMarkerInput
  output: UpdateLineupMarkerOutput
}

structure UpdateLineupMarkerInput {
  @required
  @httpLabel
  markerId: MarkerId

  title: String
  starts_on: ISO8601Date
  ends_on: ISO8601Date
  color: String
  description: String
}

structure UpdateLineupMarkerOutput {
  @httpPayload
  marker: LineupMarker
}

/// Delete a lineup marker
@http(method: "DELETE", uri: "/lineup/markers/{markerId}.json")
operation DeleteLineupMarker {
  input: DeleteLineupMarkerInput
  output: DeleteLineupMarkerOutput
}

structure DeleteLineupMarkerInput {
  @required
  @httpLabel
  markerId: MarkerId
}

structure DeleteLineupMarkerOutput {}

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
