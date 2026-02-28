// Package basecamp provides a Go SDK for the Basecamp API.
package basecamp

import (
	"errors"
	"fmt"
)

// Resilience errors for circuit breaker, bulkhead, and rate limiting.
var (
	// ErrCircuitOpen is returned when the circuit breaker is open.
	ErrCircuitOpen = errors.New("circuit breaker is open")
	// ErrBulkheadFull is returned when the bulkhead has no available slots.
	ErrBulkheadFull = errors.New("bulkhead is full")
	// ErrRateLimited is returned when the rate limiter rejects a request.
	ErrRateLimited = errors.New("rate limit exceeded")
)

// Error codes for API responses.
const (
	CodeUsage      = "usage"
	CodeNotFound   = "not_found"
	CodeAuth       = "auth_required"
	CodeForbidden  = "forbidden"
	CodeRateLimit  = "rate_limit"
	CodeNetwork    = "network"
	CodeAPI        = "api_error"
	CodeValidation = "validation"
	CodeAmbiguous  = "ambiguous"
)

// Exit codes for CLI tools.
const (
	ExitOK        = 0 // Success
	ExitUsage     = 1 // Invalid arguments or flags
	ExitNotFound  = 2 // Resource not found
	ExitAuth      = 3 // Not authenticated
	ExitForbidden = 4 // Access denied (scope issue)
	ExitRateLimit = 5 // Rate limited (429)
	ExitNetwork   = 6 // Connection/DNS/timeout error
	ExitAPI       = 7 // Server returned error
	ExitAmbiguous = 8 // Multiple matches for name
)

// Error is a structured error with code, message, and optional hint.
type Error struct {
	Code       string
	Message    string
	Hint       string
	HTTPStatus int
	Retryable  bool
	Cause      error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Hint)
	}
	return e.Message
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *Error) Unwrap() error {
	return e.Cause
}

// ExitCode returns the appropriate exit code for this error.
func (e *Error) ExitCode() int {
	return ExitCodeFor(e.Code)
}

// ExitCodeFor returns the exit code for a given error code.
func ExitCodeFor(code string) int {
	switch code {
	case CodeUsage:
		return ExitUsage
	case CodeNotFound:
		return ExitNotFound
	case CodeAuth:
		return ExitAuth
	case CodeForbidden:
		return ExitForbidden
	case CodeRateLimit:
		return ExitRateLimit
	case CodeNetwork:
		return ExitNetwork
	case CodeAPI:
		return ExitAPI
	case CodeAmbiguous:
		return ExitAmbiguous
	default:
		return ExitAPI
	}
}

// ErrUsage creates a usage error.
func ErrUsage(msg string) *Error {
	return &Error{Code: CodeUsage, Message: msg}
}

// ErrUsageHint creates a usage error with a hint.
func ErrUsageHint(msg, hint string) *Error {
	return &Error{Code: CodeUsage, Message: msg, Hint: hint}
}

// ErrNotFound creates a not-found error.
func ErrNotFound(resource, identifier string) *Error {
	return &Error{
		Code:    CodeNotFound,
		Message: fmt.Sprintf("%s not found: %s", resource, identifier),
	}
}

// ErrNotFoundHint creates a not-found error with a hint.
func ErrNotFoundHint(resource, identifier, hint string) *Error {
	return &Error{
		Code:    CodeNotFound,
		Message: fmt.Sprintf("%s not found: %s", resource, identifier),
		Hint:    hint,
	}
}

// ErrAuth creates an authentication error.
func ErrAuth(msg string) *Error {
	return &Error{
		Code:    CodeAuth,
		Message: msg,
	}
}

// ErrForbidden creates a forbidden error.
func ErrForbidden(msg string) *Error {
	return &Error{
		Code:       CodeForbidden,
		Message:    msg,
		HTTPStatus: 403,
	}
}

// ErrForbiddenScope creates a forbidden error due to insufficient scope.
func ErrForbiddenScope() *Error {
	return &Error{
		Code:       CodeForbidden,
		Message:    "Access denied: insufficient scope",
		Hint:       "Re-authenticate with full scope",
		HTTPStatus: 403,
	}
}

// ErrRateLimit creates a rate-limit error.
func ErrRateLimit(retryAfter int) *Error {
	hint := "Try again later"
	if retryAfter > 0 {
		hint = fmt.Sprintf("Try again in %d seconds", retryAfter)
	}
	return &Error{
		Code:       CodeRateLimit,
		Message:    "Rate limited",
		Hint:       hint,
		HTTPStatus: 429,
		Retryable:  true,
	}
}

// ErrNetwork creates a network error.
func ErrNetwork(cause error) *Error {
	return &Error{
		Code:      CodeNetwork,
		Message:   "Network error",
		Hint:      cause.Error(),
		Retryable: true,
		Cause:     cause,
	}
}

// ErrAPI creates an API error with an HTTP status code.
func ErrAPI(status int, msg string) *Error {
	return &Error{
		Code:       CodeAPI,
		Message:    msg,
		HTTPStatus: status,
	}
}

// ErrAmbiguous creates an ambiguous match error.
func ErrAmbiguous(resource string, matches []string) *Error {
	hint := "Be more specific"
	if len(matches) > 0 && len(matches) <= 5 {
		hint = fmt.Sprintf("Did you mean: %v", matches)
	}
	return &Error{
		Code:    CodeAmbiguous,
		Message: fmt.Sprintf("Ambiguous %s", resource),
		Hint:    hint,
	}
}

// AsError attempts to convert an error to an *Error.
// If the error is not an *Error, it wraps it in one.
func AsError(err error) *Error {
	if e, ok := errors.AsType[*Error](err); ok {
		return e
	}
	return &Error{
		Code:    CodeAPI,
		Message: err.Error(),
		Cause:   err,
	}
}
