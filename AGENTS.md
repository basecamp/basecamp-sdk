# Basecamp SDK Agent Guidelines

## CRITICAL: Never Write SDK Code Manually

**STOP. Do not write service methods by hand.**

All SDK code MUST be generated from the Smithy spec. This applies to:
- Go SDK service methods
- TypeScript SDK service methods
- Ruby SDK service methods

### The Only Valid Workflow

1. **Add endpoints to `spec/basecamp.smithy`** - This is the ONLY place you write API definitions
2. **Run `make smithy-build`** - Generates OpenAPI spec
3. **Run SDK-specific generators**:
   - Go: `cd go && make generate`
   - TypeScript: `make ts-generate`
   - Ruby: `make rb-generate`
4. **Verify generated code compiles/typechecks**

### What "Generate From Spec" Means

- **DO**: Add operations, structures, and shapes to `basecamp.smithy`
- **DO**: Run generators to produce SDK code
- **DO NOT**: Manually write `async function getResource()` in TypeScript
- **DO NOT**: Manually write `def get_resource` in Ruby
- **DO NOT**: Manually write Go service methods that call the generated client

If the generators don't produce what you need, fix the generators or the spec - not the output.

### Why This Matters

Manual SDK code:
- Drifts from the spec
- Has inconsistent error handling
- Misses retry/pagination behaviors
- Creates type mismatches
- Is impossible to maintain across 3+ SDKs

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

## Generated Services Reference Implementation

The SDK contains two sets of service implementations:

### Hand-Written Services (Runtime)

Located in:
- **TypeScript**: `typescript/src/services/*.ts`
- **Ruby**: `ruby/lib/basecamp/services/*_service.rb`

These are the services actually wired into the SDK clients at runtime. They:
- Have rich documentation and examples
- Include client-side validation
- Are imported by the client modules

### Generated Services (Reference)

Located in:
- **TypeScript**: `typescript/src/generated/services/*.ts`
- **Ruby**: `ruby/lib/basecamp/generated/services/*_service.rb`

These are auto-generated from the OpenAPI spec and serve as a **reference implementation**:
- Always in sync with the OpenAPI spec (167 operations across 37 services)
- Not wired into the SDK clients at runtime
- Useful for verifying spec coverage and generator correctness
- Can inform hand-written service development

### Regenerating Services

```bash
# TypeScript
make ts-generate-services

# Ruby
make rb-generate-services

# Both (full build)
make
```

### Why Keep Both?

The hand-written services offer better developer experience (richer docs, validation), while generated services ensure spec conformance. Migration to fully generated services is deferred to avoid breaking changes.

**Verification:**
- TypeScript: `client.ts` imports from `./services/*`, not `./generated/services/*`
- Ruby: `basecamp.rb` autoloads from `lib/basecamp/services/`, not `lib/basecamp/generated/services/`
