# Contributing to Basecamp SDK

Thank you for your interest in contributing to the Basecamp SDK. This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.25 or later
- [golangci-lint](https://golangci-lint.run/welcome/install/) for linting
- A Basecamp account for integration testing (optional)

### Getting Started

1. Clone the repository:
   ```bash
   git clone https://github.com/basecamp/basecamp-sdk.git
   cd basecamp-sdk
   ```

2. Install dependencies:
   ```bash
   cd go
   go mod download
   ```

3. Run the test suite:
   ```bash
   make test
   ```

4. Run all checks (formatting, linting, tests):
   ```bash
   make check
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
   cd go
   make check
   ```

2. **Ensure tests pass:**
   ```bash
   make test
   ```

3. **Check for vulnerabilities:**
   ```bash
   make vuln
   ```

4. **Update documentation** if adding new features

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

When adding support for new Basecamp API endpoints:

1. **Create the service file** (e.g., `go/pkg/basecamp/myservice.go`)
   - Define types for requests and responses
   - Implement the service with CRUD methods as applicable
   - Follow patterns from existing services

2. **Add tests** (e.g., `go/pkg/basecamp/myservice_test.go`)
   - Test all public methods
   - Mock HTTP responses
   - Cover error cases

3. **Register the service** in `client.go`:
   - Add the field to the `Client` struct
   - Add the lazy-initialization getter method

4. **Update documentation**:
   - Add to the services table in README
   - Add to CHANGELOG under `[Unreleased]`

## Reporting Issues

- Use GitHub Issues for bug reports and feature requests
- Include reproduction steps for bugs
- Provide Go version and OS information
- Include relevant error messages and logs

## Questions?

Open a GitHub Discussion for questions about contributing or using the SDK.
