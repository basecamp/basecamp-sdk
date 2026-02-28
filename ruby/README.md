# Basecamp Ruby SDK

Official Ruby SDK for the [Basecamp API](https://github.com/basecamp/bc3-api).

## Requirements

- Ruby 3.4+
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
  project_id: 12345,
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
  max_retries: 3,                          # Max retry attempts for GET requests
  base_delay: 1.0,                         # Base delay for exponential backoff
  max_pages: 100                           # Max pages for pagination
)

token_provider = Basecamp::StaticTokenProvider.new(ENV["BASECAMP_TOKEN"])
client = Basecamp::Client.new(config: config, token_provider: token_provider)
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `base_url` | `https://3.basecampapi.com` | Basecamp API base URL |
| `timeout` | `30` | HTTP request timeout (seconds) |
| `max_retries` | `3` | Maximum retry attempts for GET requests |
| `base_delay` | `1.0` | Base delay for exponential backoff (seconds) |
| `max_jitter` | `0.5` | Maximum random jitter added to delays |
| `max_pages` | `100` | Maximum pages to fetch during pagination |

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

## Services

The SDK provides 38 services covering the complete Basecamp API:

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

All list methods return lazy Enumerators that automatically handle pagination:

```ruby
# Automatically fetches all pages
account.projects.list.each do |project|
  puts project["name"]
end

# Take only what you need
first_10 = account.todos.list(todolist_id: 456).take(10)

# Convert to array (fetches all pages)
all_projects = account.projects.list.to_a
```

## Retry Behavior

GET requests automatically retry on transient failures with exponential backoff:

- **Retryable errors**: 429 (rate limit), 502, 503, 504 (gateway errors)
- **Backoff**: Exponential with jitter (1s, 2s, 4s...)
- **Rate limits**: Respects `Retry-After` header

Mutation operations (POST, PUT, DELETE) do **not** retry to prevent data duplication.

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
rescue Basecamp::APIError => e
  puts "API error (#{e.http_status}): #{e.message}"
end
```

### Error Types

| Error | Description |
|-------|-------------|
| `APIError` | Base error class for all API errors |
| `AuthError` | Authentication failures (401) |
| `ForbiddenError` | Access denied (403) |
| `NotFoundError` | Resource not found (404) |
| `ValidationError` | Invalid request data (400, 422) |
| `RateLimitError` | Rate limit exceeded (429) |
| `NetworkError` | Connection failures |

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

| Variable | Description |
|----------|-------------|
| `BASECAMP_TOKEN` | OAuth access token |
| `BASECAMP_ACCOUNT_ID` | Account ID |
| `BASECAMP_BASE_URL` | API base URL (default: `https://3.basecampapi.com`) |

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
