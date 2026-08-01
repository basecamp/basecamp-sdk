package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// testFieldErrorsAccount wires an httptest.Server to an AccountClient so service
// methods exercise the generated client and checkResponse end to end.
func testFieldErrorsAccount(t *testing.T, handler http.HandlerFunc) *AccountClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	return client.ForAccount("99999")
}

// Webhook creation rejects with the unwrapped ActiveModel::Errors rendering at
// 400 (webhooks_controller.rb:31) — no "errors" wrapper anywhere in the body.
func TestWebhooksService_Create_BareFieldMap400(t *testing.T) {
	svc := testFieldErrorsAccount(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"payload_url":["is not a valid URL"],"types":["is invalid"]}`))
	}).Webhooks()

	_, err := svc.Create(context.Background(), 1, &CreateWebhookRequest{
		PayloadURL: "https://example.com/hook",
		Types:      []string{"Comment"},
	})
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if e.Code != CodeValidation {
		t.Errorf("Code = %q, want %q", e.Code, CodeValidation)
	}
	wantMsg := "payload_url: is not a valid URL, types: is invalid"
	if e.Message != wantMsg {
		t.Errorf("Message = %q, want %q", e.Message, wantMsg)
	}
	want := map[string][]string{
		"payload_url": {"is not a valid URL"},
		"types":       {"is invalid"},
	}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
}

// UpdateMyNote now declares FieldValidationError, matching what
// my/notes_controller.rb:19 actually renders.
func TestMyNotesService_Update_FieldKeyed422(t *testing.T) {
	svc := testFieldErrorsAccount(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":{"content":["can't be blank"]}}`))
	}).MyNotes()

	_, err := svc.Update(context.Background(), "")
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if e.Code != CodeValidation {
		t.Errorf("Code = %q, want %q", e.Code, CodeValidation)
	}
	if e.Message != "content: can't be blank" {
		t.Errorf("Message = %q, want flattened field errors", e.Message)
	}
	want := map[string][]string{"content": {"can't be blank"}}
	if !reflect.DeepEqual(e.FieldErrors, want) {
		t.Errorf("FieldErrors = %v, want %v", e.FieldErrors, want)
	}
}
