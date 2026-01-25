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
    UncompleteTodo
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
