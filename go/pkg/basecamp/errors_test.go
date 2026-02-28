package basecamp

import (
	"errors"
	"fmt"
	"testing"
)

func TestError_Error(t *testing.T) {
	e := &Error{Code: CodeAuth, Message: "not authenticated"}
	if got := e.Error(); got != "not authenticated" {
		t.Errorf("Error() = %q, want %q", got, "not authenticated")
	}
}

func TestError_ErrorWithHint(t *testing.T) {
	e := &Error{Code: CodeAuth, Message: "not authenticated", Hint: "run login"}
	want := "not authenticated: run login"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("underlying")
	e := &Error{Code: CodeNetwork, Message: "network error", Cause: cause}
	if !errors.Is(e, cause) {
		t.Error("expected errors.Is to find cause")
	}
}

func TestError_UnwrapNil(t *testing.T) {
	e := &Error{Code: CodeAPI, Message: "some error"}
	if e.Unwrap() != nil {
		t.Error("expected Unwrap to return nil when no cause")
	}
}

func TestAsError_WithSDKError(t *testing.T) {
	original := &Error{Code: CodeNotFound, Message: "not found"}
	got := AsError(original)
	if got != original {
		t.Error("expected AsError to return same *Error")
	}
}

func TestAsError_WithStdError(t *testing.T) {
	original := fmt.Errorf("something went wrong")
	got := AsError(original)
	if got.Code != CodeAPI {
		t.Errorf("Code = %q, want %q", got.Code, CodeAPI)
	}
	if got.Message != "something went wrong" {
		t.Errorf("Message = %q, want %q", got.Message, "something went wrong")
	}
	if !errors.Is(got, original) {
		t.Error("expected wrapped error to preserve cause")
	}
}

func TestAsError_WrappedSDKError(t *testing.T) {
	inner := &Error{Code: CodeRateLimit, Message: "rate limited", Retryable: true}
	wrapped := fmt.Errorf("wrapped: %w", inner)
	got := AsError(wrapped)
	if got.Code != CodeRateLimit {
		t.Errorf("Code = %q, want %q", got.Code, CodeRateLimit)
	}
	if !got.Retryable {
		t.Error("expected Retryable to be preserved")
	}
}

func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		code string
		want int
	}{
		{CodeUsage, ExitUsage},
		{CodeNotFound, ExitNotFound},
		{CodeAuth, ExitAuth},
		{CodeForbidden, ExitForbidden},
		{CodeRateLimit, ExitRateLimit},
		{CodeNetwork, ExitNetwork},
		{CodeAPI, ExitAPI},
		{CodeValidation, ExitValidation},
		{CodeAmbiguous, ExitAmbiguous},
		{"unknown_code", ExitAPI},
	}

	for _, tt := range tests {
		got := ExitCodeFor(tt.code)
		if got != tt.want {
			t.Errorf("ExitCodeFor(%q) = %d, want %d", tt.code, got, tt.want)
		}
	}
}

func TestError_ExitCode(t *testing.T) {
	e := &Error{Code: CodeNotFound, Message: "not found"}
	if got := e.ExitCode(); got != ExitNotFound {
		t.Errorf("ExitCode() = %d, want %d", got, ExitNotFound)
	}
}

func TestErrAmbiguous(t *testing.T) {
	e := ErrAmbiguous("project", []string{"foo", "foobar"})
	if e.Code != CodeAmbiguous {
		t.Errorf("Code = %q, want %q", e.Code, CodeAmbiguous)
	}
	if e.Hint == "" {
		t.Error("expected non-empty hint with matches")
	}
}

func TestErrAmbiguous_TooManyMatches(t *testing.T) {
	matches := make([]string, 10)
	e := ErrAmbiguous("project", matches)
	if e.Hint != "Be more specific" {
		t.Errorf("Hint = %q, want generic hint for >5 matches", e.Hint)
	}
}

func TestErrAmbiguous_NoMatches(t *testing.T) {
	e := ErrAmbiguous("project", nil)
	if e.Hint != "Be more specific" {
		t.Errorf("Hint = %q, want generic hint for nil matches", e.Hint)
	}
}

func TestRetryable_Propagation(t *testing.T) {
	e := ErrRateLimit(5)
	if !e.Retryable {
		t.Error("expected ErrRateLimit to be Retryable")
	}
	if e.Code != CodeRateLimit {
		t.Errorf("Code = %q, want %q", e.Code, CodeRateLimit)
	}
	if e.HTTPStatus != 429 {
		t.Errorf("HTTPStatus = %d, want 429", e.HTTPStatus)
	}
}

func TestRetryable_Network(t *testing.T) {
	e := ErrNetwork(fmt.Errorf("connection refused"))
	if !e.Retryable {
		t.Error("expected ErrNetwork to be Retryable")
	}
	if e.Code != CodeNetwork {
		t.Errorf("Code = %q, want %q", e.Code, CodeNetwork)
	}
}

func TestErrUsage(t *testing.T) {
	e := ErrUsage("bad arg")
	if e.Code != CodeUsage {
		t.Errorf("Code = %q, want %q", e.Code, CodeUsage)
	}
}

func TestErrUsageHint(t *testing.T) {
	e := ErrUsageHint("bad arg", "try --help")
	if e.Hint != "try --help" {
		t.Errorf("Hint = %q, want %q", e.Hint, "try --help")
	}
}

func TestErrNotFound(t *testing.T) {
	e := ErrNotFound("project", "123")
	if e.Code != CodeNotFound {
		t.Errorf("Code = %q, want %q", e.Code, CodeNotFound)
	}
}

func TestErrNotFoundHint(t *testing.T) {
	e := ErrNotFoundHint("project", "123", "check the URL")
	if e.Hint != "check the URL" {
		t.Errorf("Hint = %q, want %q", e.Hint, "check the URL")
	}
}

func TestErrForbidden(t *testing.T) {
	e := ErrForbidden("access denied")
	if e.Code != CodeForbidden {
		t.Errorf("Code = %q, want %q", e.Code, CodeForbidden)
	}
	if e.HTTPStatus != 403 {
		t.Errorf("HTTPStatus = %d, want 403", e.HTTPStatus)
	}
}

func TestErrForbiddenScope(t *testing.T) {
	e := ErrForbiddenScope()
	if e.Code != CodeForbidden {
		t.Errorf("Code = %q, want %q", e.Code, CodeForbidden)
	}
	if e.Hint == "" {
		t.Error("expected non-empty hint")
	}
}

func TestErrAPI(t *testing.T) {
	e := ErrAPI(500, "internal server error")
	if e.Code != CodeAPI {
		t.Errorf("Code = %q, want %q", e.Code, CodeAPI)
	}
	if e.HTTPStatus != 500 {
		t.Errorf("HTTPStatus = %d, want 500", e.HTTPStatus)
	}
}

func TestSentinelErrors(t *testing.T) {
	if ErrCircuitOpen == nil {
		t.Error("ErrCircuitOpen should not be nil")
	}
	if ErrBulkheadFull == nil {
		t.Error("ErrBulkheadFull should not be nil")
	}
	if ErrRateLimited == nil {
		t.Error("ErrRateLimited should not be nil")
	}
}
