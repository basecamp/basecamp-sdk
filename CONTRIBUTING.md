# Contributing to Basecamp SDK

Thank you for your interest in contributing to the Basecamp SDK. This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

| SDK | Requirements |
|-----|-------------|
| Go | Go 1.26+, [golangci-lint](https://golangci-lint.run/welcome/install/) |
| TypeScript | Node.js 18+, npm |
| Ruby | Ruby 3.2+, Bundler |
| Swift | Swift 6.0+, Xcode 16+ |
| Kotlin | JDK 17+, Kotlin 2.0+ |

A Basecamp account is optional (for integration testing only).

### Getting Started

1. Clone the repository:
   ```bash
   git clone https://github.com/basecamp/basecamp-sdk.git
   cd basecamp-sdk
   ```

2. Install dependencies and run tests for each SDK:

   **Go:**
   ```bash
   cd go && go mod download
   make test
   make check   # formatting, linting, tests
   ```

   **TypeScript:**
   ```bash
   cd typescript && npm install
   npm test
   npm run typecheck
   npm run lint
   ```

   **Ruby:**
   ```bash
   cd ruby && bundle install
   bundle exec rake test
   bundle exec rubocop
   ```

   **Swift:**
   ```bash
   cd swift
   swift build
   swift test
   ```

   **Kotlin:**
   ```bash
   cd kotlin
   ./gradlew :sdk:jvmTest
   ```

3. Run all SDKs at once from the repo root:
   ```bash
   make check        # all 5 SDK test suites
   make conformance  # cross-SDK conformance tests
   ```

## Code Style

### Go Code

- Follow standard Go conventions and [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting (run `make fmt`)
- Keep functions focused and small
- Document all exported types, functions, and methods
- Use meaningful variable names

### Naming Conventions

- Service types: `*Service` (e.g., `ProjectsService`, `TodosService`)
- Request types: `Create*Request`, `Update*Request`
- Options types: `*Options` or `*ListOptions`
- Error constructors: `Err*` (e.g., `ErrNotFound`, `ErrAuth`)

### Error Handling

- Return structured `*Error` types with appropriate codes
- Include helpful hints for user-facing errors
- Use `ErrUsageHint()` for configuration/usage errors
- Wrap underlying errors with context

### Testing

- Write unit tests for all new functionality
- Use table-driven tests where appropriate
- Mock HTTP responses using `httptest`
- Test both success and error paths

## Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/) for clear, structured commit history.

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, semicolons, etc.)
- `refactor`: Code changes that neither fix bugs nor add features
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `build`: Build system or dependency changes
- `ci`: CI configuration changes
- `chore`: Other changes that don't modify src or test files

### Scope

Use the service or component name:
- `projects`, `todos`, `campfires`, `webhooks`, etc.
- `auth`, `client`, `config`, `errors`
- `docs`, `ci`, `deps`

### Examples

```
feat(schedules): add GetEntryOccurrence method

fix(timesheet): use bucket-scoped endpoints for reports

docs(readme): add error handling section

test(cards): add coverage for move operations
```

## Pull Request Process

### Before Submitting

1. **Run all checks locally:**
   ```bash
   make check  # runs all 5 SDK test suites from repo root
   ```

2. **Ensure conformance tests pass:**
   ```bash
   make conformance
   ```

3. **Update documentation** if adding new features

### Submitting a PR

1. Create a feature branch from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```

2. Make your changes with clear, focused commits

3. Push and open a pull request against `main`

4. Fill out the PR template with:
   - Summary of changes
   - Motivation and context
   - Testing performed
   - Breaking changes (if any)

### Review Process

- All PRs require at least one review
- CI must pass (tests, linting, security checks)
- Address review feedback promptly
- Squash commits if requested

## Adding New API Coverage

All SDKs are generated from a single Smithy specification. When adding support for new Basecamp API endpoints:

1. **Edit the Smithy model** (`spec/basecamp.smithy`)
   - Define the resource, operations, and shapes
   - Follow patterns from existing resources (e.g., `Project`, `Todo`)

2. **Regenerate the OpenAPI spec**
   ```bash
   make smithy-build
   ```

3. **Run per-SDK generators** to update generated service code:
   - **Go:** `make go-check-drift` — Go services are hand-written wrappers around the generated client; the drift check verifies all generated operations are covered
   - **TypeScript:** `make ts-generate-services`
   - **Ruby:** `make rb-generate-services`
   - **Swift:** `make swift-generate`
   - **Kotlin:** `make kt-generate-services`

4. **Add tests** for each SDK

5. **Add conformance tests** (`conformance/tests/`) covering the new operations

6. **Update documentation**:
   - Add to the services table in each SDK's README
   - Add to CHANGELOG under `[Unreleased]`

## Reporting Issues

- Use GitHub Issues for bug reports and feature requests
- Include reproduction steps for bugs
- Provide Go version and OS information
- Include relevant error messages and logs

## Questions?

Open a GitHub Discussion for questions about contributing or using the SDK.
