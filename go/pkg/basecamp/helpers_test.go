package basecamp

import (
	"io"
	"net/http"
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

	const want = `{"content":"Updated content"}`
	for attempt := 1; attempt <= 2; attempt++ {
		got, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("attempt %d: failed reading body: %v", attempt, err)
		}
		if string(got) != want {
			t.Fatalf("attempt %d: body = %q, want %q", attempt, got, want)
		}
	}
}

func TestDerefInt64(t *testing.T) {
	var v int64 = 42
	if got := derefInt64(&v); got != 42 {
		t.Errorf("derefInt64(&42) = %d, want 42", got)
	}
	if got := derefInt64(nil); got != 0 {
		t.Errorf("derefInt64(nil) = %d, want 0", got)
	}
}
