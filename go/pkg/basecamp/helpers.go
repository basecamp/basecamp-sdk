package basecamp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// marshalBody encodes a map as JSON and returns an io.Reader suitable for the
// generated client's *WithBodyWithResponse methods.
//
// This is an intentional architectural exception to the normal pattern of using
// generated typed request bodies. It exists because the generated structs for
// several Update endpoints contain value-type fields (types.Date, time.Time,
// nested structs) whose Go zero values serialize as non-empty JSON:
//
//   - types.Date{}  → "due_on": null
//   - time.Time{}   → "starts_at": "0001-01-01T00:00:00Z"
//   - struct{}      → "schedule_attributes": {}
//
// These leak into partial updates and can clear existing data server-side.
// Building a map[string]any and only inserting provided keys avoids this.
//
// Methods using this pattern (do not "simplify" back to generated bodies):
//   - TodosService.Update           (types.Date: due_on, starts_on)
//   - SchedulesService.UpdateEntry  (time.Time: starts_at, ends_at)
//   - CardsService.Update           (types.Date: due_on)
//   - CardStepsService.Update       (types.Date: due_on)
//   - ProjectsService.Update        (nested: schedule_attributes)
//   - CheckinsService.UpdateQuestion (nested: schedule)
//   - CheckinsService.CreateQuestion (nested: schedule — Hour/Minute int32 omitempty)
//   - CheckinsService.UpdateAnswer   (ISO8601Date: group_on)
//   - PeopleService.UpdateMyProfile   (person wrapper + *string clearable fields)
func marshalBody(m map[string]any) (io.Reader, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	return &rewindableReader{data: b}, nil
}

// rewindableReader wraps a byte slice as an io.Reader that auto-rewinds
// after returning EOF, so the generated client's doWithRetry can replay
// the body on each retry attempt. Safe against partial reads: once EOF
// is returned, the next Read starts from position 0.
type rewindableReader struct {
	data  []byte
	pos   int
	atEOF bool
}

func (r *rewindableReader) Read(p []byte) (int, error) {
	if r.atEOF {
		r.pos = 0
		r.atEOF = false
	}
	if r.pos >= len(r.data) {
		r.atEOF = true
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// checkResponse converts HTTP response errors to SDK errors for non-2xx responses.
// Used by all service methods that call the generated client.
// The body parameter is the raw response body bytes (already read by the generated
// client). If the body contains a JSON object with an "error" key, that message is
// used instead of the generic default.
func checkResponse(resp *http.Response, body []byte) error {
	if resp == nil {
		return nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	requestID := resp.Header.Get(requestIDHeader)
	serverMsg, serverHint := parseErrorBody(body)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &Error{Code: CodeAuth, Message: msgOrDefault(serverMsg, "authentication required"), Hint: serverHint, HTTPStatus: 401, RequestID: requestID}
	case http.StatusForbidden:
		return &Error{Code: CodeForbidden, Message: msgOrDefault(serverMsg, "access denied"), Hint: serverHint, HTTPStatus: 403, RequestID: requestID}
	case http.StatusNotFound:
		return &Error{Code: CodeNotFound, Message: msgOrDefault(serverMsg, "resource not found"), Hint: serverHint, HTTPStatus: 404, RequestID: requestID}
	case http.StatusUnprocessableEntity:
		return &Error{Code: CodeValidation, Message: msgOrDefault(serverMsg, "validation error"), Hint: serverHint, HTTPStatus: 422, RequestID: requestID}
	case http.StatusTooManyRequests:
		return &Error{Code: CodeRateLimit, Message: msgOrDefault(serverMsg, "rate limited - try again later"), Hint: serverHint, HTTPStatus: 429, Retryable: true, RequestID: requestID}
	default:
		retryable := resp.StatusCode >= 500 && resp.StatusCode < 600
		return &Error{Code: CodeAPI, Message: msgOrDefault(serverMsg, fmt.Sprintf("API error: %s", resp.Status)), Hint: serverHint, HTTPStatus: resp.StatusCode, Retryable: retryable, RequestID: requestID}
	}
}

// maxErrorMessageLen caps server error messages to prevent unbounded memory growth.
const maxErrorMessageLen = 500

// parseErrorBody tries to extract "error" and "error_description" from a JSON
// response body. Returns empty strings if the body is not JSON or missing those keys.
func parseErrorBody(body []byte) (message, hint string) {
	if len(body) == 0 {
		return "", ""
	}
	var parsed struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", ""
	}
	message = truncate(parsed.Error, maxErrorMessageLen)
	hint = truncate(parsed.Description, maxErrorMessageLen)
	return message, hint
}

// msgOrDefault returns msg if non-empty, otherwise fallback.
func msgOrDefault(msg, fallback string) string {
	if msg != "" {
		return msg
	}
	return fallback
}

// truncate returns s capped at maxLen runes. If truncated, the result is
// maxLen runes plus an appended "…" (so up to maxLen+1 runes total).
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
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
	// Truncated is true when results were capped by MaxPages or Limit, either
	// because more pages are available on the server or because items were
	// dropped within a page due to the limit.
	Truncated bool
}

// isFirstPageTruncated returns true when items were capped on the first page
// (either the page had more items than limit, or more pages are available).
func isFirstPageTruncated(resp *http.Response, itemCount, limit int) bool {
	if limit <= 0 {
		if resp == nil {
			return false
		}
		return parseNextLink(resp.Header.Get("Link")) != ""
	}
	if itemCount > limit {
		return true
	}
	if resp == nil {
		return false
	}
	return parseNextLink(resp.Header.Get("Link")) != ""
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
