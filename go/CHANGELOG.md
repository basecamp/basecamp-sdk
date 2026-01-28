# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-01-25

Initial release of the Basecamp Go SDK.

### Added

#### Core Features
- OAuth 2.0 authentication with automatic token refresh
- Static token authentication via `BASECAMP_TOKEN` environment variable
- ETag-based HTTP caching for GET requests
- Exponential backoff with jitter for retryable errors
- Automatic pagination handling via `GetAll()`
- Structured error types with exit codes for CLI integration
- Secure credential storage using system keyring (with file fallback)
- Configurable logging via `slog`

#### API Services

**Projects & Organization**
- `ProjectsService` - List, get, create, update, and trash projects
- `TemplatesService` - List and get project templates
- `ToolsService` - Manage dock tools (enable/disable/reorder)
- `PeopleService` - List and manage people in accounts and projects

**To-dos**
- `TodosService` - Full CRUD for todo items, plus complete/uncomplete/reposition
- `TodosetsService` - Access project todosets
- `TodolistsService` - List, get, create, update, and trash todolists
- `TodolistGroupsService` - Manage todolist groups (sections)

**Messages & Communication**
- `MessagesService` - List, get, create, update, and trash messages
- `MessageBoardsService` - Access project message boards
- `MessageTypesService` - List, get, create, update, and destroy message types
- `CommentsService` - List, get, create, update, and trash comments
- `CampfiresService` - Real-time chat rooms with full line management
- `ForwardsService` - Email forwarding management

**Scheduling**
- `SchedulesService` - Schedule management with entry CRUD and occurrence handling
- `LineupService` - Lineup marker management
- `CheckinsService` - Automatic check-ins with question and answer management

**Files & Documents**
- `VaultsService` - Vault (folder) management
- `AttachmentsService` - File upload with signed URL workflow

**Card Tables (Kanban)**
- `CardTablesService` - Access card tables
- `CardsService` - Full CRUD for cards with move operations
- `CardColumnsService` - Column management with watch/unwatch
- `CardStepsService` - Card workflow steps

**Reporting & Search**
- `TimesheetService` - Timesheet reports (my entries, project entries)
- `SearchService` - Full-text search across the account
- `EventsService` - Activity event streams

**Integrations**
- `WebhooksService` - Webhook subscription management
- `SubscriptionsService` - Notification subscription management
- `RecordingsService` - Archive, unarchive, and trash any recording

**Client Portal**
- `ClientApprovalsService` - Client approval workflow
- `ClientCorrespondencesService` - Client correspondence management

#### Configuration
- Environment variable configuration (`BASECAMP_PROJECT_ID`, etc.)
- JSON file configuration support
- XDG-compliant cache and config directories
- Multi-account support via `client.ForAccount(accountID)`

[Unreleased]: https://github.com/basecamp/basecamp-sdk/compare/go/v0.1.0...HEAD
[0.1.0]: https://github.com/basecamp/basecamp-sdk/releases/tag/go/v0.1.0
