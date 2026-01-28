package basecamp

import (
	"fmt"
	"net/http"
)

// checkResponse converts HTTP response errors to SDK errors for non-2xx responses.
// Used by all service methods that call the generated client.
func checkResponse(resp *http.Response) error {
	if resp == nil {
		return nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &Error{Code: "auth", Message: "authentication required", HTTPStatus: 401}
	case http.StatusForbidden:
		return &Error{Code: "forbidden", Message: "access denied", HTTPStatus: 403}
	case http.StatusNotFound:
		return &Error{Code: "not_found", Message: "resource not found", HTTPStatus: 404}
	case http.StatusUnprocessableEntity:
		return &Error{Code: "validation", Message: "validation error", HTTPStatus: 422}
	case http.StatusTooManyRequests:
		return &Error{Code: "rate_limit", Message: "rate limited - try again later", HTTPStatus: 429, Retryable: true}
	default:
		return &Error{Code: "api", Message: fmt.Sprintf("API error: %s", resp.Status), HTTPStatus: resp.StatusCode}
	}
}

// Pointer dereference helpers for converting generated types (which use pointers)
// to SDK types (which use values).

// derefInt64 safely dereferences a pointer, returning 0 if nil.
func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
