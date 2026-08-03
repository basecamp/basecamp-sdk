# Basecamp Ruby SDK

Official Ruby SDK for the [Basecamp API](https://github.com/basecamp/bc3-api).

## Requirements

- Ruby 3.2+
- Faraday HTTP client

## Installation

Add to your Gemfile:

```ruby
gem "basecamp-sdk"
```

Or install directly:

```bash
gem install basecamp-sdk
```

## Getting a token

Every Basecamp API request carries an OAuth 2.0 access token. There is no API key and no personal access token, so even a throwaway script starts here:

1. Choose the grant that matches how your code runs:

| Your integration | Grant | Who refreshes the token |
|---|---|---|
| already holds a token you obtained elsewhere | **static token** — [`Token Providers`](#token-providers) | you do |
| can receive a browser redirect (web app, or a local callback server) | **authorization code + PKCE** — [`OAuth Flow Helpers`](#oauth-flow-helpers) | `OauthTokenProvider` |
| has no browser, but a person can approve on another device (CLI, headless server, TV) | **device flow** — [`Device Authorization Grant`](#device-authorization-grant-rfc-8628) | `Basecamp::Oauth.refresh_token`, echoing the token's `resource` — **not** `OauthTokenProvider` |

The one-line rule: **a redirect URI you control → authorization code; no browser but someone to approve → device flow; a token already in hand → static token.** An unattended daemon or CI job fits none of the three on its own — the device flow needs a person to enter the user code at the verification URI — so provision a token out of band and hand it to the process as a static or refresh token.

2. Get the client credentials that grant needs:

- **Authorization code + PKCE** — register your own integration at **<https://launchpad.37signals.com/integrations>**. You get a client ID, a client secret, and whatever redirect URI you nominated.
- **Device flow** — nothing to register. It runs as the pre-registered public `basecamp-cli` client, which sends no secret, against the device endpoint that discovery returns. Launchpad advertises no device endpoint, so a client you register there is not the one this flow uses.
- **Static token** — nothing to register; you already hold the token.

`StaticTokenProvider` hands back the string you gave it and nothing more — it never refreshes, so once the token expires every call fails with `401` until you supply a new one. Use it to get a first successful call, then move to a refreshing path before you ship — `OauthTokenProvider` for an authorization-code token from Launchpad, or, for a device-flow token, `Basecamp::Oauth.refresh_token` echoing the stored `resource`. Do not hand a device-flow token to `OauthTokenProvider`: it refreshes only against Launchpad and sends no `resource`, so it fails at the first expiry.

## Finding your account ID

Every API path is scoped to an account — `https://3.basecampapi.com/{accountId}/…` — so `for_account` needs that number before your first call. One token can reach several accounts, so ask the token which. `authorization` hangs off the *top-level* client because it takes no account context. Unlike the other SDKs that ship this service (Go, Python, TypeScript all hardcode Launchpad), Ruby does not: `Http#get_authorization_document` runs resource-first discovery (SPEC.md §16) against your configured base URL and fetches `/authorization.json` from the *selected* issuer, reaching Launchpad only on a soft fallback. Point egress rules and HTTP stubs at the issuer discovery selects, not at Launchpad; a hard selection failure raises `Basecamp::Oauth::DiscoverySelectionError` before any credentialed request goes out.

```ruby
client = Basecamp.client(access_token: ENV["BASECAMP_TOKEN"])

info = client.authorization.get
# "bc3" is Basecamp; the same response also carries "hey" and other products,
# and they are not ordered — filter before you pick, or you may scope the
# client to a HEY account.
basecamp_accounts = info["accounts"].select { |a| a["product"] == "bc3" }
basecamp_accounts.each do |a|
  puts "#{a["id"]}: #{a["name"]}"
end

account = client.for_account(basecamp_accounts.first["id"])
```

The response is parsed JSON with **string** keys, not symbols. `info["expires_at"]` tells you how long the token has left, which is the quickest way to confirm a static token has not lapsed.

## Quick Start

```ruby
require "basecamp"

# Create client with access token
client = Basecamp.client(access_token: ENV["BASECAMP_TOKEN"])

# Scope to an account
account = client.for_account(ENV["BASECAMP_ACCOUNT_ID"])

# List projects
account.projects.list.each do |project|
  puts "#{project['id']}: #{project['name']}"
end

# Get a specific project
project = account.projects.get(project_id: 12345)

# Create a todo
todo = account.todos.create(
  todolist_id: 67890,
  content: "Review PR",
  due_on: "2024-12-31"
)
```

## Configuration

### Basic Configuration

```ruby
config = Basecamp::Config.new(
  base_url: "https://3.basecampapi.com",  # Default
  timeout: 30,                             # Request timeout in seconds
  max_retries: 3,                          # Total request attempts for GET requests (including the initial request)
  base_delay: 1.0,                         # Base delay for exponential backoff
  max_pages: 10_000                         # Max pages for pagination
)

token_provider = Basecamp::StaticTokenProvider.new(ENV["BASECAMP_TOKEN"])
client = Basecamp::Client.new(config: config, token_provider: token_provider)
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `base_url` | `https://3.basecampapi.com` | Basecamp API base URL |
| `timeout` | `30` | HTTP request timeout (seconds) |
| `max_retries` | `3` | Total request attempts for GET requests (including the initial request) |
| `base_delay` | `1.0` | Base delay for exponential backoff (seconds) |
| `max_jitter` | `0.1` | Maximum random jitter added to delays |
| `max_pages` | `10_000` | Maximum pages to fetch during pagination |

## OAuth Authentication

### Token Providers

The SDK supports multiple authentication patterns:

```ruby
# Static token (simplest)
token_provider = Basecamp::StaticTokenProvider.new("your-access-token")

# OAuth with refresh
token_provider = Basecamp::OauthTokenProvider.new(
  access_token: "access-token",
  refresh_token: "refresh-token",
  expires_at: Time.now + 3600,
  client_id: ENV["BASECAMP_CLIENT_ID"],
  client_secret: ENV["BASECAMP_CLIENT_SECRET"]
)
```

### OAuth Flow Helpers

```ruby
# 1. Discover OAuth configuration
config = Basecamp::Oauth.discover_launchpad

# 2. Build authorization URL (redirect user here)
auth_url = "#{config.authorization_endpoint}?" + URI.encode_www_form(
  type: "web_server",
  client_id: ENV["BASECAMP_CLIENT_ID"],
  redirect_uri: "https://myapp.com/callback"
)

# 3. Exchange code for tokens (in callback handler)
token = Basecamp::Oauth.exchange_code(
  token_endpoint: config.token_endpoint,
  code: params[:code],
  redirect_uri: "https://myapp.com/callback",
  client_id: ENV["BASECAMP_CLIENT_ID"],
  client_secret: ENV["BASECAMP_CLIENT_SECRET"],
  use_legacy_format: true  # Required for Launchpad
)

# 4. Use the token
client = Basecamp.client(access_token: token.access_token)

# 5. Refresh when needed
if token.expired?
  token = Basecamp::Oauth.refresh_token(
    token_endpoint: config.token_endpoint,
    refresh_token: token.refresh_token,
    use_legacy_format: true
  )
end
```

### Device Authorization Grant (RFC 8628)

For input-constrained clients (CLIs, TVs) that can't host a redirect URI, the
device flow trades a redirect for a user-entered code. Basecamp pre-registers the
public `basecamp-cli` client (`token_endpoint_auth_method: none`, no secret); an
omitted scope defaults to `read` — prefer pinning it explicitly with
`scope: "read"`. Pass bare origins everywhere (no trailing slash): binding is
code-point exact — a trailing-slash `expected_issuer` raises a **hard**
`expected_issuer_unavailable`, while a trailing-slash resource origin breaks
the hop-1 binding and silently soft-falls back to Launchpad.

```ruby
# The device grant runs against an ALREADY-SELECTED config whose issuer advertises
# a device_authorization_endpoint. Resolve one with resource-first discovery —
# NOT discover_launchpad: Launchpad advertises no device endpoint, so the
# capability guard would reject it.
result = Basecamp::Oauth.discover_from_resource("https://3.basecampapi.com")
raise "device login is not available for this resource" unless result.selected?
config = result.config

# One call runs the whole grant against the SELECTED config: it guards
# capability, requests a device/user code, shows it via the display hook, then
# polls the token endpoint until the user approves.
token = Basecamp::Oauth.perform_device_login(
  config: config,
  client_id: "basecamp-cli",
  display: ->(auth) do
    puts "Visit #{auth.verification_uri} and enter code: #{auth.user_code}"
    puts "Or open: #{auth.verification_uri_complete}" if auth.verification_uri_complete
  end
)

client = Basecamp.client(access_token: token.access_token)

# BC5 device logins as basecamp-cli mint MULTI-ACCOUNT refresh tokens: the
# token carries an RFC 8707 resource indicator (token.resource,
# "urn:bc:account:<id>"), and refreshing without echoing it is rejected
# (400 invalid_request). Persist token.resource alongside the tokens and
# echo it on refresh:
# A device-token response MAY omit refresh_token — guard it: without one,
# refreshing is impossible and the user must re-run the device login.
if token.expired? && token.refresh_token
  fresh = Basecamp::Oauth.refresh_token(
    token_endpoint: config.token_endpoint,
    refresh_token: token.refresh_token,
    client_id: "basecamp-cli",   # public client — no secret
    resource: token.resource
  )
  # A refresh response MAY omit resource (the binding is unchanged) — persist
  # `fresh.resource || token.resource` so the next refresh still echoes it.
end
```

The capability guard requires BOTH `config.device_authorization_endpoint` AND the
exact grant-type URN `urn:ietf:params:oauth:grant-type:device_code` in
`config.grant_types_supported` — servers advertise the full URN, so checking for
a bare `device_code` entry would wrongly conclude the flow is unavailable;
otherwise it raises
`Basecamp::Oauth::DeviceFlowError` with `reason: :unavailable` before any request
is issued. The two lower-level steps are also exposed directly:

```ruby
auth = Basecamp::Oauth.request_device_authorization(
  device_authorization_endpoint: config.device_authorization_endpoint,
  client_id: "basecamp-cli"
)

token = Basecamp::Oauth.poll_device_token(
  token_endpoint: config.token_endpoint,
  client_id: "basecamp-cli",
  device_code: auth.device_code,
  interval: auth.interval,
  expires_in: auth.expires_in
)
```

The polling loop waits at least `interval` seconds between polls against a
**monotonic** deadline, sustains a `slow_down` bump (+5s for every later poll),
and backs off exponentially on connection timeouts. A terminal outcome raises
`DeviceFlowError` carrying a `reason` whose parent `type` is derived from it:

| `reason` | `type` | Meaning |
|---|---|---|
| `:access_denied` | `auth` | The user declined the request |
| `:expired` | `auth` | The code expired before approval |
| `:transport` | `network` (retryable) | A network failure ended the flow |
| `:unavailable` | `validation` | The config can't do device flow |
| `:cancelled` | `usage` | The caller cancelled the flow |

`poll_device_token` and `perform_device_login` accept injectable `clock`,
`sleeper`, and `cancelled` callables — pass a `cancelled` probe (e.g. one that
checks a signal set by SIGINT) to abort a pending login cleanly.

### Resource-First Discovery (RFC 9728 + RFC 8414)

BC5's Authorization Server (AS) metadata lives only at the canonical issuer (the
web host), so discovery starts from the **resource** (the API host) rather than
probing the API host for AS metadata. Three composable operations are provided:

```ruby
# RFC 8414 — AS metadata for a known issuer, bound to the requested issuer by
# code-point. token_endpoint is required; authorization_endpoint is OPTIONAL
# (device-only servers omit it) — authorization-code consumers must assert it.
config = Basecamp::Oauth.discover("https://launchpad.37signals.com")

# RFC 9728 — protected-resource metadata for a resource origin. resource is
# bound by code-point; authorization_servers preserves absent (nil) vs [].
resource = Basecamp::Oauth.discover_protected_resource("https://3.basecampapi.com")

# Orchestrator — resource-first selection + stage-sensitive fallback.
result = Basecamp::Oauth.discover_from_resource(
  "https://3.basecampapi.com",
  expected_issuer: nil # optional: authoritative issuer selection
)

if result.selected?
  config = result.config # bound AS config for the selected issuer
else
  # Only two SOFT reasons ever yield a fallback (→ Launchpad):
  #   "resource_discovery_failed" | "no_as_advertised"
  result.reason
end
```

`discover_from_resource` returns a `DiscoveryResult` that is either **selected**
or a **soft fallback**. Every *hard* failure — an ambiguous advertised set, an
unavailable `expected_issuer`, an invalid advertised issuer origin, an AS-metadata
fetch failure, or an issuer-binding mismatch after a BC5 issuer was selected —
raises `Basecamp::Oauth::DiscoverySelectionError` (carrying a `reason`). A hard
failure is **never** silently converted into a Launchpad request.

All discovery fetches are SSRF-hardened: origins are validated against the
origin-root profile (HTTPS-only, localhost exempt) with Ruby's `URI` parser before
any socket opens, redirects are not followed, timeouts are bounded, non-2xx maps
to `api_error`, and the response body is read under a genuine streaming cap that
aborts before an oversized body is buffered.

## Services

The SDK provides 46 account-scoped services. The table below covers the common ones; see `lib/basecamp/generated/services/` for the authoritative, complete set:

| Service | Description |
|---------|-------------|
| `projects` | Project management |
| `todos` | Todo items |
| `todolists` | Todo lists |
| `todosets` | Todo set containers |
| `todolist_groups` | Todolist grouping/folders |
| `people` | People/users |
| `comments` | Comments on recordings |
| `messages` | Message posts |
| `message_boards` | Message boards |
| `message_types` | Message categories |
| `campfires` | Chat rooms |
| `schedules` | Calendar schedules |
| `documents` | Documents |
| `vaults` | File folders |
| `uploads` | File uploads |
| `attachments` | Binary attachments |
| `recordings` | Generic recordings |
| `webhooks` | Webhook subscriptions |
| `subscriptions` | Notification subscriptions |
| `templates` | Project templates |
| `events` | Activity events |
| `checkins` | Automatic check-ins |
| `forwards` | Email forwards |
| `cards` | Card table cards |
| `card_tables` | Card tables (kanban) |
| `card_columns` | Card table columns |
| `card_steps` | Card workflow steps |
| `wormholes` | Card table wormholes (cross-project moves) |
| `lineup` | Card lineup view |
| `tools` | Project dock tools |
| `search` | Full-text search |
| `reports` | Activity reports |
| `timeline` | Activity timeline |
| `timesheet` | Time tracking reports |
| `client_approvals` | Client approval workflows |
| `client_correspondences` | Client communications |
| `client_replies` | Client replies |
| `authorization` | Auth info |

## Pagination

All list methods return a lazy `ListEnumerator` — an `Enumerator` subclass
that automatically handles pagination and carries metadata. The first page is
fetched when the method is called; later pages are fetched only as iteration
demands them:

```ruby
# Automatically fetches all pages
account.projects.list.each do |project|
  puts project["name"]
end

# Take only what you need — no extra pages are fetched
first_10 = account.todos.list(todolist_id: 456).take(10)

# Convert to array (fetches all pages)
all_projects = account.projects.list.to_a
```

Pagination is automatic: the SDK follows Link headers up to `config.max_pages`
(default: 10,000). The enumerator's `meta` exposes pagination metadata:

```ruby
projects = account.projects.list
projects.meta.total_count  # X-Total-Count from the first page (0 if absent),
                           # available immediately — page 1 is fetched eagerly
projects.to_a
projects.meta.truncated    # true if items beyond those yielded were available
```

Every list method also accepts a `max_items` keyword to cap how many items are
yielded. Enumeration stops as soon as the cap is met, without fetching further
pages. Zero or negative values disable the cap, as in the other SDKs:

```ruby
recent = account.projects.list(max_items: 50)
recent.to_a
recent.meta.truncated  # true only if more items were available
```

`meta.truncated` is final once enumeration completes, and is `true` only when
items beyond those yielded were available — items were dropped by `max_items`,
or the last-fetched page still advertised a next page when enumeration stopped
(at `max_items` or the `max_pages` safety cap). Landing exactly on the final
item is not truncation: when `truncated` is `false` after full enumeration,
the result is definitely complete.

### The `page` keyword

A positive `page` selects exactly that page: one request, that page's items,
no link-following.

```ruby
page_3 = account.projects.list(page: 3).to_a
```

Omit `page` (or pass `0`) to auto-paginate the whole collection. `max_items`
still trims a pinned page.

All six SDKs share these semantics — one request, that page only, no
link-following. See SPEC section 8.

## Downloading Files

Fetch an upload's file content in one call. The SDK fetches the upload
metadata, then follows the authenticated-hop + 302 flow against the
signed storage URL.

```ruby
result = account.uploads.download(upload_id: 1069479400)
File.binwrite("uploaded.bin", result.body)
# result.content_type, result.content_length, result.filename are also available
```

For any authenticated download URL (e.g. a `download_url` you already
have in hand), use `AccountClient#download_url`:

```ruby
result = account.download_url(url)
```

## Retry Behavior

Only plain GET requests retry — automatically, with exponential backoff. Mutation operations (POST, PUT, DELETE) do **not** retry to prevent data duplication, and the raw upload and download paths skip the retry loop entirely (the upload path is strictly one request; the download hop keeps only the one-shot 401 replay below).

- **Which errors**: Retry keys off the error's `retryable?` classification, not a declared status list — 429 (rate limit), 500, 502, 503, 504, and any other 5xx all retry, as does `NetworkError` (connection failures, including DNS and connect-phase timeouts). Read timeouts are the exception: Faraday surfaces them as a status-less `ApiError` with `retryable? == false`, so a GET that times out mid-response fails on the first attempt. 400, 401, 403, 404, and 422 never retry.
- **`max_retries`**: Total request attempts for GET requests, including the initial request — the default `3` means one initial attempt plus two retries. **`max_retries: 0` sends zero requests** and raises `Basecamp::ApiError` (`"Request failed after 0 attempts"`).
- **Backoff**: Exponential with jitter — `base_delay * 2^(attempt - 1) + rand * max_jitter` — uncapped, bounded in practice by the attempt budget.
- **Rate limits**: A 429's `Retry-After` header overrides the calculated backoff. Only 429 carries it: 5xx and network errors always use the exponential backoff.
- **401 responses**: With a refresh-capable token provider, the SDK refreshes the token and replays the request **once** — for all methods, including mutations — outside the `max_retries` budget. A second 401 is surfaced. The raw upload path has no 401 replay.
- **Per-operation metadata**: The retry policy operations declare (`retry_on` statuses, per-operation `max`) is inert in Ruby — every API GET issued through the client, including the Launchpad authorization fetch, rides the same classification-based loop bounded by `config.max_retries` alone. (The download flow's redirect hop and OAuth discovery use their own single-attempt transports.)
- **`retryable?`**: Unlike SDKs where the error classification is only a hint for your own code, in Ruby an error's `retryable?` (and `retry_after`) is exactly what the transport acts on for GET requests.

## Error Handling

```ruby
begin
  account.projects.get(project_id: 99999)
rescue Basecamp::NotFoundError => e
  puts "Project not found: #{e.message}"
rescue Basecamp::RateLimitError => e
  puts "Rate limited, retry after: #{e.retry_after} seconds"
rescue Basecamp::AuthError => e
  puts "Authentication failed: #{e.message}"
rescue Basecamp::ApiError => e
  puts "API error (#{e.http_status}): #{e.message}"
end
```

### Error Types

| Error | Description |
|-------|-------------|
| `ApiError` | Base error class for all API errors |
| `AuthError` | Authentication failures (401) |
| `ForbiddenError` | Access denied (403) |
| `NotFoundError` | Resource not found (404) |
| `ValidationError` | Invalid request data (400, 422) |
| `RateLimitError` | Rate limit exceeded (429) |
| `NetworkError` | Connection failures |

### Validation Errors

Basecamp rejects invalid writes with a body keyed by field. The SDK folds those
messages into the message and keeps the raw map in `field_errors`, so you can
drive a form without re-parsing the message:

```ruby
begin
  account.calendars.update_calendar(calendar_id: calendar_id, calendar: { "color" => "chartreuse" })
rescue Basecamp::ValidationError => e
  puts e.message # => "color: is not a valid color"

  e.field_errors&.each do |field, messages|
    messages.each { |message| puts "  #{field} #{message}" }
  end
end
```

`field_errors` is `nil` for every other error shape, and its messages are the
raw ones — the message is capped at 500 bytes, the map is not.

## Observability Hooks

Monitor SDK behavior with hooks:

```ruby
class MyHooks
  include Basecamp::Hooks

  def on_request_start(info)
    puts "Starting #{info.method} #{info.url}"
  end

  def on_request_end(info, result)
    puts "Completed in #{result.duration}s with status #{result.status_code}"
  end

  def on_retry(info, attempt, error, delay)
    puts "Retrying attempt #{attempt} after #{delay}s"
  end

  def on_paginate(url, page)
    puts "Fetching page #{page}"
  end
end

client = Basecamp::Client.new(
  config: config,
  token_provider: token_provider,
  hooks: MyHooks.new
)
```

## Environment Variables

`Basecamp::Config.from_env` (and `#load_from_env` on an existing config) reads these three. They are the only `BASECAMP_*` variables the SDK reads anywhere; the sole other environment read is `XDG_CONFIG_HOME`, for `Config.global_config_dir`.

| Variable | Description |
|----------|-------------|
| `BASECAMP_BASE_URL` | API base URL (default: `https://3.basecampapi.com`) |
| `BASECAMP_TIMEOUT` | Request timeout in seconds (default: `30`) |
| `BASECAMP_MAX_RETRIES` | Total request attempts for GET requests, including the initial request (default: `3`) |

```ruby
config = Basecamp::Config.from_env
client = Basecamp::Client.new(config: config, token_provider: token_provider)
```

Credentials are **not** among them. `BASECAMP_TOKEN` and `BASECAMP_ACCOUNT_ID` appear in the examples above only because the caller reads them and passes the values in; the SDK never looks them up. Pass the token to `Basecamp.client(access_token:)` or `StaticTokenProvider`, and the account ID to `#for_account`.

## Development

```bash
# Install dependencies
bundle install

# Run tests
bundle exec rake test

# Run linter
bundle exec rubocop

# Generate types from OpenAPI
ruby scripts/generate-types.rb
```

## License

MIT
