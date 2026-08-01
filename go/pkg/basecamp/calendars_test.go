package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// testCalendarsServer creates an httptest.Server and a CalendarsService wired
// to it. The handler receives all requests; caller is responsible for routing.
func testCalendarsServer(t *testing.T, handler http.HandlerFunc) *CalendarsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Calendars()
}

// A 422 whose errors map mixes a valid entry with a malformed one must reach
// the tolerant SPEC §6 parser (checkResponse), not die in the generated
// client's strict unmarshal into FieldValidationErrorResponseContent. The
// error-status response parsing is made tolerant by
// scripts/normalize-go-error-response-parsing.sh.
func TestCalendarsService_Update_MixedValidityFieldErrorsReachTolerantParser(t *testing.T) {
	svc := testCalendarsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(422)
		w.Write([]byte(`{"errors": {"color": ["is not a valid color"], "base": "invalid"}}`))
	})

	_, err := svc.Update(context.Background(), 123, "chartreuse")
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if e.Code != CodeValidation {
		t.Errorf("Code = %q, want %q", e.Code, CodeValidation)
	}
	if e.Message != "color: is not a valid color" {
		t.Errorf("Message = %q, want the valid entry flattened despite the malformed sibling", e.Message)
	}
	want := map[string][]string{"color": {"is not a valid color"}}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
}

// A fully well-formed field-keyed 422 through the generated client still maps
// to the structured validation error (the tolerant rewrite must not disturb
// the happy error path).
func TestCalendarsService_Update_FieldKeyed422(t *testing.T) {
	svc := testCalendarsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(422)
		w.Write([]byte(`{"errors": {"color": ["is not a valid color"]}}`))
	})

	_, err := svc.Update(context.Background(), 123, "chartreuse")
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if e.Message != "color: is not a valid color" {
		t.Errorf("Message = %q, want flattened field errors", e.Message)
	}
	want := map[string][]string{"color": {"is not a valid color"}}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
}
