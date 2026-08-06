package basecamp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// marshalBody encodes a map as JSON and returns an io.Reader suitable for the
// generated client's *WithBodyWithResponse methods. It returns a *bytes.Reader
// so net/http snapshots it into req.GetBody, which the generated client's
// doWithRetry uses to replay the body across retry attempts (these SDK-owned
// serialized bodies must stay retryable — see the naturally-idempotent PUT/
// DELETE conformance case).
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
	return bytes.NewReader(b), nil
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
	serverMsg, serverHint, fieldErrors := parseErrorBody(body)

	switch resp.StatusCode {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return validationError(serverMsg, serverHint, fieldErrors, resp.StatusCode, requestID)
	case http.StatusUnauthorized:
		return &Error{Code: CodeAuth, Message: msgOrDefault(serverMsg, "authentication required"), Hint: serverHint, HTTPStatus: 401, RequestID: requestID}
	case http.StatusForbidden:
		return &Error{Code: CodeForbidden, Message: msgOrDefault(serverMsg, "access denied"), Hint: serverHint, HTTPStatus: 403, RequestID: requestID}
	case http.StatusNotFound:
		return &Error{Code: CodeNotFound, Message: msgOrDefault(serverMsg, "resource not found"), Hint: serverHint, HTTPStatus: 404, RequestID: requestID}
	case http.StatusTooManyRequests:
		return &Error{Code: CodeRateLimit, Message: msgOrDefault(serverMsg, "rate limited - try again later"), Hint: serverHint, HTTPStatus: 429, Retryable: true, RequestID: requestID}
	case http.StatusInsufficientStorage:
		// A 5xx status carrying a client fact: the account is out of storage, or
		// at its webhook ceiling. Retrying cannot satisfy it, so this must be
		// decided before the 5xx catch-all below.
		return &Error{Code: CodeLimitExceeded, Message: msgOrDefault(serverMsg, "account limit reached"), Hint: serverHint, HTTPStatus: 507, Retryable: false, RequestID: requestID}
	default:
		retryable := resp.StatusCode >= 500 && resp.StatusCode < 600
		return &Error{Code: CodeAPI, Message: msgOrDefault(serverMsg, fmt.Sprintf("API error: %s", resp.Status)), Hint: serverHint, HTTPStatus: resp.StatusCode, Retryable: retryable, RequestID: requestID}
	}
}

// maxErrorMessageLen caps server error messages to prevent unbounded memory growth.
const maxErrorMessageLen = 500

// parseErrorBody tries to extract "error" (falling back to "message"),
// "error_description", and the field-keyed errors map from a JSON response
// body — either wrapped in an "errors" key or, for the controllers that render
// ActiveModel::Errors with no wrapper, as the body itself. Returns empty values
// if the body is not JSON or missing those keys.
func parseErrorBody(body []byte) (message, hint string, fieldErrors map[string][]string) {
	if len(body) == 0 {
		return "", "", nil
	}
	// Members decode independently (SPEC §6: a key is used only when its
	// value is a string) so a malformed scalar sibling — e.g.
	// {"error": {}, "errors": {...}} — cannot discard a usable field map.
	var parsed struct {
		Error       json.RawMessage `json:"error"`
		Description json.RawMessage `json:"error_description"`
		Message     json.RawMessage `json:"message"`
		Errors      json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", nil
	}
	message = truncate(stringFromRaw(parsed.Error))
	if message == "" {
		// SPEC §6 step 4: "message" is the fallback for APIs that use it
		// instead of "error".
		message = truncate(stringFromRaw(parsed.Message))
	}
	hint = truncate(stringFromRaw(parsed.Description))

	fieldErrors = parseFieldErrors(parsed.Errors)
	if fieldErrors == nil && len(parsed.Errors) == 0 {
		// SPEC §6 step 2: no "errors" wrapper — the body may be the field map
		// itself. A flat {"error": "..."} body needs no exclusion here: its
		// member is a string, which the all-members gate rejects on shape.
		fieldErrors = parseBareFieldErrors(body)
	}
	return message, hint, fieldErrors
}

