package basecamp

import (
	"fmt"
	"net/http"
	"strconv"
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
		return &Error{Code: CodeAuth, Message: "authentication required", HTTPStatus: 401}
	case http.StatusForbidden:
		return &Error{Code: CodeForbidden, Message: "access denied", HTTPStatus: 403}
	case http.StatusNotFound:
		return &Error{Code: CodeNotFound, Message: "resource not found", HTTPStatus: 404}
	case http.StatusUnprocessableEntity:
		return &Error{Code: CodeAPI, Message: "validation error", HTTPStatus: 422}
	case http.StatusTooManyRequests:
		return &Error{Code: CodeRateLimit, Message: "rate limited - try again later", HTTPStatus: 429, Retryable: true}
	default:
		return &Error{Code: CodeAPI, Message: fmt.Sprintf("API error: %s", resp.Status), HTTPStatus: resp.StatusCode}
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

// ListMeta contains pagination metadata from list operations.
type ListMeta struct {
	// TotalCount is the total number of items available (from X-Total-Count header).
	// Zero if the header was not present or could not be parsed.
	TotalCount int
}

// parseTotalCount extracts the total count from X-Total-Count header.
// Returns 0 if the header is missing or cannot be parsed.
func parseTotalCount(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	header := resp.Header.Get("X-Total-Count")
	if header == "" {
		return 0
	}
	count, err := strconv.Atoi(header)
	if err != nil || count < 0 {
		return 0
	}
	return count
}
