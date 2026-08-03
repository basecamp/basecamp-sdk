# Basecamp TypeScript SDK

[![npm version](https://img.shields.io/npm/v/@37signals/basecamp.svg)](https://www.npmjs.com/package/@37signals/basecamp)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0+-blue.svg)](https://www.typescriptlang.org/)
[![Test](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml/badge.svg)](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml)

Official TypeScript SDK for the [Basecamp API](https://github.com/basecamp/bc3-api).

## Features

- Full type safety with TypeScript generics
- 30+ services covering the complete Basecamp API
- OAuth 2.0 with PKCE support
- ETag-based HTTP caching (opt-in)
- Automatic retry with exponential backoff
- Pagination helpers for large result sets
- Observability hooks for logging, metrics, and tracing
- OpenTelemetry integration

## Installation

```bash
npm install @37signals/basecamp
```

Requires Node.js 22.12+ and TypeScript 5.0+.

## Getting a token

Every Basecamp API request carries an OAuth 2.0 access token. There is no API key and no personal access token, so even a throwaway script starts here:

1. Choose the grant that matches how your code runs:

| Your integration | Grant | Who refreshes the token |
|---|---|---|
| already holds a token you obtained elsewhere | **static token** — [`Token Providers`](#token-providers) | you do |
| can receive a browser redirect (web app, or a local callback server) | **authorization code + PKCE** — [`Manual Authorization Flow`](#manual-authorization-flow), or [`Interactive Login`](#interactive-login-cli--desktop) for a CLI | your `accessToken` function, which the SDK re-invokes per request |
| has no browser, but a person can approve on another device (CLI, headless server, TV) | **device flow** — [`Device flow`](#device-flow-rfc-8628) | your `accessToken` function, which the SDK re-invokes per request |

The one-line rule: **a redirect URI you control → authorization code; no browser but someone to approve → device flow; a token already in hand → static token.** An unattended daemon or CI job fits none of the three on its own — the device flow needs a person to enter the user code at the verification URI — so provision a token out of band and hand it to the process as a static or refresh token.

2. Get the client credentials that grant needs:

- **Authorization code + PKCE** — register your own integration at **<https://launchpad.37signals.com/integrations>**. You get a client ID, a client secret, and whatever redirect URI you nominated.
- **Device flow** — nothing to register. It runs as the pre-registered public `basecamp-cli` client, which sends no secret, against the device endpoint that discovery returns. Launchpad advertises no device endpoint, so a client you register there is not the one this flow uses.
- **Static token** — nothing to register; you already hold the token.

A bare `accessToken` string is never refreshed — once it expires every call fails with `401` until you supply a new one. Use it to get a first successful call, then pass an async token provider or an `AuthStrategy` before you ship.

## Finding your account ID

Every API path is scoped to an account — `https://3.basecampapi.com/{accountId}/…` — so `createBasecampClient` needs that number before your first call. One token can reach several accounts, so ask the token which. `getInfo()` addresses Launchpad by default, which is right for a Launchpad-issued token; a **device-flow** token is issued by the discovered BC5 server, so pass that issuer as the `endpoint` option:

```ts
import { createBasecampClient } from "@37signals/basecamp";

const token = process.env.BASECAMP_TOKEN!;

// createBasecampClient always requires an accountId, but authorization.getInfo()
// talks to Launchpad rather than the account-scoped API, so a placeholder is fine
// for the bootstrap call.
const bootstrap = createBasecampClient({ accountId: "0", accessToken: token });

// "bc3" is Basecamp; the unfiltered response also carries "hey" and other products.
// endpoint defaults to Launchpad, which is right for a Launchpad-issued token.
// For a device-flow token, pass the discovered issuer:
//   { filterProduct: "bc3", endpoint: `${issuer}/authorization.json` }
const info = await bootstrap.authorization.getInfo({ filterProduct: "bc3" });
for (const account of info.accounts) {
  console.log(`${account.id}: ${account.name}`);
}

// Rebuild with the real account ID — that is the client you keep.
const client = createBasecampClient({
  accountId: String(info.accounts[0].id),
  accessToken: token,
});
```

`info.expiresAt` is a `Date` telling you how long the token has left, which is the quickest way to confirm a static token has not lapsed.

## Quick Start

```ts
import { createBasecampClient } from "@37signals/basecamp";

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
import { createBasecampClient } from "@37signals/basecamp";

const client = createBasecampClient({
  // Required
  accountId: "12345",
  accessToken: "your-token", // or async token provider

  // Optional
  baseUrl: "https://3.basecampapi.com/12345", // default
  userAgent: "my-app/1.0",
  enableCache: true, // ETag caching (default: false)
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

The SDK includes utilities for implementing OAuth 2.0 with automatic PKCE negotiation.
PKCE parameters are included only when the server's discovery metadata advertises
`code_challenge_methods_supported: ["S256"]` (per [RFC 8414](https://www.rfc-editor.org/rfc/rfc8414)
and [RFC 7636](https://www.rfc-editor.org/rfc/rfc7636)).

### Interactive Login (CLI / Desktop)

`performInteractiveLogin` handles the full flow — discovery, PKCE negotiation, local
callback server, browser launch, code exchange, and token storage:

```ts
import { performInteractiveLogin } from "@37signals/basecamp";
import open from "open";

const token = await performInteractiveLogin({
  clientId: CLIENT_ID,
  clientSecret: CLIENT_SECRET,
  store: myTokenStore,
  openBrowser: (url) => open(url),
  onStatus: (msg) => console.log(msg),
});
```

### Manual Authorization Flow

For web apps or custom flows, use the lower-level helpers directly:

```ts
import {
  discoverLaunchpad,
  buildAuthorizationUrl,
  generatePKCE,
  generateState,
  exchangeCode,
  refreshToken,
  isTokenExpired,
} from "@37signals/basecamp";

// 1. Discover OAuth endpoints
const config = await discoverLaunchpad();

// 2. Generate PKCE (only if the server supports S256) and state
const supportsPKCE = config.codeChallengeMethodsSupported?.includes("S256") ?? false;
const pkce = supportsPKCE ? await generatePKCE() : undefined;
const state = generateState();

// Store pkce?.verifier and state in session for later

// 3. Build authorization URL
const authUrl = buildAuthorizationUrl({
  authorizationEndpoint: config.authorizationEndpoint,
  clientId: CLIENT_ID,
  redirectUri: REDIRECT_URI,
  state,
  pkce,
});
// Redirect user to authUrl.toString()

// 4. Exchange code for tokens (in callback handler)
const token = await exchangeCode({
  tokenEndpoint: config.tokenEndpoint,
  code: callbackParams.code,
  redirectUri: REDIRECT_URI,
  clientId: CLIENT_ID,
  clientSecret: CLIENT_SECRET,
  codeVerifier: pkce?.verifier,
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

### Resource-first discovery (BC5)

BC5 serves its Authorization Server metadata only at its canonical issuer (the
web host), so discovery starts from the **resource** (RFC 9728) and composes with
AS discovery (RFC 8414):

```ts
import { discoverFromResource, DiscoverySelectionError } from "@37signals/basecamp";

const result = await discoverFromResource("https://3.basecampapi.com");
if (result.kind === "selected") {
  // result.config is bound + validated for result.issuer
} else {
  // result.reason is "resource_discovery_failed" | "no_as_advertised"
  // → fall back to Launchpad (discoverLaunchpad())
}
```

`performInteractiveLogin` supports this via `resourceBaseUrl` (mutually exclusive
with the legacy `baseUrl`; supplying both is a `usage` error):

```ts
await performInteractiveLogin({
  clientId: "basecamp-cli",
  store: myTokenStore,
  resourceBaseUrl: "https://3.basecampapi.com",
  expectedIssuer, // optional: authoritative, non-heuristic selection
  openBrowser: (url) => open(url),
});
```

**Selection.** With `expectedIssuer` (production canonical:
`https://app.basecamp.com`), the advertised member equal by code-point is
selected (else a hard `expected_issuer_unavailable`). Without it, the SDK uses a
documented Basecamp-profile heuristic: exactly one non-Launchpad issuer → selected;
≥2 → hard `ambiguous_issuers` (never guesses); zero → Launchpad.

Pass bare origins — no trailing slash. Binding is code-point exact, and the
failure mode depends on which parameter carries the slash: a trailing-slash
`expectedIssuer` fails the advertised-member lookup and throws a **hard**
`expected_issuer_unavailable`, while a trailing-slash *resource* origin breaks
the hop-1 resource binding and silently soft-falls back to Launchpad.

**Fallback is allowed only before a first-party issuer is committed.** Once valid
resource metadata advertises it and it is selected, every later failure is fatal —
the SDK **never** silently falls back to Launchpad:

| Failure | Result |
|---|---|
| Hop-1 fetch/parse fails, or `resource` mismatch | soft `resource_discovery_failed` → Launchpad |
| Valid metadata omits the first-party issuer | soft `no_as_advertised` → Launchpad |
| ≥2 non-Launchpad issuers (no `expectedIssuer`) | throws `ambiguous_issuers` |
| `expectedIssuer` not advertised | throws `expected_issuer_unavailable` |
| Committed issuer origin invalid | throws `invalid_issuer_origin` |
| Committed AS metadata fetch fails | throws `as_fetch_failed` |
| Committed issuer binding mismatch | throws `issuer_mismatch` |

Hard cases throw `DiscoverySelectionError` (with a `.reason`); soft cases return a
`{ kind: "fallback", reason }`.

> **Breaking-ish:** `OAuthConfig.authorizationEndpoint` is now **optional** —
> device-only servers omit it. Authorization-code consumers must assert it before
> use (`performInteractiveLogin` does this and errors if it is missing). Both
> discovery hops are SSRF-hardened (HTTPS-only origins, suppressed redirects,
> bounded body reads).

### Device flow (RFC 8628)

For CLIs and headless/remote environments, use the device authorization grant
with the public `basecamp-cli` client (no secret). `performDeviceLogin` takes an
already-selected config, guards the device capability, shows the user code via a
display hook, and polls for the token:

The device authorization endpoint lives on the **first-party** authorization
server, not Launchpad — so select a first-party issuer with `discoverFromResource`
and hand its config to `performDeviceLogin`. (Launchpad advertises no device
endpoint, so `performDeviceLogin`'s capability guard rejects it with
`DeviceFlowError("unavailable")`.)

```ts
import {
  createBasecampClient,
  discoverFromResource,
  performDeviceLogin,
  refreshToken,
  isTokenExpired,
  DeviceFlowError,
} from "@37signals/basecamp";

// 1. Select an AS config. Discovery can select a config WITHOUT the device
//    endpoint or device_code grant — performDeviceLogin then rejects it with
//    DeviceFlowError("unavailable").
const result = await discoverFromResource("https://3.basecampapi.com");
if (result.kind !== "selected") {
  // "resource_discovery_failed" | "no_as_advertised" → Launchpad has no device
  // flow; use the interactive (authorization-code) login instead.
  throw new Error(`Device flow unavailable: ${result.reason}`);
}

// 2. Run the device grant against the selected config.
const abortController = new AbortController();
try {
  const token = await performDeviceLogin({
    config: result.config,
    clientId: "basecamp-cli",
    // scope pinned explicitly — "read" is also the server default, but an
    // explicit scope never depends on registry defaults staying put.
    scope: "read",
    display: ({ userCode, verificationUri }) => {
      console.log(`Visit ${verificationUri} and enter code: ${userCode}`);
    },
    signal: abortController.signal, // optional: cancel the poll
  });
  // Use the token — hand it to a client or persist it via your token store.
  // Never print the token value itself.
  const client = createBasecampClient({
    accountId: process.env.BASECAMP_ACCOUNT_ID!,
    accessToken: token.accessToken,
  });
  // A long-lived CLI should PERSIST the whole token and mint a fresh one once it
  // expires — a client built from a static accessToken stops working when the
  // device access token expires. A device-token response MAY omit refresh_token,
  // so GUARD it: refresh only when one was issued, otherwise re-run the device
  // login to reauthenticate. Refresh hits the SAME first-party token endpoint
  // with the standard grant (device tokens never use Launchpad's legacy
  // `type=refresh` format):
  if (isTokenExpired(token)) {
    if (token.refreshToken) {
      const fresh = await refreshToken({
        tokenEndpoint: result.config.tokenEndpoint,
        clientId: "basecamp-cli", // public client — no secret
        refreshToken: token.refreshToken,
        // ECHO the token's RFC 8707 resource indicator: BC5 device logins as
        // basecamp-cli mint MULTI-ACCOUNT refresh tokens, and refreshing one
        // without `resource` is rejected (400 invalid_request).
        resource: token.resource,
      });
      // A refresh response MAY omit refresh_token (the server keeps the current
      // one) and MAY omit resource (the binding is unchanged). Persist the fresh
      // access token but FALL BACK to the prior values so the next refresh still
      // works:
      const nextRefresh = fresh.refreshToken ?? token.refreshToken;
      const nextResource = fresh.resource ?? token.resource;
      // ...persist { ...fresh, refreshToken: nextRefresh, resource: nextResource }
      // and rebuild the client. (TokenManager does all of this automatically.)
    } else {
      // No refresh token was issued: refreshing is impossible, so the user must
      // authorize again. Re-run the device login to get a new token — pass the
      // SAME abort signal so Ctrl-C cancels this second poll too, and don't keep
      // using the expired one.
      const reauthed = await performDeviceLogin({
        config: result.config,
        clientId: "basecamp-cli",
        display: ({ userCode, verificationUri }) => {
          console.log(`Visit ${verificationUri} and enter code: ${userCode}`);
        },
        signal: abortController.signal,
      });
      // ...persist `reauthed` and rebuild the client from reauthed.accessToken.
    }
  }
} catch (err) {
  if (err instanceof DeviceFlowError) {
    // err.reason: "access_denied" | "expired" | "transport" | "unavailable" | "cancelled"
    // err.code (parent category) is derived from the reason:
    //   access_denied/expired → auth_required, transport → network,
    //   unavailable → validation, cancelled → usage
  } else {
    // performDeviceLogin can also reject with BasecampError (e.g. a malformed
    // or non-2xx server response) — don't swallow those.
    throw err;
  }
}
```

The poll loop honors `interval`, sustains `slow_down` (+5s), backs off
exponentially on connection timeouts, and enforces a monotonic expiry deadline.
`requestDeviceAuthorization` and `pollDeviceToken` are exported for lower-level
use; the polling clock is injectable for testing.

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
| `todos` | list, get, create, update, complete, uncomplete, reposition |
| `todolists` | list, get, create, update, trash, reposition |
| `todosets` | get |
| `todolistGroups` | list, get, create, reposition |

### Messages & Communication

| Service | Methods |
|---------|---------|
| `messages` | list, get, create, update, pin, unpin |
| `messageBoards` | get |
| `messageTypes` | list, get, create, update, delete |
| `comments` | list, get, create, update |
| `campfires` | list, get, listLines, getLine, createLine, updateLine, deleteLine |

### Card Tables (Kanban)

| Service | Methods |
|---------|---------|
| `cardTables` | get, listColumns |
| `cards` | list, get, create, update, move |
| `cardColumns` | get, create, update, move |
| `cardSteps` | list, get, create, update, complete, uncomplete |
| `wormholes` | create, update, delete |

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
| `forwards` | list, get, listReplies, getReply |

## Downloading Files

Fetch an upload's file content in one call. The SDK fetches the upload
metadata, then follows the authenticated-hop + 302 flow against the signed
storage URL.

```ts
const result = await client.uploads.download(1069479400);
// result.body is a ReadableStream<Uint8Array>
const bytes = new Uint8Array(await new Response(result.body).arrayBuffer());
// result.contentType, result.contentLength, result.filename are also available
```

For any authenticated download URL (e.g. a `download_url` you already have
in hand), use `client.downloadURL`:

```ts
const result = await client.downloadURL(url);
```

## Pagination

Service `list()` methods auto-paginate: they follow the API's `Link: rel="next"` headers and return every item across all pages (up to a 10,000-page safety cap).

The result is a `ListResult<T>` — an `Array<T>` subclass that works with `for...of`, `.map()`, `.filter()`, spread, `.length`, indexing, and `Array.isArray()` — plus a `.meta` property with pagination metadata:

```ts
const projects = await client.projects.list();

console.log(`${projects.length} of ${projects.meta.totalCount} projects`);
if (projects.meta.truncated) console.warn("more results were available");
projects.forEach(p => console.log(p.name));
```

`meta.totalCount` is parsed from the `X-Total-Count` response header (0 when the header is absent). `meta.truncated` is `true` when items beyond those returned were available — more items than `maxItems` arrived, or the last-fetched page still advertised a next page (including when the page safety cap stopped pagination early); when it is `false`, the result is complete.

To bound the work, pass `maxItems` — every list options type extends `PaginationOptions`. When `maxItems` is omitted or `0`, all pages are fetched:

```ts
const firstFifty = await client.projects.list({ maxItems: 50 });
```

### The `page` option

`page` sets where the walk *starts*, not which single page you get. Link-following
continues from there to the end of the collection, so `{ page: 3 }` against a
10-page collection returns pages 3–10 concatenated. Pair it with `maxItems` to
bound the result.

This differs from the Go SDK, where a positive `Page` fetches exactly that page
and turns auto-pagination off. Converging the six SDKs on Go's single-page
semantics is a breaking change tracked in
[#566](https://github.com/basecamp/basecamp-sdk/issues/566).

For endpoints not covered by a service, drive pagination yourself over a raw `client.GET` response with `fetchAllPages` (collects all pages into one array) or `paginateAll` (async generator that yields one page at a time):

```ts
import { fetchAllPages, paginateAll } from "@37signals/basecamp";

const initial = await client.GET("/projects.json");

// Option 1: fetchAllPages - returns all results as an array
const all = await fetchAllPages(initial.response, (r) => r.json() as Promise<any[]>);

// Option 2: paginateAll - async generator for streaming large result sets
for await (const page of paginateAll(initial.response, (r) => r.json() as Promise<any[]>)) {
  for (const project of page) {
    console.log(project.name);
  }
}
```

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
import { BasecampError, isBasecampError, isErrorCode } from "@37signals/basecamp";

try {
  await client.todos.get(todoId);
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
| `auth_required` | 401 | 3 | Authentication required |
| `forbidden` | 403 | 4 | Access denied |
| `not_found` | 404 | 2 | Resource not found |
| `rate_limit` | 429 | 5 | Rate limit exceeded (retryable) |
| `network` | - | 6 | Network error (retryable) |
| `api_error` | 5xx | 7 | Server error |
| `ambiguous` | - | 8 | Multiple matches found |
| `validation` | 400, 422 | 9 | Invalid request data |
| `usage` | - | 1 | Configuration or argument error |

### Validation Errors

Basecamp rejects invalid writes with a body keyed by field. The SDK folds those
messages into `message` and keeps the raw map in `fieldErrors`, so you can drive
a form without re-parsing the message:

```typescript
try {
  await client.calendars.updateCalendar(calendarId, { calendar: { color: "chartreuse" } });
} catch (error) {
  if (isErrorCode(error, "validation")) {
    // "color: is not a valid color"
    console.error(error.message);

    for (const [field, messages] of Object.entries(error.fieldErrors ?? {})) {
      console.error(`  ${field}: ${messages.join(", ")}`);
    }
  }
}
```

`fieldErrors` is `undefined` for every other error shape, and its messages are
the raw ones — `message` is capped at 500 characters, the map is not. It is a
null-prototype object, so a field literally named `__proto__` is an ordinary
key.

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

The SDK can do ETag-based HTTP caching to reduce API calls and respect Basecamp's rate limits, but it is **off by default** — `enableCache` defaults to `false`, and a client built without it sends no `If-None-Match` and stores no responses. Opt in:

```ts
const client = createBasecampClient({
  accountId: "12345",
  accessToken: "token",
  enableCache: true,
});

// First request fetches from the API and stores the ETag
const projects = await client.projects.list();

// Second request revalidates with If-None-Match; a 304 is served from the store
const projects2 = await client.projects.list();
```

The store is in-memory and per-client — it lives and dies with the client instance — holds at most 1,000 entries, and is keyed by URL plus a hash of the `Authorization` header, so a refreshed or swapped token never reads another token's entries.

## Observability

### Console Logging

For debugging or verbose CLI modes:

```ts
import { createBasecampClient, consoleHooks } from "@37signals/basecamp";

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
import type { BasecampHooks } from "@37signals/basecamp";

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
import { createBasecampClient, otelHooks } from "@37signals/basecamp";
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
import { chainHooks, consoleHooks, otelHooks } from "@37signals/basecamp";

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
const todos = await client.todos.list(todolistId);

// Create a todo with assignees
const todo = await client.todos.create(todolistId, {
  content: "Review pull request",
  description: "<p>Check the new auth flow</p>",
  dueOn: "2026-02-01",
  assigneeIds: [12345, 67890],
});

// Complete a todo
await client.todos.complete(todo.id);

// Reposition a todo to the top
await client.todos.reposition(todo.id, { position: 1 });
```

### Working with Messages

```ts
// Get a message board
const board = await client.messageBoards.get(boardId);

// List messages
const messages = await client.messages.list(board.id);

// Create a message
const msg = await client.messages.create(board.id, {
  subject: "Weekly Update",
  content: "<p>Here's what we accomplished...</p>",
});

// Pin a message
await client.messages.pin(msg.id);
```

### Working with Campfire

```ts
// List campfires
const campfires = await client.campfires.list();

// Send a message
await client.campfires.createLine(campfireId, {
  content: "Hello, team!",
});

// List recent messages
const lines = await client.campfires.listLines(campfireId);
```

### Working with Webhooks

```ts
const bucketId = 12345; // project/bucket ID

// Create a webhook
const webhook = await client.webhooks.create(bucketId, {
  payloadUrl: "https://example.com/webhook",
  types: ["Todo", "Comment"],
});

// List webhooks
const webhooks = await client.webhooks.list(bucketId);

// Delete a webhook
await client.webhooks.delete(webhook.id);
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
} from "@37signals/basecamp";

function processTodo(todo: Todo): void {
  console.log(todo.content);
}

function createTodo(data: CreateTodoRequest): Promise<Todo> {
  return client.todos.create(todolistId, data);
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
