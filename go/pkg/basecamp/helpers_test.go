package basecamp

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestCheckResponse_NilResponse(t *testing.T) {
	if err := checkResponse(nil, nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckResponse_SuccessStatuses(t *testing.T) {
	for _, code := range []int{200, 201, 204, 299} {
		resp := &http.Response{StatusCode: code}
		if err := checkResponse(resp, nil); err != nil {
			t.Errorf("status %d: expected nil, got %v", code, err)
		}
	}
}

func TestCheckResponse_ErrorCodes(t *testing.T) {
	tests := []struct {
		status    int
		wantCode  string
		wantRetry bool
	}{
		{400, CodeValidation, false},
		{401, CodeAuth, false},
		{403, CodeForbidden, false},
		{404, CodeNotFound, false},
		{422, CodeValidation, false},
		{429, CodeRateLimit, true},
		{500, CodeAPI, true},
		{502, CodeAPI, true},
	}

	for _, tt := range tests {
		resp := &http.Response{StatusCode: tt.status, Status: http.StatusText(tt.status)}
		err := checkResponse(resp, nil)
		if err == nil {
			t.Fatalf("status %d: expected error, got nil", tt.status)
		}
		e, ok := err.(*Error)
		if !ok {
			t.Fatalf("status %d: expected *Error, got %T", tt.status, err)
		}
		if e.Code != tt.wantCode {
			t.Errorf("status %d: Code = %q, want %q", tt.status, e.Code, tt.wantCode)
		}
		if e.HTTPStatus != tt.status {
			t.Errorf("status %d: HTTPStatus = %d, want %d", tt.status, e.HTTPStatus, tt.status)
		}
		if e.Retryable != tt.wantRetry {
			t.Errorf("status %d: Retryable = %v, want %v", tt.status, e.Retryable, tt.wantRetry)
		}
	}
}

func TestCheckResponse_JSONErrorBody(t *testing.T) {
	resp := &http.Response{StatusCode: 403, Header: http.Header{}}
	body := []byte(`{"error":"No todolists are tracked on the hill chart"}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Message != "No todolists are tracked on the hill chart" {
		t.Errorf("Message = %q, want server error message", e.Message)
	}
	if e.Code != CodeForbidden {
		t.Errorf("Code = %q, want %q", e.Code, CodeForbidden)
	}
}

func TestCheckResponse_JSONErrorWithDescription(t *testing.T) {
	resp := &http.Response{StatusCode: 403, Header: http.Header{}}
	body := []byte(`{"error":"access denied","error_description":"You do not have access to this resource"}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Message != "access denied" {
		t.Errorf("Message = %q, want %q", e.Message, "access denied")
	}
	if e.Hint != "You do not have access to this resource" {
		t.Errorf("Hint = %q, want error_description value", e.Hint)
	}
}

func TestCheckResponse_RequestID(t *testing.T) {
	resp := &http.Response{StatusCode: 500, Header: http.Header{}}
	resp.Header.Set("X-Request-Id", "req-sdk-123")

	err := checkResponse(resp, nil)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.RequestID != "req-sdk-123" {
		t.Fatalf("RequestID = %q, want %q", e.RequestID, "req-sdk-123")
	}
}

func TestCheckResponse_EmptyBody(t *testing.T) {
	resp := &http.Response{StatusCode: 403, Header: http.Header{}}
	err := checkResponse(resp, nil)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Message != "access denied" {
		t.Errorf("Message = %q, want default fallback", e.Message)
	}
}

func TestCheckResponse_InvalidJSON(t *testing.T) {
	resp := &http.Response{StatusCode: 403, Header: http.Header{}}
	body := []byte(`not json`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Message != "access denied" {
		t.Errorf("Message = %q, want default fallback for invalid JSON", e.Message)
	}
}

// SPEC §6 step 5: an empty or unparsable body on the default arm falls back
// to the fixed code-bearing phrase — never resp.Status, the wire reason
// phrase, which does not exist under HTTP/2 and is blank for an unregistered
// code like 599.
func TestCheckResponse_DefaultArmFixedPhrase(t *testing.T) {
	tests := []struct {
		name   string
		status int
		wire   string
		body   []byte
		want   string
	}{
		{"empty body", 418, "418 I'm a teapot", nil, "Request failed (HTTP 418)"},
		{"unparsable body", 418, "418 I'm a teapot", []byte("not json"), "Request failed (HTTP 418)"},
		{"unregistered status, blank reason phrase", 599, "", nil, "Request failed (HTTP 599)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tt.status, Status: tt.wire, Header: http.Header{}}
			err := checkResponse(resp, tt.body)
			e, ok := err.(*Error)
			if !ok {
				t.Fatalf("expected *Error, got %T", err)
			}
			if e.Message != tt.want {
				t.Errorf("Message = %q, want %q", e.Message, tt.want)
			}
			if strings.Contains(e.Message, "teapot") {
				t.Errorf("Message renders the wire reason phrase: %q", e.Message)
			}
		})
	}
}

func TestCheckResponse_FieldKeyed422(t *testing.T) {
	resp := &http.Response{StatusCode: 422, Header: http.Header{}}
	body := []byte(`{"errors":{"color":["is not a valid color"]}}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Message != "color: is not a valid color" {
		t.Errorf("Message = %q, want flattened field errors standing alone", e.Message)
	}
	want := map[string][]string{"color": {"is not a valid color"}}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
}

func TestCheckResponse_FieldKeyed422SortedMultiField(t *testing.T) {
	resp := &http.Response{StatusCode: 422, Header: http.Header{}}
	body := []byte(`{"errors":{"name":["can't be blank","is too short"],"color":["is not a valid color"]}}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	wantMsg := "color: is not a valid color, name: can't be blank; is too short"
	if e.Message != wantMsg {
		t.Errorf("Message = %q, want %q", e.Message, wantMsg)
	}
	want := map[string][]string{
		"color": {"is not a valid color"},
		"name":  {"can't be blank", "is too short"},
	}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
}

func TestCheckResponse_FieldKeyed422AppendsToTopLevelError(t *testing.T) {
	resp := &http.Response{StatusCode: 422, Header: http.Header{}}
	body := []byte(`{"error":"Validation failed","errors":{"color":["is not a valid color"]}}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	wantMsg := "Validation failed (color: is not a valid color)"
	if e.Message != wantMsg {
		t.Errorf("Message = %q, want %q", e.Message, wantMsg)
	}
}

func TestCheckResponse_FieldKeyed400(t *testing.T) {
	resp := &http.Response{StatusCode: 400, Header: http.Header{}}
	body := []byte(`{"errors":{"color":["is not a valid color"]}}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Message != "color: is not a valid color" {
		t.Errorf("Message = %q, want flattened field errors", e.Message)
	}
	if e.FieldErrors == nil {
		t.Errorf("FieldErrors = nil, want populated map on 400")
	}
}

func TestCheckResponse_FieldKeyedNotExtractedOutsideValidation(t *testing.T) {
	resp := &http.Response{StatusCode: 403, Header: http.Header{}}
	body := []byte(`{"errors":{"color":["is not a valid color"]}}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.FieldErrors != nil {
		t.Errorf("FieldErrors = %v, want nil outside 400/422", e.FieldErrors)
	}
	if e.Message != "access denied" {
		t.Errorf("Message = %q, want default fallback", e.Message)
	}
}

func TestCheckResponse_FieldKeyed422SkipsMalformedEntries(t *testing.T) {
	resp := &http.Response{StatusCode: 422, Header: http.Header{}}
	body := []byte(`{"errors":{"color":"not an array","name":["can't be blank"],"empty":[],"mixed":[42,"is invalid"]}}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	want := map[string][]string{
		"mixed": {"is invalid"},
		"name":  {"can't be blank"},
	}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
	wantMsg := "mixed: is invalid, name: can't be blank"
	if e.Message != wantMsg {
		t.Errorf("Message = %q, want %q", e.Message, wantMsg)
	}
}

func TestCheckResponse_FieldKeyed422MalformedShapeFallsBack(t *testing.T) {
	resp := &http.Response{StatusCode: 422, Header: http.Header{}}
	for _, body := range []string{
		`{"errors":{"color":"not an array"}}`,
		`{"errors":[]}`,
		`{"errors":"nope"}`,
		`{"errors":{}}`,
	} {
		err := checkResponse(resp, []byte(body))
		e, ok := err.(*Error)
		if !ok {
			t.Fatalf("body %s: expected *Error, got %T", body, err)
		}
		if e.FieldErrors != nil {
			t.Errorf("body %s: FieldErrors = %v, want nil", body, e.FieldErrors)
		}
		if e.Message != "validation error" {
			t.Errorf("body %s: Message = %q, want default fallback", body, e.Message)
		}
	}
}

func TestCheckResponse_FieldKeyed422TruncatesAfterFlattening(t *testing.T) {
	long := strings.Repeat("x", 600)
	resp := &http.Response{StatusCode: 422, Header: http.Header{}}
	body := []byte(`{"errors":{"color":["` + long + `"]}}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	// The composed message is capped after flattening: at most 500 bytes with
	// the last 3 replaced by "..." (SPEC §9).
	if got := len(e.Message); got != maxErrorMessageLen {
		t.Errorf("len(Message) = %d bytes, want %d", got, maxErrorMessageLen)
	}
	if !strings.HasPrefix(e.Message, "color: xxx") {
		t.Errorf("Message = %q, want flattened prefix", e.Message[:20])
	}
	if !strings.HasSuffix(e.Message, "...") {
		t.Errorf("Message = %q, want %q suffix", e.Message, "...")
	}
	// The structured slot keeps the raw server-sent messages.
	if got := e.FieldErrors["color"][0]; got != long {
		t.Errorf("FieldErrors[color][0] length = %d, want raw %d-char message", len(got), len(long))
	}
}

func TestCheckResponse_FieldKeyed422SurvivesNonStringErrorSibling(t *testing.T) {
	resp := &http.Response{StatusCode: 422, Header: http.Header{}}
	body := []byte(`{"error":{},"error_description":42,"errors":{"color":["is not a valid color"]}}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Message != "color: is not a valid color" {
		t.Errorf("Message = %q, want flattened field errors despite malformed siblings", e.Message)
	}
	want := map[string][]string{"color": {"is not a valid color"}}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
	if e.Hint != "" {
		t.Errorf("Hint = %q, want empty for non-string error_description", e.Hint)
	}
}

func TestCheckResponse_FieldKeyed422AppendsAfterMessageFallback(t *testing.T) {
	resp := &http.Response{StatusCode: 422, Header: http.Header{}}
	body := []byte(`{"message":"Validation failed","errors":{"color":["is not a valid color"]}}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Message != "Validation failed (color: is not a valid color)" {
		t.Errorf("Message = %q, want the message-key fallback composed with the flattened field errors", e.Message)
	}
	want := map[string][]string{"color": {"is not a valid color"}}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
}

func TestCheckResponse_ErrorKeyWinsOverMessageKey(t *testing.T) {
	resp := &http.Response{StatusCode: 404, Header: http.Header{}}
	body := []byte(`{"error":"from error","message":"from message"}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Message != "from error" {
		t.Errorf("Message = %q, want the error key to win over the message key", e.Message)
	}
}

func TestCheckResponse_Plain422UnchangedByFieldErrorSupport(t *testing.T) {
	resp := &http.Response{StatusCode: 422, Header: http.Header{}}
	body := []byte(`{"error":"Name can't be blank"}`)
	err := checkResponse(resp, body)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Message != "Name can't be blank" {
		t.Errorf("Message = %q, want plain server error message", e.Message)
	}
	if e.FieldErrors != nil {
		t.Errorf("FieldErrors = %v, want nil for a flat error body", e.FieldErrors)
	}
}

func TestParseTotalCount(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"valid", "42", 42},
		{"zero", "0", 0},
		{"empty", "", 0},
		{"negative", "-1", 0},
		{"non-numeric", "abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}}
			if tt.header != "" {
				resp.Header.Set("X-Total-Count", tt.header)
			}
			got := parseTotalCount(resp)
			if got != tt.want {
				t.Errorf("parseTotalCount(%q) = %d, want %d", tt.header, got, tt.want)
			}
		})
	}
}

func TestParseTotalCount_NilResponse(t *testing.T) {
	if got := parseTotalCount(nil); got != 0 {
		t.Errorf("parseTotalCount(nil) = %d, want 0", got)
	}
}

func TestMarshalBody_ReturnsReplayableReader(t *testing.T) {
	reader, err := marshalBody(map[string]any{"content": "Updated content"})
	if err != nil {
		t.Fatalf("marshalBody returned error: %v", err)
	}

	// The body must be snapshotable by net/http so the generated client's
	// doWithRetry can replay it via GetBody across retries — otherwise these
	// SDK-owned serialized bodies would be sent once and never retried.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, "https://example.com", reader)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if req.GetBody == nil {
		t.Fatal("marshalBody body is not snapshotable: req.GetBody is nil (retries would be disabled)")
	}

	const want = `{"content":"Updated content"}`
	for attempt := 1; attempt <= 2; attempt++ {
		rc, err := req.GetBody()
		if err != nil {
			t.Fatalf("attempt %d: GetBody: %v", attempt, err)
		}
		got, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("attempt %d: failed reading body: %v", attempt, err)
		}
		if string(got) != want {
			t.Fatalf("attempt %d: replayed body = %q, want %q", attempt, got, want)
		}
	}
}

func TestDeref(t *testing.T) {
	var v int64 = 42
	if got := deref(&v); got != 42 {
		t.Errorf("deref(&42) = %d, want 42", got)
	}
	if got := deref[int64](nil); got != 0 {
		t.Errorf("deref[int64](nil) = %d, want 0", got)
	}
	if got := deref[string](nil); got != "" {
		t.Errorf("deref[string](nil) = %q, want \"\"", got)
	}
}

// TestCheckResponse_BareFieldMap covers the unwrapped ActiveModel::Errors
// rendering (SPEC §6 step 2): webhooks, chat integrations, and message-type
// categories emit the field map as the whole body at 400; lineup markers do the
// same at 422.
func TestCheckResponse_BareFieldMap(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantMessage string
		wantFields  map[string][]string
	}{
		{
			name:        "single field at 400",
			status:      400,
			body:        `{"payload_url":["is not a valid URL"]}`,
			wantMessage: "payload_url: is not a valid URL",
			wantFields:  map[string][]string{"payload_url": {"is not a valid URL"}},
		},
		{
			name:        "multiple fields sort and join",
			status:      400,
			body:        `{"types":["is invalid"],"payload_url":["is not a valid URL","is too long"]}`,
			wantMessage: "payload_url: is not a valid URL; is too long, types: is invalid",
			wantFields: map[string][]string{
				"payload_url": {"is not a valid URL", "is too long"},
				"types":       {"is invalid"},
			},
		},
		{
			name:        "lineup markers emit the bare map at 422",
			status:      422,
			body:        `{"name":["can't be blank"]}`,
			wantMessage: "name: can't be blank",
			wantFields:  map[string][]string{"name": {"can't be blank"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tt.status, Header: http.Header{}}
			err := checkResponse(resp, []byte(tt.body))
			e, ok := err.(*Error)
			if !ok {
				t.Fatalf("expected *Error, got %T", err)
			}
			if e.Code != CodeValidation {
				t.Errorf("Code = %q, want %q", e.Code, CodeValidation)
			}
			if e.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", e.Message, tt.wantMessage)
			}
			if !reflect.DeepEqual(e.FieldErrors, tt.wantFields) {
				t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, tt.wantFields)
			}
		})
	}
}

