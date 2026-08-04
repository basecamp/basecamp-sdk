# Basecamp Python SDK

[![PyPI](https://img.shields.io/pypi/v/basecamp-sdk)](https://pypi.org/project/basecamp-sdk/)
[![Test](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml/badge.svg)](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml)
[![Python 3.11+](https://img.shields.io/badge/python-3.11%2B-blue)](https://www.python.org/downloads/)

Official Python SDK for the [Basecamp API](https://github.com/basecamp/bc3-api).

**Upgrading to v0.13.0?** Read [MIGRATING.md](../MIGRATING.md#python) before you bump the version — Python carries seven breaks with no signal at all, and no compile step to catch anything else.

## Features

- **Full API coverage** — 46 generated services covering projects, todos, messages, schedules, campfires, card tables, and more
- **OAuth 2.0 authentication** — PKCE support, token refresh, resource-first (RFC 9728 + RFC 8414) discovery
- **Static token authentication** — Simple setup for personal integrations
- **Automatic retry with backoff** — Exponential backoff with jitter, respects `Retry-After` headers
- **Pagination handling** — Automatic Link header-based pagination with `ListResult`
- **Structured errors** — Typed exceptions with error codes, hints, and CLI-friendly exit codes
- **Observability hooks** — Integration points for logging, metrics, and tracing
- **Webhook verification** — HMAC signature verification, deduplication, glob-based routing
- **Async support** — Full async/await API via `AsyncClient` backed by httpx
- **File downloads** — Authenticated downloads with redirect following
- **Type hints** — Full type annotations for IDE support

## Requirements

- Python 3.11 or later
- [httpx](https://www.python-httpx.org/) (installed automatically)

## Installation

```bash
pip install basecamp-sdk
```

Or with [uv](https://docs.astral.sh/uv/):

```bash
uv add basecamp-sdk
```

## Getting a token

Every Basecamp API request carries an OAuth 2.0 access token. There is no API key and no personal access token, so even a throwaway script starts here:

1. Choose the grant that matches how your code runs:

| Your integration | Grant | Who refreshes the token |
|---|---|---|
| already holds a token you obtained elsewhere | **static token** — [`Static Token`](#static-token) | you do |
| can receive a browser redirect (web app, or a local callback server) | **authorization code + PKCE** — [`PKCE and Authorization URL`](#pkce-and-authorization-url) | `OAuthTokenProvider` |
| has no browser, but a person can approve on another device (CLI, headless server, TV) | **device flow** — [`Device Authorization Grant`](#device-authorization-grant-rfc-8628) | `basecamp.oauth.exchange.refresh_token`, echoing the token's `resource` — **not** `OAuthTokenProvider` |

The one-line rule: **a redirect URI you control → authorization code; no browser but someone to approve → device flow; a token already in hand → static token.** An unattended daemon or CI job fits none of the three on its own — the device flow needs a person to enter the user code at the verification URI — so provision a token out of band and hand it to the process as a static or refresh token.

2. Get the client credentials that grant needs:

- **Authorization code + PKCE** — register your own integration at **<https://launchpad.37signals.com/integrations>**. You get a client ID, a client secret, and whatever redirect URI you nominated.
- **Device flow** — nothing to register. It runs as the pre-registered public `basecamp-cli` client, which sends no secret, against the device endpoint that discovery returns. Launchpad advertises no device endpoint, so a client you register there is not the one this flow uses.
- **Static token** — nothing to register; you already hold the token.

`Client(access_token=...)` wraps the string in a `StaticTokenProvider`, which never refreshes — once the token expires every call fails with `401` until you supply a new one. Use it to get a first successful call, then move to a refreshing path before you ship — `OAuthTokenProvider` for an authorization-code token from Launchpad, or, for a device-flow token, `basecamp.oauth.exchange.refresh_token` echoing the stored `resource`. Do not hand a device-flow token to `OAuthTokenProvider`: it refreshes only against Launchpad and sends no `resource`, so it fails at the first expiry.

## Finding your account ID

Every API path is scoped to an account — `https://3.basecampapi.com/{accountId}/…` — so `for_account` needs that number before your first call. One token can reach several accounts, so ask the token which. `authorization` hangs off the *top-level* `Client` because the endpoint lives on the authorization server rather than the Basecamp API, and so takes no account context. It is pinned to Launchpad and `authorization.get()` accepts no override, which is right for a Launchpad-issued token; a **device-flow** token is issued by the discovered BC5 server, so fetch `/authorization.json` from that issuer yourself:

```python
import os
from basecamp import Client

client = Client(access_token=os.environ["BASECAMP_TOKEN"])

info = client.authorization.get()
# "bc3" is Basecamp; the same response also carries "hey" and other products,
# and they are not ordered — filter before you pick, or you may scope the
# client to a HEY account.
basecamp_accounts = [a for a in info["accounts"] if a["product"] == "bc3"]
for a in basecamp_accounts:
    print(f"{a['id']}: {a['name']}")

account = client.for_account(basecamp_accounts[0]["id"])
```

The response is a plain `dict` of parsed JSON. `info["expires_at"]` tells you how long the token has left, which is the quickest way to confirm a static token has not lapsed. On `AsyncClient`, the same call is `await client.authorization.get()`.

## Quick Start

```python
import os
from basecamp import Client

client = Client(access_token=os.environ["BASECAMP_TOKEN"])
account = client.for_account(os.environ["BASECAMP_ACCOUNT_ID"])

projects = account.projects.list()
for project in projects:
    print(f"{project['id']}: {project['name']}")
```

### Async

```python
import asyncio
import os
from basecamp import AsyncClient

async def main():
    async with AsyncClient(access_token=os.environ["BASECAMP_TOKEN"]) as client:
        account = client.for_account(os.environ["BASECAMP_ACCOUNT_ID"])
        projects = await account.projects.list()
        for project in projects:
            print(f"{project['id']}: {project['name']}")

asyncio.run(main())
```

## Configuration

### Environment Variables

Nothing here is read automatically. These apply only when you call `Config.from_env()`; a plain `Config()` reads no environment at all. They are also the only three environment variables the SDK reads anywhere — `from_env` does not consult `base_delay`, `max_jitter`, or `max_pages`, so those keep their defaults unless you pass them.

| Variable | Description | Default |
|----------|-------------|---------|
| `BASECAMP_BASE_URL` | API base URL | `https://3.basecampapi.com` |
| `BASECAMP_TIMEOUT` | Request timeout (seconds) | `30` |
| `BASECAMP_MAX_RETRIES` | Total attempts including the initial request | `3` |

`BASECAMP_TOKEN` and `BASECAMP_ACCOUNT_ID` appear in the examples above only because the caller reads them and passes the values in; the SDK never looks them up. Pass the token to `Client(access_token=...)` and the account ID to `for_account`.

### Programmatic Configuration

```python
from basecamp import Config

# Load from environment variables
config = Config.from_env()

# Or configure programmatically
config = Config(
    base_url="https://3.basecampapi.com",
    timeout=30.0,
    max_retries=3,
    base_delay=1.0,
    max_jitter=0.1,
    max_pages=10_000,
)

client = Client(access_token="...", config=config)
```

Configuration is immutable (frozen dataclass). Create a new `Config` to change settings.

## Authentication

### Static Token

```python
from basecamp import Client

client = Client(access_token="your-token")
```

### OAuth Token Provider

```python
from basecamp import Client, OAuthTokenProvider

provider = OAuthTokenProvider(
    access_token="...",
    client_id="your-client-id",
    client_secret="your-client-secret",
    refresh_token="...",
    expires_at=1234567890.0,
    on_refresh=lambda access, refresh, expires_at: save_tokens(access, refresh, expires_at),
)

client = Client(token_provider=provider)
```

The `OAuthTokenProvider` automatically refreshes expired tokens before each request.

### Custom Auth Strategy

Implement the `AuthStrategy` protocol for custom authentication:

```python
from basecamp import Client, AuthStrategy

class MyAuth:
    def authenticate(self, headers: dict[str, str]) -> None:
        headers["Authorization"] = "Bearer " + get_token()

client = Client(auth=MyAuth())
```

## OAuth 2.0

The SDK provides helpers for the full OAuth 2.0 authorization code flow with PKCE.

### Discovery

```python
from basecamp.oauth import discover_launchpad

config = discover_launchpad()
# config.authorization_endpoint  # Optional[str] — may be None (see below)
# config.token_endpoint
```

`config.authorization_endpoint` is now **optional** (`str | None`): device-only
authorization servers omit it, so authorization-code consumers MUST assert its
presence before use. `token_endpoint` stays required.

#### Resource-first discovery (RFC 9728 + RFC 8414)

BC5's Authorization Server metadata lives only at its canonical issuer (the web
host), not the API host. Discovery therefore starts from the **resource** and
composes with AS discovery. Two composable operations plus an orchestrator:

```python
from basecamp.oauth import (
    discover,                      # RFC 8414 AS metadata + issuer binding
    discover_protected_resource,   # RFC 9728 resource metadata (hop 1)
    discover_from_resource,        # orchestrator: selection + fallback
)

result = discover_from_resource("https://3.basecampapi.com")
if result.kind == "selected":
    config = result.config         # bound OAuthConfig from the selected issuer
else:  # result.kind == "fallback"
    config = discover_launchpad()  # result.reason explains why
```

**Selection.** Pass `expected_issuer="https://app.basecamp.com"` (the
production canonical issuer) for an explicit, authoritative choice (raises
`expected_issuer_unavailable` if it is not advertised — never a silent
fallback). Without it, the SDK identifies BC5 by exclusion: exactly one
non-Launchpad issuer is selected; two or more raise `ambiguous_issuers`; zero
falls back to Launchpad.

Pass bare origins — no trailing slash. Binding is code-point exact, and the
failure mode depends on which parameter carries the slash: a trailing-slash
`expected_issuer` fails the advertised-member lookup and raises a **hard**
`expected_issuer_unavailable`, while a trailing-slash *resource* origin breaks
the hop-1 resource binding and silently soft-falls back to Launchpad.

**Stage-sensitive fallback.** `discover_from_resource` returns a
`DiscoveryResult` that is either `selected` or a soft `fallback` whose `reason`
is one of **only** two `FallbackReason` values. Every hard case raises
`DiscoverySelectionError` (carrying a `.reason`) — no consumer may convert a
raise into a Launchpad request.

| Stage / failure | Outcome |
|---|---|
| Hop-1 fetch/parse fails, or `resource` mismatch | soft `resource_discovery_failed` → Launchpad |
| Valid metadata omits BC5 (absent / `[]` / only-Launchpad) | soft `no_as_advertised` → Launchpad |
| ≥2 non-Launchpad issuers, no `expected_issuer` | **raise** `ambiguous_issuers` |
| `expected_issuer` not advertised | **raise** `expected_issuer_unavailable` |
| BC5 committed → invalid issuer origin | **raise** `invalid_issuer_origin` |
| BC5 committed → AS-metadata fetch fails (5xx / network) | **raise** `as_fetch_failed` |
| BC5 committed → issuer binding mismatch | **raise** `issuer_mismatch` |
| BC5 committed → missing per-grant endpoint (consumer-asserted) | **raise** `capability_unavailable` |

`discover_protected_resource` preserves `authorization_servers` absent (`None`)
vs present-but-empty (`[]`) distinctly.

**SSRF hardening.** Both hops require HTTPS (localhost exempt), validate the
origin-root with the transport URL parser before any socket opens, suppress
redirects, bound the timeout, and read the body under a genuine streaming cap
that aborts before an oversized body is buffered. Non-2xx on either hop maps to
`api_error`.

**Timing contract.** A bounded OAuth request returns within its `timeout` plus
at most 1 second of cleanup grace, spent only when the request already missed
its deadline. Budget `timeout + 1` when folding these calls into your own
deadlines.

### PKCE and Authorization URL

```python
from basecamp.oauth import generate_pkce, generate_state, build_authorization_url

pkce = generate_pkce()
state = generate_state()

url = build_authorization_url(
    endpoint=config.authorization_endpoint,
    client_id="your-client-id",
    redirect_uri="https://yourapp.com/callback",
    state=state,
    pkce=pkce,
)
# Redirect user to url
```

### Token Exchange

```python
from basecamp.oauth import exchange_code

token = exchange_code(
    token_endpoint=config.token_endpoint,
    code="authorization-code-from-callback",
    redirect_uri="https://yourapp.com/callback",
    client_id="your-client-id",
    client_secret="your-client-secret",
    code_verifier=pkce.verifier,
)
# token.access_token, token.refresh_token, token.expires_at
```

### Token Refresh

```python
from basecamp.oauth import refresh_token

new_token = refresh_token(
    token_endpoint=config.token_endpoint,
    refresh_tok=token.refresh_token,
    client_id="your-client-id",
    client_secret="your-client-secret",
    # Echo the stored token's RFC 8707 resource indicator. BC5 multi-account
    # refresh tokens (e.g. basecamp-cli device logins) REJECT a refresh
    # without it (400 invalid_request); it is sent only when set.
    resource=token.resource,
)
# A refresh response MAY omit resource (binding unchanged) AND refresh_token
# (no rotation) — carry BOTH forward when persisting, or the next refresh
# loses its token:
#   stored_resource = new_token.resource or token.resource
#   stored_refresh  = new_token.refresh_token or token.refresh_token
```

### Launchpad Legacy Format

Basecamp's Launchpad uses a non-standard token format. Pass `use_legacy_format=True` for compatibility:

```python
token = exchange_code(
    token_endpoint=config.token_endpoint,
    code=code,
    redirect_uri=redirect_uri,
    client_id=client_id,
    client_secret=client_secret,
    code_verifier=pkce.verifier,
    use_legacy_format=True,
)
```

### Device Authorization Grant (RFC 8628)

For input-constrained clients (CLIs, TVs), the device flow trades a redirect for
a user code the person enters on another device. `perform_device_login` accepts
an already-selected `OAuthConfig` (from discovery), guards the device capability,
requests a code, surfaces it through your `display` hook, then polls for a token.

The device grant lives on BC5's authorization server, not Launchpad — so the
config MUST come from resource-first discovery. Launchpad advertises no device
authorization endpoint, and the capability guard rejects it before any request.

```python
from basecamp import Client
from basecamp.oauth import discover_from_resource, perform_device_login

result = discover_from_resource("https://3.basecampapi.com")
if result.kind != "selected":
    raise RuntimeError(f"device flow requires a discovered AS (got fallback: {result.reason})")
config = result.selected_config()  # enforces the selected-result invariant, typed OAuthConfig

def show(auth):
    print(f"Visit {auth.verification_uri} and enter code: {auth.user_code}")

token = perform_device_login(config, "basecamp-cli", display=show)
# Use the token to build a client — never print or log its value.
client = Client(access_token=token.access_token)
```

The capability guard requires both `config.device_authorization_endpoint` and
`"urn:ietf:params:oauth:grant-type:device_code"` in `config.grant_types_supported`;
otherwise it raises `DeviceFlowError(reason="unavailable")` before any request.
An omitted `scope` lets the server apply its default (`read`) — prefer pinning
it explicitly with `scope="read"`. The returned token MAY carry an RFC 8707
`resource` indicator (`token.resource`) — persist it and echo it on refresh
(see Token Refresh above).

The two building blocks compose directly when you want finer control:

```python
import time

from basecamp.oauth import request_device_authorization, poll_device_token

# `device_authorization_endpoint` is optional on a discovered config (device-only
# servers omit it). `perform_device_login` guards this for you; when composing the
# building blocks yourself, assert the capability before requesting a code.
endpoint = config.device_authorization_endpoint
if endpoint is None:
    raise RuntimeError("selected AS advertises no device_authorization_endpoint")

auth = request_device_authorization(endpoint, "basecamp-cli")

# The code's lifetime starts at issuance, not after display: anchor before the
# display hook and poll with the remaining lifetime, so a slow display eats into
# the deadline instead of extending it. (`perform_device_login` does this for you.)
issued_at = time.monotonic()
show(auth)
token = poll_device_token(
    config.token_endpoint,
    "basecamp-cli",
    auth.device_code,
    auth.interval,
    auth.expires_in - (time.monotonic() - issued_at),
    # The EXACT issuance-anchored deadline: a suspension between the remaining
    # computation above and poller entry must not extend the code lifetime.
    deadline_at=issued_at + auth.expires_in,
)
```

`poll_device_token` runs the RFC 8628 §3.5 loop: it waits at least `interval`
seconds, honors `slow_down` (sustained +5s) and `authorization_pending`, backs
off exponentially on connection timeouts, and enforces a monotonic expiry
deadline. Pass `should_cancel` (any `() -> bool`, e.g. `threading.Event.is_set`)
for cooperative cancellation. The `clock` and `sleep` seams are injectable for
deterministic tests.

A terminal device-flow outcome raises `DeviceFlowError` carrying one of these
five `.reason` values, with the parent error category derived from it:

| `reason` | `.code` | Retryable |
|----------|---------|-----------|
| `access_denied` | `auth_required` | no |
| `expired` | `auth_required` | no |
| `transport` | `network` | yes |
| `unavailable` | `validation` | no |
| `cancelled` | `usage` | no |

Only these five outcomes are `DeviceFlowError`. A malformed response from an
otherwise-successful round-trip — an unparseable body, a `2xx` missing
`access_token`, or an unrecognized `error` code — is a server fault, not a
device-flow outcome, so it raises `OAuthError` with type `api_error` instead.
`.reason`-derived retryability is authoritative: only `transport` retries, and a
caller cannot override it.

### Token Expiry

```python
from basecamp.oauth import OAuthToken

token = OAuthToken(access_token="...", expires_in=7200)
token.is_expired()                 # False
token.is_expired(buffer_seconds=60)  # True if expiring within 60s
```

## Services

All services are accessed through an `AccountClient`, obtained via `client.for_account(account_id)`. The table below covers the common ones; see `basecamp/generated/services/` for the full 45-service set.

| Category | Service | Accessor |
|----------|---------|----------|
| **Projects** | Projects | `account.projects` |
| | Templates | `account.templates` |
| | Tools | `account.tools` |
| | People | `account.people` |
| **To-dos** | Todos | `account.todos` |
| | Todolists | `account.todolists` |
| | Todosets | `account.todosets` |
| | TodolistGroups | `account.todolist_groups` |
| | HillCharts | `account.hill_charts` |
| **Messages** | Messages | `account.messages` |
| | MessageBoards | `account.message_boards` |
| | MessageTypes | `account.message_types` |
| | Comments | `account.comments` |
| **Chat** | Campfires | `account.campfires` |
| **Scheduling** | Schedules | `account.schedules` |
| | Timeline | `account.timeline` |
| | Lineup | `account.lineup` |
| | Checkins | `account.checkins` |
| **Files** | Vaults | `account.vaults` |
| | Documents | `account.documents` |
| | Uploads | `account.uploads` |
| | Attachments | `account.attachments` |
| **Card Tables** | CardTables | `account.card_tables` |
| | Cards | `account.cards` |
| | CardColumns | `account.card_columns` |
| | CardSteps | `account.card_steps` |
| | Wormholes | `account.wormholes` |
| **Client Portal** | ClientApprovals | `account.client_approvals` |
| | ClientCorrespondences | `account.client_correspondences` |
| | ClientReplies | `account.client_replies` |
| | ClientVisibility | `account.client_visibility` |
| **Automation** | Webhooks | `account.webhooks` |
| | Subscriptions | `account.subscriptions` |
| | Events | `account.events` |
| | Automation | `account.automation` |
| | Boosts | `account.boosts` |
| **Reporting** | Search | `account.search` |
| | Reports | `account.reports` |
| | Timesheets | `account.timesheets` |
| | Recordings | `account.recordings` |
| **Email** | Forwards | `account.forwards` |

The `authorization` service is on the top-level `Client`:

```python
auth = client.authorization.get()
```

All service methods use keyword-only arguments:

```python
# All parameters after * are keyword-only
todo = account.todos.get(todo_id=123)
project = account.projects.create(name="My Project", description="A new project")
todos = account.todos.list(todolist_id=456, status="active")
```

## Downloading Files

Fetch an upload's file content in one call. The SDK fetches the upload
metadata, then follows the authenticated-hop + 302 flow against the signed
storage URL.

```python
# Sync
result = account.uploads.download(upload_id=1069479400)
with open("uploaded.bin", "wb") as f:
    f.write(result.body)

# Async
result = await account.uploads.download(upload_id=1069479400)
```

For any authenticated download URL (e.g. a `download_url` you already
have in hand), use `AccountClient.download_url` /
`AsyncAccountClient.download_url`:

```python
result = account.download_url(url)          # sync
result = await account.download_url(url)    # async
```

## Pagination

Paginated methods return a `ListResult`, which is a `list` subclass with a `.meta` attribute:

```python
projects = account.projects.list()

# ListResult is a list - iterate directly
for project in projects:
    print(project["name"])

# Access pagination metadata
print(projects.meta.total_count)   # total items across all pages
print(projects.meta.truncated)     # True if items beyond those returned were available

# Standard list operations work
print(len(projects))
first = projects[0]
sliced = projects[:5]
```

Pagination is automatic. The SDK follows Link headers and collects all pages up to `config.max_pages` (default: 10,000).

Every paginated method also accepts a `max_items` keyword to cap how many items are collected. Collection stops as soon as the cap is met, without fetching further pages. Zero or negative values disable the cap, as in the other SDKs:

```python
recent = account.projects.list(max_items=50)
print(recent.meta.truncated)  # True only if more items were available
```

`meta.truncated` is `True` only when items beyond those returned were available — items were dropped by `max_items`, or the last-fetched page still advertised a next page when collection stopped (at `max_items` or the `max_pages` safety cap). When it is `False`, the result is definitely complete.

### The `page` keyword

A positive `page` selects exactly that page: one request, that page's items,
no link-following.

```python
page_3 = account.projects.list(page=3)
print(page_3.meta.truncated)  # True when a further page existed
```

Omit `page` (or pass `0`) to auto-paginate the whole collection. `max_items`
still trims a pinned page.

All six SDKs share these semantics — one request, that page only, no
link-following. See SPEC section 8.

## Error Handling

```python
from basecamp import Client, NotFoundError, RateLimitError, AuthError, BasecampError

client = Client(access_token="...")
account = client.for_account("12345")

try:
    project = account.projects.get(project_id=999)
except NotFoundError as e:
    print(f"Not found: {e}")
    print(f"HTTP status: {e.http_status}")
    print(f"Request ID: {e.request_id}")
except RateLimitError as e:
    print(f"Rate limited, retry after: {e.retry_after}s")
except AuthError as e:
    print(f"Authentication failed: {e.hint}")
except BasecampError as e:
    print(f"API error [{e.code}]: {e}")
```

### Error Hierarchy

All exceptions inherit from `BasecampError`:

| Exception | `ErrorCode` value | HTTP Status | Retryable |
|-----------|-------------------|-------------|-----------|
| `UsageError` | `usage` | - | No |
| `NotFoundError` | `not_found` | 404 | No |
| `AuthError` | `auth_required` | 401 | No |
| `ForbiddenError` | `forbidden` | 403 | No |
| `RateLimitError` | `rate_limit` | 429 | Yes |
| `NetworkError` | `network` | - | Yes |
| `ApiError` | `api_error` | 5xx, other | Yes for 500/502/503/504; No otherwise |
| `AmbiguousError` | `ambiguous` | - | No |
| `ValidationError` | `validation` | 400, 422 | No |

Every `BasecampError` provides:
- `code` - `ErrorCode` enum value
- `hint` - Human-readable suggestion
- `http_status` - HTTP status code (if applicable)
- `retryable` - Whether the error is safe to retry. This is a classification hint for your own code, not a prediction of transport behavior: for Basecamp API operations, what actually gets retried is the operation's declared status set (see [Retry Behavior](#retry-behavior)), so a 500 reports `retryable=True` but is not retried
- `retry_after` - Seconds to wait before retry (for rate limits)
- `request_id` - Server request ID (if available)
- `exit_code` - CLI-friendly exit code (`ExitCode` enum)

### Validation Errors

Basecamp rejects invalid writes with a body keyed by field. The SDK folds those
messages into the message and `ValidationError` keeps the raw map in
`field_errors`, so you can drive a form without re-parsing the message:

```python
from basecamp import ValidationError

try:
    account.calendars.update_calendar(calendar_id=calendar_id, calendar={"color": "chartreuse"})
except ValidationError as e:
    print(e)  # "color: is not a valid color"

    for field, messages in (e.field_errors or {}).items():
        for message in messages:
            print(f"  {field} {message}")
```

`field_errors` is `None` for every other error shape, and its messages are the
raw ones — the message is capped at 500 bytes, the map is not.

## Retry Behavior

The SDK automatically retries failed requests with exponential backoff:

- **Which statuses** - For Basecamp API operations, only the statuses that operation declares retryable. Every operation declares `429, 503`, and the declared set is exhaustive: a status outside it — **including 500, 502, and 504** — is surfaced to you on the first attempt rather than retried
- **GET requests** - Retried on the declared statuses above, plus `NetworkError`, which carries no HTTP status and so is governed by its error classification instead
- **Idempotent mutations** - Operations marked idempotent in the OpenAPI metadata also retry through the same path
- **Non-idempotent mutations** - NOT retried to prevent duplicate operations
- **401 responses** - Token refresh attempted, then single retry for all methods (regardless of idempotency)
- **Backoff** - Exponential with jitter (`base_delay * 2^(attempt-1) + random() * max_jitter`)
- **Retry-After** - Respected for 429 responses (overrides calculated backoff)
- **Max retries** - `min(config.max_retries, the operation's declared maximum)`. `config.max_retries` is a total attempt count including the initial request (default: 3 attempts; `0` means a single request with no retry). An operation declaring a lower maximum wins — the declared value can only lower the cap, never raise it
- **`client.authorization.get()`** - The Launchpad authorization endpoint is not a Basecamp API operation, so it carries no declared policy and neither rule above applies to it. It retries whatever `retryable` reports — **including 500, 502 and 504** — bounded only by `config.max_retries`

## Observability

### Console Hooks

```python
from basecamp import Client
from basecamp.hooks import console_hooks

client = Client(access_token="...", hooks=console_hooks())
# Logs all operations and requests to stderr
```

### Custom Hooks

Subclass `BasecampHooks` and override the methods you need:

```python
from basecamp import Client
from basecamp.hooks import BasecampHooks, OperationInfo, OperationResult, RequestInfo, RequestResult

class MyHooks(BasecampHooks):
    def on_operation_start(self, info: OperationInfo):
        print(f"-> {info.service}.{info.operation}")

    def on_operation_end(self, info: OperationInfo, result: OperationResult):
        status = "ok" if result.error is None else "error"
        print(f"<- {info.service}.{info.operation} {status} ({result.duration_ms}ms)")

    def on_request_start(self, info: RequestInfo):
        print(f"   {info.method} {info.url} (attempt {info.attempt})")

    def on_request_end(self, info: RequestInfo, result: RequestResult):
        print(f"   {result.status_code} ({result.duration:.3f}s)")

    def on_retry(self, info: RequestInfo, attempt: int, error: BaseException, delay: float):
        print(f"   retry {attempt} in {delay:.1f}s: {error}")

    def on_paginate(self, url: str, page: int):
        print(f"   page {page}: {url}")

client = Client(access_token="...", hooks=MyHooks())
```

### Chaining Hooks

```python
from basecamp.hooks import chain_hooks, console_hooks

combined = chain_hooks(console_hooks(), MyHooks())
client = Client(access_token="...", hooks=combined)
```

`chain_hooks` composes multiple hooks. `on_end` callbacks fire in reverse order (LIFO).

### Hook Safety

Hook exceptions are caught and logged to stderr. A failing hook never interrupts SDK operations.

## Webhooks

### Receiver

```python
from basecamp.webhooks import WebhookReceiver

receiver = WebhookReceiver(secret="your-webhook-secret")

def handle_todos(event):
    print(f"Todo event: {event['kind']}")

def handle_message(event):
    print(f"New message: {event['recording']['title']}")

def handle_all(event):
    print(f"Event: {event['kind']}")

receiver.on("todo_*", handle_todos)
receiver.on("message_created", handle_message)
receiver.on_any(handle_all)

# In your web framework handler:
result = receiver.handle_request(
    raw_body=request.body,
    headers=dict(request.headers),
)
```

### Signature Verification

```python
from basecamp.webhooks import verify_signature, compute_signature

# Verify a webhook signature (returns bool)
if not verify_signature(
    request.body,
    "your-webhook-secret",
    request.headers["X-Basecamp-Signature"],
):
    raise ValueError("Invalid webhook signature")

# Compute a signature
sig = compute_signature(request.body, "your-webhook-secret")
```

### Middleware

```python
def log_events(event, next_fn):
    print(f"Processing: {event['kind']}")
    return next_fn()

receiver.use(log_events)
```

### Deduplication

The receiver automatically deduplicates events by `event["id"]` using an LRU window (default: 1,000 events). Configure with `dedup_window_size`:

```python
receiver = WebhookReceiver(secret="...", dedup_window_size=5000)
```

## Async Support

Every service method has a sync and async variant. The async client mirrors the sync API:

```python
from basecamp import AsyncClient

async with AsyncClient(access_token="...") as client:
    account = client.for_account("12345")

    # All service methods are awaitable
    projects = await account.projects.list()
    todo = await account.todos.get(todo_id=123)

    # Downloads are async too
    result = await account.download_url("https://...")
```

Use `AsyncClient` with `async with` for automatic cleanup, or call `await client.close()` manually.

## Downloads

Download files from Basecamp with authentication and redirect handling:

```python
# Sync
result = account.download_url("https://3.basecampapi.com/.../download/file.pdf")
print(result.filename)        # "file.pdf"
print(result.content_type)    # "application/pdf"
print(result.content_length)  # 12345
with open(result.filename, "wb") as f:
    f.write(result.body)

# Async
result = await account.download_url("https://...")
```

Downloads resolve signed URLs with an authenticated request, then fetch file content via a second unauthenticated request so credentials are never sent to the signed URL.

## Development

```bash
# Install dependencies (from repo root)
cd python && uv sync && cd ..

# Run all checks (tests, types, lint, format, drift)
make py-check

# Run tests only
make py-test

# Type checking
make py-typecheck

# Regenerate services from OpenAPI spec
make py-generate

# Check for service drift
make py-check-drift
```

## License

MIT
