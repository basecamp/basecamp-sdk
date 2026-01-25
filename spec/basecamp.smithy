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
    TrashTodolistGroup
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
