# Basecamp SDK Agent Guidelines

## SDK Development Rules

### NEVER Do These (Hard Rules)

1. **NEVER edit files under `*/generated/`** - They get overwritten by generators
2. **NEVER add API endpoints to hand-written services without ALSO adding them to `spec/basecamp.smithy`** - Creates spec drift
3. **NEVER skip running `make smithy-build` after Smithy changes** - Keeps OpenAPI in sync
4. **NEVER construct API paths manually in SDK code** - Use the generated client methods

### Always Do These

1. **All new API coverage starts in `spec/basecamp.smithy`**
2. **Run generators after spec changes**: `make smithy-build` then SDK-specific generators
3. **Update hand-written services (TypeScript/Ruby) when adding new operations** - They're the runtime implementation
4. **Fix generators when output is wrong** - Don't patch generated files directly

### SDK Architecture

**Target architecture (all SDKs):**
```
Smithy Spec → OpenAPI → Generated Client → Service Layer → User
```

| SDK | Generated Client | Service Layer | Status |
|-----|-----------------|---------------|--------|
| **Go** | `pkg/generated/client.gen.go` | `pkg/basecamp/*.go` (wraps generated client) | ✅ Complete |
| **TypeScript** | `openapi-fetch` + `schema.d.ts` | `src/generated/services/*.ts` | ✅ Complete |
| **Ruby** | HTTP client | `lib/basecamp/generated/services/*.rb` | ✅ Complete |

**All three SDKs have complete generated service layers** covering 167 operations across 37 services.

**TypeScript/Ruby runtime**: Currently wired to hand-written services (`src/services/`, `lib/basecamp/services/`) which serve as a quality benchmark. Switchover to generated services is a one-line import path change when ready.

### Required Workflow for Adding API Coverage

Every new API endpoint MUST follow this sequence:

1. **Add operation to `spec/basecamp.smithy`** - This is mandatory, not optional
2. **Run `make smithy-build`** - Regenerates `openapi.json`
3. **Run SDK generators**:
   - `cd go && make generate`
   - `make ts-generate-services`
   - `make rb-generate-services`
4. **Update hand-written services** (TypeScript/Ruby) - Match the generated services
5. **Run `make`** - Verifies all SDKs build and pass tests

Skipping steps 1-3 and only updating hand-written services creates spec drift.

---

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
- `TodoParent` / `RecordingParent` - Parent reference (id, title, type, url, app_url)
- `TodoBucket` / `RecordingBucket` - Project reference in recordings

---

## TypeScript/Ruby Switchover

Generated services are complete. Switchover from hand-written to generated is pending quality validation.

| Implementation | Location | Status |
|---------------|----------|--------|
| **Generated** | `src/generated/services/` (TS), `lib/basecamp/generated/services/` (Ruby) | ✅ Complete (167 ops) |
| **Hand-written** | `src/services/` (TS), `lib/basecamp/services/` (Ruby) | Current runtime (quality benchmark) |

### To Switch Over

**TypeScript**: Change imports in `client.ts` from `./services/*` to `./generated/services/*`

**Ruby**: Remove `loader.ignore("#{__dir__}/basecamp/generated")` from `basecamp.rb`

### Quality Checklist Before Switchover

- [ ] Generated services have adequate JSDoc/YARD comments
- [ ] Error messages are clear and actionable
- [ ] Pagination works correctly
- [ ] Type safety is equivalent or better
- [ ] No regressions in usability
