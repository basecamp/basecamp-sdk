# Basecamp SDK Agent Guidelines

## Current Status

| Component | Status | Details |
|-----------|--------|---------|
| **Smithy Spec** | 175 operations | Single source of truth for all APIs |
| **Go SDK** | Production-ready | Full generated client + service wrappers |
| **TypeScript SDK** | Production-ready | 37 generated services, openapi-fetch based |
| **Ruby SDK** | Production-ready | 37 generated services |
| **Swift SDK** | Production-ready | 38 generated services, URLSession-based |

All four SDKs share the same architecture: **Smithy spec -> OpenAPI -> Generated services**. No hand-written API methods exist in any SDK runtime.

---

## SDK Development Rules

### NEVER Do These (Hard Rules)

1. **NEVER edit files under `*/generated/`** - They get overwritten by generators
2. **NEVER add hand-written service methods for API operations** - All API ops come from generators
3. **NEVER skip running `make smithy-build` after Smithy changes** - Keeps OpenAPI in sync
4. **NEVER construct API paths manually in SDK code** - Use the generated client methods

### Always Do These

1. **All new API coverage starts in `spec/basecamp.smithy`**
2. **Run generators after spec changes**: `make smithy-build` then SDK-specific generators
3. **Fix generators when output is wrong** - Don't patch generated files directly

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
| **Swift** | `URLSession` via `Transport` protocol | `Sources/Basecamp/Generated/Services/*.swift` | ✅ Complete |

**All four production SDKs have complete generated service layers** covering 175 operations across 38 services.

**TypeScript/Ruby runtime**: Wired to generated services (`src/generated/services/`, `lib/basecamp/generated/services/`). Hand-written services remain only for infrastructure (`base.ts`/`base_service.rb`) and OAuth (`authorization.ts`/`authorization_service.rb`).

### Required Workflow for Adding API Coverage

Every new API endpoint MUST follow this sequence:

1. **Add operation to `spec/basecamp.smithy`** - This is mandatory, not optional
2. **Run `make smithy-build`** - Regenerates `openapi.json`
3. **Run SDK generators**:
   - `cd go && make generate`
   - `make ts-generate-services`
   - `make rb-generate-services`
   - `make swift-generate`
4. **Run `make`** - Verifies all SDKs build and pass tests

That's it. Generated services are the runtime—no hand-written service updates needed.

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

## TypeScript/Ruby Service Layer

**Switchover complete.** Both SDKs use generated services at runtime.

| SDK | Runtime Services | Infrastructure |
|-----|-----------------|----------------|
| **TypeScript** | `src/generated/services/*.ts` | `src/services/base.ts`, `src/services/authorization.ts` |
| **Ruby** | `lib/basecamp/generated/services/*.rb` | `lib/basecamp/services/base_service.rb`, `lib/basecamp/services/authorization_service.rb` |
| **Swift** | `swift/Sources/Basecamp/Generated/Services/*.swift` | `swift/Sources/Basecamp/Services/BaseService.swift` |

Hand-written API services in `src/services/` (TS) and `lib/basecamp/services/` (Ruby) are NOT loaded at runtime. They exist only as reference implementations.
