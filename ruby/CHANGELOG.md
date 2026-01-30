# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-01-28

### Added

- Initial release of the Basecamp Ruby SDK
- 38 services covering the complete Basecamp 3 API:
  - Projects, Todos, Todolists, Todosets, Todolist Groups
  - People, Comments, Messages, Message Boards, Message Types
  - Campfires, Schedules, Documents, Vaults, Uploads, Attachments
  - Recordings, Webhooks, Subscriptions, Templates, Events
  - Checkins, Forwards, Cards, Card Tables, Card Columns, Card Steps
  - Lineup, Tools, Search, Reports, Timeline, Timesheet
  - Client Approvals, Client Correspondences, Client Replies
  - Authorization
- OAuth helpers for authorization flow:
  - `Basecamp::Oauth.discover_launchpad` for OAuth configuration discovery
  - `Basecamp::Oauth.exchange_code` for code-to-token exchange
  - `Basecamp::Oauth.refresh_token` for token refresh
  - Support for Launchpad legacy format
- Token providers:
  - `StaticTokenProvider` for simple token-based auth
  - `OauthTokenProvider` for OAuth with automatic refresh
- Retry with exponential backoff for GET requests
  - Retries on 429 (rate limit), 502, 503, 504 errors
  - Respects `Retry-After` header
  - Configurable max retries and base delay
- Link-header pagination with lazy Ruby Enumerators
- Observability hooks for monitoring requests
- Comprehensive error hierarchy:
  - `APIError`, `AuthError`, `ForbiddenError`, `NotFoundError`
  - `ValidationError`, `RateLimitError`, `NetworkError`
- Generated types from OpenAPI specification
- 370+ unit tests
- Requires Ruby 3.2+, uses Faraday HTTP client

[0.1.0]: https://github.com/basecamp/basecamp-sdk/releases/tag/ruby/v0.1.0
