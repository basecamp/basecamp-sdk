# Basecamp SDK Agent Guidelines

## Smithy-First Development Workflow

When extending the Basecamp SDK, follow a Smithy-first approach where the API specification drives implementation.

### 1. Design in Smithy First

Before writing Go code, add operations and shapes to `/spec/basecamp.smithy`:

1. **Add operations to the service definition** - Include new operations in the `Basecamp` service's `operations` list
2. **Define the HTTP binding** - Use `@http` with method, uri, and appropriate traits
3. **Create input/output structures** - Define request parameters with `@httpLabel`, `@httpQuery`, `@httpPayload` as needed
4. **Add shared shapes** - Reuse existing types (ProjectId, PersonId, ISO8601Timestamp, etc.) where possible

### 2. Smithy Patterns

Follow existing patterns in the spec:

```smithy
/// Operation documentation
@http(method: "GET", uri: "/buckets/{projectId}/resources/{resourceId}.json")
operation GetResource {
  input: GetResourceInput
  output: GetResourceOutput
}

structure GetResourceInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  resourceId: ResourceId
}

structure GetResourceOutput {
  @httpPayload
  resource: Resource
}
```

### 3. Reference Sources

When adding new API coverage:

- **Go SDK** (`go/pkg/basecamp/*.go`) - Check existing implementations for operation signatures
- **BC3 API docs** (`~/Work/basecamp/bc3-api/sections/*.md`) - Authoritative HTTP endpoint documentation
- **Existing Smithy** (`spec/basecamp.smithy`) - Follow established patterns and reuse types

### 4. Naming Conventions

- Operations: `Verb` + `Noun` (e.g., `ListTodos`, `GetProject`, `CreateMessage`, `TrashComment`)
- Input structures: `{OperationName}Input`
- Output structures: `{OperationName}Output`
- IDs: `{Resource}Id` as `long` type (e.g., `MessageId`, `CommentId`)
- Status enums: Use `@documentation` string with valid values (e.g., `"active|archived|trashed"`)

### 5. Common Patterns

**Bucket-scoped resources** (most Basecamp resources):
```
/buckets/{projectId}/{resources}/{resourceId}.json
```

**Recording operations** (trash, archive, unarchive):
```
/buckets/{projectId}/recordings/{recordingId}/status/trashed.json
/buckets/{projectId}/recordings/{recordingId}/status/archived.json
/buckets/{projectId}/recordings/{recordingId}/status/active.json
```

**Nested resources** (e.g., comments on recordings):
```
/buckets/{projectId}/recordings/{recordingId}/comments.json
```

**Account-level reports**:
```
/reports/{reportType}.json
```

### 6. Shape Reuse

Reuse these common shapes throughout the spec:

- `ProjectId` - Long identifier for projects (buckets)
- `PersonId` - Long identifier for people
- `ISO8601Timestamp` - String for datetime fields
- `ISO8601Date` - String for date-only fields
- `Person` - Full person object structure
- `Parent` - Parent reference (id, title, type, url, app_url)
- `Bucket` / `TodoBucket` - Project reference in recordings