// stringFromRaw decodes a JSON value as a string, returning "" for absent or
// non-string values.
func stringFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// parseFieldErrors decodes the field-keyed validation errors map — the Rails
// RecordInvalid rendering {"errors": {"field": ["msg", ...]}}. Entries whose
// value is not an array are skipped, non-string elements are dropped, and a map
// with no usable entries is treated as absent (nil).
//
// Callers reach the unwrapped rendering through parseErrorBody, which falls
// through to parseBareFieldErrors when there is no "errors" key.
func parseFieldErrors(raw json.RawMessage) map[string][]string {
	if len(raw) == 0 {
		return nil
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	fieldErrors := make(map[string][]string, len(entries))
	for field, value := range entries {
		var values []any
		if err := json.Unmarshal(value, &values); err != nil {
			continue
		}
		messages := make([]string, 0, len(values))
		for _, v := range values {
			if s, ok := v.(string); ok {
				messages = append(messages, s)
			}
		}
		if len(messages) > 0 {
			fieldErrors[field] = messages
		}
	}
	if len(fieldErrors) == 0 {
		return nil
	}
	return fieldErrors
}

// parseBareFieldErrors decodes an unwrapped field map — the
// `render json: @webhook.errors` rendering, where the whole body is
// {"field": ["msg", ...]}. The gate is all-or-nothing by design (SPEC §6 step
// 2): with no "errors" key to declare intent, only shape distinguishes a field
// map from any other JSON object, so a single non-conforming member means this
// is not one. Returns nil unless every member is a non-empty array of non-empty
// strings.
//
// Only "errors" is structurally reserved (it belongs to step 1). "error" and
// "message" are not excluded by name: a flat body carries them as strings,
// which the shape gate already rejects, so a validated field genuinely named
// "message" is still recognized.
func parseBareFieldErrors(body []byte) map[string][]string {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(body, &members); err != nil || len(members) == 0 {
		return nil
	}
	fieldErrors := make(map[string][]string, len(members))
	for field, raw := range members {
		var elems []any
		// A JSON null decodes into a slice without error, leaving it nil — the
		// length check below is what rejects it.
		if err := json.Unmarshal(raw, &elems); err != nil || len(elems) == 0 {
			return nil
		}
		messages := make([]string, 0, len(elems))
		for _, elem := range elems {
			s, ok := elem.(string)
			if !ok || s == "" {
				return nil
			}
			messages = append(messages, s)
		}
		fieldErrors[field] = messages
	}
	return fieldErrors
}

// flattenFieldErrors renders a field-keyed errors map as
// "field: msg1; msg2, other: msg" — fields sorted lexicographically, a field's
// messages joined with "; ", fields joined with ", ". This shape is shared by
// all six SDKs; change it everywhere or nowhere.
func flattenFieldErrors(fieldErrors map[string][]string) string {
	if len(fieldErrors) == 0 {
		return ""
	}
	fields := make([]string, 0, len(fieldErrors))
	for field := range fieldErrors {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, field+": "+strings.Join(fieldErrors[field], "; "))
	}
	return strings.Join(parts, ", ")
}

// composeValidationMessage merges the top-level server message with the
// flattened field-keyed errors: appended in parentheses when both are present,
// standing alone when only the field errors are. The composed result is
// truncated after flattening so the appended tail is capped too.
func composeValidationMessage(serverMsg string, fieldErrors map[string][]string) string {
	flat := flattenFieldErrors(fieldErrors)
	switch {
	case flat == "":
		return serverMsg
	case serverMsg == "":
		return truncate(flat)
	default:
		return truncate(serverMsg + " (" + flat + ")")
	}
}

// validationError builds the 400/422 error: the composed message, the raw
// field map, and the hint. Both the generated service layer (checkResponse) and
// the raw Client escape hatch (doRequest) construct it here, so the composition
// rules cannot drift between the two paths.
func validationError(serverMsg, serverHint string, fieldErrors map[string][]string, status int, requestID string) *Error {
	return &Error{
		Code:        CodeValidation,
		Message:     msgOrDefault(composeValidationMessage(serverMsg, fieldErrors), "validation error"),
		Hint:        serverHint,
		FieldErrors: fieldErrors,
		HTTPStatus:  status,
		RequestID:   requestID,
	}
}

