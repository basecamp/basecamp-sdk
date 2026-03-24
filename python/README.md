# Basecamp Python SDK

[![PyPI](https://img.shields.io/pypi/v/basecamp-sdk)](https://pypi.org/project/basecamp-sdk/)
[![Test](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml/badge.svg)](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml)
[![Python 3.11+](https://img.shields.io/badge/python-3.11%2B-blue)](https://www.python.org/downloads/)

Official Python SDK for the [Basecamp API](https://github.com/basecamp/bc3-api).

## Features

- **Full API coverage** — 40 generated services covering projects, todos, messages, schedules, campfires, card tables, and more
- **OAuth 2.0 authentication** — PKCE support, token refresh, Launchpad discovery
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

| Variable | Description | Default |
|----------|-------------|---------|
| `BASECAMP_BASE_URL` | API base URL | `https://3.basecampapi.com` |
| `BASECAMP_TIMEOUT` | Request timeout (seconds) | `30` |
| `BASECAMP_MAX_RETRIES` | Maximum retries (up to N+1 total attempts) | `3` |

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
# config.authorization_endpoint
# config.token_endpoint
```

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
)
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

### Token Expiry

```python
from basecamp.oauth import OAuthToken

token = OAuthToken(access_token="...", expires_in=7200)
token.is_expired()                 # False
token.is_expired(buffer_seconds=60)  # True if expiring within 60s
```

## Services

All services are accessed through an `AccountClient`, obtained via `client.for_account(account_id)`.

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

## Pagination

Paginated methods return a `ListResult`, which is a `list` subclass with a `.meta` attribute:

```python
projects = account.projects.list()

# ListResult is a list - iterate directly
for project in projects:
    print(project["name"])

# Access pagination metadata
print(projects.meta.total_count)   # total items across all pages
print(projects.meta.truncated)     # True if max_pages was reached

# Standard list operations work
print(len(projects))
first = projects[0]
sliced = projects[:5]
```

Pagination is automatic. The SDK follows Link headers and collects all pages up to `config.max_pages` (default: 10,000).

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
- `retryable` - Whether the error is safe to retry
- `retry_after` - Seconds to wait before retry (for rate limits)
- `request_id` - Server request ID (if available)
- `exit_code` - CLI-friendly exit code (`ExitCode` enum)

## Retry Behavior

The SDK automatically retries failed requests with exponential backoff:

- **GET requests** - Retried on `RateLimitError` (429), `NetworkError`, and retryable `ApiError` (500, 502, 503, 504)
- **Idempotent mutations** - Operations marked idempotent in the OpenAPI metadata also retry through the same path
- **Non-idempotent mutations** - NOT retried to prevent duplicate operations
- **401 responses** - Token refresh attempted, then single retry for all methods (regardless of idempotency)
- **Backoff** - Exponential with jitter (`base_delay * 2^(attempt-1) + random() * max_jitter`)
- **Retry-After** - Respected for 429 responses (overrides calculated backoff)
- **Max retries** - Controlled by `config.max_retries` (default: 3 retries, up to 4 total attempts including the initial request)

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