// TestCheckResponse_BareFieldMapStrictGate pins SPEC §6 step 2's deliberate
// asymmetry: an unwrapped body is recognizable by shape alone, so one
// non-conforming member means the object is not a field map at all — unlike the
// wrapped shape, which filters per entry because the "errors" key already
// declares intent.
func TestCheckResponse_BareFieldMapStrictGate(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "member is not an array", body: `{"id":1}`},
		{name: "one member of several is not an array", body: `{"color":["is invalid"],"count":3}`},
		{name: "member array is empty", body: `{"color":[]}`},
		{name: "member array holds an empty string", body: `{"color":["","is invalid"]}`},
		{name: "member array holds a non-string", body: `{"color":["is invalid",42]}`},
		{name: "member array holds null", body: `{"color":[null]}`},
		{name: "empty object", body: `{}`},
		{name: "JSON array body", body: `[1,2]`},
		{name: "JSON string body", body: `"nope"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: 400, Header: http.Header{}}
			err := checkResponse(resp, []byte(tt.body))
			e, ok := err.(*Error)
			if !ok {
				t.Fatalf("expected *Error, got %T", err)
			}
			if e.FieldErrors != nil {
				t.Errorf("FieldErrors = %v, want nil for a non-field-map body", e.FieldErrors)
			}
			if e.Message != "validation error" {
				t.Errorf("Message = %q, want the generic fallback", e.Message)
			}
		})
	}
}

// TestCheckResponse_BareFieldMapStaysFlatForFlatBodies keeps flat bodies flat.
// Only "errors" is excluded by name; a flat body's "error"/"message" is a
// string, and the all-members shape gate rejects a string-valued member — so
// these bodies stay flat on shape, not on the key's name. See
// TestCheckResponse_BareFieldMapAllowsReservedFieldNames for the other half:
// array-valued "error"/"message" members ARE recognized as fields.
func TestCheckResponse_BareFieldMapStaysFlatForFlatBodies(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantMessage string
	}{
		{
			name:        "error key",
			body:        `{"error":"Webhook is invalid","payload_url":["is not a valid URL"]}`,
			wantMessage: "Webhook is invalid",
		},
		{
			name:        "message key",
			body:        `{"message":"Webhook is invalid","payload_url":["is not a valid URL"]}`,
			wantMessage: "Webhook is invalid",
		},
		{
			name:        "empty errors key",
			body:        `{"errors":{},"payload_url":["is not a valid URL"]}`,
			wantMessage: "validation error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: 400, Header: http.Header{}}
			err := checkResponse(resp, []byte(tt.body))
			e, ok := err.(*Error)
			if !ok {
				t.Fatalf("expected *Error, got %T", err)
			}
			if e.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", e.Message, tt.wantMessage)
			}
			if e.FieldErrors != nil {
				t.Errorf("FieldErrors = %v, want nil", e.FieldErrors)
			}
		})
	}
}

// Only "errors" is reserved by name. A record whose validated attribute is
// called "message" or "error" still gets its field map recognized: the flat
// shape carries those keys as strings, which the gate rejects on shape alone.
func TestCheckResponse_BareFieldMapAllowsReservedFieldNames(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantMessage string
		wantFields  map[string][]string
	}{
		{
			name:        "field named message",
			body:        `{"message":["can't be blank"]}`,
			wantMessage: "message: can't be blank",
			wantFields:  map[string][]string{"message": {"can't be blank"}},
		},
		{
			name:        "field named error alongside another",
			body:        `{"error":["is invalid"],"name":["can't be blank"]}`,
			wantMessage: "error: is invalid, name: can't be blank",
			wantFields:  map[string][]string{"error": {"is invalid"}, "name": {"can't be blank"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: 400, Header: http.Header{}}
			err := checkResponse(resp, []byte(tt.body))
			e, ok := err.(*Error)
			if !ok {
				t.Fatalf("expected *Error, got %T", err)
			}
			if e.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", e.Message, tt.wantMessage)
			}
			if !reflect.DeepEqual(e.FieldErrors, tt.wantFields) {
				t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, tt.wantFields)
			}
		})
	}
}

// TestCheckResponse_BareFieldMapNotExtractedOutsideValidation mirrors the
// wrapped-shape rule: the slot is populated for 400/422 only.
func TestCheckResponse_BareFieldMapNotExtractedOutsideValidation(t *testing.T) {
	for _, status := range []int{403, 404, 500} {
		resp := &http.Response{StatusCode: status, Header: http.Header{}}
		err := checkResponse(resp, []byte(`{"payload_url":["is not a valid URL"]}`))
		e, ok := err.(*Error)
		if !ok {
			t.Fatalf("expected *Error, got %T", err)
		}
		if e.FieldErrors != nil {
			t.Errorf("status %d: FieldErrors = %v, want nil outside 400/422", status, e.FieldErrors)
		}
	}
}