// msgOrDefault returns msg if non-empty, otherwise fallback.
func msgOrDefault(msg, fallback string) string {
	if msg != "" {
		return msg
	}
	return fallback
}

// truncate caps s at maxErrorMessageLen bytes. If s exceeds the limit, the
// last 3 bytes are replaced with "..." so the result is at most
// maxErrorMessageLen long (SPEC §9; byte-level truncation may split a
// multi-byte codepoint, which §9 documents as accepted behavior).
func truncate(s string) string {
	if len(s) <= maxErrorMessageLen {
		return s
	}
	return s[:maxErrorMessageLen-3] + "..."
}

// Pointer helpers for the generated-optional-pointer contract (SPEC.md §10):
// generated optional fields are pointers (absence = nil); the hand-written SDK
// surface keeps ergonomic value types where absence collapsing to the zero
// value is acceptable for reads.

// deref safely dereferences an optional-field pointer, returning the zero
// value when the field was absent.
//
// The internal spelling of the exported [Deref], kept as the vocabulary the
// hundreds of existing conversion sites already read in. Forwarding rather than
// reimplementing keeps one definition, so the contract callers get cannot drift
// from the contract this package relies on.
func deref[T any](p *T) T {
	return Deref(p)
}

// omitzero converts a value-typed wrapper option to a generated request's
// optional pointer, preserving omit-when-zero wire behavior: the zero value
// means "not provided" and maps to nil (field omitted on the wire).
func omitzero[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

// ptr returns a pointer to v, for optional fields where the value — zero
// included — must be sent.
//
// The internal spelling of the exported [Ptr]; see deref for why it forwards
// rather than reimplements.
func ptr[T any](v T) *T {
	return Ptr(v)
}

// intPtrFrom converts an optional generated int32 pointer to the SDK's *int,
// preserving nil. Widening through deref would manufacture a pointer to zero
// and destroy the absence the pointer exists to carry.
//
// Constrained to ~int32 deliberately: int is 32-bit on some Go targets, so
// admitting ~int64 here would silently truncate. Every generated field this
// converts is an int32; a future int64 one needs its own range-checked
// conversion rather than a wider constraint.
func intPtrFrom[T ~int32](p *T) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
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

// pageParam narrows a wrapper's Page option to the *int32 the generated params
// carry. A page number too large to survive the narrowing is a usage error
// rather than a silent wraparound into a negative page on the wire.
//
// The result is a pointer because generated optional query params are pointers
// (#560): nil is absence, and callers only reach here once Page is positive, so
// a successful call always yields a non-nil page.
func pageParam(page int) (*int32, error) {
	if page > math.MaxInt32 {
		return nil, ErrUsage("page is out of range")
	}
	return ptr(int32(page)), nil // #nosec G115 -- bounded above by the MaxInt32 guard
}

// hasNextPage reports whether a response advertises a further page.
//
// The page-selected path uses it: a positive Page returns exactly the page the
// caller asked for, and the rel="next" Link this SDK deliberately does not
// follow is what makes that result truncated under SPEC §8's ListMeta ("true
// only when items beyond those returned were available").
func hasNextPage(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	return parseNextLink(resp.Header.Get("Link")) != ""
}

// pageCap reports how many items a page-selected result keeps and whether it
// is truncated.
//
// Page selection returns exactly the page the caller asked for, so the
// per-operation DEFAULT limits (DefaultTodoLimit and friends) must not apply
// here — only a Limit the caller set explicitly, which is the same composition
// max_items has with a pinned page in the other five SDKs (SPEC §8). Truncated
// is true when that cap dropped items, or when the page advertised a further
// page this SDK deliberately did not follow.
func pageCap(count, limit int, resp *http.Response) (keep int, truncated bool) {
	if limit > 0 && count > limit {
		return limit, true
	}
	return count, hasNextPage(resp)
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
