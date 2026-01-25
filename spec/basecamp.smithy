$version: "2"

namespace basecamp

use smithy.api#documentation
use smithy.api#http
use smithy.api#httpLabel
use smithy.api#httpQuery
use smithy.api#httpPayload
use smithy.api#required

/// Basecamp API (projects slice)
service Basecamp {
  version: "2026-01-25"
  operations: [
    ListProjects,
    GetProject,
    CreateProject,
    UpdateProject,
    TrashProject
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
