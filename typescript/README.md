# Basecamp TypeScript SDK

[![npm version](https://img.shields.io/npm/v/@basecamp/sdk.svg)](https://www.npmjs.com/package/@basecamp/sdk)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0+-blue.svg)](https://www.typescriptlang.org/)
[![Test](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml/badge.svg)](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml)

Official TypeScript SDK for the [Basecamp 3 API](https://github.com/basecamp/bc3-api).

## Features

- Full type safety with TypeScript generics
- 30+ services covering the complete Basecamp API
- OAuth 2.0 with PKCE support
- ETag-based HTTP caching
- Automatic retry with exponential backoff
- Pagination helpers for large result sets
- Observability hooks for logging, metrics, and tracing
- OpenTelemetry integration

## Installation

```bash
npm install @basecamp/sdk
```

Requires Node.js 18+ and TypeScript 5.0+.

## Quick Start

```ts
import { createBasecampClient } from "@basecamp/sdk";

const client = createBasecampClient({
  accountId: process.env.BASECAMP_ACCOUNT_ID!,
  accessToken: process.env.BASECAMP_TOKEN!,
});

// List all projects
const projects = await client.projects.list();
for (const project of projects) {
  console.log(`${project.id}: ${project.name}`);
}
```

## Configuration

### Client Options

```ts
import { createBasecampClient } from "@basecamp/sdk";

const client = createBasecampClient({
  // Required
  accountId: "12345",
  accessToken: "your-token", // or async token provider

  // Optional
  baseUrl: "https://3.basecampapi.com/12345", // default
  userAgent: "my-app/1.0",
  enableCache: true, // ETag caching (default: true)
  enableRetry: true, // Auto retry 429 and 503 (default: true)
  hooks: myHooks, // Observability hooks
});
```

### Token Providers

For simple use cases, pass a static token string:

```ts
const client = createBasecampClient({
  accountId: "12345",
  accessToken: "your-access-token",
});
```

For token refresh scenarios, pass an async function:

```ts
const client = createBasecampClient({
  accountId: "12345",
  accessToken: async () => {
    // Fetch or refresh your token
    const token = await myTokenStore.getValidToken();
    return token.accessToken;
  },
});
```

## OAuth 2.0

The SDK includes utilities for implementing OAuth 2.0 with PKCE support.

### Authorization Flow

```ts
import {
  discoverLaunchpad,
  generatePKCE,
  generateState,
  exchangeCode,
  refreshToken,
  isTokenExpired,
} from "@basecamp/sdk";

// 1. Discover OAuth endpoints
const config = await discoverLaunchpad();

// 2. Generate PKCE challenge and state
const pkce = await generatePKCE();
const state = generateState();

// Store pkce.verifier and state in session for later

// 3. Build authorization URL with PKCE challenge
const authUrl = new URL(config.authorizationEndpoint);
authUrl.searchParams.set("type", "web_server");
authUrl.searchParams.set("client_id", CLIENT_ID);
authUrl.searchParams.set("redirect_uri", REDIRECT_URI);
authUrl.searchParams.set("state", state);
authUrl.searchParams.set("code_challenge", pkce.challenge);
authUrl.searchParams.set("code_challenge_method", "S256");
// Redirect user to authUrl.toString()

// 4. Exchange code for tokens (in callback handler)
const token = await exchangeCode({
  tokenEndpoint: config.tokenEndpoint,
  code: callbackParams.code,
  redirectUri: REDIRECT_URI,
  clientId: CLIENT_ID,
  clientSecret: CLIENT_SECRET,
  codeVerifier: pkce.verifier, // PKCE verifier from step 2
  useLegacyFormat: true, // Required for Basecamp Launchpad
});

// 5. Refresh when expired
if (isTokenExpired(token)) {
  const newToken = await refreshToken({
    tokenEndpoint: config.tokenEndpoint,
    refreshToken: token.refreshToken!,
    useLegacyFormat: true,
  });
}
```

## Services

The SDK provides typed services for the complete Basecamp API:

### Projects & Organization

| Service | Methods |
|---------|---------|
| `projects` | list, get, create, update, trash |
| `templates` | list, get, createProject |
| `tools` | list, get, update |
| `people` | list, get, me, listPingable |

### To-dos

| Service | Methods |
|---------|---------|
| `todos` | list, get, create, update, trash, complete, uncomplete, reposition |
| `todolists` | list, get, create, update, trash |
| `todosets` | get |
| `todolistGroups` | list, get, create, reposition |

### Messages & Communication

| Service | Methods |
|---------|---------|
| `messages` | list, get, create, update, pin, unpin |
| `messageBoards` | get |
| `messageTypes` | list, get, create, update, delete |
| `comments` | list, get, create, update |
| `campfires` | list, get, listLines, getLine, createLine, deleteLine |

### Card Tables (Kanban)

| Service | Methods |
|---------|---------|
| `cardTables` | get, listColumns |
| `cards` | list, get, create, update, move |
| `cardColumns` | get, create, update, move |
| `cardSteps` | list, get, create, update, complete, uncomplete |

### Scheduling

| Service | Methods |
|---------|---------|
| `schedules` | get, listEntries, getEntry, createEntry, updateEntry, trashEntry |
| `lineup` | create, update, delete |
| `checkins` | get, listQuestions, getQuestion, listAnswers, getAnswer |

### Files & Documents

| Service | Methods |
|---------|---------|
| `vaults` | list, get, create, update |
| `documents` | list, get, create, update, trash |
| `uploads` | list, get, create, update, trash |
| `attachments` | createUploadUrl, create |

### Integrations & Events

| Service | Methods |
|---------|---------|
| `webhooks` | list, get, create, update, delete |
| `subscriptions` | get, subscribe, unsubscribe, update |
| `events` | list, listForRecording |
| `recordings` | archive, unarchive, trash |

### Search & Reports

| Service | Methods |
|---------|---------|
| `search` | search |
| `reports` | progress, upcoming, assigned, overdue, personProgress |
| `timesheets` | forRecording, forProject, report |
| `timeline` | get |

### Client Portal

| Service | Methods |
|---------|---------|
| `clientApprovals` | list, get |
| `clientCorrespondences` | list, get |
| `clientReplies` | list, get |
| `clientVisibility` | get, update |

### Email

| Service | Methods |
|---------|---------|
| `forwards` | list, get, createReply |

## Pagination

List methods return a single page of results by default. Use the pagination helpers with low-level API calls to fetch all pages:

```ts
import { fetchAllPages, paginateAll } from "@basecamp/sdk";

// First, fetch the initial page using the low-level client
const initialResponse = await client.GET("/projects.json");

// Option 1: fetchAllPages - returns all results as an array
const allProjects = await fetchAllPages(
  initialResponse.response,
  (response) => response.json()
);

// Option 2: paginateAll - async generator for streaming large result sets
for await (const page of paginateAll(
  initialResponse.response,
  (response) => response.json()
)) {
  for (const project of page) {
    console.log(project.name);
  }
}
```

Paginated endpoints include an `X-Total-Count` HTTP header when available. You can access this header via the `response.headers` field on low-level `client.GET`/`client.POST` calls.

## Low-Level API Access

For endpoints not covered by services or advanced use cases, use the raw typed client:

```ts
// Direct API calls with full type inference
const { data, error, response } = await client.GET("/projects.json");

if (error) {
  console.error("Failed:", error);
} else {
  console.log(data.map((p) => p.name));
}

// With path parameters
const { data: project } = await client.GET("/projects/{projectId}", {
  params: { path: { projectId: 12345 } },
});

// POST with body
const { data: newProject } = await client.POST("/projects.json", {
  body: { name: "My Project", description: "A new project" },
});
```

## Error Handling

The SDK provides structured errors with codes, hints, and exit codes for CLI applications:

```ts
import { BasecampError, isBasecampError, isErrorCode } from "@basecamp/sdk";

try {
  await client.todos.get(projectId, todoId);
} catch (err) {
  if (isBasecampError(err)) {
    console.error(`Error [${err.code}]: ${err.message}`);

    if (err.hint) {
      console.error(`Hint: ${err.hint}`);
    }

    if (err.retryable && err.retryAfter) {
      console.log(`Retry after ${err.retryAfter} seconds`);
    }

    // Use exit codes for CLI applications
    process.exit(err.exitCode);
  }
  throw err;
}
```

### Error Codes

| Code | HTTP Status | Exit Code | Description |
|------|-------------|-----------|-------------|
| `auth` | 401 | 3 | Authentication required |
| `forbidden` | 403 | 4 | Access denied |
| `not_found` | 404 | 2 | Resource not found |
| `rate_limit` | 429 | 5 | Rate limit exceeded (retryable) |
| `validation` | 400, 422 | 1 | Invalid request data |
| `network` | - | 6 | Network error (retryable) |
| `api_error` | 5xx | 7 | Server error |
| `usage` | - | 1 | Configuration or argument error |

## Retry Behavior

The SDK automatically retries requests on transient failures:

- **Retryable errors**: 429 (rate limit) and 503 (service unavailable)
- **Backoff**: Exponential with jitter
- **Rate limits**: Respects `Retry-After` header
- **Max retries**: 3 attempts by default

Disable retry for specific use cases:

```ts
const client = createBasecampClient({
  accountId: "12345",
  accessToken: "token",
  enableRetry: false,
});
```

## Caching

The SDK uses ETag-based HTTP caching to reduce API calls and respect Basecamp's rate limits:

```ts
// First request fetches from API
const projects = await client.projects.list();

// Second request returns cached data if unchanged (304 Not Modified)
const projects2 = await client.projects.list();
```

Disable caching if needed:

```ts
const client = createBasecampClient({
  accountId: "12345",
  accessToken: "token",
  enableCache: false,
});
```

## Observability

### Console Logging

For debugging or verbose CLI modes:

```ts
import { createBasecampClient, consoleHooks } from "@basecamp/sdk";

const client = createBasecampClient({
  accountId: "12345",
  accessToken: "token",
  hooks: consoleHooks({
    logOperations: true,
    logRequests: true, // More verbose
    logRetries: true,
    minDurationMs: 100, // Only log slow requests
  }),
});
```

Output:
```
[Basecamp] Projects.List
[Basecamp] -> GET https://3.basecampapi.com/12345/projects.json
[Basecamp] <- GET https://3.basecampapi.com/12345/projects.json 200 (145ms)
[Basecamp] Projects.List completed (147ms)
```

### Custom Hooks

Implement the `BasecampHooks` interface for custom observability:

```ts
import type { BasecampHooks } from "@basecamp/sdk";

const metricsHooks: BasecampHooks = {
  onOperationStart(info) {
    metrics.startTimer(`${info.service}.${info.operation}`);
  },

  onOperationEnd(info, result) {
    metrics.recordDuration(`${info.service}.${info.operation}`, result.durationMs);
    if (result.error) {
      metrics.incrementError(`${info.service}.${info.operation}`);
    }
  },

  onRetry(info, attempt, error, delayMs) {
    logger.warn(`Retrying ${info.method} ${info.url} (attempt ${attempt})`);
  },
};

const client = createBasecampClient({
  accountId: "12345",
  accessToken: "token",
  hooks: metricsHooks,
});
```

### OpenTelemetry Integration

For distributed tracing and metrics:

```ts
import { createBasecampClient, otelHooks } from "@basecamp/sdk";
import { trace, metrics } from "@opentelemetry/api";

const tracer = trace.getTracer("my-app");
const meter = metrics.getMeter("my-app");

const client = createBasecampClient({
  accountId: "12345",
  accessToken: "token",
  hooks: otelHooks({
    tracer,
    meter,
    recordRequestSpans: true, // Include HTTP-level spans
  }),
});
```

Creates spans and metrics:
- `basecamp.operation.duration` - Histogram of operation durations
- `basecamp.operations.total` - Counter of operations
- `basecamp.errors.total` - Counter of errors
- `basecamp.retries.total` - Counter of retry attempts

### Combining Multiple Hooks

```ts
import { chainHooks, consoleHooks, otelHooks } from "@basecamp/sdk";

const client = createBasecampClient({
  accountId: "12345",
  accessToken: "token",
  hooks: chainHooks(
    consoleHooks(),
    otelHooks({ tracer, meter }),
    myCustomHooks,
  ),
});
```

## Examples

### Working with Todos

```ts
// List todos in a todolist
const todos = await client.todos.list(projectId, todolistId);

// Create a todo with assignees
const todo = await client.todos.create(projectId, todolistId, {
  content: "Review pull request",
  description: "<p>Check the new auth flow</p>",
  dueOn: "2026-02-01",
  assigneeIds: [12345, 67890],
});

// Complete a todo
await client.todos.complete(projectId, todo.id);

// Reposition a todo to the top
await client.todos.reposition(projectId, todo.id, { position: 1 });
```

### Working with Messages

```ts
// Get the message board for a project
const board = await client.messageBoards.get(projectId);

// List messages
const messages = await client.messages.list(projectId, board.id);

// Create a message
const msg = await client.messages.create(projectId, board.id, {
  subject: "Weekly Update",
  content: "<p>Here's what we accomplished...</p>",
});

// Pin a message
await client.messages.pin(projectId, msg.id);
```

### Working with Campfire

```ts
// List campfires
const campfires = await client.campfires.list();

// Send a message
await client.campfires.createLine(projectId, campfireId, {
  content: "Hello, team!",
});

// List recent messages
const lines = await client.campfires.listLines(projectId, campfireId);
```

### Working with Webhooks

```ts
// Create a webhook
const webhook = await client.webhooks.create(projectId, {
  payloadUrl: "https://example.com/webhook",
  types: ["Todo", "Comment"],
});

// List webhooks
const webhooks = await client.webhooks.list(projectId);

// Delete a webhook
await client.webhooks.delete(projectId, webhook.id);
```

## TypeScript Types

All types are exported for use in your code:

```ts
import type {
  Project,
  Todo,
  Message,
  Person,
  CreateTodoRequest,
  BasecampError,
  ErrorCode,
} from "@basecamp/sdk";

function processTodo(todo: Todo): void {
  console.log(todo.content);
}

function createTodo(data: CreateTodoRequest): Promise<Todo> {
  return client.todos.create(projectId, todolistId, data);
}
```

## Development

```bash
# Install dependencies
npm install

# Generate types from OpenAPI spec
npm run generate

# Build
npm run build

# Run tests
npm test

# Type check
npm run typecheck

# Lint
npm run lint
```

## License

MIT
